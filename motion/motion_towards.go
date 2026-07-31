package motion

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// MoveEntityTowards starts moving an entity one bounded step toward
// destination: it follows the A* path from FindPathBetween but only as far
// as MaxMoveActionDistance, respecting walkable terrain and tile
// reservations. When no waypoint along the path is directly reachable, it
// falls back to the reachable adjacent tile that gets closest to the path. A
// destination within the entity's current tile starts a move to that tile's
// center, unless the entity is already there.
//
// The returned MoveStart reports whether a step was started; callers use it
// to update entity states or fall through to a wait directive.
// MoveOutcomeNoPath means no walkable route toward destination was found (the
// entity is boxed in), which also covers the case where the entity is
// already exactly at destination. The entity must have a Spatial;
// MoveEntityTowards panics otherwise. MaxMoveActionDistance must be
// configured (> 0) and opts.Speed must be positive; MoveEntityTowards panics
// otherwise. The step moves under opts (average speed in tiles per second,
// and the easing curve shaping it).
func (s *System) MoveEntityTowards(entityId ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart {
	if s.MaxMoveActionDistance <= 0 {
		panic(fmt.Sprintf("moving entity %v towards destination: MaxMoveActionDistance not configured", entityId))
	}

	if opts.Speed <= 0 {
		panic(fmt.Sprintf("moving entity %v towards destination: speed must be positive, got %v", entityId, opts.Speed))
	}

	sc, ok := s.Spatials.Get(entityId)
	if !ok {
		panic(fmt.Sprintf("moving entity %v towards destination: no Spatial component", entityId))
	}

	currentPos := sc.Position

	path := s.FindPathBetween(currentPos, destination)
	if len(path) == 0 {
		// Either already at the destination or fully boxed in.
		return MoveStart{Outcome: MoveOutcomeNoPath, Destination: destination}
	}

	return s.moveAlongPath(entityId, currentPos, destination, path, opts)
}

// moveAlongPath starts one bounded step along an already planned path, which
// must be a non-empty route from currentPos to destination. Callers that
// planned the path themselves use it to move without planning again;
// MoveEntityTowards documents what a step is and when it is refused.
func (s *System) moveAlongPath(entityId ecs.EntityId, currentPos, destination geometry.Vector2, path []geometry.Vector2, opts MoveOptions) MoveStart {
	// Move to the first reachable waypoint within a single step.
	for _, waypoint := range path {
		if currentPos.DistanceTo(waypoint) > s.MaxMoveActionDistance {
			break
		}
		if s.CanReach(entityId, waypoint) {
			return s.MoveEntity(entityId, waypoint, opts)
		}
	}

	// No waypoint along the path is directly reachable: fall back to the
	// reachable adjacent tile that gets closest to the first waypoint.
	currentTile := tilemap.WorldPositionToTile(currentPos)
	var reachableTiles []tilemap.TileCoord

	maxTileDistance := int(math.Ceil(s.MaxMoveActionDistance))
	for dx := -maxTileDistance; dx <= maxTileDistance; dx++ {
		for dy := -maxTileDistance; dy <= maxTileDistance; dy++ {
			targetTile := tilemap.TileCoord{X: currentTile.X + dx, Y: currentTile.Y + dy}
			targetPos := tilemap.TileToWorldPosition(targetTile)

			distance := currentPos.DistanceTo(targetPos)
			if distance <= s.MaxMoveActionDistance && distance > 0.01 && s.CanReach(entityId, targetPos) {
				reachableTiles = append(reachableTiles, targetTile)
			}
		}
	}

	if len(reachableTiles) > 0 {
		bestTile := reachableTiles[0]
		bestDistance := math.MaxFloat64
		for _, tile := range reachableTiles {
			distanceToTarget := tilemap.TileToWorldPosition(tile).DistanceTo(path[0])
			if distanceToTarget < bestDistance {
				bestDistance = distanceToTarget
				bestTile = tile
			}
		}
		return s.MoveEntity(entityId, tilemap.TileToWorldPosition(bestTile), opts)
	}

	return MoveStart{Outcome: MoveOutcomeNoPath, Destination: destination}
}

// MoveEntityTowardsArea starts moving an entity one bounded step toward a
// circular area defined by center and radius (in tiles): it finds the
// reachable tile center inside the area that is closest to the entity and
// takes the same step toward it that MoveEntityTowards would.
//
// The returned MoveStart reports MoveOutcomeAtDestination when the entity is
// already inside the area and MoveOutcomeNoPath when no tile in the area is
// reachable (normal flow when every tile around the target is occupied). The
// entity must have a Spatial, MaxMoveActionDistance must be configured (> 0)
// and opts.Speed must be positive; MoveEntityTowardsArea panics otherwise.
// The step moves under opts (average speed in tiles per second, and the
// easing curve shaping it).
func (s *System) MoveEntityTowardsArea(entityId ecs.EntityId, center geometry.Vector2, radius float64, opts MoveOptions) MoveStart {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("move_towards_area", time.Since(start)) }(time.Now())
	}

	if s.MaxMoveActionDistance <= 0 {
		panic(fmt.Sprintf("moving entity %v towards area: MaxMoveActionDistance not configured", entityId))
	}

	if opts.Speed <= 0 {
		panic(fmt.Sprintf("moving entity %v towards area: speed must be positive, got %v", entityId, opts.Speed))
	}

	sc, ok := s.Spatials.Get(entityId)
	if !ok {
		panic(fmt.Sprintf("moving entity %v towards area: no Spatial component", entityId))
	}

	currentPos := sc.Position

	if currentPos.DistanceTo(center) <= radius {
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: currentPos}
	}

	target, path, found := s.findAreaTarget(currentPos, center, radius)
	if !found {
		return MoveStart{Outcome: MoveOutcomeNoPath, Destination: center}
	}
	return s.moveAlongPath(entityId, currentPos, target, path, opts)
}

// areaCandidate is a tile inside a target area that an entity could head for,
// paired with its distance to that entity.
type areaCandidate struct {
	center   geometry.Vector2
	distance float64
}

// findAreaTarget picks the tile center inside the circular area (center,
// radius in tiles) that an entity at currentPos should head for: the one
// closest to currentPos among the reachable tiles of the innermost ring that
// holds any. It returns that target along with the path to it, so the caller
// can step along the path rather than plan it a second time, and reports
// false when the area holds no reachable tile.
//
// Candidates that no path can end on — out of bounds, unwalkable or reserved
// — are discarded by tile lookup, and the rest are searched nearest first, so
// a decision normally costs a single search. What still costs a search that
// finds nothing is a candidate open enough that FindPath cannot reject it
// either — walkable, free, with an enterable tile beside it — yet cut off
// from the entity by terrain further away.
func (s *System) findAreaTarget(currentPos, center geometry.Vector2, radius float64) (target geometry.Vector2, path []geometry.Vector2, found bool) {
	centerTile := tilemap.WorldPositionToTile(center)
	originTile := tilemap.WorldPositionToTile(currentPos)
	maxTileDistance := int(math.Ceil(radius))

	// Check tiles in concentric squares of increasing distance from the
	// center; among reachable tiles at the same ring, prefer the one closest
	// to the entity.
	for ring := 0; ring <= maxTileDistance; ring++ {
		var tilesToCheck []tilemap.TileCoord

		if ring == 0 {
			tilesToCheck = append(tilesToCheck, centerTile)
		} else {
			for dx := -ring; dx <= ring; dx++ {
				for dy := -ring; dy <= ring; dy++ {
					// Only tiles on the perimeter of the square.
					if math.Abs(float64(dx)) != float64(ring) && math.Abs(float64(dy)) != float64(ring) {
						continue
					}
					tile := tilemap.TileCoord{X: centerTile.X + dx, Y: centerTile.Y + dy}
					if tilemap.TileToWorldPosition(tile).DistanceTo(center) <= radius {
						tilesToCheck = append(tilesToCheck, tile)
					}
				}
			}
		}

		candidates := make([]areaCandidate, 0, len(tilesToCheck))
		for _, tile := range tilesToCheck {
			if !s.canEndPathOnTile(tile, originTile) {
				continue
			}
			tileCenter := tilemap.TileToWorldPosition(tile)
			candidates = append(candidates, areaCandidate{
				center:   tileCenter,
				distance: currentPos.DistanceTo(tileCenter),
			})
		}

		// Sort stably so that candidates equally close to the entity keep
		// their scan order: consumers replay on the tile this picks.
		slices.SortStableFunc(candidates, func(a, b areaCandidate) int {
			return cmp.Compare(a.distance, b.distance)
		})

		for _, candidate := range candidates {
			path = s.FindPathBetween(currentPos, candidate.center)
			if len(path) == 0 {
				continue
			}
			return candidate.center, path, true
		}
	}

	return geometry.Vector2{}, nil, false
}

// canEndPathOnTile reports whether a path from an entity standing on
// originTile could end on tile, by tile lookup alone. It mirrors the
// rejections FindPathBetween makes without searching: nothing reaches a tile
// that is out of bounds, unwalkable or reserved. A tile the entity already
// stands on is exempt, matching FindPathBetween's within-tile case, which
// steers to the tile center whatever the tile holds.
func (s *System) canEndPathOnTile(tile, originTile tilemap.TileCoord) bool {
	if tile == originTile {
		return true
	}
	if s.Terrain == nil {
		// FindPathBetween panics without terrain; let it, rather than
		// swallowing the candidate here.
		return true
	}
	if !s.Terrain.IsInBounds(tile.X, tile.Y) || !s.Terrain.IsWalkable(tile.X, tile.Y) {
		return false
	}
	if s.Occupancy != nil {
		// Any reservation blocks the goal, including this entity's own:
		// pathfinding routes around reserved tiles without exception.
		if _, occupied := s.Occupancy.GetOccupant(tile); occupied {
			return false
		}
	}
	return true
}
