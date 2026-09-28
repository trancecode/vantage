package tilemap

// Quarter is one of the four quarters of a tile, named by the corner it holds.
// Overlay art for a kind of ground is assembled quarter by quarter, each
// quarter choosing its piece from the neighbours on its side.
type Quarter int

const (
	QuarterNone Quarter = iota
	QuarterNorthWest
	QuarterNorthEast
	QuarterSouthWest
	QuarterSouthEast
)

// Quarters lists the four quarters, north-west first, in row order.
var Quarters = [4]Quarter{QuarterNorthWest, QuarterNorthEast, QuarterSouthWest, QuarterSouthEast}

// Offset returns the direction of the quarter's corner from the tile's
// centre: -1 or 1 on each axis, y growing southwards. It panics for
// QuarterNone.
func (q Quarter) Offset() (dx, dy int) {
	switch q {
	case QuarterNorthWest:
		return -1, -1
	case QuarterNorthEast:
		return 1, -1
	case QuarterSouthWest:
		return -1, 1
	case QuarterSouthEast:
		return 1, 1
	}
	panic("Quarter.Offset: QuarterNone has no offset")
}

// EdgeShape is the piece of an overlay's edge art one quarter of a covered
// tile shows, decided by whether the three neighbours on that quarter's side
// are covered by the same overlay.
type EdgeShape int

const (
	EdgeShapeNone EdgeShape = iota
	// EdgeShapeOuterCorner: the vertical and horizontal neighbours are both
	// uncovered.
	EdgeShapeOuterCorner
	// EdgeShapeHorizontalEdge: only the vertical neighbour is uncovered, so
	// the edge runs horizontally along the tile's top or bottom.
	EdgeShapeHorizontalEdge
	// EdgeShapeVerticalEdge: only the horizontal neighbour is uncovered, so
	// the edge runs vertically along the tile's left or right side.
	EdgeShapeVerticalEdge
	// EdgeShapeInnerCorner: both are covered and the diagonal is not.
	EdgeShapeInnerCorner
	// EdgeShapeFill: all three are covered.
	EdgeShapeFill
)

// QuarterEdgeShape returns the edge shape quarter q of tile (x, y) shows, where
// covered reports whether a tile is covered by the overlay being drawn. The
// vertical neighbour of a north quarter is the tile to the north, and so on.
// It returns EdgeShapeNone for an uncovered tile, which draws no piece.
func QuarterEdgeShape(covered func(x, y int) bool, x, y int, q Quarter) EdgeShape {
	if !covered(x, y) {
		return EdgeShapeNone
	}
	dx, dy := q.Offset()
	vertical := covered(x, y+dy)
	horizontal := covered(x+dx, y)
	switch {
	case !vertical && !horizontal:
		return EdgeShapeOuterCorner
	case !vertical:
		return EdgeShapeHorizontalEdge
	case !horizontal:
		return EdgeShapeVerticalEdge
	case !covered(x+dx, y+dy):
		return EdgeShapeInnerCorner
	}
	return EdgeShapeFill
}
