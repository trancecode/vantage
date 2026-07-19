package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/easing"
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

// MoveOptions describes how a body moves: how fast overall, and how that speed
// is distributed over the move.
type MoveOptions struct {
	// Speed is the average movement speed in tiles per second. It must be
	// positive; the move entry points panic otherwise.
	Speed float64

	// Ease shapes the speed over the move's duration. The zero value,
	// easing.CurveLinear, is constant speed, which is what every move did
	// before easing existed.
	//
	// The curve spans the move as issued, so a single move across many
	// tiles accelerates once over the whole distance; a game wanting a
	// curve per step issues a move per step, as MoveEntityTowards does.
	// Easing suits committed, point-to-point moves: redirecting a move
	// re-anchors its curve, so a body retargeted every tick under a
	// symmetric curve never leaves the slow opening of the curve. Use
	// easing.CurveLinear for continuous steering and pursuit.
	Ease easing.Curve
}

// MoveEntity starts moving an entity toward destination under opts (average
// speed in tiles per second, and the easing curve shaping it). When the System
// has an Occupancy manager, the destination tile must be free (or reserved by
// this entity); the entity's reservation moves from its current tile to the
// destination tile as the move starts.
//
// The entity's facing direction is set toward the destination. The entity must
// have a Spatial and opts.Speed must be positive; MoveEntity panics otherwise.
// A move started on an entity that is already moving is re-anchored from its
// current position, so the new move takes its full distance divided by its
// speed. MoveEntity is intended for entities settled on their reserved tile:
// redirecting an entity mid-move can strand its old destination reservation
// and clear a tile it no longer holds.
func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart {
	if opts.Speed <= 0 {
		panic(fmt.Sprintf("moving entity %v: speed must be positive, got %v", id, opts.Speed))
	}

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

	distance := sc.Position.DistanceTo(destination)
	total := time.Duration(distance / opts.Speed * float64(time.Second))

	// Re-anchor every parametric field: a stale Start or Total from a
	// previous move would make the body jump or arrive at the wrong time.
	mc := s.Movements.GetOrAdd(id, Movement{})
	mc.Destination = destination
	mc.Speed = opts.Speed
	mc.Ease = opts.Ease
	mc.Start = sc.Position
	mc.Elapsed = 0
	mc.Total = total
	sc.Direction = destination.Sub(sc.Position)

	if s.Occupancy != nil {
		s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(destination), id)
	}

	return MoveStart{
		Outcome:     MoveOutcomeStarted,
		Destination: destination,
		Distance:    distance,
		Duration:    total,
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
