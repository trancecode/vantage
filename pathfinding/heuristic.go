package pathfinding

import (
	"fmt"
	"math"
)

// Heuristic supplies the estimate FindPath orders its search by: for each
// tile, a guess of the cost of reaching the goal from it. A guess that never
// exceeds the true cost returns optimal routes; a closer guess expands fewer
// nodes. FindPath never reopens a node it has closed, so a guess that jumps
// between neighbouring tiles costs route quality, never correctness: the
// search still reads walkability, step costs and occupancy from the terrain.
type Heuristic interface {
	// ForSearch returns the estimate for one search from start to goal.
	// FindPath calls it once per search that runs, after the goal rejections
	// that need no search. The returned Estimate may keep per-search state and
	// is used by that search only.
	ForSearch(start, goal Coord) Estimate
}

// Estimate returns the estimated cost of reaching one search's goal from tile.
type Estimate func(tile Coord) float64

// octile returns the octile distance between two coords, where a diagonal move
// costs sqrt(2) and a cardinal move 1. It is FindPath's estimate when no
// Heuristic is configured, and exact on open ground at speed 1.0.
func octile(from, to Coord) float64 {
	dx := math.Abs(float64(to.X - from.X))
	dy := math.Abs(float64(to.Y - from.Y))
	return cardinalCost*math.Max(dx, dy) + (diagonalCost-cardinalCost)*math.Min(dx, dy)
}

// ScaledOctile is octile distance divided by the fastest speed multiplier any
// tile reports. It never overestimates, so FindPath returns optimal routes even
// onto terrain faster than 1.0, such as roads; the price is a weaker estimate
// everywhere, about 0.8 x length² expansions on open grass at a declared speed
// of 2.0. See docs/pathfinding_performance.md.
type ScaledOctile struct {
	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. It must be positive. Declaring less than the fastest tile brings
	// back the overestimate, so routes onto those tiles can be missed silently;
	// declaring more keeps routes optimal and only costs expansions.
	MaxSpeed float64
}

var _ Heuristic = ScaledOctile{}

// ForSearch returns octile distance to goal divided by MaxSpeed. It panics when
// MaxSpeed is not positive.
func (s ScaledOctile) ForSearch(start, goal Coord) Estimate {
	if !(s.MaxSpeed > 0) {
		panic(fmt.Sprintf("finding path from %v to %v: ScaledOctile.MaxSpeed must be positive, got %v", start, goal, s.MaxSpeed))
	}
	maxSpeed := s.MaxSpeed
	return func(tile Coord) float64 { return octile(tile, goal) / maxSpeed }
}
