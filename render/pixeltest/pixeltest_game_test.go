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

		var mismatch *visualtest.Mismatch
		if sc.masked {
			mismatch = compareMasked(g.cropped, camera, pos, sc, want, got)
		} else {
			mismatch = visualtest.CompareImages(want, got)
		}

		g.results = append(g.results, scenarioResult{
			name:     sc.name,
			mismatch: mismatch,
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
	sprite.DrawAnimationScaled(img, camera, pos, sc.animation, sc.elapsed, sc.scale)
	return readPixels(img)
}

// compareMasked compares want and got the way a masked scenario needs: it
// excludes the ring between the cropped frame's on-screen quad and that quad
// expanded by featherMargin (the band FilterLinear legitimately feathers past
// a tight crop but not past a padded cell, per buildScenarios), and requires
// an exact match everywhere else, both inside the quad and beyond the
// feather. That is stricter than only checking the quad's interior: it also
// pins down that the padded side's feather does not reach any further than
// expected.
func compareMasked(cropped *render.Sprite, camera *render.Camera, pos geometry.Vector2, sc scenario, want, got *image.RGBA) *visualtest.Mismatch {
	quad := croppedQuadOnScreen(cropped, camera, pos, sc.animation, sc.scale)
	expanded := quad.Inset(-featherMargin(sc.scale))

	wantMasked := cloneRGBA(want)
	gotMasked := cloneRGBA(got)
	blankAnnulus(wantMasked, expanded, quad)
	blankAnnulus(gotMasked, expanded, quad)

	return visualtest.CompareImages(wantMasked, gotMasked)
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
