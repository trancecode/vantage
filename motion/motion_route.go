package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
	"github.com/trancecode/vantage/tilemap"
)

// FindRoute returns the points a body walks through from from to to, for
// bodies that stand anywhere rather than on tile centres. In order: the centre
// of from's tile unless from is exactly on it, the centres of the tiles between
// found by pathfinding.FindBodyPath, the centre of to's tile unless to is
// exactly on it, and to itself. The route never cuts a blocked corner and
// ignores reservations, since bodies pass through each other.
//
// Both end tiles' centres are what make the route walkable in straight legs:
// for a body of radius under 0.5 whose from and to are legal positions
// (WalkIsClear of a point to itself), every step between consecutive points
// passes WalkIsClear, so TightenRoute always finds a leg. A route running from
// the last centre between straight to to could graze a blocked corner of to's
// tile and stall a body one tile short.
//
// It reports false, with a nil route, when to's tile is out of bounds or not
// walkable, or when the search found nothing within MaxPathExpansions. When
// from equals to the route is empty. Terrain and MaxPathExpansions must be
// set; FindRoute panics otherwise. The time spent is recorded under the
// "pathfinding" phase.
func (s *System) FindRoute(from, to geometry.Vector2) ([]geometry.Vector2, bool) {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("pathfinding", time.Since(start)) }(time.Now())
	}
	if s.Terrain == nil {
		panic(fmt.Sprintf("finding route from %v to %v: System.Terrain is nil", from, to))
	}
	if s.MaxPathExpansions <= 0 {
		panic(fmt.Sprintf("finding route from %v to %v: MaxPathExpansions not configured", from, to))
	}

	if from == to {
		return []geometry.Vector2{}, true
	}

	startTile := tilemap.WorldPositionToTile(from)
	goalTile := tilemap.WorldPositionToTile(to)
	if !s.Terrain.IsInBounds(goalTile.X, goalTile.Y) || !s.Terrain.IsWalkable(goalTile.X, goalTile.Y) {
		return nil, false
	}

	var between []pathfinding.Coord
	if startTile != goalTile {
		path, _ := pathfinding.FindBodyPath(s.Terrain,
			pathfinding.Coord{X: startTile.X, Y: startTile.Y},
			pathfinding.Coord{X: goalTile.X, Y: goalTile.Y},
			s.MaxPathExpansions, s.Heuristic)
		if path == nil {
			return nil, false
		}
		between = path[1 : len(path)-1]
	}

	route := make([]geometry.Vector2, 0, len(between)+3)
	add := func(point geometry.Vector2) {
		last := from
		if len(route) > 0 {
			last = route[len(route)-1]
		}
		if point != last {
			route = append(route, point)
		}
	}
	add(tilemap.TileToWorldPosition(startTile))
	for _, coord := range between {
		add(tilemap.TileToWorldPosition(tilemap.TileCoord{X: coord.X, Y: coord.Y}))
	}
	add(tilemap.TileToWorldPosition(goalTile))
	add(to)
	return route, true
}
