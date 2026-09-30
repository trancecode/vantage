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

// TestAutoCropTightensEachFrame covers the core measurement: each frame gets a
// crop box around its own content and records the box's corner as its offset,
// while the animation keeps the sheet-wide anchor, so the drawn result is
// unchanged.
func TestAutoCropTightensEachFrame(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	for i, want := range []struct {
		width, height int
		offset        image.Point
	}{
		{8, 8, image.Pt(4, 4)},
		{4, 4, image.Pt(2, 2)},
	} {
		frame := down.Frames[i]
		if frame.Rect.Dx() != want.width || frame.Rect.Dy() != want.height {
			t.Fatalf("frame %d size = %dx%d, want %dx%d", i, frame.Rect.Dx(), frame.Rect.Dy(), want.width, want.height)
		}
		if frame.Offset != want.offset {
			t.Fatalf("frame %d offset = %v, want %v", i, frame.Offset, want.offset)
		}
	}
	if got, want := down.Anchor, geometry.NewVector2(8, 16); got != want {
		t.Fatalf("anchor = %v, want the sheet anchor %v", got, want)
	}

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

	rect := specs[AnimationIdleDown].Frames[0].Rect
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
			for i := range spec.Frames {
				if spec.Frames[i] != firstSpecs[a].Frames[i] {
					t.Fatalf("animation %s frame %d = %v, want %v", a, i, spec.Frames[i], firstSpecs[a].Frames[i])
				}
			}
		}
		if string(next.Pix) != string(first.Pix) {
			t.Fatal("atlas pixels differ between runs")
		}
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

// TestAutoCropOffsetResistsTransposition covers the offset recorded for a crop
// box. A box whose width equals its height and whose corner sits on the
// diagonal cannot tell an offset from one with X and Y swapped. This fixture's
// box is 8x2 and starts off the diagonal at (3,7), so a transposed offset
// reads back as (7,3) and fails.
func TestAutoCropOffsetResistsTransposition(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropAsymmetricTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.NewVector2(20, 30))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	frame := specs[AnimationIdleDown].Frames[0]
	if frame.Rect.Dx() != 8 || frame.Rect.Dy() != 2 {
		t.Fatalf("crop size = %dx%d, want 8x2", frame.Rect.Dx(), frame.Rect.Dy())
	}
	if got, want := frame.Offset, image.Pt(3, 7); got != want {
		t.Fatalf("offset = %v, want %v", got, want)
	}
	if got, want := specs[AnimationIdleDown].Anchor, geometry.NewVector2(20, 30); got != want {
		t.Fatalf("anchor = %v, want the sheet anchor %v", got, want)
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

	downMin := specs[AnimationIdleDown].Frames[0].Rect.Min
	if r, g, b, _ := atlas.At(downMin.X, downMin.Y).RGBA(); r == 0 || g != 0 || b != 0 {
		t.Fatalf("IdleDown pixel at %v is not red: r=%d g=%d b=%d", downMin, r, g, b)
	}
	rightMin := specs[AnimationIdleRight].Frames[0].Rect.Min
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
// anchor to itself. In the uniform sprite a frame is the whole cell, so a pixel
// at cell-local (qx, qy) is at frame-local (qx, qy). In the cropped sprite the
// same pixel is at frame-local (qx-originX, qy-originY), where origin is that
// frame's crop box corner in the cell. origin is hardcoded from
// autoCropTestSheet's fixture rather than read back from autoCropAtlas, so a
// wrong offset cannot cancel itself out: cell 0's content is an 8x8 block at
// cell-local (4,4), cell 1's a 4x4 block at cell-local (2,2).
func TestAutoCroppedDrawsWhereTheUncroppedSpriteWould(t *testing.T) {
	sheet := autoCropTestSheet()
	// IdleDown's two frames crop to different boxes; IdleLeft is drawn by
	// flipping IdleRight.
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0, 1},
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

	// probes are cell-local points inside the frame's box: its corner, so an
	// offset error shows up, and a second point, so a scale error does too.
	cases := []struct {
		a            AnimationType
		frame        int
		requiresFlip bool
		origin       image.Point
		probes       []image.Point
	}{
		{AnimationIdleDown, 0, false, image.Pt(4, 4), []image.Point{{4, 4}, {11, 11}}},
		{AnimationIdleDown, 1, false, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
		{AnimationIdleRight, 0, false, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
		{AnimationIdleLeft, 0, true, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
	}

	for _, tc := range cases {
		source, _ := cropped.resolveAnimation(tc.a)
		offset := source.Frames[tc.frame].Offset
		uniformOp := uniform.buildDrawOp(p, tc.a, image.Point{}, tc.requiresFlip, c, 1.0)
		croppedOp := cropped.buildDrawOp(p, tc.a, offset, tc.requiresFlip, c, 1.0)

		for _, q := range tc.probes {
			wantX, wantY := uniformOp.GeoM.Apply(float64(q.X), float64(q.Y))
			gotX, gotY := croppedOp.GeoM.Apply(float64(q.X-tc.origin.X), float64(q.Y-tc.origin.Y))
			if diff := gotX - wantX; diff > eps || diff < -eps {
				t.Errorf("%s frame %d: sheet pixel %v X = %v, want %v", tc.a, tc.frame, q, gotX, wantX)
			}
			if diff := gotY - wantY; diff > eps || diff < -eps {
				t.Errorf("%s frame %d: sheet pixel %v Y = %v, want %v", tc.a, tc.frame, q, gotY, wantY)
			}
		}
	}
}

// TestAutoCropStoresAnEmptyFrameAsOnePixel covers frames with nothing drawn,
// such as the blank tail of a death animation: each becomes a single
// transparent pixel at offset zero rather than a full cell, whether or not the
// rest of its animation has content.
func TestAutoCropStoresAnEmptyFrameAsOnePixel(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown:  {0, 2},
		AnimationIdleRight: {2, 2},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	for _, frame := range []FrameSpec{
		specs[AnimationIdleDown].Frames[1],
		specs[AnimationIdleRight].Frames[0],
		specs[AnimationIdleRight].Frames[1],
	} {
		if frame.Rect.Dx() != 1 || frame.Rect.Dy() != 1 {
			t.Fatalf("empty frame size = %dx%d, want 1x1", frame.Rect.Dx(), frame.Rect.Dy())
		}
		if frame.Offset != (image.Point{}) {
			t.Fatalf("empty frame offset = %v, want zero", frame.Offset)
		}
		if _, _, _, a := atlas.At(frame.Rect.Min.X, frame.Rect.Min.Y).RGBA(); a != 0 {
			t.Fatalf("empty frame pixel at %v has alpha %d, want transparent", frame.Rect.Min, a)
		}
	}

	// An animation with no content at all still loads.
	if _, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleRight: {2},
	}, nil, geometry.Zero2D()); err != nil {
		t.Fatalf("LoadSpriteAutoCropped of an all-empty animation returned error: %v", err)
	}
}

// TestShelfPackKeepsTheGutterForMixedSizes covers shelving frames whose sizes
// differ, which per-frame crop boxes make the normal case: every frame keeps
// its own size, stays inside the atlas, and never touches another.
func TestShelfPackKeepsTheGutterForMixedSizes(t *testing.T) {
	sizes := []image.Point{{5, 5}, {3, 7}, {1, 1}, {8, 2}, {4, 4}, {6, 3}, {2, 9}}
	frames := make([]placement, len(sizes))
	for i, size := range sizes {
		frames[i] = placement{source: image.Rect(0, 0, size.X, size.Y)}
	}

	atlas, placed := shelfPack(frames)
	for i, p := range placed {
		if p.dest.Size() != sizes[i] {
			t.Fatalf("frame %d placed at size %v, want %v", i, p.dest.Size(), sizes[i])
		}
		if !p.dest.In(atlas.Bounds()) {
			t.Fatalf("frame %d at %v is outside the atlas %v", i, p.dest, atlas.Bounds())
		}
		for j := i + 1; j < len(placed); j++ {
			if p.dest.Inset(-1).Overlaps(placed[j].dest) {
				t.Fatalf("frame %d %v and frame %d %v are adjacent or overlapping", i, p.dest, j, placed[j].dest)
			}
		}
	}
}

// TestSetZeroPositionAfterAutoCropIsANoOp covers the hazard per-frame anchors
// would have created: the auto-cropped anchor is the sheet anchor in cell
// coordinates, so setting that same anchor again afterwards changes neither
// the anchor nor where any frame draws.
func TestSetZeroPositionAfterAutoCropIsANoOp(t *testing.T) {
	anchor := geometry.NewVector2(8, 16)
	s, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil, anchor)
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	frames := s.Animations[AnimationIdleDown].Frames
	before := make([]*ebiten.DrawImageOptions, len(frames))
	for i, frame := range frames {
		before[i] = s.buildDrawOp(p, AnimationIdleDown, frame.Offset, false, c, 1.0)
	}

	s.SetZeroPosition(anchor)

	if got := s.Anchor(AnimationIdleDown); got != anchor {
		t.Fatalf("Anchor after SetZeroPosition = %v, want %v", got, anchor)
	}
	for i, frame := range frames {
		if after := s.buildDrawOp(p, AnimationIdleDown, frame.Offset, false, c, 1.0); !geoMEquals(after, before[i]) {
			t.Fatalf("frame %d draw moved: %v, want %v", i, after.GeoM, before[i].GeoM)
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
	if got := s.Animations[AnimationIdleDown].Frames[0].Image.Bounds().Dx(); got != 8 {
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
