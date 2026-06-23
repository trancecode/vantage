package render

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/trancecode/vantage/asset"
	"github.com/trancecode/vantage/geometry"
)

const (
	// DefaultMaxZoomForText is the default maximum zoom level for text visibility
	// when scaling is disabled. Text will be hidden at zoom levels above this value.
	DefaultMaxZoomForText = 5.0

	// DefaultNameplateGapPixels is the default pixel gap between the bottom
	// of the nameplate and the visible top of the sprite. Constant in pixels
	// at any camera zoom.
	DefaultNameplateGapPixels = 4.0
)

var (
	TextDefault = NewTextWriter()
)

// HAlignment represents horizontal text alignment options.
type HAlignment int

const (
	// Left aligns text to the left.
	Left HAlignment = iota
	// Center centers text horizontally.
	Center
	// Right aligns text to the right.
	Right
)

// textSegment represents a segment of text with a specific color.
type textSegment struct {
	text  string
	color color.Color
}

// TextWriter provides a fluent API for rendering text with various styling options.
type TextWriter struct {
	Size              int
	Font              *text.GoTextFaceSource
	Color             color.Color
	Background        *color.Color  // Optional background color
	BackgroundPadding int           // Padding around background (default: 2)
	Scaling           bool          // Whether text scales with camera zoom
	MaxZoom           float64       // Max zoom level for text visibility (0 = use default)
	HAlign            HAlignment    // Horizontal alignment
	segments          []textSegment // Built text segments

	// Cached background image to avoid per-frame GPU allocations
	cachedBgImage *ebiten.Image
	cachedBgW     int
	cachedBgH     int
}

func NewTextWriter() *TextWriter {
	return &TextWriter{
		Size:              12,
		Font:              asset.DefaultProportionalFont,
		Color:             color.White,
		Background:        nil,
		BackgroundPadding: 2,
		Scaling:           false,
		MaxZoom:           0, // Use DefaultMaxZoomForText
		HAlign:            Left,
		segments:          nil,
	}
}

func (t *TextWriter) Printf(screen *ebiten.Image, x, y int, format string, a ...interface{}) {
	t.Print(screen, x, y, fmt.Sprintf(format, a...))
}

func (t *TextWriter) Print(screen *ebiten.Image, x, y int, msg string) {
	op := &text.DrawOptions{}
	op.ColorScale.ScaleWithColor(t.Color)
	op.GeoM.Translate(float64(x), float64(y))
	text.Draw(screen, msg, &text.GoTextFace{
		Source: t.Font,
		Size:   float64(t.Size),
	}, op)
}

func (t *TextWriter) WithSize(size int) *TextWriter {
	n := *t
	n.Size = size
	return &n
}

func (t *TextWriter) WithColor(color color.Color) *TextWriter {
	n := *t
	n.Color = color
	return &n
}

func (t *TextWriter) WithFont(font *text.GoTextFaceSource) *TextWriter {
	n := *t
	n.Font = font
	return &n
}

// WithScaling configures whether text scales with camera zoom.
// When enabled is false, text maintains fixed pixel size regardless of zoom.
// Optional maxZoom parameter sets the maximum zoom level for text visibility;
// text is hidden at zoom levels above this value. If not specified, uses DefaultMaxZoomForText.
func (t *TextWriter) WithScaling(enabled bool, maxZoom ...float64) *TextWriter {
	n := *t
	n.Scaling = enabled
	n.MaxZoom = 0 // Use default
	if len(maxZoom) > 0 {
		n.MaxZoom = maxZoom[0]
	}

	if n.MaxZoom != 0 && n.Scaling {
		panic("maxZoom parameter is only valid when scaling is disabled")
	}

	return &n
}

// WithHAlignment sets the horizontal alignment of the text.
func (t *TextWriter) WithHAlignment(align HAlignment) *TextWriter {
	n := *t
	n.HAlign = align
	return &n
}

// WithBackground sets an optional background color for the text.
// The background is rendered as a rectangle behind the text with padding.
func (t *TextWriter) WithBackground(bg color.Color) *TextWriter {
	n := *t
	n.Background = &bg
	return &n
}

// WithBackgroundPadding sets the padding around the background rectangle.
// Default padding is 2 pixels if not specified.
func (t *TextWriter) WithBackgroundPadding(padding int) *TextWriter {
	n := *t
	n.BackgroundPadding = padding
	return &n
}

// Text adds a text segment with the current color to the builder.
// This allows building multi-segment text where each segment can have different styling.
func (t *TextWriter) Text(msg string) *TextWriter {
	n := *t
	n.segments = append([]textSegment(nil), t.segments...)
	n.segments = append(n.segments, textSegment{text: msg, color: t.Color})
	return &n
}

// ColoredText adds a text segment with a specific color to the builder.
// This allows creating multi-colored text without string templating overhead.
func (t *TextWriter) ColoredText(msg string, c color.Color) *TextWriter {
	n := *t
	n.segments = append([]textSegment(nil), t.segments...)
	n.segments = append(n.segments, textSegment{text: msg, color: c})
	return &n
}

// Clear removes all built text segments, allowing the builder to be reused
// with different text content while keeping the same styling configuration.
func (t *TextWriter) Clear() *TextWriter {
	n := *t
	n.segments = nil
	return &n
}

// RenderedHeight returns the pixel height of the text as it would be drawn
// without scaling. Scaling-enabled TextWriters multiply this by camera zoom
// at draw time; this method does not apply that multiplier.
func (t *TextWriter) RenderedHeight() float64 {
	face := &text.GoTextFace{Source: t.Font, Size: float64(t.Size)}
	metrics := face.Metrics()
	return metrics.HAscent + metrics.HDescent
}

// Draw renders the built text segments to the screen at the specified position.
// The position is in world coordinates, which are converted to screen coordinates
// using the camera's WorldToScreen transformation.
//
// When scaling is disabled (WithScaling(false)), text maintains fixed pixel size
// and is hidden at zoom levels above the configured maximum zoom threshold.
//
// The text segments are rendered with the configured alignment and optional background.
func (t *TextWriter) Draw(screen *ebiten.Image, camera *Camera, position geometry.Vector2) {
	// Check zoom threshold if scaling is disabled
	if !t.Scaling {
		maxZoom := t.MaxZoom
		if maxZoom == 0 {
			maxZoom = DefaultMaxZoomForText
		}
		if camera.Zoom() > maxZoom {
			return // Hide text at extreme zoom levels
		}
	}

	// Convert world position to screen position using camera
	screenPos := camera.WorldToScreen(position)

	// Calculate effective text size
	size := t.Size
	if t.Scaling {
		size = int(float64(t.Size) * camera.Zoom())
	}

	// If no segments were built, nothing to render
	if len(t.segments) == 0 {
		return
	}

	// Measure total text width for alignment
	totalWidth := 0.0
	face := &text.GoTextFace{Source: t.Font, Size: float64(size)}
	for _, seg := range t.segments {
		advance := text.Advance(seg.text, face)
		totalWidth += advance
	}

	// Calculate text height for background
	metrics := face.Metrics()
	textHeight := metrics.HAscent + metrics.HDescent

	// Calculate starting X based on alignment
	startX := screenPos.X()
	switch t.HAlign {
	case Center:
		startX -= totalWidth / 2
	case Right:
		startX -= totalWidth
		// Left is default (no adjustment)
	}

	// Render background if set
	if t.Background != nil {
		padding := float64(t.BackgroundPadding)
		if padding == 0 {
			padding = 2.0 // Default padding
		}

		// Create or reuse cached background image
		bgWidth := int(totalWidth + padding*2)
		bgHeight := int(textHeight + padding*2)
		if t.cachedBgImage == nil || t.cachedBgW != bgWidth || t.cachedBgH != bgHeight {
			t.cachedBgImage = ebiten.NewImage(bgWidth, bgHeight)
			t.cachedBgW = bgWidth
			t.cachedBgH = bgHeight
			t.cachedBgImage.Fill(*t.Background)
		}
		bgImage := t.cachedBgImage

		// Draw background centered around the text
		// text.Draw positions text with Y at the top of the bounding box,
		// so the background starts at padding above screenPos.Y()
		bgY := screenPos.Y() - padding
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(startX-padding, bgY)
		screen.DrawImage(bgImage, op)
	}

	// Render each segment
	currentX := startX
	for _, seg := range t.segments {
		op := &text.DrawOptions{}
		op.GeoM.Translate(currentX, screenPos.Y())
		op.ColorScale.ScaleWithColor(seg.color)
		text.Draw(screen, seg.text, face, op)

		// Advance X for next segment
		advance := text.Advance(seg.text, face)
		currentX += advance
	}
}
