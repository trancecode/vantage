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
