package render

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

type Drawable interface {
	Draw(screen *ebiten.Image, c *Camera)
}
