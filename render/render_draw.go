package render

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Drawable is implemented by anything that can render itself to screen using a Camera for the world-to-screen transform.
type Drawable interface {
	Draw(screen *ebiten.Image, c *Camera)
}
