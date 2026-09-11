package pathfinding

import (
	"container/heap"
	"math"
)

// cellRates are a cell's crossing rates: the cheapest real cost per tile of
// crossing it west to east and north to south, infinite when the cell cannot
// be crossed that way. A cell straddling the map's edge is measured over its
// in-bounds part only, so its out-of-bounds tiles never make it uncrossable.
type cellRates struct {
	westEast   float64
	northSouth float64
}

// uncrossable reports whether the cell can be crossed neither way.
func (r cellRates) uncrossable() bool {
	return math.IsInf(r.westEast, 1) && math.IsInf(r.northSouth, 1)
}

// buildCellRates reads the size x size tiles of cell once and returns its
// crossing rates. Each rate comes from a multi-source Dijkstra search
// restricted to the cell, with FindPath's step cost and corner-cutting rule,
// from every walkable tile of the first column (or row) holding an in-bounds
// tile to the first tile of the last such column (or row) it closes, divided by
// the distance between those two columns (or rows). For a cell entirely in
// bounds that distance is size - 1. A cell straddling the map's edge is
// measured between its first and last in-bounds columns (or rows) instead; when
// only one is in bounds, the rate is the inverse of the highest speed among the
// cell's walkable tiles, and infinite when none is walkable.
func buildCellRates(terrain TerrainProvider, cell Coord, size int) cellRates {
	walkable := make([]bool, size*size)
	speed := make([]float64, size*size)
	firstColumn, lastColumn, firstRow, lastRow := size, -1, size, -1
	fastest := 0.0
	for j := range size {
		for i := range size {
			x, y := cell.X*size+i, cell.Y*size+j
			if !terrain.IsInBounds(x, y) {
				continue
			}
			firstColumn, lastColumn = min(firstColumn, i), max(lastColumn, i)
			firstRow, lastRow = min(firstRow, j), max(lastRow, j)
			if terrain.IsWalkable(x, y) {
				walkable[j*size+i] = true
				speed[j*size+i] = terrain.GetTerrainSpeedMultiplier(x, y)
				fastest = max(fastest, speed[j*size+i])
			}
		}
	}
	return cellRates{
		westEast:   crossingRate(walkable, speed, size, true, firstColumn, lastColumn, fastest),
		northSouth: crossingRate(walkable, speed, size, false, firstRow, lastRow, fastest),
	}
}

// crossingRate returns the cheapest cost per tile of crossing the cell from its
// first in-bounds column (or row) to its last, where fastest is the highest
// speed among its walkable tiles. When first and last are one column (or row),
// or no tile is in bounds, the crossing has no length and the rate is the
// inverse of fastest, or +Inf when no tile is walkable.
func crossingRate(walkable []bool, speed []float64, size int, westEast bool, first, last int, fastest float64) float64 {
	if last <= first {
		if fastest > 0 {
			return 1 / fastest
		}
		return math.Inf(1)
	}
	return crossingCost(walkable, speed, size, westEast, first, last) / float64(last-first)
}

// crossingCost returns the cheapest cost of reaching column last from any
// walkable tile of column first when westEast is true, or row last from row
// first otherwise, or +Inf when nothing gets across.
func crossingCost(walkable []bool, speed []float64, size int, westEast bool, first, last int) float64 {
	cost := make([]float64, size*size)
	for i := range cost {
		cost[i] = math.Inf(1)
	}
	open := &tileQueue{}
	for s := range size {
		index := s*size + first // column first, row s
		if !westEast {
			index = first*size + s // row first, column s
		}
		if walkable[index] {
			cost[index] = 0
			heap.Push(open, tileEntry{index: index})
		}
	}

	for open.Len() > 0 {
		entry := heap.Pop(open).(tileEntry)
		if entry.cost > cost[entry.index] {
			continue // superseded by a cheaper entry
		}
		x, y := entry.index%size, entry.index/size
		if (westEast && x == last) || (!westEast && y == last) {
			return entry.cost
		}
		for _, dir := range directions {
			nx, ny := x+dir.X, y+dir.Y
			if nx < 0 || ny < 0 || nx >= size || ny >= size {
				continue
			}
			next := ny*size + nx
			if !walkable[next] {
				continue
			}
			step := cardinalCost
			if !isCardinalDirection(dir.X, dir.Y) {
				if !walkable[y*size+nx] && !walkable[ny*size+x] {
					continue
				}
				step = diagonalCost
			}
			multiplier := (speed[entry.index] + speed[next]) / 2
			if multiplier <= 0 {
				continue
			}
			nextCost := entry.cost + step/multiplier
			if nextCost < cost[next] {
				cost[next] = nextCost
				heap.Push(open, tileEntry{index: next, cost: nextCost})
			}
		}
	}
	return math.Inf(1)
}

// tileEntry is a tile index waiting in a crossing search, at its cost when
// queued.
type tileEntry struct {
	index int
	cost  float64
}

// tileQueue is the crossing search's priority queue, cheapest first.
type tileQueue []tileEntry

func (q tileQueue) Len() int           { return len(q) }
func (q tileQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q tileQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (q *tileQueue) Push(x any) { *q = append(*q, x.(tileEntry)) }

func (q *tileQueue) Pop() any {
	old := *q
	entry := old[len(old)-1]
	*q = old[:len(old)-1]
	return entry
}
