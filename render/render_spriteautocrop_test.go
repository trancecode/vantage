package render

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
)

// autoCropTestSheet builds a 2x2 grid of 16 pixel cells where cell 0 has an 8x8
// opaque block at (4,4), cell 1 has a 4x4 block at (2,2), cell 2 is entirely
// transparent, and cell 3 is untouched by any animation.
func autoCropTestSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
			}
		}
	}
	fill(4, 4, 12, 12)     // cell 0, cell-local (4,4)-(12,12)
	fill(16+2, 2, 16+6, 6) // cell 1, cell-local (2,2)-(6,6)
	// cell 2 at (0,16) stays transparent; cell 3 at (16,16) is never referenced.
	return img
}

// TestAutoCropTightensEachAnimation covers the core measurement: each animation
// gets a crop box around its own content, and its anchor is rebased into that
// box so the drawn result is unchanged.
func TestAutoCropTightensEachAnimation(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	if got := down.Frames[0].Dx(); got != 8 {
		t.Fatalf("IdleDown crop width = %d, want 8", got)
	}
	if got := down.Frames[0].Dy(); got != 8 {
		t.Fatalf("IdleDown crop height = %d, want 8", got)
	}
	// The box starts at cell-local (4,4), so the anchor moves by that much.
	if got, want := down.Anchor, geometry.NewVector2(4, 12); got != want {
		t.Fatalf("IdleDown anchor = %v, want %v", got, want)
	}

	right := specs[AnimationIdleRight]
	if got := right.Frames[0].Dx(); got != 4 {
		t.Fatalf("IdleRight crop width = %d, want 4", got)
	}
	if got, want := right.Anchor, geometry.NewVector2(6, 14); got != want {
		t.Fatalf("IdleRight anchor = %v, want %v", got, want)
	}

	// The packed atlas is smaller than the source it came from.
	srcArea := 32 * 32
	atlasArea := atlas.Bounds().Dx() * atlas.Bounds().Dy()
	if atlasArea >= srcArea {
		t.Fatalf("atlas area %d is not smaller than the source area %d", atlasArea, srcArea)
	}
}

// TestAutoCropCopiesThePixels covers that the crop is a real copy into a new
// image rather than a narrower view: the packed frame must carry the content.
func TestAutoCropCopiesThePixels(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	rect := specs[AnimationIdleDown].Frames[0]
	// Every pixel of a tight crop around a solid block is opaque.
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if _, _, _, a := atlas.At(x, y).RGBA(); a == 0 {
				t.Fatalf("packed pixel at (%d,%d) is transparent", x, y)
			}
		}
	}
}

// TestAutoCropIsReproducible covers that the atlas does not depend on Go's map
// iteration order, which it would if animations were packed as they were ranged.
func TestAutoCropIsReproducible(t *testing.T) {
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}
	first, firstSpecs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, indexes, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	for range 8 {
		next, nextSpecs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, indexes, nil, geometry.Zero2D())
		if err != nil {
			t.Fatalf("autoCropAtlas returned error: %v", err)
		}
		if next.Bounds() != first.Bounds() {
			t.Fatalf("atlas bounds = %v, want %v", next.Bounds(), first.Bounds())
		}
		for a, spec := range nextSpecs {
			if spec.Frames[0] != firstSpecs[a].Frames[0] {
				t.Fatalf("animation %s frame = %v, want %v", a, spec.Frames[0], firstSpecs[a].Frames[0])
			}
		}
		if string(next.Pix) != string(first.Pix) {
			t.Fatal("atlas pixels differ between runs")
		}
	}
}

// TestAutoCropFallsBackForAnAllTransparentAnimation covers the pathological
// case: an animation with nothing drawn keeps its full cell rather than
// producing a degenerate box or an error.
func TestAutoCropFallsBackForAnAllTransparentAnimation(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {2},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	if got := specs[AnimationIdleDown].Frames[0].Dx(); got != 16 {
		t.Fatalf("all-transparent crop width = %d, want the full cell width 16", got)
	}
}

// TestAutoCropCarriesDurations covers that durations survive the repack, with
// the same one-second default the other loaders use.
func TestAutoCropCarriesDurations(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2,
		map[AnimationType][]int{AnimationIdleDown: {0}, AnimationIdleRight: {1}},
		map[AnimationType]time.Duration{AnimationIdleDown: 250 * time.Millisecond},
		geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	if got, want := specs[AnimationIdleDown].Duration, 250*time.Millisecond; got != want {
		t.Fatalf("IdleDown duration = %v, want %v", got, want)
	}
	if got := specs[AnimationIdleRight].Duration; got != 0 {
		t.Fatalf("IdleRight duration = %v, want 0 so the loader defaults it", got)
	}
}

// TestAutoCropRejectsABadGrid covers that a grid that cannot describe the image
// is named rather than producing a zero-sized cell.
func TestAutoCropRejectsABadGrid(t *testing.T) {
	for _, tc := range []struct{ columns, rows int }{{0, 2}, {2, 0}, {64, 2}} {
		if _, _, err := autoCropAtlas(autoCropTestSheet(), tc.columns, tc.rows,
			map[AnimationType][]int{AnimationIdleDown: {0}}, nil, geometry.Zero2D()); err == nil {
			t.Fatalf("autoCropAtlas accepted a %dx%d grid", tc.columns, tc.rows)
		}
	}
}

// TestAutoCropUnionsFramesOfOneAnimation covers the central semantic of this
// task: an animation's box is the union across all of its frames, not any one
// frame's own box. Frame 0's content sits at cell-local (4,4)-(12,12) and frame
// 1's sits at cell-local (2,2)-(6,6), so the union is (2,2)-(12,12): both frames
// must come out at that shared 10x10 size, and the anchor rebases against the
// union's origin rather than either frame's own.
func TestAutoCropUnionsFramesOfOneAnimation(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	for i, frame := range down.Frames {
		if got := frame.Dx(); got != 10 {
			t.Fatalf("frame %d width = %d, want the union width 10", i, got)
		}
		if got := frame.Dy(); got != 10 {
			t.Fatalf("frame %d height = %d, want the union height 10", i, got)
		}
	}
	// The union starts at cell-local (2,2), so the anchor moves by that much,
	// not by frame 0's own (4,4) origin.
	if got, want := down.Anchor, geometry.NewVector2(6, 14); got != want {
		t.Fatalf("IdleDown anchor = %v, want %v", got, want)
	}
}

// autoCropColorTestSheet builds a 2x2 grid of 16 pixel cells where cell 0 and
// cell 1 each carry an 8x8 block at the same cell-local offset (4,4), but in
// different colors: cell 0 is red, cell 1 is green. Same-sized, differently
// colored content lets a test tell whether the wrong animation's pixels landed
// in a rectangle, which same-colored fixtures cannot.
func autoCropColorTestSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
	fill(4, 4, 12, 12, color.RGBA{R: 255, A: 255})       // cell 0, red block at cell-local (4,4)-(12,12)
	fill(16+4, 4, 16+12, 12, color.RGBA{G: 255, A: 255}) // cell 1, green block at the same cell-local offset
	return img
}

// TestAutoCropDoesNotSwapAnimationsPixels covers the exact failure mode of the
// two sorted traversals diverging: if the placement built for one animation
// were matched to another, the copied pixels would still be correctly sized
// and non-transparent but would be the wrong animation's content. Same-sized
// but differently colored crop boxes catch that, where same-colored fixtures
// cannot.
func TestAutoCropDoesNotSwapAnimationsPixels(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropColorTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	downMin := specs[AnimationIdleDown].Frames[0].Min
	if r, g, b, _ := atlas.At(downMin.X, downMin.Y).RGBA(); r == 0 || g != 0 || b != 0 {
		t.Fatalf("IdleDown pixel at %v is not red: r=%d g=%d b=%d", downMin, r, g, b)
	}
	rightMin := specs[AnimationIdleRight].Frames[0].Min
	if r, g, b, _ := atlas.At(rightMin.X, rightMin.Y).RGBA(); g == 0 || r != 0 || b != 0 {
		t.Fatalf("IdleRight pixel at %v is not green: r=%d g=%d b=%d", rightMin, r, g, b)
	}
}

// TestAutoCroppedDrawsWhereTheUncroppedSpriteWould is the property the whole
// change has to preserve. The same sheet loaded uniformly and auto-cropped must
// put the anchor pixel on the same screen point for every animation, so the
// repack is invisible in the game.
func TestAutoCroppedDrawsWhereTheUncroppedSpriteWould(t *testing.T) {
	sheet := autoCropTestSheet()
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}
	anchor := geometry.NewVector2(8, 16)

	uniform, err := LoadSprite(ebiten.NewImageFromImage(sheet), 2, 2, indexes, nil)
	if err != nil {
		t.Fatalf("LoadSprite returned error: %v", err)
	}
	uniform.SetZeroPosition(anchor)

	cropped, err := LoadSpriteAutoCropped(sheet, 2, 2, indexes, nil, anchor)
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}

	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	const eps = 1e-9
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		wantAnchor := uniform.Anchor(a)
		wantOp := uniform.buildDrawOp(p, a, false, c, 1.0)
		wantX, wantY := wantOp.GeoM.Apply(wantAnchor.X(), wantAnchor.Y())

		gotAnchor := cropped.Anchor(a)
		gotOp := cropped.buildDrawOp(p, a, false, c, 1.0)
		gotX, gotY := gotOp.GeoM.Apply(gotAnchor.X(), gotAnchor.Y())

		if diff := gotX - wantX; diff > eps || diff < -eps {
			t.Errorf("animation %s: anchor X = %v, want %v", a, gotX, wantX)
		}
		if diff := gotY - wantY; diff > eps || diff < -eps {
			t.Errorf("animation %s: anchor Y = %v, want %v", a, gotY, wantY)
		}
	}
}

// TestLoadSpriteAutoCroppedShrinksTheFrames covers that the loaded sprite really
// carries the tight frames rather than the padded cells.
func TestLoadSpriteAutoCroppedShrinksTheFrames(t *testing.T) {
	s, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}
	if got := s.Animations[AnimationIdleDown].Images[0].Bounds().Dx(); got != 8 {
		t.Fatalf("frame width = %d, want the cropped 8 rather than the 16 pixel cell", got)
	}
}
