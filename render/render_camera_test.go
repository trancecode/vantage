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
	over := NewCamera(800, 600)
	over.SetZoom(1000) // far above maxZoom
	atMax := NewCamera(800, 600)
	atMax.SetZoom(5) // maxZoom
	if over.Zoom() != atMax.Zoom() {
		t.Fatalf("SetZoom not clamped: over=%v atMax=%v", over.Zoom(), atMax.Zoom())
	}
}

func TestAddZoomClampsToMin(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZoom(1.0)
	c.AddZoom(-1000) // far below minZoom
	atMin := NewCamera(800, 600)
	atMin.SetZoom(0.2) // minZoom
	if c.Zoom() != atMin.Zoom() {
		t.Fatalf("AddZoom not clamped to min: got=%v atMin=%v", c.Zoom(), atMin.Zoom())
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
