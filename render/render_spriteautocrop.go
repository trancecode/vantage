package render

import (
	"cmp"
	"fmt"
	"image"
	"image/draw"
	"math"
	"slices"
	"time"

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

// autoCropAtlas measures a tight crop box per animation over the uniform grid
// described by columns, rows and indexes, packs every referenced frame into a new
// atlas at that box's size, and rebases anchor into each box.
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

		// One box per animation: the union over its frames, so every frame is
		// stored at the same size and the anchor stays per animation.
		box := image.Rectangle{}
		for _, index := range frameIndexes {
			if index < 0 || index >= columns*rows {
				return nil, nil, fmt.Errorf("animation %s: frame index %d is outside a %dx%d grid", a, index, columns, rows)
			}
			if frameBox, ok := cropBoxIn(alphaAt, cellAt(index)); ok {
				box = box.Union(frameBox)
			}
		}
		if box.Empty() {
			// Nothing is drawn in any frame. Keeping the full cell is safe and
			// costs only what the uncropped sheet already cost.
			box = image.Rect(0, 0, cellWidth, cellHeight)
		}

		frames := make([]image.Rectangle, 0, len(frameIndexes))
		for _, index := range frameIndexes {
			source := box.Add(cellAt(index).Min)
			frames = append(frames, source)
			pending = append(pending, placement{source: source})
		}
		specs[a] = AnimationSpec{
			Frames:   frames,
			Anchor:   anchor.Sub(geometry.NewVector2(box.Min.X, box.Min.Y)),
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
			spec.Frames[i] = placed[next].dest
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
// Shelf packing is deliberately simple. Frames of one animation all share a size,
// so they shelf neatly, and the win being chased here is dropping transparent
// padding rather than the last few percent of packing efficiency.
func shelfPack(frames []placement) (*image.RGBA, []placement) {
	if len(frames) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	}

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
		if penX > 0 && penX+w > targetWidth {
			penX, penY = 0, penY+shelfHeight
			shelfHeight = 0
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
