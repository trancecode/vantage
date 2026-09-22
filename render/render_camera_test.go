package render

import (
	"fmt"
	"math"
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

	for _, factor := range []float64{1.1, 1.1, 1.5, 0.5, 0.6, 1.2} {
		cc.zoomBy(factor)
		requireWorldNear(t, centre, screenCentreWorld(cc.Camera), fmt.Sprintf("after zooming by a factor of %v to %v", factor, cc.Camera.Zoom()))
	}
}

// TestCameraControllerZoomKeepsAPannedScreenCentre tests that a zoom after a pan
// in the same frame keeps the panned view's centre, since HandleInput pans
// before it zooms.
func TestCameraControllerZoomKeepsAPannedScreenCentre(t *testing.T) {
	cc := NewCameraController(cameraCentredOn(geometry.NewVector2(9000, -4200)))
	cc.Camera.Move(geometry.NewVector2(-37, 12))
	centre := screenCentreWorld(cc.Camera)

	cc.zoomBy(1.3)

	requireWorldNear(t, centre, screenCentreWorld(cc.Camera), "after panning then zooming")
}

// TestCameraControllerZoomAtALimitLeavesThePositionAlone tests that a zoom step
// the camera clamps away changes neither the zoom nor the position.
func TestCameraControllerZoomAtALimitLeavesThePositionAlone(t *testing.T) {
	for _, limit := range []struct {
		name   string
		factor float64
		zoom   func(*Camera) float64
	}{
		{"max", 1.1, (*Camera).MaxZoom},
		{"min", 1 / 1.1, (*Camera).MinZoom},
	} {
		c := cameraCentredOn(geometry.NewVector2(9000, -4200))
		c.SetZoom(limit.zoom(c))
		position := c.Position()
		cc := NewCameraController(c)

		cc.zoomBy(limit.factor)

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

// TestSetZoomLimitsWidensTheReachableRange tests that a game can set a zoom
// floor below the engine default and then actually reach it.
func TestSetZoomLimitsWidensTheReachableRange(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZoomLimits(0.02, 20)

	if c.MinZoom() != 0.02 || c.MaxZoom() != 20 {
		t.Fatalf("limits = %v to %v, want 0.02 to 20", c.MinZoom(), c.MaxZoom())
	}
	c.SetZoom(0.02)
	if c.Zoom() != 0.02 {
		t.Fatalf("zoom = %v at the new floor, want 0.02", c.Zoom())
	}
	c.SetZoom(0.001)
	if c.Zoom() != 0.02 {
		t.Fatalf("zoom = %v below the new floor, want it clamped to 0.02", c.Zoom())
	}
}

// TestSetZoomLimitsClampsTheCurrentZoom tests that narrowing the range around a
// camera already outside it brings the camera back in.
func TestSetZoomLimitsClampsTheCurrentZoom(t *testing.T) {
	c := NewCamera(800, 600)
	c.SetZoom(3)

	c.SetZoomLimits(0.02, 0.5)

	if c.Zoom() != 0.5 {
		t.Fatalf("zoom = %v after the range moved below it, want 0.5", c.Zoom())
	}
}

// TestSetZoomLimitsRejectsAnInvalidRange tests that a range with no value to
// clamp to, or one admitting a zoom of zero, panics rather than leaving the
// camera with a transform that collapses the world.
func TestSetZoomLimitsRejectsAnInvalidRange(t *testing.T) {
	for _, limits := range []struct {
		name     string
		min, max float64
	}{
		{"zero minimum", 0, 5},
		{"negative minimum", -1, 5},
		{"maximum below minimum", 2, 1},
		// A NaN limit passes every ordered comparison, so it would be stored
		// and then silently disable the clamp on that side.
		{"NaN minimum", math.NaN(), 5},
		{"NaN maximum", 0.02, math.NaN()},
	} {
		t.Run(limits.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("SetZoomLimits(%v, %v) did not panic", limits.min, limits.max)
				}
			}()
			NewCamera(800, 600).SetZoomLimits(limits.min, limits.max)
		})
	}
}

// TestPanSpeedIsConstantInScreenPixelsAtEveryZoom tests that a frame of
// keyboard panning moves the view by the same number of screen pixels however
// far the camera is zoomed out, so a wide view does not crawl.
func TestPanSpeedIsConstantInScreenPixelsAtEveryZoom(t *testing.T) {
	pan := func(zoom float64) geometry.Vector2 {
		c := NewCamera(800, 600)
		c.SetZoomLimits(0.02, 5)
		c.SetZoom(zoom)
		before := c.Position()
		NewCameraController(c).applyPanInput(false, true, false, true)
		return c.Position().Sub(before)
	}

	atOne := pan(1)
	if atOne == geometry.Zero2D() {
		t.Fatal("panning at zoom 1 moved the camera nowhere")
	}
	for _, zoom := range []float64{0.02, 0.5, 5} {
		if got := pan(zoom); got != atOne {
			t.Fatalf("pan at zoom %v moved by (%v, %v), want (%v, %v) as at zoom 1", zoom, got.X(), got.Y(), atOne.X(), atOne.Y())
		}
	}
}

// TestZoomInputStepsAreMultiplicative tests that one input step changes the
// zoom by a constant ratio rather than a constant amount, in both directions.
func TestZoomInputStepsAreMultiplicative(t *testing.T) {
	cc := NewCameraController(NewCamera(800, 600))
	cc.Camera.SetZoomLimits(0.02, 5)

	for _, step := range []struct {
		name  string
		wheel float64
		want  float64
	}{
		{"one step in", 1, 1.1},
		{"a second step in", 1, 1.1 * 1.1},
		{"two steps out", -2, 1},
	} {
		cc.applyZoomInput(step.wheel, false, false)
		if diff := cc.Camera.Zoom() - step.want; diff > 1e-9 || diff < -1e-9 {
			t.Fatalf("%s: zoom = %v, want %v", step.name, cc.Camera.Zoom(), step.want)
		}
	}
}

// TestZoomInputReachesAWidenedFloor tests that multiplicative steps cross the
// orders of magnitude between the default framing and a continent-wide view,
// which additive steps of ZoomSpeed could never do.
func TestZoomInputReachesAWidenedFloor(t *testing.T) {
	cc := NewCameraController(NewCamera(800, 600))
	cc.Camera.SetZoomLimits(0.02, 5)

	for range 100 {
		cc.applyZoomInput(0, true, false)
	}

	if cc.Camera.Zoom() != cc.Camera.MinZoom() {
		t.Fatalf("zoom = %v after 100 zoom-out steps, want the floor %v", cc.Camera.Zoom(), cc.Camera.MinZoom())
	}
}

// TestZoomInputRejectsAStepBaseThatIsNotPositive tests that a ZoomSpeed which
// would make a step multiply by a non-positive base is refused. Raising such a
// base to the fractional step count a trackpad produces gives NaN, and a NaN
// zoom passes every clamp comparison, so it would stick silently.
func TestZoomInputRejectsAStepBaseThatIsNotPositive(t *testing.T) {
	for _, zoomSpeed := range []float64{-1, -2, math.NaN()} {
		t.Run(fmt.Sprintf("ZoomSpeed %v", zoomSpeed), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("a ZoomSpeed of %v did not panic", zoomSpeed)
				}
			}()
			cc := NewCameraController(NewCamera(800, 600))
			cc.ZoomSpeed = zoomSpeed
			cc.applyZoomInput(0.5, false, false)
		})
	}
}

// TestZoomInputHandlesAFractionalWheelStep tests that a trackpad's fractional
// scroll zooms by a fractional power of the step base rather than a whole
// notch.
func TestZoomInputHandlesAFractionalWheelStep(t *testing.T) {
	cc := NewCameraController(NewCamera(800, 600))

	cc.applyZoomInput(0.5, false, false)

	want := math.Pow(1.1, 0.5)
	if diff := cc.Camera.Zoom() - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("zoom = %v after half a wheel step, want %v", cc.Camera.Zoom(), want)
	}
}
