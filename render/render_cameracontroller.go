package render

import (
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
	// MoveSpeed is the pan speed in world units per frame, before zoom scaling.
	MoveSpeed float64
	// ZoomSpeed is the zoom increment applied per input step.
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
	moveSpeed := cc.MoveSpeed * c.EffectiveZoom()
	delta := geometry.Zero2D()

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		delta = geometry.NewVector2(delta.X(), delta.Y()+moveSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		delta = geometry.NewVector2(delta.X(), delta.Y()-moveSpeed)
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		delta = geometry.NewVector2(delta.X()-moveSpeed, delta.Y())
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		delta = geometry.NewVector2(delta.X()+moveSpeed, delta.Y())
	}
	c.Move(delta)

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

// applyZoomInput accumulates one frame's zoom input, the wheel's vertical
// scroll and whether Q (zoom out) and E (zoom in) are held, into a single
// delta, clamped once, and zooms by it about the screen centre.
func (cc *CameraController) applyZoomInput(wheelY float64, zoomOut, zoomIn bool) {
	zoomDelta := wheelY * cc.ZoomSpeed
	if zoomOut {
		zoomDelta -= cc.ZoomSpeed
	}
	if zoomIn {
		zoomDelta += cc.ZoomSpeed
	}
	if zoomDelta != 0 {
		cc.zoomBy(zoomDelta)
	}
}

// zoomBy adjusts the camera's zoom by delta, clamped to its limits, keeping the
// world position at the centre of the screen in place. The camera keeps its
// position in screen pixels, and the world position at the screen centre is
// -position / (TileSize * EffectiveZoom), so a zoom change alone would scale the
// view about world (0, 0). Multiplying the position by after / before, the
// ratio of the new effective zoom to the old, keeps that quotient, and so the
// centre, unchanged. A step the clamp absorbs leaves the position untouched.
func (cc *CameraController) zoomBy(delta float64) {
	c := cc.Camera
	before := c.EffectiveZoom()
	c.AddZoom(delta)
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
