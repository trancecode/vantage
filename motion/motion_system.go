package motion

import (
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/pathfinding"
	"github.com/trancecode/vantage/tilemap"
)

// System advances moving entities and starts new moves. It bundles the
// component handles and spatial indexes movement operates on; the consuming
// game constructs one per world.
//
// Tick satisfies the sim.TickSystem interface, so a System can be registered
// on a sim.Driver directly or wrapped by a game tick system that also
// advances the game clock and records metrics.
//
// Game policy stays with the caller: System never touches entity states or
// AI scheduling. Move attempts report what happened through MoveStart so the
// game can update its own components and log with its own logger.
type System struct {
	// Spatials accesses each entity's position and facing.
	Spatials ecs.Accessor[Spatial]

	// Movements accesses each entity's in-progress move.
	Movements ecs.Accessor[Movement]

	// Grid, when non-nil, is kept in sync as entities move.
	Grid *tilemap.SpatialGrid

	// Occupancy, when non-nil, tracks tile reservations: MoveEntity refuses
	// destinations reserved by another entity and moves the reservation as
	// the entity departs.
	Occupancy *tilemap.TileOccupancyManager

	// Terrain provides walkability for CanReach and the pathfinding helpers.
	// FindTilePath, FindPathBetween, MoveEntityTowards and
	// MoveEntityTowardsArea require it and panic when it is nil; a nil
	// Terrain makes CanReach treat every tile as walkable.
	Terrain pathfinding.TerrainProvider

	// RecordPhase, when non-nil, receives wall time spent in instrumented
	// hot spots ("pathfinding", "move_towards_area") so games can feed their
	// benchmark reports.
	RecordPhase func(name string, elapsed time.Duration)

	// MaxMoveActionDistance caps how far a single MoveEntityTowards step may
	// reach, in tiles. Games use a value just above sqrt(2) for
	// one-tile-per-action movement including diagonals. Required (> 0) by
	// MoveEntityTowards and MoveEntityTowardsArea.
	MaxMoveActionDistance float64

	// OnArrival, when non-nil, is called for each entity that reaches its
	// destination during a Tick, after its Movement has been removed.
	OnArrival func(MovementResult)
}

// Tick moves every entity that has a Movement by elapsed game time. Entities
// that reach their destination have their Movement removed and are reported
// through OnArrival. Entities with a Movement but no Spatial are skipped.
func (s *System) Tick(elapsed time.Duration) {
	// Collect completed movements and remove them after the loop so removal
	// order does not depend on iteration order.
	var completed []MovementResult

	for id, mc := range s.Movements.All() {
		sc, ok := s.Spatials.Get(id)
		if !ok {
			continue
		}

		original := sc.Position
		newPosition, done := ProcessMovement(sc.Position, mc.Destination, mc.Speed, elapsed)
		sc.Position = newPosition

		if s.Grid != nil {
			s.Grid.UpdateEntityPosition(id, original, newPosition)
		}

		if done {
			completed = append(completed, MovementResult{
				EntityId:         id,
				OriginalPosition: original,
				NewPosition:      newPosition,
				Completed:        true,
			})
		}
	}

	for _, result := range completed {
		s.Movements.Remove(result.EntityId)
		if s.OnArrival != nil {
			s.OnArrival(result)
		}
	}
}
