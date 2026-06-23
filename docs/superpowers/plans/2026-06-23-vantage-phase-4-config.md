# Vantage extraction — Phase 4: configuration service

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the engine a layered configuration service it owns: a generic, dependency-free `config` loader (embedded defaults → game-registered defaults → local file → `--config_override` key=value), an engine `Settings` schema with an embedded `settings.toml` and engine-declared flags, the removal of `render`'s package-level flag, and a settings-driven `App`.

**Architecture:** A new dependency-free `config` package provides the generic machinery (register target structs, layer defaults, decode a local file, and apply `section.key=value` overrides by routing to the owning target via TOML-tagged sections). The engine's own settings live in `app.Settings` (TOML-sectioned, embedded defaults, flag bindings, and an `Apply` that sets the engine's global toggles). `App` is built from `*Settings` instead of the Phase-3 `Config` struct.

**Tech Stack:** Go 1.26.4, `github.com/BurntSushi/toml`, `github.com/spf13/pflag`, the vantage packages from Phases 1–3.

## Global Constraints

* Module `github.com/trancecode/vantage`; Go `1.26.4` (canonical; do not change).
* Set `GOMODCACHE=/tmp/go-mod-cache` before any Go command. Ebiten-dependent tests run under `xvfb-run -a`; the `config` package has no Ebiten dependency and tests run with plain `go test`.
* `gofmt -l` clean on all new/changed files.
* Override format is `section.key=value`. The loader uses the TOML-fragment decode approach (build `[section]\nkey = value` and decode it) with a quoted-string fallback, so durations and other non-bare literals work unquoted (e.g. `screenshot.delay=10s`).
* Precedence, lowest to highest: embedded engine defaults → game-registered default documents (in the order added) → local file → `--config_override` entries → explicit command-line flags. (Flags are bound with the loaded values as their defaults and parsed last, so an explicitly-set flag wins; this is documented behavior.)
* Commit author: name `Claude Code`, email `herve.quiroz+claude@gmail.com`. No `Co-Authored-By`.
* Work directly on `main`; commit and push per task.
* Carry-forward: do NOT modify `util`/`geometry` (keep byte-identical to nrg).

## Scope notes

* **In scope:** the `config` package; `app.Settings` (schema, embedded `settings.toml`, `RegisterFlags`, `Apply`); removing `render`'s `use_placeholder_sprite_images` package flag; reworking `App` to take `*Settings`.
* **Wired now:** `util.DebugMode` and `render.UsePlaceholderSpriteImages` (via `Settings.Apply`); window, screenshot, and run-for (via `App`).
* **Deferred to Phase 6 (game assembly):** consuming `[camera]` (the game sets its `CameraController.MoveSpeed/ZoomSpeed`), `[log].level` (needs a `util` log-level setter), and `[debug].http_*` (starting the debug HTTP server). These are defined in the schema and exposed as flags now, but their consumers live in the game's `main`.

## File structure (Phase 4)

* `config/config.go` — generic `Loader`, `Duration`. `config/doc.go`. `config/config_test.go`.
* `render/render_sprite.go` — package-level `flag.Bool` replaced with exported `var UsePlaceholderSpriteImages bool`.
* `app/settings.go` — `Settings` struct + sub-structs, `LoadSettings`, `RegisterFlags`, `Apply`. `app/settings.toml` (embedded). `app/settings_test.go`.
* `app/app.go` — `App` built from `*Settings` (replaces the `Config` struct); `app/app_screenshot.go` capturer constructed from primitives. `app/app_test.go`, `app/app_screenshot_test.go` updated.

---

## Task 1: Generic `config` package

**Files:**
- Create: `config/config.go`, `config/doc.go`
- Test: `config/config_test.go`

**Interfaces:**
- Produces: `github.com/trancecode/vantage/config` exporting `Duration` (TOML-friendly `time.Duration`), `Loader` with `New() *Loader`, `(*Loader).RegisterTarget(name string, ptr any)`, `(*Loader).AddDefaults(doc []byte)`, `(*Loader).AddDefaultsFile(path string) error`, `(*Loader).Load(localPath string, overrides []string) error`.

- [ ] **Step 1: Write `config/config.go`**

```go
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
```

- [ ] **Step 2: Write `config/doc.go`**

```go
// Package config provides a layered configuration loader. A Loader merges, in
// increasing precedence, embedded default documents, game-registered default
// documents, a local TOML file, and section.key=value overrides into one or
// more registered target structs. It is generic and game-agnostic: the engine
// registers its settings and a game may register its own.
package config
```

- [ ] **Step 3: Write `config/config_test.go`**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
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
```

- [ ] **Step 4: Resolve deps, vet, format, test (no display needed)**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./config/... && gofmt -l config/ && go test ./config/...
```
Expected: vet clean; `gofmt -l` silent; all `config` tests PASS.

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add config/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add generic layered config loader

A dependency-free config.Loader merges embedded defaults, game-registered
default documents, a local TOML file, and section.key=value overrides into
registered target structs, routing each override to the target that owns its
section. Includes a TOML-friendly Duration type."
git push origin main
```

---

## Task 2: Remove `render`'s package-level flag

Replace the package-level `flag.Bool` in `render_sprite.go` with a plain exported variable, so importing `render` no longer registers a global flag. The engine config sets it (Task 3).

**Files:**
- Modify: `render/render_sprite.go`

**Interfaces:**
- Produces: exported `render.UsePlaceholderSpriteImages bool` (default `false`); the `flag`/pflag import is removed from `render_sprite.go`.

- [ ] **Step 1: Replace the flag with an exported variable**

In `render/render_sprite.go`:
1. Delete the import line `flag "github.com/spf13/pflag"`.
2. Replace the declaration
```go
var usePlaceholderSpriteImages = flag.Bool("use_placeholder_sprite_images", false, "Use a placeholder image when there's no image for a given animation type.")
```
with
```go
// UsePlaceholderSpriteImages, when true, makes Sprite.Image return a placeholder
// for a missing animation type instead of panicking. Set by engine configuration.
var UsePlaceholderSpriteImages bool
```
3. At the use site (the `Image` method), change `if !(*usePlaceholderSpriteImages) {` to `if !UsePlaceholderSpriteImages {`.

- [ ] **Step 2: Build, vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && grep -n 'usePlaceholderSpriteImages\|spf13/pflag' render/*.go && echo "LEFTOVER" || echo "clean"
go vet ./render/... && gofmt -l render/ && xvfb-run -a go test ./render/...
```
Expected: `clean`; vet clean; `gofmt -l` silent; render tests PASS.

- [ ] **Step 3: Commit**

```bash
cd ~/src/vantage
git add render/render_sprite.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Replace render placeholder flag with an exported variable

Removes the package-level use_placeholder_sprite_images flag so importing
render no longer registers a global command-line flag. The value is now the
exported render.UsePlaceholderSpriteImages, set by engine configuration."
git push origin main
```

---

## Task 3: Engine `Settings` (schema, defaults, flags, apply)

**Files:**
- Create: `app/settings.go`, `app/settings.toml`
- Test: `app/settings_test.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/config`, `.../render`, `.../util`, `github.com/spf13/pflag`.
- Produces: `app.Settings` (with sub-structs `WindowSettings`, `CameraSettings`, `DebugSettings`, `ScreenshotSettings`, `RunSettings`, `LogSettings`, `RenderSettings`); `app.LoadSettings(localPath string, overrides []string) (*Settings, error)`; `(*Settings).RegisterFlags(fs *pflag.FlagSet)`; `(*Settings).Apply()`.

- [ ] **Step 1: Write `app/settings.toml`**

```toml
[window]
title = "Vantage"
width = 0
height = 0
fullscreen = true

[camera]
move_speed = 5.0
zoom_speed = 0.1

[debug]
enabled = true
http_enabled = false
http_port = 8967

[screenshot]
path = ""
delay = "0s"
frequency = "0s"

[run]
for = "0s"

[log]
level = "info"

[render]
use_placeholder_sprite_images = false
```

- [ ] **Step 2: Write `app/settings.go`**

```go
package app

import (
	_ "embed"
	"fmt"

	flag "github.com/spf13/pflag"

	"github.com/trancecode/vantage/config"
	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/util"
)

//go:embed settings.toml
var defaultSettingsTOML []byte

// Settings is the engine's configuration, loaded from layered TOML and flags.
type Settings struct {
	Window     WindowSettings     `toml:"window"`
	Camera     CameraSettings     `toml:"camera"`
	Debug      DebugSettings      `toml:"debug"`
	Screenshot ScreenshotSettings `toml:"screenshot"`
	Run        RunSettings        `toml:"run"`
	Log        LogSettings        `toml:"log"`
	Render     RenderSettings     `toml:"render"`
}

// WindowSettings configures the OS window. A zero Width or Height means the
// engine uses the monitor size and goes fullscreen.
type WindowSettings struct {
	Title      string `toml:"title"`
	Width      int    `toml:"width"`
	Height     int    `toml:"height"`
	Fullscreen bool   `toml:"fullscreen"`
}

// CameraSettings holds default pan/zoom speeds for the camera controller. The
// engine does not consume these directly; a game applies them to its
// CameraController.
type CameraSettings struct {
	MoveSpeed float64 `toml:"move_speed"`
	ZoomSpeed float64 `toml:"zoom_speed"`
}

// DebugSettings configures debug mode and the debug HTTP server.
type DebugSettings struct {
	Enabled     bool `toml:"enabled"`
	HTTPEnabled bool `toml:"http_enabled"`
	HTTPPort    int  `toml:"http_port"`
}

// ScreenshotSettings configures automatic screenshot capture.
type ScreenshotSettings struct {
	Path      string          `toml:"path"`
	Delay     config.Duration `toml:"delay"`
	Frequency config.Duration `toml:"frequency"`
}

// RunSettings configures run duration. A zero For means run until closed.
type RunSettings struct {
	For config.Duration `toml:"for"`
}

// LogSettings configures logging. Consumed by a game at startup.
type LogSettings struct {
	Level string `toml:"level"`
}

// RenderSettings holds render toggles.
type RenderSettings struct {
	UsePlaceholderSpriteImages bool `toml:"use_placeholder_sprite_images"`
}

// LoadSettings loads engine settings from the embedded defaults, an optional
// local TOML file, and section.key=value overrides.
func LoadSettings(localPath string, overrides []string) (*Settings, error) {
	s := &Settings{}
	l := config.New()
	l.RegisterTarget("engine", s)
	l.AddDefaults(defaultSettingsTOML)
	if err := l.Load(localPath, overrides); err != nil {
		return nil, fmt.Errorf("loading engine settings: %w", err)
	}
	return s, nil
}

// RegisterFlags binds the engine's command-line flags to the settings, using
// the current values as defaults. Call this after LoadSettings and before
// parsing, so an explicitly-set flag overrides the loaded value.
func (s *Settings) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.Window.Title, "window_title", s.Window.Title, "Window title")
	fs.IntVar(&s.Window.Width, "width", s.Window.Width, "Window width in pixels (0 = fullscreen at monitor size)")
	fs.IntVar(&s.Window.Height, "height", s.Window.Height, "Window height in pixels (0 = fullscreen at monitor size)")
	fs.BoolVar(&s.Debug.Enabled, "debug", s.Debug.Enabled, "Enable debug mode")
	fs.BoolVar(&s.Debug.HTTPEnabled, "enable_debug_http_server", s.Debug.HTTPEnabled, "Enable the debug HTTP server")
	fs.IntVar(&s.Debug.HTTPPort, "debug_http_port", s.Debug.HTTPPort, "Port for the debug HTTP server")
	fs.StringVar(&s.Screenshot.Path, "screenshot_path", s.Screenshot.Path, "Screenshot path pattern (use %d for frame sequences)")
	fs.DurationVar(&s.Screenshot.Delay.Duration, "screenshot_delay", s.Screenshot.Delay.Duration, "Wait this long before the first screenshot")
	fs.DurationVar(&s.Screenshot.Frequency.Duration, "screenshot_frequency", s.Screenshot.Frequency.Duration, "Interval between screenshots")
	fs.DurationVar(&s.Run.For.Duration, "run_for", s.Run.For.Duration, "Exit after this duration (0 = run until closed)")
	fs.StringVar(&s.Log.Level, "log_level", s.Log.Level, "Minimum log level: trace, debug, info, warn, error")
}

// Apply applies the settings that control engine-global toggles.
func (s *Settings) Apply() {
	util.DebugMode = s.Debug.Enabled
	render.UsePlaceholderSpriteImages = s.Render.UsePlaceholderSpriteImages
}
```

- [ ] **Step 3: Write `app/settings_test.go`**

```go
package app

import (
	"testing"
	"time"

	flag "github.com/spf13/pflag"
)

func TestLoadSettingsDefaults(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Window.Title != "Vantage" {
		t.Fatalf("window title = %q", s.Window.Title)
	}
	if s.Camera.MoveSpeed != 5.0 || s.Camera.ZoomSpeed != 0.1 {
		t.Fatalf("camera defaults = %v/%v", s.Camera.MoveSpeed, s.Camera.ZoomSpeed)
	}
	if !s.Debug.Enabled || s.Debug.HTTPPort != 8967 {
		t.Fatalf("debug defaults = %+v", s.Debug)
	}
	if s.Log.Level != "info" {
		t.Fatalf("log level = %q", s.Log.Level)
	}
}

func TestLoadSettingsOverrides(t *testing.T) {
	s, err := LoadSettings("", []string{
		"window.width=1280",
		"camera.move_speed=9.5",
		"screenshot.delay=3s",
		"debug.enabled=false",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Window.Width != 1280 {
		t.Fatalf("width = %d", s.Window.Width)
	}
	if s.Camera.MoveSpeed != 9.5 {
		t.Fatalf("move_speed = %v", s.Camera.MoveSpeed)
	}
	if s.Screenshot.Delay.Duration != 3*time.Second {
		t.Fatalf("delay = %v", s.Screenshot.Delay.Duration)
	}
	if s.Debug.Enabled {
		t.Fatal("debug.enabled should be false")
	}
}

func TestRegisterFlagsOverrideLoadedValues(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--width=640", "--debug=false"}); err != nil {
		t.Fatal(err)
	}
	if s.Window.Width != 640 {
		t.Fatalf("flag did not override width: %d", s.Window.Width)
	}
	if s.Debug.Enabled {
		t.Fatal("flag did not override debug")
	}
	// Unprovided flag keeps the loaded default.
	if s.Window.Title != "Vantage" {
		t.Fatalf("unprovided flag changed title: %q", s.Window.Title)
	}
}

func TestApplySetsGlobals(t *testing.T) {
	s, err := LoadSettings("", []string{"debug.enabled=true", "render.use_placeholder_sprite_images=true"})
	if err != nil {
		t.Fatal(err)
	}
	s.Apply()
	if !util.DebugMode {
		t.Fatal("Apply did not set util.DebugMode")
	}
	if !render.UsePlaceholderSpriteImages {
		t.Fatal("Apply did not set render.UsePlaceholderSpriteImages")
	}
}
```
The `TestApplySetsGlobals` test references `util` and `render`; add those imports (`"github.com/trancecode/vantage/render"`, `"github.com/trancecode/vantage/util"`) to the test file.

- [ ] **Step 4: Vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./app/... && gofmt -l app/ && xvfb-run -a go test ./app/... -run 'Settings|Apply'
```
Expected: vet clean; `gofmt -l` silent; the settings/apply tests PASS. (The full `app` suite runs in Task 4 after the App rework.)

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add app/settings.go app/settings.toml app/settings_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add engine Settings schema with embedded defaults, flags, and apply

Settings models the engine's configuration (window, camera, debug, screenshot,
run, log, render) loaded via the config loader from an embedded settings.toml.
RegisterFlags declares the engine's command-line flags; Apply sets the
engine-global toggles util.DebugMode and render.UsePlaceholderSpriteImages."
git push origin main
```

---

## Task 4: Build `App` from `*Settings`

Replace the Phase-3 `Config`/`ScreenshotConfig` structs with `*Settings`. The App reads window, screenshot, and run-for from settings and applies settings at the start of `Run`.

**Files:**
- Modify: `app/app.go`, `app/app_screenshot.go`, `app/app_test.go`, `app/app_screenshot_test.go`

**Interfaces:**
- Produces: `app.New(settings *Settings) *App`; `App` no longer has a `Config` type. `Run` applies settings and uses `settings.Window.*`, `settings.Screenshot.*`, `settings.Run.For`. The capturer constructor becomes `newScreenshotCapturer(path string, delay, frequency time.Duration)`.

- [ ] **Step 1: Update `app/app.go` to use `*Settings`**

1. Delete the `Config` struct entirely (the `WindowTitle`/`WindowWidth`/`WindowHeight`/`ExitAfter`/`Screenshot` struct from Phase 3).
2. In the `App` struct, replace the `config Config` field with `settings *Settings`.
3. Replace `New`:
```go
// New returns an App driven by the given settings, with an empty scene Manager.
func New(settings *Settings) *App {
	return &App{
		settings: settings,
		manager:  scene.NewManager(),
	}
}
```
4. Rewrite `Run` to use settings and apply them:
```go
// Run applies settings, sets up the window, initializes scenes, and runs the
// Ebiten loop. It returns nil on a clean exit.
func (a *App) Run() error {
	a.settings.Apply()

	if a.settings.Window.Width > 0 && a.settings.Window.Height > 0 {
		ebiten.SetWindowSize(a.settings.Window.Width, a.settings.Window.Height)
	} else {
		w, h := ebiten.Monitor().Size()
		ebiten.SetWindowSize(w, h)
		ebiten.SetFullscreen(true)
	}
	ebiten.SetWindowTitle(a.settings.Window.Title)

	a.screenWidth, a.screenHeight = ebiten.Monitor().Size()
	a.manager.Init(a.screenWidth, a.screenHeight)

	if a.settings.Run.For.Duration > 0 {
		a.exitAt = time.Now().Add(a.settings.Run.For.Duration)
	}

	if a.settings.Screenshot.Path != "" {
		a.screenshot = newScreenshotCapturer(
			a.settings.Screenshot.Path,
			a.settings.Screenshot.Delay.Duration,
			a.settings.Screenshot.Frequency.Duration,
		)
		util.Logger.Info().Msgf("Screenshot capture enabled: path=%s delay=%s frequency=%s",
			a.settings.Screenshot.Path, a.settings.Screenshot.Delay.Duration, a.settings.Screenshot.Frequency.Duration)
	}

	if err := ebiten.RunGame(a); err != nil {
		if errors.Is(err, ErrExit) {
			return nil
		}
		return err
	}
	return nil
}
```
Leave `Update`, `Draw`, `Layout`, `RequestExit`, `Manager`, `OnUpdate`, the watchdog, and `ErrExit` unchanged. (`Update` still reads `util.DebugMode`, which `Apply` set.)

- [ ] **Step 2: Update `app/app_screenshot.go` to a primitive constructor**

Replace `ScreenshotConfig` and `newScreenshotCapturer(cfg ScreenshotConfig)` with a primitive constructor (the `ScreenshotConfig` type is superseded by `Settings.Screenshot`):
```go
func newScreenshotCapturer(path string, delay, frequency time.Duration) *screenshotCapturer {
	return &screenshotCapturer{
		path:      path,
		delay:     delay,
		frequency: frequency,
		sequence:  strings.Contains(path, "%"),
	}
}
```
Delete the `ScreenshotConfig` type. Keep the `screenshotCapturer` struct, `tick`, `capture`, and `SaveScreenshot` unchanged.

- [ ] **Step 3: Update `app/app_test.go`**

Change every `New(Config{...})` to `New(&Settings{})`. The existing tests (`TestNewAppHasManager`, `TestRequestExitMakesUpdateReturnErrExit`, `TestOnUpdateErrorStopsLoopBeforeScenes`) construct an App and call `Update`/`Manager` directly, so a zero-value `&Settings{}` is sufficient — none read window/screenshot. Keep the `countingScene` helper. Do not call `Run` in tests.

- [ ] **Step 4: Update `app/app_screenshot_test.go`**

Change the two capturer constructions from the `ScreenshotConfig{...}` literal to the primitive call:
```go
c := newScreenshotCapturer("/tmp/shot.png", time.Second, 0)
```
and
```go
c := newScreenshotCapturer("/tmp/frame-%d.png", 0, time.Second)
```
Keep the rest of those tests unchanged.

- [ ] **Step 5: Vet, format, and run the full module under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./... && gofmt -l app/ && go build ./... && xvfb-run -a go test ./...
```
Expected: vet clean; `gofmt -l` silent; every package (`app`, `asset`, `config`, `geometry`, `render`, `scene`, `ui`, `util`) builds and tests PASS.

- [ ] **Step 6: Commit**

```bash
cd ~/src/vantage
git add app/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Drive App from engine Settings

App.New takes *Settings instead of the Phase-3 Config struct. Run applies
settings (setting global toggles) and reads window, screenshot, and run-for
from them. The screenshot capturer now takes primitive arguments, and the
superseded ScreenshotConfig type is removed."
git push origin main
```

---

## Self-review (Phase 4)

* **Spec coverage:** Implements the design spec's configuration section — a layered loader (embedded defaults → game-registered defaults → local file → `--config_override`), reflection-routed `section.key` overrides across registered targets, an engine settings struct with an embedded `settings.toml`, engine-declared flags, and migration of the `render` package-level flag plus `util.DebugMode` into the settings (`Apply`). The App is settings-driven.
* **Placeholders:** none. The `config` loader, `Settings`, and the App rework are complete code. Deferred consumers (`[camera]`, `[log]`, `[debug].http_*`) are explicitly scoped to Phase 6 and are still defined, loadable, and flag-exposed now.
* **Type consistency:** `config.Loader`/`config.Duration` defined in Task 1 are used by `Settings` in Task 3; `Settings`/`LoadSettings`/`Apply` from Task 3 are used by the App in Task 4; `render.UsePlaceholderSpriteImages` from Task 2 is set by `Settings.Apply` in Task 3; `newScreenshotCapturer`'s new signature in Task 4 Step 2 matches its callers in Step 1 and the tests in Step 4.
* **Layering:** `config` is dependency-free; `app` imports `config`, `render`, `util`, `scene` — no cycles. `render` loses its `pflag` dependency; `app` keeps `pflag` (used by `RegisterFlags`).
* **Deferred:** game-assembly wiring (Settings→CameraController, log level, debug HTTP server) is Phase 6; the Phase-3 screenshot-on-exit-frame edge case remains a tracked Phase-6 item.
