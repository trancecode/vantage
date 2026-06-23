package scene

import (
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/render"
)

// BaseScene provides a base implementation of common scene functionality.
type BaseScene struct {
	Visible bool
	Focus   bool

	Camera *render.Camera
}

// Update processes the scene update. Default implementation does nothing.
func (b *BaseScene) Update(duration time.Duration) error {
	return nil
}

// Draw renders the scene. Default implementation does nothing.
func (b *BaseScene) Draw(screen *ebiten.Image) {
}

// IsVisible returns whether the scene is visible.
func (b *BaseScene) IsVisible() bool {
	return b.Visible
}

// HasFocus returns whether the scene has input focus.
func (b *BaseScene) HasFocus() bool {
	return b.Focus
}

// SetVisible sets the scene visibility.
func (b *BaseScene) SetVisible(v bool) {
	b.Visible = v
}

// SetFocus sets the scene input focus.
func (b *BaseScene) SetFocus(f bool) {
	b.Focus = f
}
