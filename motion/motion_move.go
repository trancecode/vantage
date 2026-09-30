package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
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
	// speed. Zero unless Outcome is MoveOutcomeStarted. It is truncated to
	// whole nanoseconds, so a move whose duration is under a nanosecond
	// reports zero.
	Duration time.Duration
}

// Started reports whether a move was actually set in motion.
func (m MoveStart) Started() bool { return m.Outcome == MoveOutcomeStarted }

// MoveOptions describes how a body moves: how fast overall, and how that speed
// is distributed over the move.
type MoveOptions struct {
	// Speed is the average movement speed in tiles per second. It must be
	// positive unless Duration is set; the move entry points panic otherwise.
	Speed float64

	// Duration, when positive, fixes how long the move takes, and Speed is then
	// not needed: the move runs at distance divided by Duration. Such a move is
	// timed (see Movement.Timed), so its position is independent of how game
	// time is sliced into ticks. Zero leaves the duration to Speed. The tile
	// route helpers (MoveEntityTowards, MoveEntityTowardsArea) still require
	// a positive Speed.
	Duration time.Duration

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
	//
	// An eased move owns the body's position for its duration: the eased
	// position is derived only from Start, Destination and progress, so
	// writing Spatial.Position mid-move (knockback, a collision push-out, a
	// debug teleport) is undone on the next tick, unlike a constant-speed
	// move, which continues from wherever the body was put. Displace a body
	// only after cancelling or restarting its move.
	Ease easing.Curve
}

// MoveEntity starts moving an entity toward destination under opts (average
// speed in tiles per second, and the easing curve shaping it). When the System
// has an Occupancy, the destination must be available to the entity, and
// Occupancy.Claim moves the entity's reservation to it as the move starts.
//
// The entity's facing direction is set toward the destination. The entity must
// have a Spatial and opts must carry a positive Speed or Duration; MoveEntity panics otherwise.
// A move started on an entity that is already moving is re-anchored from its
// current position, so the new move takes its full distance divided by its
// speed. MoveEntity is intended for entities settled on their reserved tile:
// redirecting an entity mid-move can strand its old destination reservation
// and clear a tile it no longer holds.
func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart {
	if opts.Speed <= 0 && opts.Duration <= 0 {
		panic(fmt.Sprintf("moving entity %v: needs a positive speed or duration, got speed %v and duration %v", id, opts.Speed, opts.Duration))
	}

	sc, ok := s.Spatials.Get(id)
	if !ok {
		panic(fmt.Sprintf("moving entity %v: no Spatial component", id))
	}

	if s.Occupancy != nil && !s.Occupancy.Claim(id, sc.Position, destination) {
		return MoveStart{Outcome: MoveOutcomeDestinationOccupied, Destination: destination}
	}

	if sc.Position == destination {
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: destination}
	}

	distance := sc.Position.DistanceTo(destination)
	speed := opts.Speed
	total := time.Duration(distance / speed * float64(time.Second))
	if opts.Duration > 0 {
		speed = distance / opts.Duration.Seconds()
		total = opts.Duration
	}

	// Re-anchor every parametric field: a stale Start or Total from a
	// previous move would make the body jump or arrive at the wrong time.
	mc := s.Movements.GetOrAdd(id, Movement{})
	mc.Destination = destination
	mc.Speed = speed
	mc.Ease = opts.Ease
	mc.Start = sc.Position
	mc.Elapsed = 0
	mc.Total = total
	mc.Timed = opts.Duration > 0
	sc.Direction = destination.Sub(sc.Position)

	return MoveStart{
		Outcome:     MoveOutcomeStarted,
		Destination: destination,
		Distance:    distance,
		Duration:    total,
	}
}

// CancelMove stops id's in-flight move where the body now stands, and reports
// whether there was one. It removes the Movement and, when the System has an
// Occupancy, moves the entity's claim from the cancelled destination to its
// current position through Occupancy.Stop. With no move in flight it changes
// nothing. Cancel before displacing a body (a push, a teleport): an eased or
// timed move would otherwise pull it back on its next tick. An entity with a
// Movement must have a Spatial; CancelMove panics otherwise.
func (s *System) CancelMove(id ecs.EntityId) bool {
	mc, ok := s.Movements.Get(id)
	if !ok {
		return false
	}
	destination := mc.Destination
	s.Movements.Remove(id)

	if s.Occupancy != nil {
		sc, ok := s.Spatials.Get(id)
		if !ok {
			panic(fmt.Sprintf("cancelling move of entity %v: no Spatial component", id))
		}
		s.Occupancy.Stop(id, destination, sc.Position)
	}
	return true
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
