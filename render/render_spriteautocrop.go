package render

import (
	"cmp"
	"fmt"
	"image"
	"image/draw"
	"math"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
)

// alphaReaderFor returns a function reading the alpha at a pixel of src, taking a
// direct Pix path for the concrete types sheets decode to. The generic At path
// allocates a color.Color per pixel, which matters when a sheet is tens of
// millions of pixels.
func alphaReaderFor(src image.Image) func(x, y int) uint32 {
	switch img := src.(type) {
	case *image.RGBA:
		return func(x, y int) uint32 { return uint32(img.Pix[img.PixOffset(x, y)+3]) }
	case *image.NRGBA:
		return func(x, y int) uint32 { return uint32(img.Pix[img.PixOffset(x, y)+3]) }
	}
	return func(x, y int) uint32 {
		_, _, _, a := src.At(x, y).RGBA()
		return a
	}
}

// cropBoxIn returns the tight rectangle around non-transparent pixels of cell,
// in coordinates local to cell's origin, and false when the cell is empty.
func cropBoxIn(alphaAt func(x, y int) uint32, cell image.Rectangle) (image.Rectangle, bool) {
	minX, minY := cell.Dx(), cell.Dy()
	maxX, maxY := -1, -1
	for y := cell.Min.Y; y < cell.Max.Y; y++ {
		for x := cell.Min.X; x < cell.Max.X; x++ {
			if alphaAt(x, y) == 0 {
				continue
			}
			lx, ly := x-cell.Min.X, y-cell.Min.Y
			minX, minY = min(minX, lx), min(minY, ly)
			maxX, maxY = max(maxX, lx), max(maxY, ly)
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

// placement is one frame waiting to be copied into the atlas.
type placement struct {
	source image.Rectangle
	dest   image.Rectangle
}

// autoCropAtlas measures a tight crop box per frame over the uniform grid
// described by columns, rows and indexes, and packs every referenced frame into
// a new atlas at its own box's size. Each frame records its box's corner as its
// offset in the cell, and every animation keeps anchor, which is already in
// cell coordinates, unchanged.
//
// Cells no animation references are never visited, so a sheet laid out one
// animation per row does not pay for the empty tail of a short row. The result is
// deterministic: animations are processed in sorted order rather than map order.
//
// It takes an image.Image rather than an *ebiten.Image because the whole point is
// to crop before anything reaches the GPU. Cropping an uploaded texture by
// sub-imaging saves nothing, since a sub-image shares its parent's storage.
func autoCropAtlas(
	src image.Image,
	columns, rows int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) (*image.RGBA, map[AnimationType]AnimationSpec, error) {
	if columns <= 0 || rows <= 0 {
		return nil, nil, fmt.Errorf("grid is %dx%d cells, want both positive", columns, rows)
	}
	bounds := src.Bounds()
	cellWidth, cellHeight := bounds.Dx()/columns, bounds.Dy()/rows
	if cellWidth <= 0 || cellHeight <= 0 {
		return nil, nil, fmt.Errorf("a %dx%d grid over a %dx%d image gives %dx%d cells, want both positive",
			columns, rows, bounds.Dx(), bounds.Dy(), cellWidth, cellHeight)
	}

	alphaAt := alphaReaderFor(src)
	cellAt := func(index int) image.Rectangle {
		x := bounds.Min.X + (index%columns)*cellWidth
		y := bounds.Min.Y + (index/columns)*cellHeight
		return image.Rect(x, y, x+cellWidth, y+cellHeight)
	}

	specs := make(map[AnimationType]AnimationSpec, len(indexes))
	var pending []placement

	for _, a := range sortedAnimationTypes(indexes) {
		frameIndexes := indexes[a]
		if len(frameIndexes) == 0 {
			continue
		}

		frames := make([]FrameSpec, 0, len(frameIndexes))
		for _, index := range frameIndexes {
			if index < 0 || index >= columns*rows {
				return nil, nil, fmt.Errorf("animation %s: frame index %d is outside a %dx%d grid", a, index, columns, rows)
			}
			cell := cellAt(index)
			box, ok := cropBoxIn(alphaAt, cell)
			if !ok {
				// Nothing is drawn in this frame. The cell's corner pixel is
				// transparent, so one pixel of it stands in for the frame at
				// almost no cost, where a full cell would charge a blank frame
				// for padding.
				box = image.Rect(0, 0, 1, 1)
			}
			source := box.Add(cell.Min)
			frames = append(frames, FrameSpec{Rect: source, Offset: box.Min})
			pending = append(pending, placement{source: source})
		}
		specs[a] = AnimationSpec{
			Frames:   frames,
			Anchor:   anchor,
			Duration: durations[a],
		}
	}

	atlas, placed := shelfPack(pending)

	// Rewrite each animation's frames to where they actually landed. pending was
	// built in the same sorted order, so a running index matches them up.
	next := 0
	for _, a := range sortedAnimationTypes(specs) {
		spec := specs[a]
		for i := range spec.Frames {
			spec.Frames[i].Rect = placed[next].dest
			next++
		}
		specs[a] = spec
	}

	for _, p := range placed {
		draw.Draw(atlas, p.dest, src, p.source.Min, draw.Src)
	}

	return atlas, specs, nil
}

// shelfPack lays the given frames out in rows no wider than a target width,
// tallest first, and returns the atlas to copy them into along with where each
// one goes. The order of the returned placements matches the input.
//
// Shelf packing is deliberately simple. Tallest-first shelving keeps each
// shelf's wasted height down even for frames of mixed sizes, and the win being
// chased here is dropping transparent padding rather than the last few percent
// of packing efficiency.
//
// Every pair of placements leaves a one pixel gutter between them, along a
// shelf and between shelves alike. Ebitengine's builtin shader picks the
// AddressUnsafe sampling mode for the linear filter, which reads up to half a
// texel past a frame's edge without clamping to its sub-image. Packed edge to
// edge, that overshoot would sample a neighbouring animation's opaque pixels
// and fringe with its color; the gutter gives it only dead, transparent space
// to land in instead. Frame rectangles are unaffected by this: each still
// describes only its own pixels, and only the space between placements grows.
func shelfPack(frames []placement) (*image.RGBA, []placement) {
	if len(frames) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	}

	const gutter = 1

	// A roughly square atlas, never narrower than the widest frame.
	area, widest := 0, 0
	for _, f := range frames {
		area += f.source.Dx() * f.source.Dy()
		widest = max(widest, f.source.Dx())
	}
	targetWidth := max(widest, int(math.Ceil(math.Sqrt(float64(area)))))

	// Tallest first keeps each shelf's wasted height down. Ties break on the
	// input position, so the layout does not depend on sort stability.
	order := make([]int, len(frames))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		if c := cmp.Compare(frames[j].source.Dy(), frames[i].source.Dy()); c != 0 {
			return c
		}
		return cmp.Compare(i, j)
	})

	placed := make([]placement, len(frames))
	penX, penY, shelfHeight, atlasWidth := 0, 0, 0, 0
	for _, i := range order {
		w, h := frames[i].source.Dx(), frames[i].source.Dy()
		switch {
		case penX > 0 && penX+gutter+w > targetWidth:
			// No room left on this shelf, even with a leading gutter: start a
			// new one, itself separated from this one by a gutter row.
			penX, penY = 0, penY+shelfHeight+gutter
			shelfHeight = 0
		case penX > 0:
			// Not the first frame on this shelf: leave a gutter before it.
			penX += gutter
		}
		placed[i] = placement{
			source: frames[i].source,
			dest:   image.Rect(penX, penY, penX+w, penY+h),
		}
		penX += w
		shelfHeight = max(shelfHeight, h)
		atlasWidth = max(atlasWidth, penX)
	}

	return image.NewRGBA(image.Rect(0, 0, atlasWidth, penY+shelfHeight)), placed
}

// LoadSpriteAutoCropped builds a sprite from a uniform sheet, cropping each
// frame to its own content and repacking the frames into a smaller texture
// before upload. anchor is the sheet-wide anchor in cell-local pixels, which is
// the sprite's frame space: every animation keeps it as its anchor, and each
// frame records where its crop box sat in its cell. SetZeroPosition afterwards
// therefore behaves as it would on the uniform sheet.
//
// It takes an image.Image, not an *ebiten.Image, because the crop must happen
// before the sheet is uploaded: a sheet is mostly transparent padding, and
// padding costs nothing on disk but a full texture in video memory.
//
// As in LoadSprite, width and height are column and row counts, not pixel
// dimensions.
func LoadSpriteAutoCropped(
	src image.Image,
	width, height int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) (*Sprite, error) {
	atlas, specs, err := autoCropAtlas(src, width, height, indexes, durations, anchor)
	if err != nil {
		return nil, fmt.Errorf("cropping sprite sheet: %w", err)
	}
	return LoadSpriteAnimations(ebiten.NewImageFromImage(atlas), specs)
}

// MustLoadSpriteAutoCropped is like LoadSpriteAutoCropped but panics on error.
func MustLoadSpriteAutoCropped(
	src image.Image,
	width, height int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) *Sprite {
	sprite, err := LoadSpriteAutoCropped(src, width, height, indexes, durations, anchor)
	if err != nil {
		panic(fmt.Sprintf("loading auto-cropped sprite: %v", err))
	}
	return sprite
}
