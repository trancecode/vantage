package util

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchdogExpired(t *testing.T) {
	expiredCh := make(chan struct{})
	done := newWatchdog(100*time.Millisecond, func() {
		close(expiredCh)
	})
	defer done()

	select {
	case <-expiredCh:
		// Expired as expected.
	case <-time.After(2 * time.Second):
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

	// No deterministic event to wait on here: we are asserting an absence of
	// expiration, so a bounded sleep past the original timeout is unavoidable.
	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&expired) != 0 {
		t.Errorf("Watchdog expired when it should not have")
	}
}

func TestReusableWatchdogKickDone(t *testing.T) {
	w := NewReusableWatchdog("test", 100*time.Millisecond)
	t.Cleanup(w.Stop)

	// Simulate three cycles of kick/done, each completing before timeout
	for i := 0; i < 3; i++ {
		w.Kick()
		time.Sleep(20 * time.Millisecond)
		w.Done()
	}

	// No deterministic event to wait on here: we are asserting an absence of
	// late expiration, so a bounded sleep past the original timeout is unavoidable.
	time.Sleep(200 * time.Millisecond)
}
