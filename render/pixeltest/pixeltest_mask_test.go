package pixeltest

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
)

// croppedQuadOnScreen computes the on-screen rectangle
// [render.Sprite.DrawAnimationScaled] draws animation a into, for the given
// camera, position and scale. It has to replicate Sprite.buildDrawOp's math,
// since that method is unexported, using only exported API: [render.Sprite.Anchor],
// [render.Sprite.TileRatio], the frame's own size from [render.Sprite.Image], and
// [render.Camera.Adjust]. Mirrors [render.Sprite.DrawAnimationScaled]'s own
// resolution of a mirrored animation type, since [render.Sprite.Image] (unlike
// Anchor) does not resolve one itself.
//
// This is what a comparison masks down to for FilterLinear scenarios that compare
// a padded frame against a genuinely smaller cropped one: see buildScenarios for
// why the region outside this quad is expected to differ and is not what those
// scenarios check.
func croppedQuadOnScreen(sprite *render.Sprite, camera *render.Camera, pos geometry.Vector2, a render.AnimationType, scale float64) image.Rectangle {
	drawFrom := a
	requiresFlip := false
	if !sprite.HasAnimation(a) {
		if mirror, ok := render.MirroredAnimations[a]; ok {
			drawFrom = mirror
			requiresFlip = true
		}
	}
	frame := sprite.Image(drawFrom).Bounds()
	w, h := float64(frame.Dx()), float64(frame.Dy())

	effectiveScale := sprite.TileRatio() * scale
	anchor := sprite.Anchor(a)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(effectiveScale, effectiveScale)
	op.GeoM.Translate(-anchor.X()*effectiveScale, -anchor.Y()*effectiveScale)
	if requiresFlip {
		op.GeoM.Scale(-1, 1)
	}
	camera.Adjust(op, pos)

	x0, y0 := op.GeoM.Apply(0, 0)
	x1, y1 := op.GeoM.Apply(w, h)
	return image.Rect(
		int(math.Round(min(x0, x1))), int(math.Round(min(y0, y1))),
		int(math.Round(max(x0, x1))), int(math.Round(max(y0, y1))),
	)
}

// featherMargin is how far outside its tight quad linear filtering can
// legitimately spread a cropped frame's content on the padded (uniform) side:
// half a source texel, scaled to screen pixels by scale and rounded up to a
// whole pixel. A difference further out than this is not the feather; it is a
// real mismatch.
func featherMargin(scale float64) int {
	return int(math.Ceil(0.5 * scale))
}

// cloneRGBA returns an independent copy of img, so blanking a region on the
// copy leaves the original untouched.
func cloneRGBA(img *image.RGBA) *image.RGBA {
	clone := &image.RGBA{
		Pix:    make([]byte, len(img.Pix)),
		Stride: img.Stride,
		Rect:   img.Rect,
	}
	copy(clone.Pix, img.Pix)
	return clone
}

// blankAnnulus zeroes every pixel of img that lies within outer but outside
// inner, both clamped to img's own bounds. Used to exclude the feather band
// (the ring between a cropped frame's tight quad and that quad expanded by
// featherMargin) from a comparison, while leaving the quad's own interior, and
// everything beyond the feather, intact and checked.
func blankAnnulus(img *image.RGBA, outer, inner image.Rectangle) {
	outer = outer.Intersect(img.Bounds())
	for y := outer.Min.Y; y < outer.Max.Y; y++ {
		for x := outer.Min.X; x < outer.Max.X; x++ {
			if (image.Point{X: x, Y: y}).In(inner) {
				continue
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 0, 0, 0
		}
	}
}
