package render

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/geometry"
)

const (
	TileSize = 16
)

// Camera represents the game's camera.
type Camera struct {
	pos                       geometry.Vector2
	zoom                      float64
	screenMultiplier          float64 // Multiplier to normalize zoom across different screen sizes
	minZoom                   float64
	maxZoom                   float64
	screenWidth, screenHeight int
}

// NewCamera creates and returns a new Camera with default values.
func NewCamera(screenWidth, screenHeight int) *Camera {
	// Calculate multiplier to show exactly 20 tiles vertically
	targetTilesVertical := 20.0
	screenMultiplier := float64(screenHeight) / (targetTilesVertical * TileSize)

	return &Camera{
		pos:              geometry.NewVector2(0, 0),
		zoom:             1.0, // User-facing default zoom is 1.0
		screenMultiplier: screenMultiplier,
		minZoom:          0.2,
		maxZoom:          5,
		screenWidth:      screenWidth,
		screenHeight:     screenHeight,
	}
}

// NewScreenCamera creates a camera for screen-space rendering.
// This camera uses an identity transformation (no zoom, no world offset),
// making it suitable for UI elements that should remain in screen coordinates.
func NewScreenCamera(screenWidth, screenHeight int) *Camera {
	return &Camera{
		pos:              geometry.NewVector2(0, 0),
		zoom:             1,
		screenMultiplier: 1.0,
		minZoom:          1,
		maxZoom:          1,
		screenWidth:      screenWidth,
		screenHeight:     screenHeight,
	}
}

// Position returns the camera's position.
func (c *Camera) Position() geometry.Vector2 {
	return c.pos
}

// SetPosition sets the camera's position.
func (c *Camera) SetPosition(pos geometry.Vector2) {
	c.pos = pos
}

// SetZeroAsCenter sets the camera's position so that (0,0) in world space is at the center of the screen.
func (c *Camera) SetZeroAsCenter() {
	c.pos = geometry.NewVector2(0, 0)
}

// SetZeroAsTopLeft sets the camera's position so that (0,0) in world space is at the top-left of the screen.
func (c *Camera) SetZeroAsTopLeft() {
	c.pos = geometry.NewVector2(-c.screenWidth/2, -c.screenHeight/2)
}

// Zoom returns the camera's effective zoom level (user zoom * screen multiplier).
func (c *Camera) Zoom() float64 {
	return c.zoom * c.screenMultiplier
}

// SetZoom sets the camera's zoom level, clamped to the camera's limits.
func (c *Camera) SetZoom(zoom float64) {
	c.zoom = zoom
	c.clampZoom()
}

// AddZoom adjusts the zoom level by delta, clamped to the camera's limits.
func (c *Camera) AddZoom(delta float64) {
	c.zoom += delta
	c.clampZoom()
}

// Move moves the camera by the given delta.
func (c *Camera) Move(delta geometry.Vector2) {
	c.pos = c.pos.Add(delta)
}

func (c *Camera) clampZoom() {
	if c.zoom < c.minZoom {
		c.zoom = c.minZoom
	}
	if c.zoom > c.maxZoom {
		c.zoom = c.maxZoom
	}
}

// ScreenWidth returns the camera's screen width.
func (c *Camera) ScreenWidth() int {
	return c.screenWidth
}

// ScreenHeight returns the camera's screen height.
func (c *Camera) ScreenHeight() int {
	return c.screenHeight
}

func (c *Camera) CameraDebugInfo() string {
	return fmt.Sprintf("Camera X: %f | Camera Y: %f | Camera Zoom: %f", c.pos.X(), c.pos.Y(), c.Zoom())
}

func (c *Camera) DrawImageOptions(p geometry.Vector2) *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(p.X(), p.Y())
	effectiveZoom := c.Zoom()
	op.GeoM.Scale(effectiveZoom, effectiveZoom)

	// Apply camera's position and the offset to center (0,0)
	op.GeoM.Translate(c.pos.X()+float64(c.screenWidth/2), c.pos.Y()+float64(c.screenHeight/2))
	return op
}

// Adjust adjust the camera for a draw operation
func (c *Camera) Adjust(op *ebiten.DrawImageOptions, p geometry.Vector2) {
	op.GeoM.Translate(float64(p.X())*TileSize, float64(p.Y())*TileSize) // Apply TileSize
	effectiveZoom := c.Zoom()
	op.GeoM.Scale(effectiveZoom, effectiveZoom)
	// Apply camera's position and the offset to center (0,0)
	op.GeoM.Translate(c.pos.X()+float64(c.screenWidth/2), c.pos.Y()+float64(c.screenHeight/2))
}

// ScreenToWorld converts screen coordinates to world coordinates.
func (c *Camera) ScreenToWorld(screenPos geometry.Vector2) geometry.Vector2 {
	// Reverse the camera translation and centering offset, and adjust for zoom
	effectiveZoom := c.Zoom()
	worldX := (screenPos.X() - (c.pos.X() + float64(c.screenWidth)/2)) / effectiveZoom
	worldY := (screenPos.Y() - (c.pos.Y() + float64(c.screenHeight)/2)) / effectiveZoom

	// Adjust for tile size (if your world coordinates are in tiles)
	return geometry.NewVector2(worldX/TileSize, worldY/TileSize)
}

// WorldToScreen converts world coordinates to screen coordinates.
func (c *Camera) WorldToScreen(worldPos geometry.Vector2) geometry.Vector2 {
	// Adjust for tile size first
	worldX := worldPos.X() * TileSize
	worldY := worldPos.Y() * TileSize

	// Apply zoom, camera translation, and centering offset
	effectiveZoom := c.Zoom()
	screenX := worldX*effectiveZoom + (c.pos.X() + float64(c.screenWidth)/2)
	screenY := worldY*effectiveZoom + (c.pos.Y() + float64(c.screenHeight)/2)
	return geometry.NewVector2(screenX, screenY)
}
