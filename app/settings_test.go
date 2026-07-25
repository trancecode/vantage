package app

import (
	"testing"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/util"
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
	util.DebugMode = false
	s.Apply()
	if !util.DebugMode {
		t.Fatal("Apply did not set util.DebugMode")
	}
	if !render.UsePlaceholderSpriteImages {
		t.Fatal("Apply did not set render.UsePlaceholderSpriteImages")
	}
}

func TestLoadSettingsSceneShowDefaultsEmpty(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 0 {
		t.Fatalf("scene.show default = %v, want empty", s.Scene.Show)
	}
}

func TestSceneFlagRepeats(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene=rts", "--scene=dialog"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 2 || s.Scene.Show[0] != "rts" || s.Scene.Show[1] != "dialog" {
		t.Fatalf("scene.show = %v, want [rts dialog]", s.Scene.Show)
	}
}

func TestSceneFlagOverridesLoadedValue(t *testing.T) {
	s, err := LoadSettings("", []string{"scene.show=rts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 1 || s.Scene.Show[0] != "rts" {
		t.Fatalf("scene.show from override = %v, want [rts]", s.Scene.Show)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene=sprite_showcase"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 1 || s.Scene.Show[0] != "sprite_showcase" {
		t.Fatalf("flag did not override scene.show: %v", s.Scene.Show)
	}
}

// TestSceneShowCommaSeparatedMatchesAcrossFlagAndOverride pins down that a
// comma-separated scene.show means the same thing whether it arrives through
// the [scene] show override or through the repeatable --scene flag: both must
// CSV-split the value the same way, since pflag's StringSliceVar does.
func TestSceneShowCommaSeparatedMatchesAcrossFlagAndOverride(t *testing.T) {
	viaOverride, err := LoadSettings("", []string{"scene.show=rts,dialog"})
	if err != nil {
		t.Fatal(err)
	}

	viaFlag, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	viaFlag.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene=rts,dialog"}); err != nil {
		t.Fatal(err)
	}

	if len(viaOverride.Scene.Show) != len(viaFlag.Scene.Show) {
		t.Fatalf("override gave %v, flag gave %v", viaOverride.Scene.Show, viaFlag.Scene.Show)
	}
	for i := range viaOverride.Scene.Show {
		if viaOverride.Scene.Show[i] != viaFlag.Scene.Show[i] {
			t.Fatalf("override gave %v, flag gave %v", viaOverride.Scene.Show, viaFlag.Scene.Show)
		}
	}
}
