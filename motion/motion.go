package motion

import (
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
)

const (
	SpeedWalk = 1.0 // Speed in tiles per second
	SpeedRun  = 2.0 // Speed in tiles per second
)

// Movement holds an entity's in-progress move: where it is headed, how fast,
// and how its speed is shaped over the move.
//
// Every field is plain data so consumers can persist an in-flight move in a
// savegame. A Movement decoded without the easing fields is a constant-speed
// move, which is what the engine did before easing existed.
type Movement struct {
	// Destination is the target position the entity is moving towards.
	Destination geometry.Vector2

	// Speed is the movement speed in tiles per second. On an eased move it
	// is the average speed rather than the instantaneous one: the total
	// duration is still the distance divided by Speed.
	Speed float64

	// Ease shapes progress along the move. The zero value, easing.
	// CurveLinear, selects the incremental constant-speed path and ignores
	// the fields below.
	Ease easing.Curve

	// Start is the position the move began from. Eased positions are
	// computed from it, so starting or redirecting a move re-anchors it.
	Start geometry.Vector2

	// Elapsed is the game time spent on this move so far.
	Elapsed time.Duration

	// Total is the game time the move takes end to end, fixed when the move
	// starts as the distance divided by Speed.
	Total time.Duration
}

// Progress reports how far through its duration the move is, from 0 to 1.
// Games use it to drive animation, which cannot read the rate off Speed on an
// eased move. It returns 0 for a move with no recorded Total, such as one
// decoded from a save written before easing existed.
//
// On a constant-speed move Progress is informational: arrival there is decided
// by the incremental overshoot check, so Progress can read slightly under 1 on
// the tick the move completes.
func (m Movement) Progress() float64 {
	if m.Total <= 0 {
		return 0
	}
	if m.Elapsed >= m.Total {
		return 1
	}
	return float64(m.Elapsed) / float64(m.Total)
}

// MovementResult represents the result of processing a single entity's movement.
type MovementResult struct {
	EntityId         ecs.EntityId
	NewPosition      geometry.Vector2
	Completed        bool
	OriginalPosition geometry.Vector2
}

// ProcessMovement calculates the new position for a moving entity based on elapsed time.
// It returns a MovementResult indicating the new position and whether movement is complete.
//
// Parameters:
//   - currentPosition: The entity's current position
//   - destination: The target position
//   - speed: Movement speed in tiles per second
//   - duration: Time elapsed since last update
//
// Returns:
//   - newPosition: The calculated new position
//   - completed: true if the entity has reached or passed the destination
func ProcessMovement(currentPosition, destination geometry.Vector2, speed float64, duration time.Duration) (newPosition geometry.Vector2, completed bool) {
	// Check if entity has already reached its destination
	if currentPosition == destination {
		return currentPosition, true
	}

	// A zero-duration tick moves nothing and must not complete the move:
	// with no progress, distanceAfter equals distanceBefore and the
	// overshoot check below would misread the stall as an arrival.
	if duration <= 0 {
		return currentPosition, false
	}

	direction := destination.Sub(currentPosition).Unit()

	// Calculate the movement vector based on direction, speed, and elapsed time.
	movement := direction.Scale(speed * duration.Seconds())

	// Calculate the new position after applying the movement.
	newPos := currentPosition.Add(movement)

	// Calculate the distance to the destination before and after the move.
	distanceBefore := currentPosition.DistanceTo(destination)
	distanceAfter := newPos.DistanceTo(destination)

	// Use a small tolerance for floating point comparison
	const tolerance = 0.0001

	// Check if the entity has reached or moved past the destination.
	// This includes cases where floating point arithmetic results in the entity
	// being very close to the destination.
	if distanceAfter < tolerance || distanceAfter >= distanceBefore {
		// If so, set the position directly to the destination and mark as completed.
		return destination, true
	}

	// Otherwise, return the new position (not yet at destination).
	return newPos, false
}

// ProcessMove advances a move by duration and returns the movement with its
// Elapsed advanced, the entity's new position, and whether the move completed.
// It routes constant-speed moves to ProcessMovement and eased moves to the
// parametric formula, where position is a pure function of the move's start,
// destination and progress, and therefore independent of how the elapsed game
// time was sliced into ticks.
//
// A zero or negative duration moves nothing and never completes a move, on
// either path.
func ProcessMove(mc Movement, currentPosition geometry.Vector2, duration time.Duration) (updated Movement, newPosition geometry.Vector2, completed bool) {
	if duration <= 0 {
		return mc, currentPosition, false
	}

	if mc.Ease == easing.CurveLinear {
		newPosition, completed = ProcessMovement(currentPosition, mc.Destination, mc.Speed, duration)
		mc.Elapsed += duration
		return mc, newPosition, completed
	}

	// A move with no duration to spread the curve over has nowhere to be
	// except its destination.
	if mc.Total <= 0 {
		mc.Elapsed = mc.Total
		return mc, mc.Destination, true
	}

	mc.Elapsed += duration
	if mc.Elapsed >= mc.Total {
		mc.Elapsed = mc.Total
		return mc, mc.Destination, true
	}

	eased := mc.Ease.Apply(float64(mc.Elapsed) / float64(mc.Total))
	return mc, mc.Start.Lerp(mc.Destination, eased), false
}
