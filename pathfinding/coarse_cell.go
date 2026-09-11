package pathfinding

import (
	"container/heap"
	"math"
)

// cellRates are a cell's crossing rates: the cheapest real cost per tile of
// crossing it west to east and north to south, infinite when the cell cannot
// be crossed that way.
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
// from every walkable tile of the first column (or row) to the first tile of
// the last column (or row) it closes, divided by size - 1.
func buildCellRates(terrain TerrainProvider, cell Coord, size int) cellRates {
	walkable := make([]bool, size*size)
	speed := make([]float64, size*size)
	for j := range size {
		for i := range size {
			x, y := cell.X*size+i, cell.Y*size+j
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				walkable[j*size+i] = true
				speed[j*size+i] = terrain.GetTerrainSpeedMultiplier(x, y)
			}
		}
	}
	span := float64(size - 1)
	return cellRates{
		westEast:   crossingCost(walkable, speed, size, true) / span,
		northSouth: crossingCost(walkable, speed, size, false) / span,
	}
}

// crossingCost returns the cheapest cost of reaching the cell's last column
// from any walkable tile of its first column when westEast is true, or its
// last row from its first row otherwise, or +Inf when nothing gets across.
func crossingCost(walkable []bool, speed []float64, size int, westEast bool) float64 {
	cost := make([]float64, size*size)
	for i := range cost {
		cost[i] = math.Inf(1)
	}
	open := &tileQueue{}
	for s := range size {
		index := s * size // first column, row s
		if !westEast {
			index = s // first row, column s
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
		if (westEast && x == size-1) || (!westEast && y == size-1) {
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
