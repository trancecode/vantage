package util

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchdogExpired(t *testing.T) {
	var expired int32
	done := newWatchdog(100*time.Millisecond, func() {
		atomic.StoreInt32(&expired, 1)
	})
	time.Sleep(200 * time.Millisecond)

	// Signal that the watchdog has been reset
	done()
	if atomic.LoadInt32(&expired) == 0 {
		t.Errorf("Watchdog did not expire as expected")
	}
}

func TestWatchdogReset(t *testing.T) {
	var expired int32
	done := newWatchdog(100*time.Millisecond, func() {
		atomic.StoreInt32(&expired, 1)
	})
	time.Sleep(50 * time.Millisecond)

	// Reset the watchdog before it expires
	done()

	// Wait longer than the timeout
	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&expired) != 0 {
		t.Errorf("Watchdog expired when it should not have")
	}
}

func TestReusableWatchdogKickDone(t *testing.T) {
	w := NewReusableWatchdog("test", 100*time.Millisecond)

	// Simulate three cycles of kick/done, each completing before timeout
	for i := 0; i < 3; i++ {
		w.Kick()
		time.Sleep(20 * time.Millisecond)
		w.Done()
	}

	// After all cycles, wait to ensure no late expiration
	time.Sleep(200 * time.Millisecond)
}
