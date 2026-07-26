package render

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

func TestCameraWorldScreenRoundTrip(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZeroAsCenter()
	world := geometry.NewVector2(3.5, -2.0)
	got := c.ScreenToWorld(c.WorldToScreen(world))
	const eps = 1e-9
	if diff := got.X() - world.X(); diff > eps || diff < -eps {
		t.Fatalf("round-trip X = %v, want %v", got.X(), world.X())
	}
	if diff := got.Y() - world.Y(); diff > eps || diff < -eps {
		t.Fatalf("round-trip Y = %v, want %v", got.Y(), world.Y())
	}
}

func TestSetZoomClampsToMax(t *testing.T) {
	// Two different far-above-max zoom requests must clamp to the same value,
	// without the test needing to know the exact maxZoom limit.
	a := NewCamera(800, 600)
	a.SetZoom(1000)
	b := NewCamera(800, 600)
	b.SetZoom(500)
	if a.Zoom() != b.Zoom() {
		t.Fatalf("SetZoom not clamped to max: %v vs %v", a.Zoom(), b.Zoom())
	}
}

func TestAddZoomClampsToMin(t *testing.T) {
	// Two different far-below-min zoom requests must clamp to the same value,
	// without the test needing to know the exact minZoom limit.
	a := NewCamera(800, 600)
	a.AddZoom(-1000)
	b := NewCamera(800, 600)
	b.AddZoom(-500)
	if a.Zoom() != b.Zoom() {
		t.Fatalf("AddZoom not clamped to min: %v vs %v", a.Zoom(), b.Zoom())
	}
}

func TestAddZoomClampsToMax(t *testing.T) {
	// Two different far-above-max AddZoom requests must clamp to the same value.
	a := NewCamera(800, 600)
	a.AddZoom(1000)
	b := NewCamera(800, 600)
	b.AddZoom(500)
	if a.Zoom() != b.Zoom() {
		t.Fatalf("AddZoom not clamped to max: %v vs %v", a.Zoom(), b.Zoom())
	}
}

func TestNewCameraControllerDefaults(t *testing.T) {
	cc := NewCameraController(NewCamera(800, 600))
	if cc.Camera == nil {
		t.Fatal("controller camera is nil")
	}
	if cc.MoveSpeed != 5 || cc.ZoomSpeed != 0.1 {
		t.Fatalf("unexpected defaults: MoveSpeed=%v ZoomSpeed=%v", cc.MoveSpeed, cc.ZoomSpeed)
	}
}

func TestTileSizeDefaultsTo16(t *testing.T) {
	// A change to the shipped default silently rescales every consuming game,
	// so it must be a deliberate act.
	if TileSize != 16 {
		t.Fatalf("TileSize = %v, want 16", TileSize)
	}
}

func TestEffectiveZoomTracksATileSizeChangedAfterTheCameraWasBuilt(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	c := NewCamera(640, 640)
	before := c.EffectiveZoom()

	TileSize = 32
	after := c.EffectiveZoom()

	if after == before {
		t.Fatal("EffectiveZoom ignored a tile size changed after construction")
	}
	if want := before / 2; after != want {
		t.Fatalf("EffectiveZoom = %v after doubling TileSize, want %v", after, want)
	}
}

func TestEffectiveZoomUnchangedAtTheDefaultTileSize(t *testing.T) {
	// The compatibility guarantee: 640/(20*16) = 2, times a user zoom of 1.
	c := NewCamera(640, 640)
	if got := c.EffectiveZoom(); got != 2 {
		t.Fatalf("EffectiveZoom = %v, want 2", got)
	}
}

func TestScreenCameraIgnoresTileSize(t *testing.T) {
	// A screen-space camera is an identity transform and must not be affected
	// by the world tile size.
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	c := NewScreenCamera(640, 480)
	before := c.EffectiveZoom()

	TileSize = 64
	if after := c.EffectiveZoom(); after != before {
		t.Fatalf("screen camera EffectiveZoom changed with TileSize: %v then %v", before, after)
	}
	if before != 1 {
		t.Fatalf("screen camera EffectiveZoom = %v, want 1", before)
	}
}
