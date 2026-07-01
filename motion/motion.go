package motion

import (
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

const (
	SpeedWalk = 1.0 // Speed in tiles per second
	SpeedRun  = 2.0 // Speed in tiles per second
)

// Movement holds an entity's in-progress move: where it is headed and how fast.
type Movement struct {
	// Destination is the target position the entity is moving towards.
	Destination geometry.Vector2

	// Speed is the movement speed in tiles per second.
	Speed float64
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
