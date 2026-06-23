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
	isMMBPressed           bool
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
	moveSpeed := cc.MoveSpeed * c.Zoom()
	delta := geometry.NewVector2(0, 0)

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
		if cc.isMMBPressed {
			deltaX := float64(currentX - cc.lastMouseX)
			deltaY := float64(currentY - cc.lastMouseY)
			c.SetPosition(geometry.NewVector2(c.Position().X()+deltaX, c.Position().Y()+deltaY))
		}
		cc.lastMouseX = currentX
		cc.lastMouseY = currentY
		cc.isMMBPressed = true
	} else {
		cc.isMMBPressed = false
	}

	// Accumulate zoom from wheel and Q/E into a single delta, clamped once,
	// matching the original single-clamp-per-frame behavior.
	zoomDelta := 0.0
	if _, wheelY := ebiten.Wheel(); wheelY != 0 {
		zoomDelta += wheelY * cc.ZoomSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		zoomDelta -= cc.ZoomSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		zoomDelta += cc.ZoomSpeed
	}
	if zoomDelta != 0 {
		cc.Camera.AddZoom(zoomDelta)
	}
}

// CursorWorldPosition returns the OS cursor position converted to world
// coordinates through the controller's camera.
func (cc *CameraController) CursorWorldPosition() geometry.Vector2 {
	return cc.Camera.ScreenToWorld(geometry.NewVector2(ebiten.CursorPosition()))
}
