package motion

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"
	"time"

	"github.com/trancecode/vantage/easing"
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

func TestProcessMovement_ZeroDurationMakesNoProgress(t *testing.T) {
	// A zero-elapsed tick must not complete an in-flight move: with no time
	// passed there is no movement, and the distanceAfter >= distanceBefore
	// overshoot check would otherwise misread "no progress" as "arrived",
	// teleporting the entity to its destination. Consuming games hit this
	// when their loop resumes on an instant that still has queued events
	// (lockstep's pause-at-ready-instant pattern).
	currentPos := geometry.NewVector2(0.0, 0.0)
	destination := geometry.NewVector2(3.0, 0.0)

	newPos, completed := ProcessMovement(currentPos, destination, 1.0, 0)

	if completed {
		t.Error("zero-duration tick must not complete an in-flight move")
	}
	if newPos != currentPos {
		t.Errorf("zero-duration tick must not move the entity: got %v, want %v", newPos, currentPos)
	}
}

func TestProcessMovement_ZeroDurationAtDestinationStaysCompleted(t *testing.T) {
	// Already standing on the destination is complete regardless of the
	// tick's duration.
	pos := geometry.NewVector2(2.0, 2.0)

	newPos, completed := ProcessMovement(pos, pos, 1.0, 0)

	if !completed {
		t.Error("an entity already at its destination is complete on any tick")
	}
	if newPos != pos {
		t.Errorf("position must not change: got %v, want %v", newPos, pos)
	}
}

// easedMovement builds an eased Movement from start to dest at speed, with
// Total derived the way MoveEntity derives it.
func easedMovement(start, dest geometry.Vector2, speed float64, curve easing.Curve) Movement {
	distance := start.DistanceTo(dest)
	return Movement{
		Destination: dest,
		Speed:       speed,
		Ease:        curve,
		Start:       start,
		Total:       time.Duration(distance / speed * float64(time.Second)),
	}
}

func TestProcessMove_LinearMatchesProcessMovement(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(10.0, 0.0)
	mc := Movement{Destination: dest, Speed: 2.0, Start: start, Total: 5 * time.Second}

	wantPos, wantDone := ProcessMovement(start, dest, 2.0, 500*time.Millisecond)
	updated, gotPos, gotDone := ProcessMove(mc, start, 500*time.Millisecond)

	if gotPos != wantPos || gotDone != wantDone {
		t.Errorf("ProcessMove = (%v, %v), want (%v, %v)", gotPos, gotDone, wantPos, wantDone)
	}
	if updated.Elapsed != 500*time.Millisecond {
		t.Errorf("expected Elapsed 500ms on the linear path, got %v", updated.Elapsed)
	}
}

// TestProcessMove_LinearZeroDurationAtDestinationStaysCompleted pins that
// ProcessMove routes a constant-speed move already at its destination to
// ProcessMovement's own zero-duration rule: an entity already standing on its
// destination is complete regardless of the tick's duration. Before FIX 1
// this was swallowed by a duration <= 0 guard that ran ahead of the
// constant-speed branch, so a Movement restored (or authored) with
// Destination already equal to the body's position never completed on a
// zero-length tick.
func TestProcessMove_LinearZeroDurationAtDestinationStaysCompleted(t *testing.T) {
	pos := geometry.NewVector2(2.0, 2.0)
	mc := Movement{Destination: pos, Speed: 1.0}

	updated, newPos, completed := ProcessMove(mc, pos, 0)

	if !completed {
		t.Error("a constant-speed move already at its destination must complete on any tick, including a zero-duration one")
	}
	if newPos != pos {
		t.Errorf("position must not change: got %v, want %v", newPos, pos)
	}
	if updated.Elapsed != 0 {
		t.Errorf("a zero-duration tick must not advance Elapsed, got %v", updated.Elapsed)
	}
}

// TestProcessMove_LinearZeroDurationInFlightMakesNoProgress mirrors
// ProcessMovement's own zero-duration rule for an in-flight move: no time
// passed means no movement and no completion.
func TestProcessMove_LinearZeroDurationInFlightMakesNoProgress(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(3.0, 0.0)
	mc := Movement{Destination: dest, Speed: 1.0}

	updated, newPos, completed := ProcessMove(mc, start, 0)

	if completed {
		t.Error("a zero-duration tick must not complete an in-flight constant-speed move")
	}
	if newPos != start {
		t.Errorf("a zero-duration tick must not move the entity: got %v, want %v", newPos, start)
	}
	if updated.Elapsed != 0 {
		t.Errorf("a zero-duration tick must not advance Elapsed, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedMidMovePosition(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut) // Total 4s

	// One second in: t = 0.25, smoothstep(0.25) = 0.15625, so x = 0.625.
	updated, pos, done := ProcessMove(mc, start, time.Second)

	if done {
		t.Error("expected the move to still be in flight")
	}
	if math.Abs(pos.X()-0.625) > 1e-12 || pos.Y() != 0 {
		t.Errorf("expected position (0.625, 0), got %v", pos)
	}
	if updated.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedInMidMovePosition(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveIn) // Total 4s

	// One second in: t = 0.25, CurveIn.Apply(0.25) = 0.25*0.25 = 0.0625, so
	// x = 4 * 0.0625 = 0.25.
	updated, pos, done := ProcessMove(mc, start, time.Second)

	if done {
		t.Error("expected the move to still be in flight")
	}
	if math.Abs(pos.X()-0.25) > 1e-12 || pos.Y() != 0 {
		t.Errorf("expected position (0.25, 0), got %v", pos)
	}
	if updated.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedOutMidMovePosition(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveOut) // Total 4s

	// One second in: t = 0.25, CurveOut.Apply(0.25) = 1-(1-0.25)^2 = 0.4375,
	// so x = 4 * 0.4375 = 1.75.
	updated, pos, done := ProcessMove(mc, start, time.Second)

	if done {
		t.Error("expected the move to still be in flight")
	}
	if math.Abs(pos.X()-1.75) > 1e-12 || pos.Y() != 0 {
		t.Errorf("expected position (1.75, 0), got %v", pos)
	}
	if updated.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedIsIndependentOfTickSlicing(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)

	coarse := easedMovement(start, dest, 1.0, easing.CurveOut)
	_, coarsePos, _ := ProcessMove(coarse, start, 2*time.Second)

	fine := easedMovement(start, dest, 1.0, easing.CurveOut)
	finePos := start
	for range 20 {
		fine, finePos, _ = ProcessMove(fine, finePos, 100*time.Millisecond)
	}

	if coarsePos.DistanceTo(finePos) > 1e-12 {
		t.Errorf("eased position depends on slicing: %v (one tick) vs %v (20 ticks)", coarsePos, finePos)
	}
}

func TestProcessMove_EasedCompletesExactlyAtTotal(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(3.0, 4.0) // distance 5, speed 2 => Total 2.5s
	mc := easedMovement(start, dest, 2.0, easing.CurveInOut)

	pos := start
	ticks := 0
	done := false
	for !done {
		ticks++
		if ticks > 100 {
			t.Fatal("eased move did not complete within 100 ticks")
		}
		mc, pos, done = ProcessMove(mc, pos, 250*time.Millisecond)
	}

	if ticks != 10 {
		t.Errorf("expected completion on tick 10 (2.5s at 250ms), got tick %d", ticks)
	}
	if pos != dest {
		t.Errorf("expected exact arrival at %v, got %v", dest, pos)
	}
	if mc.Elapsed != mc.Total {
		t.Errorf("expected Elapsed to equal Total on completion, got %v of %v", mc.Elapsed, mc.Total)
	}
}

// TestProcessMove_EasedArrivalTracksLinearWithinOneTick pins the honest
// version of the arrival-tick claim: the eased and constant-speed paths give
// a move the same nominal duration (distance divided by speed), but they do
// not always complete on the same tick. The eased path completes on the
// first tick at or after that duration; the constant-speed path completes on
// a distance tolerance and an overshoot test over an accumulated float
// position. Under a tick size that does not evenly divide the duration, the
// two can differ by one tick in either direction (never more). The test
// checks both halves of the eased path's own rule directly: the completion
// tick itself must have reached Total (never early), and the tick before it
// must not have (never late, i.e. it is the first qualifying tick).
func TestProcessMove_EasedArrivalTracksLinearWithinOneTick(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	distances := []float64{1.0, math.Sqrt2, 2.0, 3.0, 5.0, 7.3}
	ticks := []time.Duration{
		16666666 * time.Nanosecond, // 60Hz
		100 * time.Millisecond,
		700 * time.Millisecond, // ragged: does not divide most totals evenly
		time.Second,
	}

	for _, distance := range distances {
		dest := geometry.NewVector2(distance, 0.0)
		for _, tick := range ticks {
			eased := easedMovement(start, dest, 1.0, easing.CurveOut)
			easedPos := start
			easedTicks := 0
			for done := false; !done; {
				easedTicks++
				eased, easedPos, done = ProcessMove(eased, easedPos, tick)
				if easedTicks > 100000 {
					t.Fatalf("distance %v, tick %v: eased move did not complete", distance, tick)
				}
			}

			linearPos := start
			linearTicks := 0
			for done := false; !done; {
				linearTicks++
				linearPos, done = ProcessMovement(linearPos, dest, 1.0, tick)
				if linearTicks > 100000 {
					t.Fatalf("distance %v, tick %v: linear move did not complete", distance, tick)
				}
			}

			if diff := easedTicks - linearTicks; diff < -1 || diff > 1 {
				t.Errorf("distance %v, tick %v: eased completed on tick %d, linear on tick %d, differ by more than one tick", distance, tick, easedTicks, linearTicks)
			}

			total := time.Duration(distance / 1.0 * float64(time.Second))
			if got := time.Duration(easedTicks) * tick; got < total {
				t.Errorf("distance %v, tick %v: eased move completed at %v, before its Total (%v) had elapsed", distance, tick, got, total)
			}
			if late := time.Duration(easedTicks-1) * tick; late >= total {
				t.Errorf("distance %v, tick %v: eased move completed on tick %d, later than the first tick at or after Total (%v)", distance, tick, easedTicks, total)
			}
		}
	}
}

func TestProcessMove_EasedZeroDurationMakesNoProgress(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(2.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut)

	updated, pos, done := ProcessMove(mc, start, 0)

	if done {
		t.Error("a zero-duration tick must not complete a move")
	}
	if pos != start {
		t.Errorf("a zero-duration tick must not move: got %v, want %v", pos, start)
	}
	if updated.Elapsed != 0 {
		t.Errorf("a zero-duration tick must not advance Elapsed, got %v", updated.Elapsed)
	}
}

// A degenerate eased move (no recorded Total) must still respect the
// zero-duration rule before it snaps to its destination.
func TestProcessMove_EasedZeroTotalNeedsAPositiveTick(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(2.0, 0.0)
	mc := Movement{Destination: dest, Speed: 1.0, Ease: easing.CurveInOut, Start: start}

	if _, pos, done := ProcessMove(mc, start, 0); done || pos != start {
		t.Errorf("zero-duration tick on a zero-Total move: got (%v, %v), want (%v, false)", pos, done, start)
	}
	if _, pos, done := ProcessMove(mc, start, time.Millisecond); !done || pos != dest {
		t.Errorf("positive tick on a zero-Total move: got (%v, %v), want (%v, true)", pos, done, dest)
	}
}

func TestMovementProgress(t *testing.T) {
	mc := Movement{Total: 4 * time.Second, Elapsed: time.Second}
	if got := mc.Progress(); got != 0.25 {
		t.Errorf("Progress() = %v, want 0.25", got)
	}

	mc.Elapsed = 8 * time.Second
	if got := mc.Progress(); got != 1 {
		t.Errorf("Progress() past Total = %v, want 1", got)
	}

	// A Movement decoded from a save written before easing existed has no
	// recorded Total.
	legacy := Movement{Destination: geometry.NewVector2(1.0, 0.0), Speed: 1.0}
	if got := legacy.Progress(); got != 0 {
		t.Errorf("Progress() without Total = %v, want 0", got)
	}
}

func TestMovement_GobRoundTripMidEasedMove(t *testing.T) {
	start := geometry.NewVector2(1.0, 2.0)
	dest := geometry.NewVector2(5.0, 2.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut) // Total 4s

	advanced, pos, _ := ProcessMove(mc, start, time.Second)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(advanced); err != nil {
		t.Fatalf("encoding a Movement: %v", err)
	}
	var decoded Movement
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decoding a Movement: %v", err)
	}

	if decoded != advanced {
		t.Errorf("round trip changed the movement: %+v, want %+v", decoded, advanced)
	}

	// The restored move continues to the same arrival, at the same tick.
	restoredPos := pos
	ticks := 0
	for done := false; !done; {
		ticks++
		decoded, restoredPos, done = ProcessMove(decoded, restoredPos, time.Second)
	}
	if ticks != 3 || restoredPos != dest {
		t.Errorf("restored move finished in %d ticks at %v, want 3 ticks at %v", ticks, restoredPos, dest)
	}
}

// A Movement encoded before the easing fields existed decodes to a working
// constant-speed move: gob leaves unknown-to-the-encoder fields at their zero
// values.
type legacyMovement struct {
	Destination geometry.Vector2
	Speed       float64
}

func TestMovement_GobDecodesLegacyMovementAsLinear(t *testing.T) {
	legacy := legacyMovement{Destination: geometry.NewVector2(2.0, 0.0), Speed: 1.0}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(legacy); err != nil {
		t.Fatalf("encoding the legacy movement: %v", err)
	}
	var decoded Movement
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decoding into Movement: %v", err)
	}

	if decoded.Ease != easing.CurveLinear || decoded.Total != 0 || decoded.Elapsed != 0 {
		t.Errorf("expected a constant-speed move with no recorded timing, got %+v", decoded)
	}

	pos := geometry.NewVector2(0.0, 0.0)
	_, pos, done := ProcessMove(decoded, pos, time.Second)
	if done || pos.DistanceTo(geometry.NewVector2(1.0, 0.0)) > 1e-9 {
		t.Errorf("expected the legacy move to advance one tile, got %v (done=%v)", pos, done)
	}
}
