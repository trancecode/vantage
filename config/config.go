package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration wraps time.Duration so it can be expressed in TOML as a string like
// "30s" or "5m".
type Duration struct {
	time.Duration
}

// UnmarshalText parses a Go duration string (e.g. "30s").
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// MarshalText renders the duration as a Go duration string.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Loader assembles layered configuration into one or more registered target
// structs. Layers apply lowest-precedence first: default documents (in the
// order added), then a local file, then key=value overrides. Each layer is a
// partial merge — only the keys it specifies change.
type Loader struct {
	targets  []target
	defaults [][]byte
	sections map[string]int // toml section name -> index into targets
}

type target struct {
	name string
	ptr  any
}

// New returns an empty Loader.
func New() *Loader {
	return &Loader{sections: map[string]int{}}
}

// RegisterTarget registers a struct pointer to receive configuration. Each
// top-level field's toml tag becomes a routable section; a section may be owned
// by only one target.
func (l *Loader) RegisterTarget(name string, ptr any) {
	idx := len(l.targets)
	l.targets = append(l.targets, target{name: name, ptr: ptr})
	t := reflect.TypeOf(ptr).Elem()
	for i := 0; i < t.NumField(); i++ {
		section := sectionName(t.Field(i))
		if section == "" {
			continue
		}
		if _, dup := l.sections[section]; dup {
			panic(fmt.Sprintf("config section %q registered by more than one target", section))
		}
		l.sections[section] = idx
	}
}

// AddDefaults appends a default TOML document. Defaults apply in the order added.
func (l *Loader) AddDefaults(doc []byte) {
	l.defaults = append(l.defaults, doc)
}

// AddDefaultsFile reads a TOML file and appends it as a default layer.
func (l *Loader) AddDefaultsFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading default config %q: %w", path, err)
	}
	l.AddDefaults(data)
	return nil
}

// Load applies the configured layers into the registered targets. localPath may
// be empty (skipped) or name a file that need not exist (skipped when absent).
func (l *Loader) Load(localPath string, overrides []string) error {
	for i, doc := range l.defaults {
		if err := l.decodeAll(doc); err != nil {
			return fmt.Errorf("decoding default layer %d: %w", i, err)
		}
	}
	if localPath != "" {
		data, err := os.ReadFile(localPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("reading config file %q: %w", localPath, err)
			}
		} else if err := l.decodeAll(data); err != nil {
			return fmt.Errorf("decoding config file %q: %w", localPath, err)
		}
	}
	for _, o := range overrides {
		if err := l.applyOverride(o); err != nil {
			return err
		}
	}
	return nil
}

// decodeAll decodes doc into every target; a target ignores sections it does
// not define.
func (l *Loader) decodeAll(doc []byte) error {
	for _, t := range l.targets {
		if _, err := toml.Decode(string(doc), t.ptr); err != nil {
			return fmt.Errorf("target %s: %w", t.name, err)
		}
	}
	return nil
}

func (l *Loader) applyOverride(override string) error {
	eq := strings.IndexByte(override, '=')
	if eq < 0 {
		return fmt.Errorf("config override %q: missing '=' separator (expected section.key=value)", override)
	}
	path, value := override[:eq], override[eq+1:]
	dot := strings.IndexByte(path, '.')
	if dot <= 0 || dot == len(path)-1 {
		return fmt.Errorf("config override %q: key must be section.key", override)
	}
	section, key := path[:dot], path[dot+1:]
	idx, ok := l.sections[section]
	if !ok {
		return fmt.Errorf("config override %q: unknown section %q", override, section)
	}
	ptr := l.targets[idx].ptr
	fragment := fmt.Sprintf("[%s]\n%s = %s\n", section, key, value)
	if _, err := toml.Decode(fragment, ptr); err != nil {
		// Retry treating the value as a string so bare values like 10s work.
		quoted := fmt.Sprintf("[%s]\n%s = %q\n", section, key, value)
		if _, err2 := toml.Decode(quoted, ptr); err2 != nil {
			return fmt.Errorf("config override %q: %w", override, err2)
		}
	}
	return nil
}

// sectionName returns the toml section name for a struct field (its toml tag
// without options), or "" if the field has no usable toml tag.
func sectionName(f reflect.StructField) string {
	tag := f.Tag.Get("toml")
	if i := strings.IndexByte(tag, ','); i >= 0 {
		tag = tag[:i]
	}
	if tag == "" || tag == "-" {
		return ""
	}
	return tag
}
