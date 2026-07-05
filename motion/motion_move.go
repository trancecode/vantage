package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// MoveOutcome classifies the result of trying to start a move.
type MoveOutcome int

const (
	// MoveOutcomeNone is the uninitialized value.
	MoveOutcomeNone MoveOutcome = iota

	// MoveOutcomeStarted means a Movement toward the destination was set.
	MoveOutcomeStarted

	// MoveOutcomeAtDestination means the entity already is where it was asked
	// to go (or, for area moves, already inside the target area).
	MoveOutcomeAtDestination

	// MoveOutcomeDestinationOccupied means another entity has reserved the
	// destination tile. Normal AI flow during dense crowds, not an error.
	MoveOutcomeDestinationOccupied

	// MoveOutcomeNoPath means no walkable route toward the destination was
	// found. Normal flow when crowds block the entity in.
	MoveOutcomeNoPath
)

// MoveStart reports the outcome of trying to start a move so the caller can
// update game state (entity states, AI scheduling) and log the attempt.
type MoveStart struct {
	// Outcome classifies what happened.
	Outcome MoveOutcome

	// Destination is the position the move targets (the actual waypoint for
	// path-following moves, which may differ from the requested target).
	Destination geometry.Vector2

	// Distance is the length of the started move. Zero unless Outcome is
	// MoveOutcomeStarted.
	Distance float64

	// Duration is the game time the started move will take at the requested
	// speed. Zero unless Outcome is MoveOutcomeStarted.
	Duration time.Duration
}

// Started reports whether a move was actually set in motion.
func (m MoveStart) Started() bool { return m.Outcome == MoveOutcomeStarted }

// MoveEntity starts moving an entity toward destination at speed (in tiles
// per second). When the System has an Occupancy manager, the destination tile
// must be free (or reserved by this entity); the entity's reservation moves
// from its current tile to the destination tile as the move starts.
//
// The entity's facing direction is set toward the destination. The entity
// must have a Spatial; MoveEntity panics otherwise. speed must be positive,
// otherwise the returned Duration is meaningless. MoveEntity is intended for
// entities settled on their reserved tile: redirecting an entity mid-move can
// strand its old destination reservation and clear a tile it no longer
// holds.
func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, speed float64) MoveStart {
	sc, ok := s.Spatials.Get(id)
	if !ok {
		panic(fmt.Sprintf("moving entity %v: no Spatial component", id))
	}

	// Refuse destinations reserved by another entity.
	if s.Occupancy != nil {
		destTile := tilemap.WorldPositionToTile(destination)
		if occupant, occupied := s.Occupancy.GetOccupant(destTile); occupied && occupant != id {
			return MoveStart{Outcome: MoveOutcomeDestinationOccupied, Destination: destination}
		}
		s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(sc.Position))
	}

	if sc.Position == destination {
		// Keep the entity's reservation on its current tile.
		if s.Occupancy != nil {
			s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(destination), id)
		}
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: destination}
	}

	mc := s.Movements.GetOrAdd(id, Movement{})
	mc.Destination = destination
	mc.Speed = speed
	sc.Direction = destination.Sub(sc.Position)

	if s.Occupancy != nil {
		s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(destination), id)
	}

	distance := sc.Position.DistanceTo(destination)
	return MoveStart{
		Outcome:     MoveOutcomeStarted,
		Destination: destination,
		Distance:    distance,
		Duration:    time.Duration(distance / speed * float64(time.Second)),
	}
}

// FaceDirection sets an entity's facing direction without moving it. The
// entity must have a Spatial; FaceDirection panics otherwise.
func (s *System) FaceDirection(id ecs.EntityId, direction geometry.Vector2) {
	sc, ok := s.Spatials.Get(id)
	if !ok {
		panic(fmt.Sprintf("facing entity %v: no Spatial component", id))
	}
	sc.Direction = direction
}
