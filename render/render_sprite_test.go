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
	zero := geometry.NewVector2(8, 24)
	s := NewSprite()
	s.AddImage(AnimationDefault, ebiten.NewImage(16, 16))
	s.SetZeroPosition(zero)

	for _, requiresFlip := range []bool{false, true} {
		// The pre-display-scale transform, rebuilt here independently.
		want := &ebiten.DrawImageOptions{}
		want.GeoM.Translate(-zero.X(), -zero.Y())
		if requiresFlip {
			want.GeoM.Scale(-1, 1)
		}
		c.Adjust(want, p)

		got := s.buildDrawOp(p, AnimationDefault, requiresFlip, c, 1.0)
		if !geoMEquals(got, want) {
			t.Fatalf("flip=%v: buildDrawOp at display scale 1 = %v, want %v", requiresFlip, got.GeoM, want.GeoM)
		}
	}
}

// TestDisplayScaleShrinksAboutTheZeroPosition covers the interaction between
// the display scale and ZeroPosition: the scale is uniform about the zero
// position, so the anchored pixel lands on the same screen point at every
// display scale and only the drawn extent changes.
//
// The second case adds a source tile size on top, so the tile ratio, a display
// scale other than 1 and a non-zero ZeroPosition are all exercised together.
// The anchored source pixel is ZeroPosition itself whatever the ratio is,
// because ratio and display scale multiply into both the scale and the anchor
// offset: that is the invariant, and asserting where the pixel lands catches
// the two being confused in a way that recomputing the expected GeoM from the
// same expression cannot.
func TestDisplayScaleShrinksAboutTheZeroPosition(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	zero := geometry.NewVector2(8, 24)

	newAnchoredSprite := func() *Sprite {
		s := NewSprite()
		s.AddImage(AnimationDefault, ebiten.NewImage(16, 16))
		return s
	}

	for _, tc := range []struct {
		name   string
		sprite *Sprite
	}{
		{
			"no source tile size",
			newAnchoredSprite().SetZeroPosition(zero),
		},
		{
			"source tile size 64 at tile size 32, ratio 0.5",
			newAnchoredSprite().SetZeroPosition(zero).SetSourceTileSize(64),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := drawOpTestCamera()
			p := geometry.NewVector2(3, 5)
			s := tc.sprite

			// ZeroPosition is in source pixels, so it is the anchored pixel.
			anchorSourceX, anchorSourceY := zero.X(), zero.Y()
			wantAnchor := c.WorldToScreen(p)

			// The frame's top-left corner, whose distance from the anchor is
			// the drawn extent the display scale is supposed to shrink.
			cornerX, cornerY := s.buildDrawOp(p, AnimationDefault, false, c, 1.0).GeoM.Apply(0, 0)

			const eps = 1e-9
			for _, displayScale := range []float64{1.0, 0.5, 0.25} {
				op := s.buildDrawOp(p, AnimationDefault, false, c, displayScale)

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
		})
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

// TestTileRatioIsOneWithoutASourceTileSize covers the compatibility
// guarantee: a sprite that never opts into a source tile size sees no
// correction, whatever nonsensical value SourceTileSize might hold.
func TestTileRatioIsOneWithoutASourceTileSize(t *testing.T) {
	for _, source := range []float64{0, -1, -64} {
		s := NewSprite()
		s.SourceTileSize = source
		if got := s.TileRatio(); got != 1.0 {
			t.Fatalf("TileRatio with SourceTileSize %v = %v, want 1", source, got)
		}
	}
}

// TestTileRatioScalesArtToTheGameTileSize covers the core mapping: art drawn
// for a source tile size is scaled onto the game's current TileSize.
func TestTileRatioScalesArtToTheGameTileSize(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	TileSize = 32
	for _, tc := range []struct {
		source float64
		want   float64
	}{
		{32, 1.0}, // drawn for the game's own tile size
		{64, 0.5}, // finer art, drawn down
		{16, 2.0}, // coarser art, drawn up
	} {
		s := NewSprite().SetSourceTileSize(tc.source)
		if got := s.TileRatio(); got != tc.want {
			t.Fatalf("TileRatio for source %v at TileSize 32 = %v, want %v", tc.source, got, tc.want)
		}
	}
}

// TestSetSourceTileSizeChains covers that SetSourceTileSize both sets the
// field and returns the sprite, like the other chainable setters.
func TestSetSourceTileSizeChains(t *testing.T) {
	s := NewSprite()
	if got := s.SetSourceTileSize(64); got != s {
		t.Fatal("SetSourceTileSize did not return the sprite")
	}
	if s.SourceTileSize != 64 {
		t.Fatalf("SourceTileSize = %v, want 64", s.SourceTileSize)
	}
}

// TestBuildDrawOpUnchangedWithoutASourceTileSize is the compatibility
// guarantee: a sprite that does not opt in draws exactly as it did before
// source tile sizes existed.
func TestBuildDrawOpUnchangedWithoutASourceTileSize(t *testing.T) {
	c := drawOpTestCamera()
	s := NewSprite()
	s.AddImage(AnimationDefault, ebiten.NewImage(16, 16))
	s.SetZeroPosition(geometry.NewVector2(8, 24))

	got := s.buildDrawOp(geometry.NewVector2(3, 4), AnimationDefault, false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Translate(-8, -24)
	c.Adjust(want, geometry.NewVector2(3, 4))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

// TestBuildDrawOpAppliesTheTileRatio covers that the ratio alone, with no
// sprite scale, shrinks the draw as buildDrawOp's doc comment describes.
func TestBuildDrawOpAppliesTheTileRatio(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := drawOpTestCamera()
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), AnimationDefault, false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

// TestBuildDrawOpComposesRatioAndDisplayScale covers that the tile ratio and the
// display scale multiply into one uniform scale rather than only one of them
// taking effect.
func TestBuildDrawOpComposesRatioAndDisplayScale(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := drawOpTestCamera()
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), AnimationDefault, false, c, 2.0)

	// 0.5 * 2 = 1
	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(1, 1)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

// TestTileRatioScalesTheAnchor covers that the anchored point lands on the
// same world position whatever the ratio, so the translate offset scales with
// it exactly as it does with displayScale.
func TestTileRatioScalesTheAnchor(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := drawOpTestCamera()
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), AnimationDefault, false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	want.GeoM.Translate(-0*0.5, -0*0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}

	// With a real anchor, the translate is the anchor times the ratio.
	s = NewSprite().SetSourceTileSize(64)
	s.AddImage(AnimationDefault, ebiten.NewImage(16, 16))
	s.SetZeroPosition(geometry.NewVector2(10, 20))
	got = s.buildDrawOp(geometry.NewVector2(0, 0), AnimationDefault, false, c, 1.0)

	want = &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	want.GeoM.Translate(-10*0.5, -20*0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if !geoMEquals(got, want) {
		t.Fatalf("anchored GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

// TestVisibleTopAboveZeroScalesWithTheTileRatio covers that the tile ratio is
// applied on every return path from the cache, so a tile size change after
// the sprite's visible extent was first measured is still picked up.
//
// Seeded rather than measured: VisibleTopAboveZero scans pixels with img.At,
// which cannot run outside an ebiten.RunGame loop. The cache holds the
// pre-ratio value, so seeding it is exactly what a prior measurement would
// have left behind.
func TestVisibleTopAboveZeroScalesWithTheTileRatio(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	cached := 12.0
	s := NewSprite()
	s.cachedVisibleTopAboveZero = &cached

	if got := s.VisibleTopAboveZero(); got != cached {
		t.Fatalf("VisibleTopAboveZero = %v without a source tile size, want %v", got, cached)
	}

	s.SetSourceTileSize(32)
	TileSize = 16 // ratio 0.5

	if got, want := s.VisibleTopAboveZero(), cached/2; got != want {
		t.Fatalf("VisibleTopAboveZero = %v at ratio 0.5, want %v", got, want)
	}
}

// TestAnchorIsPerAnimation covers the core of per-animation geometry: two
// animations on one sprite carry different anchors, and each is used for its
// own animation rather than one of them winning for both.
func TestAnchorIsPerAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))
	s.Animations[AnimationIdleDown].ZeroPosition = geometry.NewVector2(4, 8)
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(10, 2)

	if got, want := s.Anchor(AnimationIdleDown), geometry.NewVector2(4, 8); got != want {
		t.Fatalf("Anchor(IdleDown) = %v, want %v", got, want)
	}
	if got, want := s.Anchor(AnimationIdleRight), geometry.NewVector2(10, 2); got != want {
		t.Fatalf("Anchor(IdleRight) = %v, want %v", got, want)
	}
}

// TestAnchorResolvesAMirroredAnimation covers that a left-facing animation drawn
// by flipping its right-facing mirror uses the mirror's anchor, since those are
// the frames on screen.
func TestAnchorResolvesAMirroredAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(10, 2)

	if got, want := s.Anchor(AnimationIdleLeft), geometry.NewVector2(10, 2); got != want {
		t.Fatalf("Anchor(IdleLeft) = %v, want %v", got, want)
	}
}

// TestAnchorOfAnUnknownAnimationIsZero covers that Anchor is a query and returns
// the zero vector rather than panicking, unlike the draw path.
func TestAnchorOfAnUnknownAnimationIsZero(t *testing.T) {
	if got := NewSprite().Anchor(AnimationIdleDown); !got.IsZero() {
		t.Fatalf("Anchor of an unknown animation = %v, want the zero vector", got)
	}
}

// TestSetZeroPositionWritesEveryAnimation covers the uniform-sheet convenience:
// one call sets the anchor on all animations currently on the sprite.
func TestSetZeroPositionWritesEveryAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))

	zero := geometry.NewVector2(8, 24)
	if got := s.SetZeroPosition(zero); got != s {
		t.Fatal("SetZeroPosition did not return the sprite")
	}
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		if got := s.Anchor(a); got != zero {
			t.Fatalf("Anchor(%v) = %v, want %v", a, got, zero)
		}
	}
}

// TestSetZeroPositionPanicsWithoutAnimations covers the ordering hazard left by
// deleting the sprite-level anchor: a misordered call is loud rather than
// silently writing the anchor to nothing.
func TestSetZeroPositionPanicsWithoutAnimations(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("SetZeroPosition on a sprite with no animations did not panic")
		}
	}()
	NewSprite().SetZeroPosition(geometry.NewVector2(1, 2))
}

// TestBuildDrawOpUsesTheAnimationAnchor covers that the draw path reads the
// anchor of the animation being drawn. Two animations with different anchors,
// drawn at one world position, must both land their own anchor pixel on that
// position: this is the regression that would otherwise appear in game as a
// character bobbing between the ground and mid-air as its animation changes.
func TestBuildDrawOpUsesTheAnimationAnchor(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)

	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(64, 64))
	s.Animations[AnimationIdleDown].ZeroPosition = geometry.NewVector2(8, 16)
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(32, 60)

	want := c.WorldToScreen(p)
	const eps = 1e-9
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		anchor := s.Anchor(a)
		op := s.buildDrawOp(p, a, false, c, 1.0)
		gotX, gotY := op.GeoM.Apply(anchor.X(), anchor.Y())
		if diff := gotX - want.X(); diff > eps || diff < -eps {
			t.Errorf("animation %v: anchor X = %v, want %v", a, gotX, want.X())
		}
		if diff := gotY - want.Y(); diff > eps || diff < -eps {
			t.Errorf("animation %v: anchor Y = %v, want %v", a, gotY, want.Y())
		}
	}
}
