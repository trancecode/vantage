package sim

import (
	"time"

	"github.com/trancecode/vantage/util"
)

// TickSystem consumes elapsed game time continuously, such as movement physics.
type TickSystem interface {
	// Tick advances the system by elapsed game time.
	Tick(elapsed time.Duration)
}

// EventSource drains discrete events that are due at a given game time.
// EventQueue-backed sources wrap a queue plus the game logic that handles its
// events.
type EventSource interface {
	// NextEventTime returns the time of the earliest queued event. ok is false
	// when the source has no queued events.
	NextEventTime() (t util.Time, ok bool)

	// RunDue handles every event due at now (event time at or before now).
	// Handling an event may queue new events, including at now; the driver
	// re-drains until every source is quiet, so a handler that unconditionally
	// re-queues an event at now never lets the current instant settle and stalls
	// the driver.
	RunDue(now util.Time)
}

// Driver owns the game clock and advances it event by event, running tick
// systems over each elapsed interval and draining event sources at each stop.
type Driver struct {
	now          util.Time
	tickSystems  []TickSystem
	eventSources []EventSource
}

// NewDriver returns a Driver with its clock at the zero time and no registered
// systems or sources.
func NewDriver() *Driver { return &Driver{} }

// RegisterTickSystem registers s. Tick systems run in registration order, which
// defines their phase ordering.
func (d *Driver) RegisterTickSystem(s TickSystem) {
	d.tickSystems = append(d.tickSystems, s)
}

// RegisterEventSource registers s. Event sources drain in registration order,
// which defines their phase ordering.
func (d *Driver) RegisterEventSource(s EventSource) {
	d.eventSources = append(d.eventSources, s)
}

// Now returns the current game time.
func (d *Driver) Now() util.Time { return d.now }

// RunUntil advances the clock to target, stopping at every queued event time in
// between. At each stop it runs every tick system with the elapsed duration,
// then drains every event source until all are quiet at the current instant, so
// same-instant cascades resolve before the clock moves. The clock never moves
// backward; an event scheduled in the past is dispatched at the current instant
// without rewinding the clock.
func (d *Driver) RunUntil(target util.Time) {
	for d.now < target {
		stop := target
		for _, source := range d.eventSources {
			if t, ok := source.NextEventTime(); ok && t < stop {
				stop = t
			}
		}
		// A past event (t < now) must not rewind the clock; it is dispatched
		// by the drain below at the current instant instead.
		if stop < d.now {
			stop = d.now
		}

		elapsed := stop.Sub(d.now)
		d.now = stop

		for _, tickSystem := range d.tickSystems {
			tickSystem.Tick(elapsed)
		}

		d.drain()
	}
}

// drain dispatches every event due at the current instant, repeating until no
// source has a due event, so cascades queued during dispatch resolve here.
func (d *Driver) drain() {
	for d.dueExists() {
		for _, source := range d.eventSources {
			source.RunDue(d.now)
		}
	}
}

// dueExists reports whether any source has an event due at or before the
// current instant.
func (d *Driver) dueExists() bool {
	for _, source := range d.eventSources {
		if t, ok := source.NextEventTime(); ok && t <= d.now {
			return true
		}
	}
	return false
}
