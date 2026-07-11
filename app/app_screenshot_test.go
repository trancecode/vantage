package app

import (
	"testing"
	"time"
)

func TestSingleCaptureClampsToDelay(t *testing.T) {
	c := newScreenshotCapturer("/tmp/shot.png", time.Second, 0)
	if adv := c.advance(500 * time.Millisecond); adv != 500*time.Millisecond || c.shouldCapture {
		t.Fatalf("before delay: advance=%v capture=%v", adv, c.shouldCapture)
	}
	// A 600ms frame would reach 1.1s; it must clamp so game time lands on 1s.
	adv := c.advance(600 * time.Millisecond)
	if adv != 500*time.Millisecond {
		t.Fatalf("clamped advance = %v, want 500ms", adv)
	}
	if !c.shouldCapture {
		t.Fatal("expected a capture at the delay target")
	}
	if c.gameTime != time.Second {
		t.Fatalf("gameTime = %v, want exactly 1s", c.gameTime)
	}
	if c.carry != 100*time.Millisecond {
		t.Fatalf("carry = %v, want 100ms deferred", c.carry)
	}
}

func TestAdvanceHoldsUntilCaptured(t *testing.T) {
	c := newScreenshotCapturer("/tmp/shot.png", 0, 0) // single shot at game time 0
	if adv := c.advance(time.Second); adv != 0 || !c.shouldCapture {
		t.Fatalf("at target: advance=%v capture=%v", adv, c.shouldCapture)
	}
	// With a capture pending (not yet drawn), the sim is held at the target.
	if adv := c.advance(time.Second); adv != 0 {
		t.Fatalf("while a capture is pending, advance should be 0, got %v", adv)
	}
}

// TestSequenceLandsOnExactIntervals feeds irregular frame durations and checks
// that captures land on the exact game-time targets (1s, 2s, 3s), independent
// of frame pacing.
func TestSequenceLandsOnExactIntervals(t *testing.T) {
	c := newScreenshotCapturer("/tmp/frame-%d.png", time.Second, time.Second)
	if !c.sequence {
		t.Fatal("expected sequence mode for a path with a percent-d verb")
	}
	var captureTimes []time.Duration
	for range 40 {
		c.advance(120 * time.Millisecond) // irregular, non-divisor frame size
		if c.shouldCapture {
			captureTimes = append(captureTimes, c.gameTime)
			c.shouldCapture = false // simulate the Draw consuming the capture
			c.captureCount++
		}
	}
	want := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}
	if len(captureTimes) < 3 || captureTimes[0] != want[0] || captureTimes[1] != want[1] || captureTimes[2] != want[2] {
		t.Fatalf("capture game-times = %v, want first three to be %v", captureTimes, want)
	}
}
