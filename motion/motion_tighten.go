package motion

import (
	"fmt"
	"math"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
)

// TightenRoute returns where a body of radius standing at from should end its
// next leg along route, normally one from System.FindRoute: the farthest
// route point it can walk to in a straight line (WalkIsClear), shortened to
// maxLength along that line. Pass math.Inf(1) for no cap.
//
// It walks the route in order and stops at the first point it cannot walk to,
// and also right after the first point farther than max(maxLength, 1) from
// from, so a long route costs a handful of walk tests. The scan always looks
// at least one tile ahead, so the centre of the tile the body stands in, never
// more than sqrt(2)/2 away, cannot end it. It reports false when no point is
// walkable, or when the only one is from itself. Should rounding push the
// shortened end onto a blocked tile, it falls back to the farthest scanned
// point within maxLength, or reports false when there is none.
//
// For a radius under 0.5 and a route from FindRoute whose ends are legal
// positions, the first point is always walkable, so tightening always makes
// progress. A larger body can meet a gap narrower than itself on a route,
// since routes know nothing of clearance: tightening then stops short or
// reports false, but never returns a leg through a blocked tile. It panics on
// a negative radius or a maxLength that is not positive.
func TightenRoute(terrain pathfinding.TerrainProvider, from geometry.Vector2, route []geometry.Vector2, radius, maxLength float64) (geometry.Vector2, bool) {
	if radius < 0 {
		panic(fmt.Sprintf("tightening route from %v: radius must not be negative, got %v", from, radius))
	}
	if !(maxLength > 0) {
		panic(fmt.Sprintf("tightening route from %v: maxLength must be positive, got %v", from, maxLength))
	}

	lookahead := math.Max(maxLength, 1)
	var best geometry.Vector2
	found := false
	var withinCap geometry.Vector2
	haveWithinCap := false
	for _, point := range route {
		if !WalkIsClear(terrain, from, point, radius) {
			break
		}
		best, found = point, true
		distance := from.DistanceTo(point)
		if distance <= maxLength && point != from {
			withinCap, haveWithinCap = point, true
		}
		if distance > lookahead {
			break
		}
	}
	if !found || best == from {
		return geometry.Vector2{}, false
	}

	distance := from.DistanceTo(best)
	if distance <= maxLength {
		return best, true
	}
	capped := from.Lerp(best, maxLength/distance)
	if WalkIsClear(terrain, from, capped, radius) {
		return capped, true
	}
	return withinCap, haveWithinCap
}
