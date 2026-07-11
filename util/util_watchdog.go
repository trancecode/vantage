package util

import (
	"sync"
	"time"
)

// NewWatchdog creates a one-shot watchdog with the specified name and timeout duration.
// It returns a function that must be called to signal completion before the timeout.
// If the timeout elapses without the function being called, it panics.
func NewWatchdog(name string, timeout time.Duration) func() {
	return newWatchdog(timeout, func() {
		panic("Watchdog " + name + " triggered after " + DurationString(timeout))
	})
}

func newWatchdog(timeout time.Duration, onExpiration func()) func() {
	done := make(chan struct{}, 1) // Buffered channel

	go func() {
		select {
		case <-time.After(timeout):
			onExpiration()
		case <-done:
			return
		}
	}()

	var mu sync.Mutex
	var doneInvoked bool
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if !doneInvoked {
			doneInvoked = true
			done <- struct{}{}
		}
	}
}

// Watchdog is a reusable watchdog timer for repeated monitoring (e.g., per-frame checks).
// Call Kick before each operation to arm the timer, and Done after the operation completes.
// A single goroutine and timer are reused across kicks. Call Stop when the watchdog is no
// longer needed to release its background goroutine.
type Watchdog struct {
	timer    *time.Timer
	timeout  time.Duration
	name     string
	stop     chan struct{}
	stopOnce sync.Once
}

// NewReusableWatchdog creates a Watchdog that can be kicked and done repeatedly.
func NewReusableWatchdog(name string, timeout time.Duration) *Watchdog {
	t := time.NewTimer(timeout)
	t.Stop()

	w := &Watchdog{
		timer:   t,
		timeout: timeout,
		name:    name,
		stop:    make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-w.timer.C:
				panic("Watchdog " + w.name + " triggered after " + DurationString(w.timeout))
			case <-w.stop:
				return
			}
		}
	}()

	return w
}

// Kick arms the watchdog timer. Must be followed by Done before the timeout. Calling
// Kick after Stop re-arms the timer but has no detection effect: see Stop.
func (w *Watchdog) Kick() {
	w.timer.Reset(w.timeout)
}

// Done disarms the watchdog timer for this cycle.
func (w *Watchdog) Done() {
	w.timer.Stop()
}

// Stop disarms the watchdog and terminates its background goroutine. It is safe to call
// multiple times and safe to call from any goroutine. Once Stop returns, the watchdog is
// permanently inert: no goroutine remains to observe the timer, so a later Kick re-arms it
// but a real hang will not be detected, panic, or otherwise signal that monitoring has
// stopped. A stopped Watchdog must not be reused; create a new one instead.
func (w *Watchdog) Stop() {
	w.timer.Stop()
	w.stopOnce.Do(func() {
		close(w.stop)
	})
}
