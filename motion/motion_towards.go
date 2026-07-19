package motion

import (
	"fmt"
	"math"
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
// configured (> 0); MoveEntityTowards panics otherwise. The step moves under
// opts (average speed in tiles per second, and the easing curve shaping it).
func (s *System) MoveEntityTowards(entityId ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart {
	if s.MaxMoveActionDistance <= 0 {
		panic(fmt.Sprintf("moving entity %v towards destination: MaxMoveActionDistance not configured", entityId))
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
// steps toward it via MoveEntityTowards.
//
// The returned MoveStart reports MoveOutcomeAtDestination when the entity is
// already inside the area and MoveOutcomeNoPath when no tile in the area is
// reachable (normal flow when every tile around the target is occupied). The
// entity must have a Spatial; MoveEntityTowardsArea panics otherwise. The step
// moves under opts (average speed in tiles per second, and the easing curve
// shaping it).
func (s *System) MoveEntityTowardsArea(entityId ecs.EntityId, center geometry.Vector2, radius float64, opts MoveOptions) MoveStart {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("move_towards_area", time.Since(start)) }(time.Now())
	}

	if s.MaxMoveActionDistance <= 0 {
		panic(fmt.Sprintf("moving entity %v towards area: MaxMoveActionDistance not configured", entityId))
	}

	sc, ok := s.Spatials.Get(entityId)
	if !ok {
		panic(fmt.Sprintf("moving entity %v towards area: no Spatial component", entityId))
	}

	currentPos := sc.Position

	if currentPos.DistanceTo(center) <= radius {
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: currentPos}
	}

	centerTile := tilemap.WorldPositionToTile(center)
	maxTileDistance := int(math.Ceil(radius))

	// Check tiles in concentric squares of increasing distance from the
	// center; among reachable tiles at the same ring, prefer the one closest
	// to the entity.
	var targetPos *geometry.Vector2
	minDistanceToEntity := math.MaxFloat64

	for distance := 0; distance <= maxTileDistance && targetPos == nil; distance++ {
		var tilesToCheck []tilemap.TileCoord

		if distance == 0 {
			tilesToCheck = append(tilesToCheck, centerTile)
		} else {
			for dx := -distance; dx <= distance; dx++ {
				for dy := -distance; dy <= distance; dy++ {
					// Only tiles on the perimeter of the square.
					if math.Abs(float64(dx)) != float64(distance) && math.Abs(float64(dy)) != float64(distance) {
						continue
					}
					tile := tilemap.TileCoord{X: centerTile.X + dx, Y: centerTile.Y + dy}
					if tilemap.TileToWorldPosition(tile).DistanceTo(center) <= radius {
						tilesToCheck = append(tilesToCheck, tile)
					}
				}
			}
		}

		for _, tile := range tilesToCheck {
			tileCenter := tilemap.TileToWorldPosition(tile)
			if len(s.FindPathBetween(currentPos, tileCenter)) == 0 {
				continue
			}
			if distToEntity := currentPos.DistanceTo(tileCenter); distToEntity < minDistanceToEntity {
				minDistanceToEntity = distToEntity
				targetPos = &tileCenter
			}
		}
	}

	if targetPos != nil {
		return s.MoveEntityTowards(entityId, *targetPos, opts)
	}
	return MoveStart{Outcome: MoveOutcomeNoPath, Destination: center}
}
