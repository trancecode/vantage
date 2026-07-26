package app

import (
	_ "embed"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

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
	Scene      SceneSettings      `toml:"scene"`
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

// SceneSettings forces which registered scenes are shown at startup. An empty
// Show leaves scene visibility entirely to the game.
type SceneSettings struct {
	Show []string `toml:"show"`
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
	UsePlaceholderSpriteImages bool          `toml:"use_placeholder_sprite_images"`
	Filter                     FilterSetting `toml:"filter"`
	TileSize                   float64       `toml:"tile_size"`
}

// FilterSetting is the texture filter used when sprites are drawn at anything
// other than their native size, expressed in TOML as "nearest" or "linear". It
// decodes from text so an unknown name fails when settings load, with the same
// diagnostic a bad duration gets.
type FilterSetting struct {
	ebiten.Filter
}

// UnmarshalText parses a filter name.
func (f *FilterSetting) UnmarshalText(text []byte) error {
	filter, err := render.ParseFilter(string(text))
	if err != nil {
		return err
	}
	f.Filter = filter
	return nil
}

// MarshalText renders the filter as its settings name.
func (f FilterSetting) MarshalText() ([]byte, error) {
	return []byte(render.FilterName(f.Filter)), nil
}

// String implements pflag.Value, which shows the current name as the flag's
// default.
func (f *FilterSetting) String() string {
	return render.FilterName(f.Filter)
}

// Set implements pflag.Value.
func (f *FilterSetting) Set(value string) error {
	return f.UnmarshalText([]byte(value))
}

// Type implements pflag.Value, naming the value shown in flag usage.
func (f *FilterSetting) Type() string {
	return "filter"
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
	fs.StringSliceVar(&s.Scene.Show, "scene", s.Scene.Show,
		"Show only the named scenes and focus the first (repeatable)")
	fs.BoolVar(&s.Debug.Enabled, "debug", s.Debug.Enabled, "Enable debug mode")
	fs.BoolVar(&s.Debug.HTTPEnabled, "enable_debug_http_server", s.Debug.HTTPEnabled, "Enable the debug HTTP server")
	fs.IntVar(&s.Debug.HTTPPort, "debug_http_port", s.Debug.HTTPPort, "Port for the debug HTTP server")
	fs.StringVar(&s.Screenshot.Path, "screenshot_path", s.Screenshot.Path, "Screenshot path pattern (use %d for frame sequences)")
	fs.DurationVar(&s.Screenshot.Delay.Duration, "screenshot_delay", s.Screenshot.Delay.Duration, "Wait this long before the first screenshot")
	fs.DurationVar(&s.Screenshot.Frequency.Duration, "screenshot_frequency", s.Screenshot.Frequency.Duration, "Interval between screenshots")
	fs.DurationVar(&s.Run.For.Duration, "run_for", s.Run.For.Duration, "Exit after this duration (0 = run until closed)")
	fs.StringVar(&s.Log.Level, "log_level", s.Log.Level, "Minimum log level: trace, debug, info, warn, error")
	fs.Var(&s.Render.Filter, "render_filter", "Texture filter for scaled sprites: nearest or linear")
	fs.Float64Var(&s.Render.TileSize, "render_tile_size", s.Render.TileSize,
		"Pixels per world tile before camera zoom")
}

// Validate reports a setting whose value cannot be used. App.Run calls it on
// the settings it loads, so a game running through the app package gets this
// for free. Call it before Apply when building a Settings by hand, so a bad
// value is a startup error rather than an engine global set to nonsense: Apply
// writes the values through whatever they are, and a tile size of zero makes
// every camera's scale a division by zero with no diagnostic anywhere.
func (s *Settings) Validate() error {
	if s.Render.TileSize <= 0 {
		return fmt.Errorf("render.tile_size must be positive, got %v", s.Render.TileSize)
	}
	return nil
}

// Apply applies the settings that control engine-global toggles.
func (s *Settings) Apply() {
	util.DebugMode = s.Debug.Enabled
	render.UsePlaceholderSpriteImages = s.Render.UsePlaceholderSpriteImages
	render.SpriteFilter = s.Render.Filter.Filter
	render.TileSize = s.Render.TileSize
}
