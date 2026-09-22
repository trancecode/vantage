package render

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/geometry"
)

// CameraController drives a Camera from user input. It implements the engine's
// default pan/zoom control scheme: WASD keyboard panning, Q/E and mouse-wheel
// zoom, and middle-mouse-button drag panning. Games wanting a different scheme
// can drive the Camera directly instead of attaching a controller.
type CameraController struct {
	// Camera is the camera this controller drives.
	Camera *Camera
	// MoveSpeed is the pan speed in pixel-space units per frame at a user zoom
	// of 1, TileSize of them to a world tile. It is held constant in screen
	// pixels at every zoom, so panning covers the same fraction of the screen
	// however far the view is zoomed out.
	MoveSpeed float64
	// ZoomSpeed is the relative zoom change per input step: 0.1 means one step
	// multiplies the zoom by 1.1 or divides it by 1.1, so a step feels the same
	// at every zoom level.
	ZoomSpeed float64

	lastMouseX, lastMouseY int
	mmbPressed             bool
}

// NewCameraController returns a controller driving the given camera with the
// engine's default pan and zoom speeds.
func NewCameraController(camera *Camera) *CameraController {
	return &CameraController{
		Camera:    camera,
		MoveSpeed: 5,
		ZoomSpeed: 0.1,
	}
}

// HandleInput reads input for the current frame and pans/zooms the camera.
func (cc *CameraController) HandleInput() {
	c := cc.Camera
	cc.applyPanInput(
		ebiten.IsKeyPressed(ebiten.KeyW),
		ebiten.IsKeyPressed(ebiten.KeyS),
		ebiten.IsKeyPressed(ebiten.KeyA),
		ebiten.IsKeyPressed(ebiten.KeyD),
	)

	// Middle mouse button drag for panning.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		currentX, currentY := ebiten.CursorPosition()
		if cc.mmbPressed {
			deltaX := float64(currentX - cc.lastMouseX)
			deltaY := float64(currentY - cc.lastMouseY)
			c.SetPosition(geometry.NewVector2(c.Position().X()+deltaX, c.Position().Y()+deltaY))
		}
		cc.lastMouseX = currentX
		cc.lastMouseY = currentY
		cc.mmbPressed = true
	} else {
		cc.mmbPressed = false
	}

	_, wheelY := ebiten.Wheel()
	cc.applyZoomInput(wheelY, ebiten.IsKeyPressed(ebiten.KeyQ), ebiten.IsKeyPressed(ebiten.KeyE))
}

// applyPanInput pans the camera by one frame's held direction keys, each of
// which moves the view by MoveSpeed.
//
// The camera's position is in screen pixels, so scaling MoveSpeed by the screen
// multiplier alone, leaving the user zoom out of it, moves the view the same
// number of screen pixels at every zoom. Scaling by the full effective zoom
// instead would move a constant world distance, which crawls across the screen
// once the view is zoomed out, and would disagree with the middle-mouse drag,
// which moves with the cursor.
func (cc *CameraController) applyPanInput(up, down, left, right bool) {
	moveSpeed := cc.MoveSpeed * cc.Camera.ScreenMultiplier()
	var x, y float64
	if up {
		y += moveSpeed
	}
	if down {
		y -= moveSpeed
	}
	if left {
		x += moveSpeed
	}
	if right {
		x -= moveSpeed
	}
	cc.Camera.Move(geometry.NewVector2(x, y))
}

// applyZoomInput accumulates one frame's zoom input, the wheel's vertical
// scroll and whether Q (zoom out) and E (zoom in) are held, into a single step
// count, and zooms by it about the screen centre. A step count is fractional
// whenever the wheel reports a fractional scroll, which trackpads do.
//
// It panics unless 1 + ZoomSpeed is positive, on the first frame rather than
// on the first scroll, because a negative step base raised to a fractional
// step count is NaN, and a NaN zoom passes every clamp comparison and so
// sticks for the rest of the run.
func (cc *CameraController) applyZoomInput(wheelY float64, zoomOut, zoomIn bool) {
	base := 1 + cc.ZoomSpeed
	if !(base > 0) {
		panic(fmt.Sprintf("camera zoom step base must be positive, got %v from a ZoomSpeed of %v", base, cc.ZoomSpeed))
	}

	steps := wheelY
	if zoomOut {
		steps--
	}
	if zoomIn {
		steps++
	}
	if steps != 0 {
		cc.zoomBy(math.Pow(base, steps))
	}
}

// zoomBy multiplies the camera's zoom by factor, clamped to its limits, keeping
// the world position at the centre of the screen in place. Zoom steps multiply
// rather than add so that a step is the same relative change at every level,
// which is what lets a few notches cross the orders of magnitude between a
// tile-level and a continent-level view.
//
// The camera keeps its position in screen pixels, and the world position at the
// screen centre is -position / (TileSize * EffectiveZoom), so a zoom change
// alone would scale the view about world (0, 0). Multiplying the position by
// after / before, the ratio of the new effective zoom to the old, keeps that
// quotient, and so the centre, unchanged. A step the clamp absorbs leaves the
// position untouched.
func (cc *CameraController) zoomBy(factor float64) {
	c := cc.Camera
	before := c.EffectiveZoom()
	c.SetZoom(c.Zoom() * factor)
	after := c.EffectiveZoom()
	if after == before {
		return
	}
	scale := after / before
	c.SetPosition(geometry.NewVector2(c.Position().X()*scale, c.Position().Y()*scale))
}

// CursorWorldPosition returns the OS cursor position converted to world
// coordinates through the controller's camera.
func (cc *CameraController) CursorWorldPosition() geometry.Vector2 {
	return cc.Camera.ScreenToWorld(geometry.NewVector2(ebiten.CursorPosition()))
}
