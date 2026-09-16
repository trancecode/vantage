package render

import (
	"fmt"
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

// cameraCentredOn returns an 800x600 camera whose screen centre shows the world
// position centre.
func cameraCentredOn(centre geometry.Vector2) *Camera {
	c := NewCamera(800, 600)
	zoom := c.EffectiveZoom()
	c.SetPosition(geometry.NewVector2(-centre.X()*TileSize*zoom, -centre.Y()*TileSize*zoom))
	return c
}

// screenCentreWorld returns the world position at the centre of the camera's
// screen.
func screenCentreWorld(c *Camera) geometry.Vector2 {
	return c.ScreenToWorld(geometry.NewVector2(float64(c.ScreenWidth())/2, float64(c.ScreenHeight())/2))
}

// requireWorldNear fails the test unless got is within 1e-9 of want on both
// axes.
func requireWorldNear(t *testing.T, want, got geometry.Vector2, context string) {
	t.Helper()
	const eps = 1e-9
	dx, dy := got.X()-want.X(), got.Y()-want.Y()
	if dx > eps || dx < -eps || dy > eps || dy < -eps {
		t.Fatalf("%s: screen centre = (%v, %v), want (%v, %v)", context, got.X(), got.Y(), want.X(), want.Y())
	}
}

// TestCameraControllerZoomKeepsScreenCentre tests that zooming through the
// controller keeps the world position at the screen centre in place, far from
// the world origin, zooming in and out.
func TestCameraControllerZoomKeepsScreenCentre(t *testing.T) {
	centre := geometry.NewVector2(9000, -4200)
	cc := NewCameraController(cameraCentredOn(centre))

	for _, delta := range []float64{0.1, 0.1, 0.3, -0.5, -0.4, 0.2} {
		cc.zoomBy(delta)
		requireWorldNear(t, centre, screenCentreWorld(cc.Camera), fmt.Sprintf("after zooming by %v to %v", delta, cc.Camera.Zoom()))
	}
}

// TestCameraControllerZoomKeepsAPannedScreenCentre tests that a zoom after a pan
// in the same frame keeps the panned view's centre, since HandleInput pans
// before it zooms.
func TestCameraControllerZoomKeepsAPannedScreenCentre(t *testing.T) {
	cc := NewCameraController(cameraCentredOn(geometry.NewVector2(9000, -4200)))
	cc.Camera.Move(geometry.NewVector2(-37, 12))
	centre := screenCentreWorld(cc.Camera)

	cc.zoomBy(0.3)

	requireWorldNear(t, centre, screenCentreWorld(cc.Camera), "after panning then zooming")
}

// TestCameraControllerZoomAtALimitLeavesThePositionAlone tests that a zoom step
// the camera clamps away changes neither the zoom nor the position.
func TestCameraControllerZoomAtALimitLeavesThePositionAlone(t *testing.T) {
	for _, limit := range []struct {
		name  string
		delta float64
		zoom  func(*Camera) float64
	}{
		{"max", 0.1, (*Camera).MaxZoom},
		{"min", -0.1, (*Camera).MinZoom},
	} {
		c := cameraCentredOn(geometry.NewVector2(9000, -4200))
		c.SetZoom(limit.zoom(c))
		position := c.Position()
		cc := NewCameraController(c)

		cc.zoomBy(limit.delta)

		if c.Zoom() != limit.zoom(c) || c.Position() != position {
			t.Fatalf("zooming past the %s limit: zoom %v position %v, want zoom %v position %v", limit.name, c.Zoom(), c.Position(), limit.zoom(c), position)
		}
	}
}

// TestCameraControllerZoomInputKeepsScreenCentre tests that one frame's zoom
// input, wheel and Q/E alike, zooms about the screen centre: it is the path
// HandleInput forwards real input to.
func TestCameraControllerZoomInputKeepsScreenCentre(t *testing.T) {
	centre := geometry.NewVector2(9000, -4200)
	cc := NewCameraController(cameraCentredOn(centre))

	for _, input := range []struct {
		name            string
		wheelY          float64
		zoomOut, zoomIn bool
	}{
		{"wheel up", 1, false, false},
		{"E held", 0, false, true},
		{"Q held", 0, true, false},
		{"wheel down with E held", -2, false, true},
	} {
		zoom := cc.Camera.Zoom()
		cc.applyZoomInput(input.wheelY, input.zoomOut, input.zoomIn)
		if cc.Camera.Zoom() == zoom {
			t.Fatalf("%s: zoom stayed at %v", input.name, zoom)
		}
		requireWorldNear(t, centre, screenCentreWorld(cc.Camera), input.name)
	}
}
