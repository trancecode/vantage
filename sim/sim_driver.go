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

// EventHandler dispatches a due event at the given game time. The game switches
// on Event.Key (and/or inspects Entity's components) to run the right logic.
// Handling may schedule new events, including at now.
type EventHandler interface {
	HandleEvent(now util.Time, e Event)
}

// Driver owns the game clock and advances it event by event, running tick
// systems over each interval and draining the event queue at each stop through
// the handler.
type Driver struct {
	now         util.Time
	tickSystems []TickSystem
	queue       *EventQueue
	handler     EventHandler
}

// NewDriver returns a Driver whose clock is at the zero time, with an empty
// event queue and the given handler.
func NewDriver(handler EventHandler) *Driver {
	return &Driver{
		queue:   NewEventQueue(),
		handler: handler,
	}
}

// RegisterTickSystem registers s. Tick systems run in registration order, which
// defines their phase ordering.
func (d *Driver) RegisterTickSystem(s TickSystem) {
	d.tickSystems = append(d.tickSystems, s)
}

// Queue returns the driver's event queue for scheduling, read-ahead, and
// snapshotting.
func (d *Driver) Queue() *EventQueue { return d.queue }

// Now returns the current game time.
func (d *Driver) Now() util.Time { return d.now }

// RestoreNow reseats the clock. It is for reloading a savegame before any
// RunUntil call; the clock is otherwise advanced only by RunUntil.
func (d *Driver) RestoreNow(t util.Time) { d.now = t }

// RunUntil advances the clock to target, stopping at each due event. At every
// stop it runs each tick system, in registration order, with the elapsed
// duration, then drains the queue: while the head is due (Time at or before
// now), it pops the event and calls the handler. Because handling may schedule
// new events at now, the drain re-checks until the instant is quiet, so
// same-instant cascades resolve before the clock moves. The clock never rewinds;
// a past-scheduled event is dispatched at the current instant.
func (d *Driver) RunUntil(target util.Time) {
	for d.now < target {
		stop := target
		if e, ok := d.queue.Peek(); ok && e.Time < stop {
			stop = e.Time
		}
		// A past event (Time < now) must not rewind the clock; it is dispatched
		// by the drain below at the current instant instead.
		if stop < d.now {
			stop = d.now
		}

		elapsed := stop.Sub(d.now)
		d.now = stop

		for _, tickSystem := range d.tickSystems {
			tickSystem.Tick(elapsed)
		}

		for {
			e, ok := d.queue.Peek()
			if !ok || e.Time > d.now {
				break
			}
			d.queue.Pop()
			d.handler.HandleEvent(d.now, e)
		}
	}
}
