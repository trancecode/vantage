package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/trancecode/vantage/asset"
)

// ButtonState represents the visual state of a button.
type ButtonState int

const (
	// ButtonStateNone is the default uninitialized button state.
	ButtonStateNone ButtonState = iota
	// ButtonStateNormal is the default button state.
	ButtonStateNormal
	// ButtonStateHovered is when the mouse cursor is over the button.
	ButtonStateHovered
	// ButtonStatePressed is when the button is being clicked.
	ButtonStatePressed
)

// Button represents a clickable UI button with text, a keyboard shortcut label, and hover/press states.
type Button struct {
	// Label is the display text for this button.
	Label string

	// ShortcutLabel is the text shown in brackets before the label (e.g., "1", "ESC").
	ShortcutLabel string

	// X is the left edge of the button in screen pixels.
	X float64

	// Y is the top edge of the button in screen pixels.
	Y float64

	// Width is the button width in screen pixels.
	Width float64

	// Height is the button height in screen pixels.
	Height float64

	// State is the current visual state of the button.
	State ButtonState

	// bgImage is a pre-allocated image for the button background.
	bgImage *ebiten.Image
}

// Update calculates the button state from the current mouse position and click state.
// mouseX and mouseY are screen coordinates. mousePressed is true if the left mouse button is down.
func (b *Button) Update(mouseX, mouseY float64, mousePressed bool) {
	if b.containsPoint(mouseX, mouseY) {
		if mousePressed {
			b.State = ButtonStatePressed
		} else {
			b.State = ButtonStateHovered
		}
	} else {
		b.State = ButtonStateNormal
	}
}

// Draw renders the button on the screen.
func (b *Button) Draw(screen *ebiten.Image, fontSize int) {
	// Background color based on state
	var bgColor color.RGBA
	switch b.State {
	case ButtonStateHovered:
		bgColor = color.RGBA{80, 80, 100, 220}
	case ButtonStatePressed:
		bgColor = color.RGBA{60, 60, 80, 240}
	default:
		bgColor = color.RGBA{50, 50, 70, 200}
	}

	// Draw background
	if b.bgImage == nil {
		b.bgImage = ebiten.NewImage(int(b.Width), int(b.Height))
	}
	b.bgImage.Fill(bgColor)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(b.X, b.Y)
	screen.DrawImage(b.bgImage, op)

	// Draw text
	face := &text.GoTextFace{Source: asset.DefaultProportionalFont, Size: float64(fontSize)}
	metrics := face.Metrics()
	textY := b.Y + (b.Height-metrics.HAscent-metrics.HDescent)/2

	// Draw shortcut label in a highlight color
	shortcutText := "[" + b.ShortcutLabel + "] "
	textX := b.X + 12

	shortcutOp := &text.DrawOptions{}
	shortcutOp.GeoM.Translate(textX, textY)
	shortcutOp.ColorScale.ScaleWithColor(color.RGBA{180, 180, 220, 255})
	text.Draw(screen, shortcutText, face, shortcutOp)

	// Draw label after shortcut
	shortcutAdvance := text.Advance(shortcutText, face)
	labelOp := &text.DrawOptions{}
	labelOp.GeoM.Translate(textX+shortcutAdvance, textY)
	labelOp.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, b.Label, face, labelOp)
}

func (b *Button) containsPoint(x, y float64) bool {
	return x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height
}
