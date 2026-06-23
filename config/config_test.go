package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type netSettings struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}
type uiSettings struct {
	Theme string  `toml:"theme"`
	Scale float64 `toml:"scale"`
}
type root struct {
	Net netSettings `toml:"net"`
	UI  uiSettings  `toml:"ui"`
}

func newRootLoader(r *root) *Loader {
	l := New()
	l.RegisterTarget("root", r)
	l.AddDefaults([]byte("[net]\nport = 80\nhost = \"localhost\"\n[ui]\ntheme = \"dark\"\nscale = 1.0\n"))
	return l
}

func TestDefaultsLoad(t *testing.T) {
	r := &root{}
	if err := newRootLoader(r).Load("", nil); err != nil {
		t.Fatal(err)
	}
	if r.Net.Port != 80 || r.Net.Host != "localhost" || r.UI.Theme != "dark" || r.UI.Scale != 1.0 {
		t.Fatalf("defaults not loaded: %+v", r)
	}
}

func TestLaterDefaultPartiallyOverrides(t *testing.T) {
	r := &root{}
	l := newRootLoader(r)
	l.AddDefaults([]byte("[net]\nport = 8080\n")) // only port; host retained
	if err := l.Load("", nil); err != nil {
		t.Fatal(err)
	}
	if r.Net.Port != 8080 {
		t.Fatalf("later default did not override port: %d", r.Net.Port)
	}
	if r.Net.Host != "localhost" {
		t.Fatalf("partial merge clobbered host: %q", r.Net.Host)
	}
}

func TestLocalFileOverridesDefaults(t *testing.T) {
	r := &root{}
	l := newRootLoader(r)
	path := filepath.Join(t.TempDir(), "settings.toml")
	if err := os.WriteFile(path, []byte("[ui]\ntheme = \"light\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := l.Load(path, nil); err != nil {
		t.Fatal(err)
	}
	if r.UI.Theme != "light" {
		t.Fatalf("local file did not override theme: %q", r.UI.Theme)
	}
}

func TestMissingLocalFileSkipped(t *testing.T) {
	r := &root{}
	if err := newRootLoader(r).Load(filepath.Join(t.TempDir(), "absent.toml"), nil); err != nil {
		t.Fatalf("missing local file should be skipped, got %v", err)
	}
	if r.Net.Port != 80 {
		t.Fatalf("defaults lost: %+v", r)
	}
}

func TestOverridesTyped(t *testing.T) {
	r := &root{}
	l := newRootLoader(r)
	err := l.Load("", []string{"net.port=9090", "ui.scale=2.5", "net.host=example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Net.Port != 9090 || r.UI.Scale != 2.5 || r.Net.Host != "example.com" {
		t.Fatalf("overrides not applied: %+v", r)
	}
}

func TestOverrideUnknownSection(t *testing.T) {
	r := &root{}
	err := newRootLoader(r).Load("", []string{"bogus.key=1"})
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
}

func TestOverrideBadFormat(t *testing.T) {
	r := &root{}
	if err := newRootLoader(r).Load("", []string{"noequalssign"}); err == nil {
		t.Fatal("expected error for missing '='")
	}
	if err := newRootLoader(r).Load("", []string{"nodot=1"}); err == nil {
		t.Fatal("expected error for missing section.key dot")
	}
}

func TestMultiTargetRouting(t *testing.T) {
	type other struct {
		AI struct {
			Level int `toml:"level"`
		} `toml:"ai"`
	}
	r := &root{}
	o := &other{}
	l := New()
	l.RegisterTarget("root", r)
	l.RegisterTarget("other", o)
	l.AddDefaults([]byte("[net]\nport = 1\n[ai]\nlevel = 2\n"))
	if err := l.Load("", []string{"ai.level=7", "net.port=8"}); err != nil {
		t.Fatal(err)
	}
	if o.AI.Level != 7 {
		t.Fatalf("ai.level routed wrong: %d", o.AI.Level)
	}
	if r.Net.Port != 8 {
		t.Fatalf("net.port routed wrong: %d", r.Net.Port)
	}
}

func TestDuplicateSectionPanics(t *testing.T) {
	type a struct {
		Net netSettings `toml:"net"`
	}
	type b struct {
		Net netSettings `toml:"net"`
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate section")
		}
	}()
	l := New()
	l.RegisterTarget("a", &a{})
	l.RegisterTarget("b", &b{})
}

func TestDurationOverride(t *testing.T) {
	type cfg struct {
		Timing struct {
			Interval Duration `toml:"interval"`
		} `toml:"timing"`
	}
	c := &cfg{}
	l := New()
	l.RegisterTarget("cfg", c)
	if err := l.Load("", []string{"timing.interval=90s"}); err != nil {
		t.Fatal(err)
	}
	if c.Timing.Interval.Duration != 90*time.Second {
		t.Fatalf("interval = %v, want 90s", c.Timing.Interval.Duration)
	}
}
