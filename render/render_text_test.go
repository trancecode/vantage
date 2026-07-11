package render

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/geometry"
)

// drawOffscreen renders the writer once so its background cache is populated.
// Pixels cannot be read back outside Ebiten's game loop, so the tests below
// assert on the cache fields the background image is rebuilt from.
func drawOffscreen(writer *TextWriter, camera *Camera) {
	writer.Draw(ebiten.NewImage(200, 200), camera, geometry.Zero2D())
}

func TestDeriveWithNewBackgroundColorRebuildsCachedImage(t *testing.T) {
	var (
		black = color.RGBA{A: 255}
		red   = color.RGBA{R: 255, A: 255}
	)
	camera := NewScreenCamera(200, 200)

	base := TextDefault.WithBackground(black).Text("Hi")
	drawOffscreen(base, camera)

	// Deriving after a draw copies the populated cache. The text and padding are
	// unchanged, so the cached dimensions still match and only the color differs.
	warning := base.WithBackground(red)
	drawOffscreen(warning, camera)

	if warning.cachedBgImage == base.cachedBgImage {
		t.Error("derived writer reused the cached background image built for a different color")
	}
	if warning.cachedBgColor != red {
		t.Errorf("cachedBgColor = %v, expected red %v", warning.cachedBgColor, red)
	}
	if base.cachedBgColor != black {
		t.Errorf("deriving mutated the source writer's cachedBgColor = %v, expected black %v", base.cachedBgColor, black)
	}
}

func TestDrawReusesCachedBackgroundWhenColorAndSizeUnchanged(t *testing.T) {
	blue := color.RGBA{B: 255, A: 255}
	camera := NewScreenCamera(200, 200)

	writer := TextDefault.WithBackground(blue).Text("Hi")
	drawOffscreen(writer, camera)
	cached := writer.cachedBgImage

	drawOffscreen(writer, camera)

	if writer.cachedBgImage != cached {
		t.Error("cached background image was rebuilt even though color and size were unchanged")
	}
}

func TestBackgroundColorSetOnExportedFieldRebuildsCachedImage(t *testing.T) {
	var (
		black                    = color.RGBA{A: 255}
		green                    = color.RGBA{G: 255, A: 255}
		greenAsColor color.Color = green
	)
	camera := NewScreenCamera(200, 200)

	writer := TextDefault.WithBackground(black).Text("Hi")
	drawOffscreen(writer, camera)
	cached := writer.cachedBgImage

	// Background is an exported field, so callers can bypass WithBackground entirely.
	writer.Background = &greenAsColor
	drawOffscreen(writer, camera)

	if writer.cachedBgImage == cached {
		t.Error("assigning Background directly reused the cached background image built for the old color")
	}
	if writer.cachedBgColor != green {
		t.Errorf("cachedBgColor = %v, expected green %v", writer.cachedBgColor, green)
	}
}
