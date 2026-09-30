package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
	"github.com/trancecode/vantage/tilemap"
)

// CanReachTile reports whether entityId can move onto tile: the tile must be
// in bounds and walkable (always true when Terrain is nil) and its centre
// available to the entity (always true when Occupancy is nil).
func (s *System) CanReachTile(entityId ecs.EntityId, tile tilemap.TileCoord) bool {
	return s.CanReach(entityId, tilemap.TileToWorldPosition(tile))
}

// CanReach reports whether entityId can move to destination: the tile holding
// it must be in bounds and walkable (always true when Terrain is nil) and
// destination available to the entity (always true when Occupancy is nil). It
// checks only the destination, not the way there; use FindPathBetween or
// WalkIsClear for that.
func (s *System) CanReach(entityId ecs.EntityId, destination geometry.Vector2) bool {
	tile := tilemap.WorldPositionToTile(destination)
	if s.Terrain != nil && (!s.Terrain.IsInBounds(tile.X, tile.Y) || !s.Terrain.IsWalkable(tile.X, tile.Y)) {
		return false
	}
	return s.Occupancy == nil || s.Occupancy.Available(entityId, destination)
}

// FindTilePath finds a tile path from start to goal using A* over the
// System's Terrain, routing around tiles reserved in a tile ledger Occupancy.
// It returns nil when no path exists or when the search exhausts
// MaxPathExpansions first. Terrain and MaxPathExpansions must be set; FindTilePath panics
// otherwise.
func (s *System) FindTilePath(start, goal tilemap.TileCoord) []tilemap.TileCoord {
	if s.Terrain == nil {
		panic("finding tile path: System.Terrain is nil")
	}
	if s.MaxPathExpansions <= 0 {
		panic(fmt.Sprintf("finding tile path from %v to %v: MaxPathExpansions not configured", start, goal))
	}

	startCoord := pathfinding.Coord{X: start.X, Y: start.Y}
	goalCoord := pathfinding.Coord{X: goal.X, Y: goal.Y}

	var isOccupied pathfinding.OccupancyChecker
	if ledger, ok := s.ledger(); ok {
		isOccupied = func(coord pathfinding.Coord) bool {
			_, occupied := ledger.GetOccupant(tilemap.TileCoord{X: coord.X, Y: coord.Y})
			return occupied
		}
	}

	path, _ := pathfinding.FindPath(s.Terrain, startCoord, goalCoord, isOccupied, s.MaxPathExpansions, s.Heuristic)
	if path == nil {
		return nil
	}

	result := make([]tilemap.TileCoord, len(path))
	for i, coord := range path {
		result[i] = tilemap.TileCoord{X: coord.X, Y: coord.Y}
	}
	return result
}

// FindPathBetween returns a sequence of world positions to move through in
// order to reach destination from origin, based on FindTilePath. The origin
// tile is skipped when origin already sits at its center, and the final
// waypoint is constrained to the goal tile's center so entities stay
// grid-aligned. It returns an empty slice when no path exists. A request
// whose origin and destination fall in the same tile returns a single
// waypoint at that tile's center (empty when the origin already sits at the
// center).
func (s *System) FindPathBetween(origin, destination geometry.Vector2) []geometry.Vector2 {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("pathfinding", time.Since(start)) }(time.Now())
	}

	startTile := tilemap.WorldPositionToTile(origin)
	goalTile := tilemap.WorldPositionToTile(destination)

	// A request within a single tile cannot be planned by the tile-level
	// pathfinder (start and goal are the same node), so handle it directly:
	// steer to the tile's center, the grid-aligned position the multi-tile
	// case also converges on. No occupancy check: the entity already stands
	// in this tile.
	if startTile == goalTile {
		tileCenter := tilemap.TileToWorldPosition(goalTile)
		if origin.DistanceTo(tileCenter) < 0.01 {
			return []geometry.Vector2{}
		}
		return []geometry.Vector2{tileCenter}
	}

	tilePath := s.FindTilePath(startTile, goalTile)
	if len(tilePath) == 0 {
		return []geometry.Vector2{}
	}

	worldPath := make([]geometry.Vector2, 0, len(tilePath))

	// Skip the first tile if we are already at its center, preventing
	// unnecessary micro-movements.
	startIndex := 0
	if origin.DistanceTo(tilemap.TileToWorldPosition(tilePath[0])) < 0.01 {
		startIndex = 1
	}

	for i := startIndex; i < len(tilePath); i++ {
		worldPath = append(worldPath, tilemap.TileToWorldPosition(tilePath[i]))
	}

	// Constrain the final destination to the goal tile's center so entities
	// always move to grid-aligned positions.
	if len(worldPath) > 0 {
		lastTileCenter := tilemap.TileToWorldPosition(tilePath[len(tilePath)-1])
		if worldPath[len(worldPath)-1].DistanceTo(lastTileCenter) > 0.01 {
			worldPath[len(worldPath)-1] = lastTileCenter
		}
	}

	return worldPath
}
