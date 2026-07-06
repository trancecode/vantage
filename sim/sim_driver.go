package sim

import (
	"reflect"
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
// Handling may schedule new events, including at now; a handler that
// unconditionally re-schedules an event at now never lets the current instant
// settle and stalls the driver's drain loop.
type EventHandler interface {
	HandleEvent(now util.Time, e Event)
}

// tickSystemEntry pairs a registered tick system with the label used to
// attribute its wall time when a profiler is attached (its concrete type name).
type tickSystemEntry struct {
	label  string
	system TickSystem
}

// Driver owns the game clock and advances it event by event, running tick
// systems over each interval and draining the event queue at each stop through
// the handler.
type Driver struct {
	now         util.Time
	tickSystems []tickSystemEntry
	queue       *EventQueue
	handler     EventHandler
	profiler    *util.Profiler
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
	d.tickSystems = append(d.tickSystems, tickSystemEntry{
		label:  reflect.TypeOf(s).String(),
		system: s,
	})
}

// SetProfiler attaches p so RunUntil records the wall time of each tick system
// (labeled by its concrete type) and of the event drain (as "sim.drain"). A nil
// profiler, the default, disables profiling with zero overhead. Set it before
// RunUntil. Recorded timings are observational only and never affect the
// simulation.
func (d *Driver) SetProfiler(p *util.Profiler) { d.profiler = p }

// Profiler returns the attached profiler, or nil. Games can Record their own
// phase timings into it alongside the driver's system timings.
func (d *Driver) Profiler() *util.Profiler { return d.profiler }

// Queue returns the driver's event queue for scheduling, read-ahead, and
// snapshotting.
func (d *Driver) Queue() *EventQueue { return d.queue }

// Now returns the current game time.
func (d *Driver) Now() util.Time { return d.now }

// RestoreNow reseats the clock. It is for reloading a savegame before any
// RunUntil call; the clock is otherwise advanced only by RunUntil.
func (d *Driver) RestoreNow(t util.Time) { d.now = t }

// RestoreQueue replaces the driver's event queue, for reloading a savegame
// before any RunUntil call. Pair it with RestoreNow.
func (d *Driver) RestoreQueue(q *EventQueue) { d.queue = q }

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

		for i := range d.tickSystems {
			ts := &d.tickSystems[i]
			if d.profiler == nil {
				ts.system.Tick(elapsed)
				continue
			}
			start := time.Now()
			ts.system.Tick(elapsed)
			d.profiler.Record(ts.label, time.Since(start))
		}

		if d.profiler == nil {
			d.drainDue()
			continue
		}
		start := time.Now()
		d.drainDue()
		d.profiler.Record("sim.drain", time.Since(start))
	}
}

// drainDue dispatches every event due at the current instant through the
// handler, re-checking after each so same-instant cascades resolve before the
// clock advances.
func (d *Driver) drainDue() {
	for {
		e, ok := d.queue.Peek()
		if !ok || e.Time > d.now {
			return
		}
		d.queue.Pop()
		d.handler.HandleEvent(d.now, e)
	}
}
