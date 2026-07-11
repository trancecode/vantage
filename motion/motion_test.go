package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
)

func TestProcessMovement_AlreadyAtDestination(t *testing.T) {
	currentPos := geometry.NewVector2(5.0, 5.0)
	destination := geometry.NewVector2(5.0, 5.0)
	speed := 1.0
	duration := time.Second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	if newPos != currentPos {
		t.Errorf("Expected position to remain at %v, got %v", currentPos, newPos)
	}

	if !completed {
		t.Error("Expected movement to be completed when already at destination")
	}
}

func TestProcessMovement_ReachesDestination(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(1.0, 0.0)
	speed := 2.0 // Fast enough to reach destination in 1 second
	duration := time.Second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	if newPos != destination {
		t.Errorf("Expected to reach destination %v, got %v", destination, newPos)
	}

	if !completed {
		t.Error("Expected movement to be completed")
	}
}

func TestProcessMovement_PartialMovement(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(10.0, 0.0)
	speed := 1.0
	duration := time.Second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	// After 1 second at speed 1.0, should move 1 unit
	expectedPos := geometry.NewVector2(1.0, 0.0)

	if newPos.DistanceTo(expectedPos) > 0.0001 {
		t.Errorf("Expected position %v, got %v", expectedPos, newPos)
	}

	if completed {
		t.Error("Expected movement to not be completed")
	}
}

func TestProcessMovement_DiagonalMovement(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(3.0, 4.0) // Distance = 5.0
	speed := 5.0                                 // Will reach in 1 second
	duration := time.Second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	// Should reach destination (with some floating point tolerance)
	distanceToDestination := newPos.DistanceTo(destination)
	if distanceToDestination > 0.01 {
		t.Errorf("Expected to reach destination %v, got %v (distance: %v)", destination, newPos, distanceToDestination)
	}

	if !completed {
		t.Errorf("Expected movement to be completed (final distance to destination: %v)", distanceToDestination)
	}
}

func TestProcessMovement_HalfSecondMovement(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(10.0, 0.0)
	speed := 2.0
	duration := 500 * time.Millisecond // Half second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	// After 0.5 seconds at speed 2.0, should move 1 unit
	expectedPos := geometry.NewVector2(1.0, 0.0)

	if newPos.DistanceTo(expectedPos) > 0.0001 {
		t.Errorf("Expected position %v, got %v", expectedPos, newPos)
	}

	if completed {
		t.Error("Expected movement to not be completed")
	}
}

func TestProcessMovement_MultipleStepsToDestination(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(5.0, 0.0)
	speed := 1.0
	duration := time.Second

	// Step 1: Move 1 unit
	newPos1, completed1 := ProcessMovement(currentPos, destination, speed, duration)
	if completed1 {
		t.Error("Step 1: Expected movement to not be completed")
	}
	expectedPos1 := geometry.NewVector2(1.0, 0.0)
	if newPos1.DistanceTo(expectedPos1) > 0.0001 {
		t.Errorf("Step 1: Expected position (1.0, 0.0), got %v", newPos1)
	}

	// Step 2: Move another 1 unit
	newPos2, completed2 := ProcessMovement(newPos1, destination, speed, duration)
	if completed2 {
		t.Error("Step 2: Expected movement to not be completed")
	}
	expectedPos2 := geometry.NewVector2(2.0, 0.0)
	if newPos2.DistanceTo(expectedPos2) > 0.0001 {
		t.Errorf("Step 2: Expected position (2.0, 0.0), got %v", newPos2)
	}

	// Continue until destination...
	currentPos = newPos2
	for range 10 { // Max 10 iterations to prevent infinite loop
		newPos, completed := ProcessMovement(currentPos, destination, speed, duration)
		currentPos = newPos
		if completed {
			if currentPos.DistanceTo(destination) > 0.0001 {
				t.Errorf("Final position should be destination %v, got %v", destination, currentPos)
			}
			return
		}
	}

	t.Error("Should have reached destination within 10 steps")
}

func TestProcessMovement_OvershotPrevention(t *testing.T) {
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(0.5, 0.0)
	speed := 2.0 // Fast enough to overshoot in 1 second
	duration := time.Second

	newPos, completed := ProcessMovement(currentPos, destination, speed, duration)

	// Should stop exactly at destination, not overshoot
	if newPos != destination {
		t.Errorf("Expected to stop at destination %v, got %v", destination, newPos)
	}

	if !completed {
		t.Error("Expected movement to be completed")
	}
}
