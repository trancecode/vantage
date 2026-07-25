package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
)

// TestLoadSpriteEmptyIndexesDoesNotPanic guards against a regression where an
// animation type mapped to an empty index slice (e.g. a not-yet-populated,
// data-driven frame list) left no entry in sprite.Animations, causing
// LoadSprite to dereference a nil *Animation when setting the duration.
func TestLoadSpriteEmptyIndexesDoesNotPanic(t *testing.T) {
	img := ebiten.NewImage(4, 4)
	indexes := map[AnimationType][]int{
		AnimationIdleUp: {},
	}

	sprite, err := LoadSprite(img, 2, 2, indexes, nil)
	if err != nil {
		t.Fatalf("LoadSprite returned error: %v", err)
	}
	if sprite.HasAnimation(AnimationIdleUp) {
		t.Fatalf("expected no animation entry for an empty index list")
	}
}

// drawOpTestCamera returns a camera with a deterministic, non-identity
// transform, so a geometry assertion exercises the camera step rather than
// passing on an identity matrix.
func drawOpTestCamera() *Camera {
	c := NewCamera(640, 480)
	c.SetZeroAsTopLeft()
	return c
}

// geoMEquals reports whether two draw options carry the same geometry matrix.
func geoMEquals(a, b *ebiten.DrawImageOptions) bool {
	for i := range 2 {
		for j := range 3 {
			if a.GeoM.Element(i, j) != b.GeoM.Element(i, j) {
				return false
			}
		}
	}
	return true
}

// TestDrawAnimationGeometryIsUnchangedByTheDisplayScale pins the draw geometry
// DrawAnimation produces against the transform it produced before the display
// scale existed. DrawAnimation delegates to DrawAnimationScaled with a display
// scale of 1, so the two share this matrix, and the assertion is that a scale
// of 1 is a no-op rather than a near-miss. The comparison is on the matrix
// rather than on drawn pixels because ebiten.Image pixels cannot be read back
// outside a running ebiten.RunGame loop, which this package's tests do not run.
func TestDrawAnimationGeometryIsUnchangedByTheDisplayScale(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	s := NewSprite().SetScale(2).SetZeroPosition(geometry.NewVector2(8, 24))

	for _, requiresFlip := range []bool{false, true} {
		// The pre-display-scale transform, rebuilt here independently.
		want := &ebiten.DrawImageOptions{}
		want.GeoM.Scale(s.Scale, s.Scale)
		want.GeoM.Translate(-s.ZeroPosition.X(), -s.ZeroPosition.Y())
		if requiresFlip {
			want.GeoM.Scale(-1, 1)
		}
		c.Adjust(want, p)

		got := s.buildDrawOp(p, requiresFlip, c, 1.0)
		if !geoMEquals(got, want) {
			t.Fatalf("flip=%v: buildDrawOp at display scale 1 = %v, want %v", requiresFlip, got.GeoM, want.GeoM)
		}
	}
}

// TestDisplayScaleShrinksAboutTheZeroPosition covers the interaction between
// the display scale and ZeroPosition: the scale is uniform about the zero
// position, so the anchored pixel lands on the same screen point at every
// display scale and only the drawn extent changes.
func TestDisplayScaleShrinksAboutTheZeroPosition(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	const scale = 2.0
	zero := geometry.NewVector2(8, 24)
	s := NewSprite().SetScale(scale).SetZeroPosition(zero)

	// ZeroPosition is expressed in post-Scale pixels, so the source pixel it
	// anchors is ZeroPosition divided by Scale.
	anchorSourceX, anchorSourceY := zero.X()/scale, zero.Y()/scale
	wantAnchor := c.WorldToScreen(p)

	// The frame's top-left corner, whose distance from the anchor is the drawn
	// extent the display scale is supposed to shrink.
	cornerX, cornerY := s.buildDrawOp(p, false, c, 1.0).GeoM.Apply(0, 0)

	const eps = 1e-9
	for _, displayScale := range []float64{1.0, 0.5, 0.25} {
		op := s.buildDrawOp(p, false, c, displayScale)

		gotX, gotY := op.GeoM.Apply(anchorSourceX, anchorSourceY)
		if diff := gotX - wantAnchor.X(); diff > eps || diff < -eps {
			t.Errorf("display scale %v: anchor X = %v, want %v", displayScale, gotX, wantAnchor.X())
		}
		if diff := gotY - wantAnchor.Y(); diff > eps || diff < -eps {
			t.Errorf("display scale %v: anchor Y = %v, want %v", displayScale, gotY, wantAnchor.Y())
		}

		gotCornerX, gotCornerY := op.GeoM.Apply(0, 0)
		wantCornerX := wantAnchor.X() + (cornerX-wantAnchor.X())*displayScale
		wantCornerY := wantAnchor.Y() + (cornerY-wantAnchor.Y())*displayScale
		if diff := gotCornerX - wantCornerX; diff > eps || diff < -eps {
			t.Errorf("display scale %v: corner X = %v, want %v", displayScale, gotCornerX, wantCornerX)
		}
		if diff := gotCornerY - wantCornerY; diff > eps || diff < -eps {
			t.Errorf("display scale %v: corner Y = %v, want %v", displayScale, gotCornerY, wantCornerY)
		}
	}
}

// TestDrawAnimationScaledDrawsEveryDisplayScale keeps the exported draw path
// covered end to end, including the mirrored-animation branch that
// DrawAnimationScaled resolves before building its draw options.
func TestDrawAnimationScaledDrawsEveryDisplayScale(t *testing.T) {
	c := drawOpTestCamera()
	screen := ebiten.NewImage(640, 480)
	s := NewSprite()
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))

	p := geometry.NewVector2(1, 1)
	s.DrawAnimation(screen, c, p, AnimationIdleRight, 100*time.Millisecond)
	for _, displayScale := range []float64{1.0, 0.5, 0.25} {
		// AnimationIdleLeft is not authored, so it is drawn by flipping
		// AnimationIdleRight.
		s.DrawAnimationScaled(screen, c, p, AnimationIdleLeft, 100*time.Millisecond, displayScale)
	}
}
