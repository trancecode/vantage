package motion

import (
	"fmt"
	"math"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
)

// walkScanMargin widens the tile rows and columns WalkIsClear considers, so
// that floating-point rounding in the slab arithmetic does not skip a tile the
// capsule touches exactly on its boundary. The margin is absolute, so it
// covers that rounding only up to tile coordinates of about 2^25; beyond
// that, rounding can exceed it. Every tile it adds is still tested exactly,
// so the margin costs a few extra checks and changes no answer.
const walkScanMargin = 1e-9

// WalkIsClear reports whether a round body of radius can walk in a straight
// line from from to to over terrain: whether the capsule the body sweeps
// touches no tile that is out of bounds or not walkable. Touching counts as
// blocked. The capsule includes the body at both ends, so a body already
// touching a blocked tile cannot walk anywhere, and WalkIsClear(terrain, p, p,
// radius) tells whether a body fits at p.
//
// It visits the tile columns the capsule spans and, in each, only the rows the
// capsule can reach there, so its cost grows with the length of the walk
// rather than with the area of its bounding box. It panics on a negative
// radius.
func WalkIsClear(terrain pathfinding.TerrainProvider, from, to geometry.Vector2, radius float64) bool {
	if radius < 0 {
		panic(fmt.Sprintf("testing walk from %v to %v: radius must not be negative, got %v", from, to, radius))
	}

	reach := radius + walkScanMargin
	minX := int(math.Floor(math.Min(from.X(), to.X()) - reach))
	maxX := int(math.Floor(math.Max(from.X(), to.X()) + reach))
	for x := minX; x <= maxX; x++ {
		low, high := segmentYRange(from, to, float64(x)-reach, float64(x+1)+reach)
		for y := int(math.Floor(low - reach)); y <= int(math.Floor(high+reach)); y++ {
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				continue
			}
			tile := geometry.NewRectangleFromPoints(x, y, x+1, y+1)
			if geometry.CapsuleTouchesRectangle(from, to, radius, tile) {
				return false
			}
		}
	}
	return true
}

// segmentYRange returns the lowest and highest y of the segment from a to b
// over the part of it whose x lies in [x0, x1]. The capsule's points in the
// tile column [x0 + reach, x1 - reach] lie within the radius of that part.
// When the slab misses the segment by a rounding margin, the range collapses
// onto the nearest end, which is still a safe place to look.
func segmentYRange(a, b geometry.Vector2, x0, x1 float64) (low, high float64) {
	if a.X() == b.X() {
		return math.Min(a.Y(), b.Y()), math.Max(a.Y(), b.Y())
	}
	lo := math.Max(x0, math.Min(a.X(), b.X()))
	hi := math.Min(x1, math.Max(a.X(), b.X()))
	if lo > hi {
		lo = hi
	}
	yAt := func(x float64) float64 {
		t := (x - a.X()) / (b.X() - a.X())
		return a.Y() + t*(b.Y()-a.Y())
	}
	y0, y1 := yAt(lo), yAt(hi)
	return math.Min(y0, y1), math.Max(y0, y1)
}
