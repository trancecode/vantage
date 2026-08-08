// Package pixeltest renders a sprite loaded two ways, uniformly with
// [render.LoadSprite] and auto-cropped with [render.LoadSpriteAutoCropped],
// and compares the actual rendered pixels. That is the one thing no other test
// of the auto-crop feature does: the packer's own tests read the CPU-side
// atlas before upload, and render's own draw-geometry tests compare a draw
// op's matrix, never a drawn color. This package closes that gap by running a
// real Ebitengine frame and reading pixels back with ReadPixels.
//
// It is a package of its own, containing only test files, because Ebitengine
// permits one ebiten.RunGame per process and a Go test binary is one process
// per package. Sharing render's own test binary would mean every future test
// in that package shares this one RunGame call too.
package pixeltest

import (
	"fmt"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
)

const (
	sheetColumns = 3
	sheetRows    = 3
	cellSize     = 20 // pixels per grid cell, both dimensions

	// canvasSize is the offscreen image size each scenario renders into. Every
	// scenario draws a full cellSize cell scaled by displayScale and centered,
	// which stays well inside this margin on every side.
	canvasSize = 96

	// displayScale is the per-draw scale every scenario in this package uses.
	// It is deliberately non-integer: at an integer position and a scale of
	// exactly 1, linear filtering samples texel centres and degenerates to
	// nearest, so the packing gutter's fringing bug would not appear. This
	// value forces linear filtering to genuinely resample between texels.
	displayScale = 2.5

	// frameCap backstops ebiten.RunGame: if the comparisons never complete,
	// Update returns an error instead of letting the loop hang forever, which
	// would hang CI rather than fail it.
	frameCap = 30
)

// buildFixtureSheet returns a synthetic sprite sheet on a 3x3 grid, split
// across two concerns the test covers separately (see buildScenarios):
//
//   - Cells 0 and 1 (row 0) are two frames of AnimationIdleDown, with content
//     at different cell-local offsets, so their union crop box is strictly
//     larger than either frame's own content and the union path is
//     exercised. Cell 2 is AnimationIdleRight and carries a non-square block
//     at an off-diagonal cell-local origin: a square block at a square
//     origin cannot distinguish an x/y transposition in the crop or the
//     anchor rebase, so this shape can. All three cells carry real cell
//     padding around their content, which is what a real sprite sheet looks
//     like, and what proves the crop and rebase math correct; they are
//     compared under FilterNearest, the one filter a padded-versus-cropped
//     pixel comparison can be made bit-exact under (see buildScenarios).
//   - Cells 6 and 8 (row 2, columns 0 and 2) are AnimationAttackDown and
//     AnimationAttackRight, each filling its whole cell with no padding at
//     all, in a third and fourth color. A cell with no padding crops to
//     exactly its own size, so [render.LoadSprite]'s frame and
//     [render.LoadSpriteAutoCropped]'s tight frame are then the same size
//     for these two, which is what makes an exact comparison achievable
//     under FilterLinear too (again, see buildScenarios). Row 1 and cell 7
//     are left untouched (transparent) precisely so that neither of these
//     two cells has any opaque neighbor within the sheet itself: without
//     that gap, LoadSprite's own rendering would bleed the adjacent cell's
//     real color under linear resampling, a sheet-layout artifact that has
//     nothing to do with the packer this test exists to check. The packer
//     ignores unreferenced cells and still lands AnimationAttackDown and
//     AnimationAttackRight on adjacent shelves one pixel apart (verified in
//     the package's report), so the packing gutter between two different
//     colors is exactly what stands between them there.
//   - Every block is a hard-edged opaque rectangle that reaches the crop box
//     it will be tightened to, since the fringing bug this package exists to
//     catch lives exactly at a frame's edge, not in a soft or centered blob.
func buildFixtureSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, sheetColumns*cellSize, sheetRows*cellSize))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, A: 255}
	cyan := color.RGBA{G: 255, B: 255, A: 255}

	// Cell 0 (col 0, row 0): AnimationIdleDown frame 0, an 8x4 block at
	// cell-local (2,5).
	fill(2, 5, 10, 9, red)
	// Cell 1 (col 1, row 0): AnimationIdleDown frame 1, a 12x8 block at
	// cell-local (6,9). Frame 0's content is (2,5)-(10,9) and frame 1's is
	// (6,9)-(18,17): neither contains the other, so their union, (2,5)-(18,17),
	// is a real union rather than one frame's box in disguise.
	fill(cellSize+6, 9, cellSize+18, 17, red)
	// Cell 2 (col 2, row 0): AnimationIdleRight, an 8x2 block at the
	// off-diagonal cell-local origin (3,7).
	fill(2*cellSize+3, 7, 2*cellSize+11, 9, blue)
	// Row 1 (cells 3, 4, 5) stays fully transparent: the isolating gap
	// described above.
	// Cell 6 (col 0, row 2): AnimationAttackDown, opaque edge to edge.
	fill(0, 2*cellSize, cellSize, 3*cellSize, yellow)
	// Cell 8 (col 2, row 2): AnimationAttackRight, opaque edge to edge, in a
	// different color so bleed from its neighbor in the packed atlas is
	// visible rather than invisible. Cell 7, between them, stays transparent.
	fill(2*cellSize, 2*cellSize, 3*cellSize, 3*cellSize, cyan)

	return img
}

// fixtureIndexes assigns the fixture sheet's referenced cells to animations.
// AnimationIdleLeft and AnimationAttackLeft are not listed because neither is
// drawn from its own cell: both are generated from their Right counterparts by
// [render.MirroredAnimations], and scenarios draw them to exercise that flip
// against a rebased anchor.
var fixtureIndexes = map[render.AnimationType][]int{
	render.AnimationIdleDown:    {0, 1},
	render.AnimationIdleRight:   {2},
	render.AnimationAttackDown:  {6},
	render.AnimationAttackRight: {8},
}

// fixtureDurations gives AnimationIdleDown an explicit, round duration so a
// scenario's elapsed time maps predictably to a frame index: two frames over
// 1000ms means each holds a 500ms slot. Every other animation is left
// unspecified; each has one frame, so any duration reads back that same frame.
var fixtureDurations = map[render.AnimationType]time.Duration{
	render.AnimationIdleDown: 1000 * time.Millisecond,
}

// fixtureAnchor is the sheet-wide anchor passed to both load paths: to
// [render.Sprite.SetZeroPosition] for the uniform sprite, and to
// [render.LoadSpriteAutoCropped] for the cropped one. Both loaders rebase it
// into per-animation coordinates their own way, and the property under test is
// that the two rebases agree on where every sheet pixel ends up on screen.
//
// Its fractional part is deliberate, not decorative. Every crop box in this
// fixture sits at an integer cell-local offset, so with an integer anchor and
// displayScale's half-integer 2.5, every quad edge would land on an exact
// multiple of 0.5 screen pixels: precisely on a pixel center, a rasterization
// tie so fine that the uniform and cropped paths' independently computed
// float64 arithmetic (different constants folded in a different order, though
// mathematically equal) can round to different pixels under FilterNearest,
// which is not the resampling bug this package exists to catch. A fractional
// offset whose product with 2.5 is not itself a half-integer, as here, moves
// every edge off that knife-edge.
var fixtureAnchor = geometry.NewVector2(10.1, 10.3)

// scenario is one comparison: draw a single animation, at a single point in
// its timeline, under a single filter, from both sprites, and diff the result.
type scenario struct {
	name      string
	animation render.AnimationType
	elapsed   time.Duration
	filter    ebiten.Filter
}

// buildScenarios returns every comparison this package runs, split into the
// two concerns buildFixtureSheet describes:
//
// AnimationIdleDown (both frames), AnimationIdleRight and AnimationIdleLeft
// exercise the crop and anchor-rebase math: an off-diagonal box, a union of
// two frames, and a mirrored flip. They are compared under FilterNearest
// only. Under FilterLinear, Ebitengine's built-in shader blends an edge pixel
// with whatever true neighboring texels exist within a frame's own declared
// bounds; [render.LoadSprite]'s frame is the whole padded cell and
// [render.LoadSpriteAutoCropped]'s is the tight crop, so even with the
// packing gutter correctly in place the two frames are different sizes and
// blend their edges differently. That is not the bug this package exists to
// catch, so pinning it down needs frames of equal size, which is what the
// second group provides.
//
// AnimationAttackDown and AnimationAttackRight (and its mirror,
// AnimationAttackLeft) fill their cells edge to edge, so their crop box is
// the whole cell: [render.LoadSprite]'s frame and
// [render.LoadSpriteAutoCropped]'s tight frame are then the same size, and an
// exact comparison is achievable under both filters. They are packed
// adjacent to each other in the atlas (verified in the package's report), so
// the only thing standing between AnimationAttackDown's opaque edge and
// AnimationAttackRight's different color is the one-pixel packing gutter,
// exactly what buildFixtureSheet's second group exists to exercise under
// resampling.
//
// AnimationIdleDown's two frames are drawn separately rather than as one
// scenario, because they are not interchangeable: autoCropAtlas packs frame
// 0's content into one half of its box and frame 1's into the other, so only
// one of them reaches a packed edge. Frame 0 alone would exercise the union
// computation without ever touching a seam.
func buildScenarios() []scenario {
	var scenarios []scenario

	geometryAnimations := []struct {
		animation render.AnimationType
		elapsed   []time.Duration
	}{
		// 1000ms over 2 frames: 250ms lands on frame 0, 750ms on frame 1.
		{render.AnimationIdleDown, []time.Duration{250 * time.Millisecond, 750 * time.Millisecond}},
		{render.AnimationIdleRight, []time.Duration{0}},
		{render.AnimationIdleLeft, []time.Duration{0}},
	}
	for _, ga := range geometryAnimations {
		for _, elapsed := range ga.elapsed {
			scenarios = append(scenarios, scenario{
				name:      fmt.Sprintf("%s/elapsed=%s/filter=%s", ga.animation, elapsed, render.FilterName(ebiten.FilterNearest)),
				animation: ga.animation,
				elapsed:   elapsed,
				filter:    ebiten.FilterNearest,
			})
		}
	}

	for _, a := range []render.AnimationType{render.AnimationAttackDown, render.AnimationAttackRight, render.AnimationAttackLeft} {
		for _, filter := range []ebiten.Filter{ebiten.FilterNearest, ebiten.FilterLinear} {
			scenarios = append(scenarios, scenario{
				name:      fmt.Sprintf("%s/filter=%s", a, render.FilterName(filter)),
				animation: a,
				elapsed:   0,
				filter:    filter,
			})
		}
	}

	return scenarios
}

// TestAutoCroppedSpriteRendersIdenticallyToUniform is the property the whole
// auto-crop feature has to preserve, checked at the one level the rest of the
// suite cannot reach: the sprite [render.LoadSprite] loads from a uniform grid
// and the same sheet [render.LoadSpriteAutoCropped] crops and repacks must
// draw pixel-identically, for every animation in the fixture, at a scale that
// genuinely resamples.
func TestAutoCroppedSpriteRendersIdenticallyToUniform(t *testing.T) {
	sheet := buildFixtureSheet()

	uniform, err := render.LoadSprite(ebiten.NewImageFromImage(sheet), sheetColumns, sheetRows, fixtureIndexes, fixtureDurations)
	if err != nil {
		t.Fatalf("loading uniform sprite: %v", err)
	}
	uniform.SetZeroPosition(fixtureAnchor)

	cropped, err := render.LoadSpriteAutoCropped(sheet, sheetColumns, sheetRows, fixtureIndexes, fixtureDurations, fixtureAnchor)
	if err != nil {
		t.Fatalf("loading auto-cropped sprite: %v", err)
	}

	game := &comparisonGame{
		uniform:   uniform,
		cropped:   cropped,
		scenarios: buildScenarios(),
	}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatalf("running comparison game: %v", err)
	}

	if len(game.results) != len(game.scenarios) {
		t.Fatalf("got %d results, want %d: the game loop stopped before every scenario ran",
			len(game.results), len(game.scenarios))
	}
	for _, r := range game.results {
		if r.mismatch != nil {
			t.Errorf("scenario %s: uniform and auto-cropped renders differ: %v", r.name, r.mismatch)
		}
	}
}
