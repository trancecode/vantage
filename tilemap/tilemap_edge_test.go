package tilemap

import "testing"

// grid builds a covered predicate from rows of '#' (covered) and '.' (not);
// coordinates outside the rows are uncovered.
func grid(rows ...string) func(x, y int) bool {
	return func(x, y int) bool {
		if y < 0 || y >= len(rows) || x < 0 || x >= len(rows[y]) {
			return false
		}
		return rows[y][x] == '#'
	}
}

func TestQuarterEdgeShapeCases(t *testing.T) {
	cases := []struct {
		name string
		rows []string
		q    Quarter
		want EdgeShape
	}{
		{"lone tile is outer corner", []string{"...", ".#.", "..."}, QuarterNorthWest, EdgeShapeOuterCorner},
		{"north open, west covered is horizontal edge", []string{"...", "##.", "..."}, QuarterNorthWest, EdgeShapeHorizontalEdge},
		{"north covered, west open is vertical edge", []string{".#.", ".#.", "..."}, QuarterNorthWest, EdgeShapeVerticalEdge},
		{"north and west covered, diagonal open is inner corner", []string{".#.", "##.", "..."}, QuarterNorthWest, EdgeShapeInnerCorner},
		{"all three covered is fill", []string{"##.", "##.", "..."}, QuarterNorthWest, EdgeShapeFill},
		{"south-east reads south, east and south-east", []string{"...", ".##", ".##"}, QuarterSouthEast, EdgeShapeFill},
		{"south-east with south-east diagonal open", []string{"...", ".##", ".#."}, QuarterSouthEast, EdgeShapeInnerCorner},
		{"north-east with east open", []string{".#.", ".#.", "..."}, QuarterNorthEast, EdgeShapeVerticalEdge},
		{"south-west with south open", []string{"...", "##.", "..."}, QuarterSouthWest, EdgeShapeHorizontalEdge},
	}
	for _, c := range cases {
		if got := QuarterEdgeShape(grid(c.rows...), 1, 1, c.q); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestQuarterEdgeShapeUncoveredTileIsNone(t *testing.T) {
	covered := grid("###", "#.#", "###")
	for _, q := range Quarters {
		if got := QuarterEdgeShape(covered, 1, 1, q); got != EdgeShapeNone {
			t.Errorf("%v of an uncovered tile: got %v, want EdgeShapeNone", q, got)
		}
	}
}

// TestQuarterEdgeShapeReadsOnlyItsThreeNeighbours checks, over all 256
// neighbourhoods, that a quarter's shape depends only on the neighbours on its
// own side: flipping any other neighbour never changes it.
func TestQuarterEdgeShapeReadsOnlyItsThreeNeighbours(t *testing.T) {
	offsets := [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
	for mask := 0; mask < 256; mask++ {
		covered := func(m int) func(x, y int) bool {
			return func(x, y int) bool {
				if x == 0 && y == 0 {
					return true
				}
				for i, o := range offsets {
					if o[0] == x && o[1] == y {
						return m&(1<<i) != 0
					}
				}
				return false
			}
		}
		for _, q := range Quarters {
			dx, dy := q.Offset()
			base := QuarterEdgeShape(covered(mask), 0, 0, q)
			for i, o := range offsets {
				relevant := (o[0] == dx || o[0] == 0) && (o[1] == dy || o[1] == 0)
				if relevant {
					continue
				}
				if got := QuarterEdgeShape(covered(mask^(1<<i)), 0, 0, q); got != base {
					t.Fatalf("mask %08b, %v: flipping (%d,%d) changed %v to %v", mask, q, o[0], o[1], base, got)
				}
			}
		}
	}
}
