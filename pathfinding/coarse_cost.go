package pathfinding

import (
	"container/heap"
	"fmt"
	"math"
)

// DefaultCoarseCellSize is the measured default side of a CoarseCost cell in
// tiles. Smaller cells settled four times as many cells for the same ground,
// and larger ones read more ground for the same route quality; see the design
// spec under docs/superpowers/specs.
const DefaultCoarseCellSize = 32

// DefaultCoarseCellBudget is the measured default number of cells one search
// may settle. It keeps every measured 1,000-tile journey on coarse values
// while capping how much ground a cold search reads.
const DefaultCoarseCellBudget = 2048

// CoarseCostConfig configures a CoarseCost. Every field must be set.
type CoarseCostConfig struct {
	// CellSize is the side of a cell in tiles, at least 2.
	CellSize int

	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. The coarse search divides its focus by it, which keeps cell values
	// from missing faster ground the search has not looked at yet. It must be
	// positive.
	MaxSpeed float64

	// CellBudget bounds how many cells one search may settle before its
	// estimates fall back to octile distance. It is what makes the coarse
	// search return on a terrain with no edge. It must be positive.
	CellBudget int
}

// CoarseCost is a Heuristic that learns where fast and slow ground lies, a
// cell at a time. Each cell of CellSize tiles records its cheapest real cost
// per tile of crossing west to east and north to south, built the first time
// a search needs it and kept for the life of the CoarseCost. Each search runs
// a coarse search from the goal over cell centers, resumed whenever the tile
// search asks for an estimate, and blends the values of the centers around a
// tile into its estimate. Routes are near-optimal rather than guaranteed
// optimal.
//
// A CoarseCost is bound to the terrain it was built with and must be given to
// FindPath together with that same terrain. It assumes the terrain does not
// change once a cell is built; a changed tile only makes estimates worse, since
// the tile search still reads the terrain itself. It is not safe for
// concurrent use: searches share its cell cache.
type CoarseCost struct {
	terrain TerrainProvider
	config  CoarseCostConfig
	cells   map[Coord]cellRates // built cells, keyed by cell coordinate
}

var _ Heuristic = (*CoarseCost)(nil)

// NewCoarseCost returns a coarse cost field over terrain. It panics when
// terrain is nil or a config field is not set to a usable value.
func NewCoarseCost(terrain TerrainProvider, config CoarseCostConfig) *CoarseCost {
	if terrain == nil {
		panic("creating coarse cost field: terrain is nil")
	}
	if config.CellSize < 2 {
		panic(fmt.Sprintf("creating coarse cost field: CellSize must be at least 2, got %d", config.CellSize))
	}
	if !(config.MaxSpeed > 0) {
		panic(fmt.Sprintf("creating coarse cost field: MaxSpeed must be positive, got %v", config.MaxSpeed))
	}
	if config.CellBudget <= 0 {
		panic(fmt.Sprintf("creating coarse cost field: CellBudget must be positive, got %d", config.CellBudget))
	}
	return &CoarseCost{terrain: terrain, config: config, cells: make(map[Coord]cellRates)}
}

// cellOf returns the cell containing tile.
func (c *CoarseCost) cellOf(tile Coord) Coord {
	return Coord{X: floorDiv(tile.X, c.config.CellSize), Y: floorDiv(tile.Y, c.config.CellSize)}
}

// center returns the tile-space position of a cell's center.
func (c *CoarseCost) center(cell Coord) (x, y float64) {
	size := c.config.CellSize
	half := float64(size-1) / 2
	return float64(cell.X*size) + half, float64(cell.Y*size) + half
}

// rates returns a cell's crossing rates, building the cell on first use. It is
// the one place the field reads cells, so a later per-cell cost source, one a
// game answers without reading tiles, replaces buildCellRates here and nowhere
// else.
func (c *CoarseCost) rates(cell Coord) cellRates {
	if rates, ok := c.cells[cell]; ok {
		return rates
	}
	rates := buildCellRates(c.terrain, cell, c.config.CellSize)
	c.cells[cell] = rates
	return rates
}

// floorDiv divides a by a positive b, rounding toward negative infinity.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// movementRates returns the rates for moving within or out of a cell. A rate
// is infinite when nothing crosses the cell edge to edge that way, yet a route
// can still enter and leave it through that side, as along a shore whose water
// covers the cell's northern edge; so an infinite rate takes the other axis's
// rate. A cell uncrossable both ways keeps both infinite.
func movementRates(rates cellRates) cellRates {
	if rates.uncrossable() {
		return rates
	}
	if math.IsInf(rates.westEast, 1) {
		rates.westEast = rates.northSouth
	}
	if math.IsInf(rates.northSouth, 1) {
		rates.northSouth = rates.westEast
	}
	return rates
}

// localCost is the cost from one point to another nearby, using one cell's
// movement rates: octile distance with each axis weighted by its rate. Both
// rates are 1.0 when the cell is uncrossable, so a local cost is always finite.
func localCost(fromX, fromY, toX, toY float64, rates cellRates) float64 {
	rates = movementRates(rates)
	westEast, northSouth := rates.westEast, rates.northSouth
	if rates.uncrossable() {
		westEast, northSouth = 1, 1
	}
	dx, dy := math.Abs(toX-fromX), math.Abs(toY-fromY)
	return dx*westEast + dy*northSouth - math.Min(dx, dy)*(westEast+northSouth)*(1-math.Sqrt2/2)
}

// coarseSearch is one search's coarse search over cell centers, from the goal
// toward the start, paused between requests for values.
type coarseSearch struct {
	field     *CoarseCost
	goal      Coord
	goalCell  Coord
	startCell Coord
	cost      map[Coord]float64 // best known cost from a center to the goal
	closed    map[Coord]bool    // centers whose cost is settled
	open      cellQueue
	settled   int // centers settled so far, bounded by CellBudget
}

// newSearch starts a coarse search for one tile search from start to goal,
// seeded with the four centers surrounding the goal at their local cost from
// it.
func (c *CoarseCost) newSearch(start, goal Coord) *coarseSearch {
	s := &coarseSearch{
		field:     c,
		goal:      goal,
		goalCell:  c.cellOf(goal),
		startCell: c.cellOf(start),
		cost:      make(map[Coord]float64),
		closed:    make(map[Coord]bool),
	}
	rates := c.rates(s.goalCell)
	for _, cell := range s.surrounding(goal) {
		x, y := c.center(cell)
		s.offer(cell, localCost(float64(goal.X), float64(goal.Y), x, y, rates))
	}
	return s
}

// surrounding returns the four cells whose centers surround tile, in the order
// west-north, east-north, west-south, east-south.
func (s *coarseSearch) surrounding(tile Coord) [4]Coord {
	size := float64(s.field.config.CellSize)
	half := (size - 1) / 2
	x := int(math.Floor((float64(tile.X) - half) / size))
	y := int(math.Floor((float64(tile.Y) - half) / size))
	return [4]Coord{{X: x, Y: y}, {X: x + 1, Y: y}, {X: x, Y: y + 1}, {X: x + 1, Y: y + 1}}
}

// focus estimates the coarse cost from a cell to the start cell. Dividing by
// the fastest speed keeps it from ever overestimating, so a settled center's
// value is the cheapest coarse route to the goal.
func (s *coarseSearch) focus(cell Coord) float64 {
	return octile(cell, s.startCell) * float64(s.field.config.CellSize) / s.field.config.MaxSpeed
}

// offer records cost as a cell's cost to the goal when it improves on the best
// known one, and queues the cell.
func (s *coarseSearch) offer(cell Coord, cost float64) {
	if best, ok := s.cost[cell]; ok && cost >= best {
		return
	}
	s.cost[cell] = cost
	heap.Push(&s.open, cellEntry{cell: cell, cost: cost, priority: cost + s.focus(cell)})
}

// value returns the cost from a cell's center to the goal, resuming the coarse
// search until that center is settled. A cell that cannot be crossed either
// way answers infinity without searching. It reports false when the center
// cannot be settled: the open set emptied, or CellBudget centers have been
// settled in this search.
func (s *coarseSearch) value(cell Coord) (float64, bool) {
	if s.field.rates(cell).uncrossable() {
		return math.Inf(1), true
	}
	for !s.closed[cell] {
		if s.open.Len() == 0 || s.settled >= s.field.config.CellBudget {
			return 0, false
		}
		entry := heap.Pop(&s.open).(cellEntry)
		if s.closed[entry.cell] || entry.cost > s.cost[entry.cell] {
			continue // settled already, or superseded by a cheaper entry
		}
		s.closed[entry.cell] = true
		s.settled++
		s.relax(entry.cell, entry.cost)
	}
	return s.cost[cell], true
}

// relax offers each neighbor of a settled cell the route through it, using
// both cells' movement rates. An east or west edge costs a cell width times the
// two cells' mean west-east rate, north and south likewise, and a diagonal edge
// sqrt(2) widths times the mean of the two cells' mean rates; only a cell
// uncrossable both ways blocks an edge. Relaxing builds the neighbor.
func (s *coarseSearch) relax(cell Coord, cost float64) {
	size := float64(s.field.config.CellSize)
	here := movementRates(s.field.rates(cell))
	for _, dir := range directions {
		next := Coord{X: cell.X + dir.X, Y: cell.Y + dir.Y}
		if s.closed[next] {
			continue
		}
		there := movementRates(s.field.rates(next))
		var edge float64
		switch {
		case !isCardinalDirection(dir.X, dir.Y):
			edge = diagonalCost * size * (meanRate(here) + meanRate(there)) / 2
		case dir.X != 0:
			edge = size * (here.westEast + there.westEast) / 2
		default:
			edge = size * (here.northSouth + there.northSouth) / 2
		}
		if math.IsInf(edge, 1) {
			continue
		}
		s.offer(next, cost+edge)
	}
}

// meanRate is the mean of a cell's two crossing rates.
func meanRate(rates cellRates) float64 {
	return (rates.westEast + rates.northSouth) / 2
}

// coarseTieBreak scales every coarse estimate. Where many routes cost within a
// few percent of the cheapest, as on an oblique journey over uniform ground or
// along the edge of a plain beside half-speed forest, an estimate accurate to
// about a percent leaves their priorities tied within its own error, so A*
// expands the whole band; the scale breaks the ties toward the goal. Measured
// on nrg's civilized cardinal 1,000-tile journey, whose routes within 1% of
// optimal cover 96,067 tiles: 115,633 expansions at 1.01, 95,533 at 1.02 and
// 33,048 at 1.05. Across the heuristic benchmark maps, 1.05 cut expansions on
// every journey that flooded (grid oblique at 2,000 tiles: 134,609 to 25,316)
// while raising the largest cost above optimal from 0.27% to 0.44%.
const coarseTieBreak = 1.05

// ForSearch returns the coarse estimate for one search from start to goal.
func (c *CoarseCost) ForSearch(start, goal Coord) Estimate {
	return c.newSearch(start, goal).estimate
}

// estimate blends the values of the four centers surrounding tile by the
// tile's position between them, leaving out centers that are infinite or that
// the cell budget left unsettled and renormalizing the rest. When every
// remaining weight is zero it takes the cheapest route through one of them.
// Within one cell of the goal's cell it is at most the local cost straight to
// the goal, and with no center to go by it is octile distance. Every estimate
// is scaled by coarseTieBreak.
func (s *coarseSearch) estimate(tile Coord) float64 {
	here := s.field.cellOf(tile)
	rates := s.field.rates(here)
	x, y := float64(tile.X), float64(tile.Y)

	size := float64(s.field.config.CellSize)
	half := (size - 1) / 2
	fx := (x - half) / size
	fx -= math.Floor(fx)
	fy := (y - half) / size
	fy -= math.Floor(fy)
	weights := [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}

	cells := s.surrounding(tile)
	var values [4]float64
	var usable [4]bool
	sum, weight := 0.0, 0.0
	for i, cell := range cells {
		value, ok := s.value(cell)
		if !ok || math.IsInf(value, 1) {
			continue
		}
		values[i], usable[i] = value, true
		sum += weights[i] * value
		weight += weights[i]
	}

	estimate := math.Inf(1)
	if weight > 0 {
		estimate = sum / weight
	}
	if weight == 0 {
		for i, cell := range cells {
			if !usable[i] {
				continue
			}
			centerX, centerY := s.field.center(cell)
			estimate = math.Min(estimate, localCost(x, y, centerX, centerY, rates)+values[i])
		}
	}
	if max(here.X-s.goalCell.X, s.goalCell.X-here.X) <= 1 && max(here.Y-s.goalCell.Y, s.goalCell.Y-here.Y) <= 1 {
		estimate = math.Min(estimate, localCost(x, y, float64(s.goal.X), float64(s.goal.Y), rates))
	}
	if math.IsInf(estimate, 1) {
		estimate = octile(tile, s.goal)
	}
	return estimate * coarseTieBreak
}

// cellEntry is a cell center waiting in a coarse search, at its cost to the
// goal when queued and its priority, that cost plus the focus.
type cellEntry struct {
	cell     Coord
	cost     float64
	priority float64
}

// cellQueue is the coarse search's priority queue, lowest priority first.
type cellQueue []cellEntry

func (q cellQueue) Len() int           { return len(q) }
func (q cellQueue) Less(i, j int) bool { return q[i].priority < q[j].priority }
func (q cellQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (q *cellQueue) Push(x any) { *q = append(*q, x.(cellEntry)) }

func (q *cellQueue) Pop() any {
	old := *q
	entry := old[len(old)-1]
	*q = old[:len(old)-1]
	return entry
}
