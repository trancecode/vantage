package motion

import (
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
	"github.com/trancecode/vantage/tilemap"
)

// CanReachTile reports whether entityId can move onto tile: the tile must be
// in bounds and walkable (always true when Terrain is nil) and not reserved
// by another entity (always true when Occupancy is nil).
func (s *System) CanReachTile(entityId ecs.EntityId, tile tilemap.TileCoord) bool {
	if s.Terrain != nil && (!s.Terrain.IsInBounds(tile.X, tile.Y) || !s.Terrain.IsWalkable(tile.X, tile.Y)) {
		return false
	}
	if s.Occupancy != nil {
		if occupant, occupied := s.Occupancy.GetOccupant(tile); occupied {
			return occupant == entityId
		}
	}
	return true
}

// CanReach reports whether entityId can move onto the tile containing
// destination. It checks only the destination tile, not the path to it; use
// FindPathBetween for a full path check.
func (s *System) CanReach(entityId ecs.EntityId, destination geometry.Vector2) bool {
	return s.CanReachTile(entityId, tilemap.WorldPositionToTile(destination))
}

// FindTilePath finds a tile path from start to goal using A* over the
// System's Terrain, routing around tiles reserved in Occupancy. It returns
// nil when no path exists. Terrain must be set; FindTilePath panics otherwise.
func (s *System) FindTilePath(start, goal tilemap.TileCoord) []tilemap.TileCoord {
	if s.Terrain == nil {
		panic("finding tile path: System.Terrain is nil")
	}

	startCoord := pathfinding.Coord{X: start.X, Y: start.Y}
	goalCoord := pathfinding.Coord{X: goal.X, Y: goal.Y}

	isOccupied := func(coord pathfinding.Coord) bool {
		if s.Occupancy == nil {
			return false
		}
		_, occupied := s.Occupancy.GetOccupant(tilemap.TileCoord{X: coord.X, Y: coord.Y})
		return occupied
	}

	path := pathfinding.FindPath(s.Terrain, startCoord, goalCoord, isOccupied)
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
// grid-aligned. It returns an empty slice when no path exists.
func (s *System) FindPathBetween(origin, destination geometry.Vector2) []geometry.Vector2 {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("pathfinding", time.Since(start)) }(time.Now())
	}

	startTile := tilemap.WorldPositionToTile(origin)
	goalTile := tilemap.WorldPositionToTile(destination)

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
	} else if origin.DistanceTo(destination) > 0.01 {
		// Origin and destination share a tile: aim for that tile's center if
		// it is reachable.
		tileCenter := tilemap.TileToWorldPosition(tilemap.WorldPositionToTile(destination))
		if s.CanReach(ecs.EntityId{}, tileCenter) {
			worldPath = append(worldPath, tileCenter)
		}
	}

	return worldPath
}
