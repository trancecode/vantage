package pixeltest

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/visualtest"
)

// scenarioResult records one scenario's outcome for the test function to
// assert on after ebiten.RunGame returns. The game loop only records results;
// it never calls a *testing.T method, since a comparison mismatch is data for
// the test to report, not a reason for the render loop itself to fail.
type scenarioResult struct {
	name     string
	mismatch *visualtest.Mismatch
}

// comparisonGame runs every scenario's uniform-versus-cropped comparison once,
// in its first Draw call, then lets Update terminate the loop. Batching every
// scenario into this one game is required, not just convenient: Ebitengine
// permits only one ebiten.RunGame per process.
type comparisonGame struct {
	uniform, cropped *render.Sprite
	scenarios        []scenario
	results          []scenarioResult

	frame int
	done  bool
}

// Update counts frames and terminates the loop once Draw has produced a
// result for every scenario. The frame cap is a backstop: if Draw is somehow
// never called or never finishes, Update fails loudly instead of leaving
// ebiten.RunGame blocked forever.
func (g *comparisonGame) Update() error {
	g.frame++
	if g.done {
		return ebiten.Termination
	}
	if g.frame > frameCap {
		return fmt.Errorf("pixeltest: comparisons did not complete within %d frames", frameCap)
	}
	return nil
}

// Draw runs every scenario exactly once, on the first frame it is called:
// for each one, it draws the uniform and the auto-cropped sprite into their
// own offscreen image, reads both back, and records their comparison. screen
// itself is left untouched, since nothing here is meant to be looked at.
func (g *comparisonGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}

	// SpriteFilter is package-level engine configuration read at draw time;
	// scenarios drive it directly, so it must be restored once every scenario
	// has run rather than left on whichever value the last scenario used.
	originalFilter := render.SpriteFilter
	defer func() { render.SpriteFilter = originalFilter }()

	camera := render.NewScreenCamera(canvasSize, canvasSize)
	pos := geometry.Zero2D()

	for _, sc := range g.scenarios {
		render.SpriteFilter = sc.filter

		want := drawToImage(g.uniform, camera, pos, sc)
		got := drawToImage(g.cropped, camera, pos, sc)

		g.results = append(g.results, scenarioResult{
			name:     sc.name,
			mismatch: visualtest.CompareImages(want, got),
		})
	}

	g.done = true
}

// Layout reports the offscreen canvas size every scenario renders into.
func (g *comparisonGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return canvasSize, canvasSize
}

// drawToImage draws one scenario's animation from sprite onto a fresh
// canvasSize offscreen image and reads the result back as an *image.RGBA.
func drawToImage(sprite *render.Sprite, camera *render.Camera, pos geometry.Vector2, sc scenario) *image.RGBA {
	img := ebiten.NewImage(canvasSize, canvasSize)
	sprite.DrawAnimationScaled(img, camera, pos, sc.animation, sc.elapsed, displayScale)
	return readPixels(img)
}

// readPixels copies img's pixels into an *image.RGBA, the pattern
// app.SaveScreenshot and visualtest/capture's imageFromScreen both use to read
// a rendered ebiten.Image back to the CPU. ReadPixels reports alpha
// premultiplied RGBA bytes in row-major order, the same layout image.RGBA
// itself uses, so they become its Pix directly rather than through a
// pixel-by-pixel copy.
func readPixels(img *ebiten.Image) *image.RGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, w*h*4)
	img.ReadPixels(pixels)
	return &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
}
