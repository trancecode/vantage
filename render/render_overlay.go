package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/trancecode/vantage/geometry"
)

// FloatingBarStyle configures a world-anchored fraction bar drawn by
// DrawFloatingBar. Width, Height, and GapPixels are in screen pixels and stay
// constant across camera zoom, so the bar keeps the same on-screen size no
// matter how far the camera is zoomed in or out.
type FloatingBarStyle struct {
	// Width is the full bar width in screen pixels.
	Width float64

	// Height is the bar height in screen pixels.
	Height float64

	// GapPixels is the constant screen-pixel gap between the bottom of the bar
	// and the visible top of the sprite.
	GapPixels float64

	// Background is the color of the full-width bar backdrop.
	Background color.Color

	// Fill is the color of the filled portion representing the fraction.
	Fill color.Color
}

// DrawNameplate draws the built text of writer horizontally centered on the
// sprite at worldPos, with the text's bottom a constant gapPixels above the
// sprite's visible top. The gap stays constant in screen pixels across camera
// zoom, so the label hugs the sprite the same way whatever the zoom level.
//
// The label's horizontal alignment, color, and visibility above extreme zoom
// come from writer; use TextAlignment AlignCenter for a nameplate centered on
// the sprite. DefaultNameplateGapPixels is the recommended value for gapPixels.
func DrawNameplate(screen *ebiten.Image, camera *Camera, worldPos geometry.Vector2, sprite *Sprite, writer *TextWriter, gapPixels float64) {
	bottom := overlayBottomScreen(camera, worldPos, sprite.VisibleTopAboveZero(), gapPixels)

	// text.Draw anchors the label by its top, so shift up by the on-screen text
	// height to land its bottom on the overlay anchor. Scaling text grows with
	// zoom, so its height must be scaled to match.
	textHeight := writer.RenderedHeight()
	if writer.Scaling {
		textHeight *= camera.EffectiveZoom()
	}

	// Draw takes a world position, so round-trip the computed screen position
	// back to world space for it to convert forward again.
	top := geometry.NewVector2(bottom.X(), bottom.Y()-textHeight)
	writer.Draw(screen, camera, camera.ScreenToWorld(top))
}

// DrawFloatingBar draws a horizontal fraction bar of the given style,
// horizontally centered on the sprite at worldPos with its bottom a constant
// style.GapPixels above the sprite's visible top. The bar is a full-width
// Background rectangle overlaid by a Fill rectangle whose width is fraction of
// the full width; fraction is clamped to [0, 1]. Sizing is in screen pixels, so
// the bar keeps a fixed on-screen size across camera zoom.
//
// The bar is always drawn: choosing the fill color and deciding when to show it
// (for example hiding it at full value) is left to the caller.
func DrawFloatingBar(screen *ebiten.Image, camera *Camera, worldPos geometry.Vector2, sprite *Sprite, fraction float64, style FloatingBarStyle) {
	bottom := overlayBottomScreen(camera, worldPos, sprite.VisibleTopAboveZero(), style.GapPixels)
	x := bottom.X() - style.Width/2
	y := bottom.Y() - style.Height

	vector.FillRect(screen, float32(x), float32(y), float32(style.Width), float32(style.Height), style.Background, false)

	if fillWidth := barFillWidth(style.Width, fraction); fillWidth > 0 {
		vector.FillRect(screen, float32(x), float32(y), float32(fillWidth), float32(style.Height), style.Fill, false)
	}
}

// overlayBottomScreen returns the screen-space point where the bottom of a
// world-anchored overlay sits: horizontally centered on the sprite at worldPos
// and a constant gapPixels above the sprite's visible top. The visible-top
// offset is measured in the sprite's own pixels, so it is scaled by the
// effective zoom to reach screen pixels, while gapPixels is not scaled — this
// is what keeps the gap constant on screen across zoom levels.
func overlayBottomScreen(camera *Camera, worldPos geometry.Vector2, visibleTopAboveZero, gapPixels float64) geometry.Vector2 {
	anchor := camera.WorldToScreen(worldPos)
	visibleTopY := anchor.Y() - visibleTopAboveZero*camera.EffectiveZoom()
	return geometry.NewVector2(anchor.X(), visibleTopY-gapPixels)
}

// barFillWidth returns the width in screen pixels of the filled portion of a bar
// of the given full width, for fraction clamped to [0, 1].
func barFillWidth(width, fraction float64) float64 {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return width * fraction
}
