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

// autoCropAsymmetricTestSheet builds a 2x2 grid of 16 pixel cells where cell 0
// carries a non-square 8x2 opaque block at a non-square, off-diagonal cell-local
// origin of (3,7). autoCropTestSheet's blocks all sit on the diagonal with equal
// width and height, so swapping X and Y anywhere in the crop or rebase math
// produces the same result and is invisible to tests built on it. This fixture's
// block tells X and Y apart in both its origin and its shape, so a transposed
// rebase lands on a different, wrong anchor.
func autoCropAsymmetricTestSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
			}
		}
	}
	fill(3, 7, 3+8, 7+2) // cell 0, cell-local (3,7)-(11,9)
	return img
}

// TestAutoCropAnchorRebaseResistsTransposition covers the anchor rebase at
// render_spriteautocrop.go's `anchor.Sub(geometry.NewVector2(box.Min.X,
// box.Min.Y))`. A fixture where a crop box's width equals its height and its
// origin sits on the diagonal cannot distinguish that rebase from one that
// swaps box.Min.X and box.Min.Y: the wrong computation reads back the same
// numbers. This fixture's box is 8x2 and starts off the diagonal at (3,7), so
// the two computations disagree, and the test would fail if they were swapped.
func TestAutoCropAnchorRebaseResistsTransposition(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropAsymmetricTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.NewVector2(20, 30))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	if got := down.Frames[0].Dx(); got != 8 {
		t.Fatalf("crop width = %d, want 8", got)
	}
	if got := down.Frames[0].Dy(); got != 2 {
		t.Fatalf("crop height = %d, want 2", got)
	}
	// The box starts at cell-local (3,7), so the correct rebase is
	// (20,30) - (3,7) = (17,23). A rebase that swapped X and Y would instead
	// subtract (7,3), giving (13,27): a different point, not just a
	// coincidentally equal one.
	if got, want := down.Anchor, geometry.NewVector2(17, 23); got != want {
		t.Fatalf("anchor = %v, want %v", got, want)
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
// change has to preserve: a given pixel of the source sheet lands on the same
// screen point whether the sprite was loaded uniformly or auto-cropped, so the
// repack is invisible in the game.
//
// This tracks a sheet pixel through both load paths rather than comparing an
// anchor to itself. In the uniform sprite, an animation's frame is the whole
// cell, so a pixel at cell-local (qx, qy) is at frame-local (qx, qy). In the
// cropped sprite, that same pixel is at frame-local (qx-originX, qy-originY),
// where origin is that animation's crop box top-left in cell-local
// coordinates. origin is hardcoded from autoCropTestSheet's fixture, not read
// back from autoCropAtlas, so a wrong rebase cannot cancel itself out of the
// comparison: cell 0's content is an 8x8 block at cell-local (4,4), cell 1's
// is a 4x4 block at cell-local (2,2).
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

	// origin is each animation's crop box top-left in cell-local coordinates.
	// probes are cell-local points inside that box: the top-left corner itself,
	// so an offset error shows up, and a second point elsewhere in the box, so
	// a scale error shows up too.
	cases := []struct {
		a       AnimationType
		originX float64
		originY float64
		probeX  []float64
		probeY  []float64
	}{
		{AnimationIdleDown, 4, 4, []float64{4, 11}, []float64{4, 11}},
		{AnimationIdleRight, 2, 2, []float64{2, 5}, []float64{2, 5}},
	}

	for _, tc := range cases {
		uniformOp := uniform.buildDrawOp(p, tc.a, false, c, 1.0)
		croppedOp := cropped.buildDrawOp(p, tc.a, false, c, 1.0)

		for i := range tc.probeX {
			qx, qy := tc.probeX[i], tc.probeY[i]
			wantX, wantY := uniformOp.GeoM.Apply(qx, qy)
			gotX, gotY := croppedOp.GeoM.Apply(qx-tc.originX, qy-tc.originY)

			if diff := gotX - wantX; diff > eps || diff < -eps {
				t.Errorf("animation %s: sheet pixel (%v,%v) X = %v, want %v", tc.a, qx, qy, gotX, wantX)
			}
			if diff := gotY - wantY; diff > eps || diff < -eps {
				t.Errorf("animation %s: sheet pixel (%v,%v) Y = %v, want %v", tc.a, qx, qy, gotY, wantY)
			}
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

// TestShelfPackLeavesAGutterBetweenFrames covers that no two placed frames ever
// touch, along a shelf or across shelves. Ebitengine's builtin linear-filter
// shader samples up to half a texel past a frame's edge without clamping to the
// sub-image (AddressUnsafe), so packing edge to edge would let a frame's linear
// sampling pick up a neighbouring animation's opaque pixels. Six 5x5 frames on a
// 13 pixel wide atlas force both a horizontal neighbor within a shelf and a
// second shelf directly below the first, so both adjacency directions are
// exercised.
func TestShelfPackLeavesAGutterBetweenFrames(t *testing.T) {
	frames := make([]placement, 6)
	for i := range frames {
		frames[i] = placement{source: image.Rect(0, 0, 5, 5)}
	}

	_, placed := shelfPack(frames)
	if len(placed) != len(frames) {
		t.Fatalf("shelfPack placed %d frames, want %d", len(placed), len(frames))
	}

	for i := range placed {
		for j := i + 1; j < len(placed); j++ {
			a, b := placed[i].dest, placed[j].dest
			// image.Rectangle.Overlaps alone would miss mere adjacency (a
			// shared edge or corner with no interior overlap), so inflate one
			// rectangle by a pixel on every side first: a touching pair then
			// overlaps, a properly gapped pair still does not.
			inflatedA := image.Rect(a.Min.X-1, a.Min.Y-1, a.Max.X+1, a.Max.Y+1)
			if inflatedA.Overlaps(b) {
				t.Fatalf("frame %d %v and frame %d %v are adjacent or overlapping", i, a, j, b)
			}
		}
	}
}

// BenchmarkAutoCropAtlas measures the scan and repack on a sheet shaped like the
// real ones: a large grid where most of each cell is transparent. The published
// sheets are 7296x10624 with a 38x64 grid, which is too large to allocate in a
// benchmark loop, so this uses the same cell size and sparsity at a tenth of the
// area and the result scales linearly with pixel count.
func BenchmarkAutoCropAtlas(b *testing.B) {
	const columns, rows, cell = 38, 6, 192
	src := image.NewRGBA(image.Rect(0, 0, columns*cell, rows*cell))
	// A block covering roughly 4% of each cell, matching the measured fill.
	for row := range rows {
		for col := range columns {
			x0 := col*cell + cell/2
			y0 := row*cell + cell/2
			for y := y0; y < y0+cell*2/10; y++ {
				for x := x0; x < x0+cell*2/10; x++ {
					src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
				}
			}
		}
	}
	indexes := map[AnimationType][]int{}
	for row := range rows {
		frames := make([]int, columns)
		for col := range columns {
			frames[col] = row*columns + col
		}
		indexes[AnimationGameBase+AnimationType(row)] = frames
	}

	b.ResetTimer()
	for b.Loop() {
		if _, _, err := autoCropAtlas(src, columns, rows, indexes, nil, geometry.Zero2D()); err != nil {
			b.Fatalf("autoCropAtlas returned error: %v", err)
		}
	}
}
