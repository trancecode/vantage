# Sim package design: deterministic event scheduling

## Context

Both games built on vantage duplicate the same scheduling machinery:

* `nrg/rts` and `lockstep/core` each hold an `ActionQueue` (a
  `util.PriorityQueue[queuedAction]` ordered by an entity's next state-change
  time) and an `EffectQueue` (a `util.PriorityQueue[queuedEffect]` ordered by
  effect resolution time).
* Each also hardcodes a time-advancement loop (`TimeSystem.RunUntil` in
  lockstep) that advances the game clock to
  `min(target, nextStateChangeTime, nextEffectTime)` and runs each system in a
  fixed sequence.

Adding a new kind of scheduled event today means a new queue type, a new
`NextXTime()` method, and edits to the time loop, in both games.

This design extracts the mechanism into a new vantage package, `sim`. Game
content (what an action tick or an effect does) stays in the games; vantage
holds only the scheduling and time-advancement machinery, consistent with the
engine charter of staying free of game rules.

## Goals

* One generic, deterministic event queue type that replaces both `ActionQueue`
  and `EffectQueue` in each game.
* Dequeue order that is a pure function of the set of queued events,
  independent of insertion order, so that future parallel producers cannot
  make outcomes diverge between runs.
* A time-advancement driver that generalizes `TimeSystem.RunUntil`, so adding
  a new event kind is a registration, not loop surgery.

## Non-goals

* Moving game rules (combat scoring, stats, equipment, artificial-intelligence
  directives) into vantage. Whether the ruleset forked between `nrg/rts` and
  `lockstep/core` is later shared through a common module is a separate,
  open decision that this design does not depend on.
* Built-in support for parallel producers. The queue's contract makes parallel
  production *safe to add later*; the gather-and-commit barrier that parallel
  producers would need is producer-side machinery, out of scope until a game
  actually parallelizes.
* Changing `util.PriorityQueue`. Pathfinding keeps using it; the new queue is
  a separate type with a stronger contract.

## Package layout

New package `sim`, depending only on `util` (for `util.Time`). It contains two
pieces: the event queue and the driver.

## Event queue

### Interface

```go
// Element is implemented by values stored in an EventQueue.
type Element[T any] interface {
	// EventTime is the game time at which the element is due.
	EventTime() util.Time

	// TieBreak defines a strict total order among elements that share the
	// same EventTime. It returns a negative value if the receiver sorts
	// before other, positive if after. It must never return 0 for two
	// distinct queued elements; see the determinism contract.
	TieBreak(other T) int
}

type EventQueue[T Element[T]] struct { ... }

func NewEventQueue[T Element[T]]() *EventQueue[T]
func (q *EventQueue[T]) Add(element T)
func (q *EventQueue[T]) Peek() (T, bool)
func (q *EventQueue[T]) Next() (T, bool)
func (q *EventQueue[T]) Len() int
```

Ordering is lexicographic and hardwired: `EventTime` first, then `TieBreak`.
The queue owns the time comparison; elements only decide ordering among
same-time peers. This keeps callers from accidentally writing a comparator
that ignores time or depends on mutable state.

### Determinism contract

* Dequeue order is a pure function of the set of queued elements. Two runs
  that queue the same elements, in any insertion order, dequeue them in the
  same order.
* Two distinct queued elements must never compare equal (same `EventTime` and
  `TieBreak` returning 0). A duplicate key is a caller bug: the relative order
  of duplicates is unspecified, and the determinism guarantee is void for
  them. The queue does not fall back to insertion order, because a silent
  first-in-first-out fallback would quietly reintroduce insertion-order
  dependence.
* Elements must be immutable while queued, or at least their key inputs must
  be. The existing `queuedAction` pattern (capture the key values into a small
  struct, never store live component references) is the required style.

### Key derivation rules

The guarantee above only holds if `TieBreak` compares values that are
themselves deterministic:

* Keys must derive from values fixed before any potentially parallel phase:
  the acting entity's identifier, the action type, a counter local to the
  source entity.
* Freshly allocated identifiers are the trap. `ScheduleEffect` today allocates
  a new entity per effect at scheduling time; if scheduling ever runs
  concurrently, those identifiers become racy and must not participate in
  keys. The effect's *source* entity identifier existed before the phase and
  is safe.
* Concretely, the expected keys are:
  * Action queue: the acting entity's identifier. Unique per time because each
    entity has exactly one queued next action.
  * Effect queue: the source entity's identifier, then a per-source counter,
    because one source can schedule several effects for the same instant.

### Entity identifier ordering (ecs dependency)

`ecs.EntityId` is deliberately opaque: no arithmetic, no integer conversion.
Tie-break comparison therefore needs a new method on the ecs module:

```go
// Compare defines a total order over entity identifiers, consistent with
// allocation order. It returns a negative value, zero, or a positive value.
func (e EntityId) Compare(other EntityId) int
```

Ordering does not expose the representation the way integer conversion would,
so it preserves the type's opacity guarantees. This is a small prerequisite
change in `github.com/trancecode/ecs`.

## Driver

The driver generalizes `TimeSystem.RunUntil`. It owns the game clock and
advances it event by event.

### Interfaces

```go
// TickSystem consumes elapsed game time continuously (movement physics).
type TickSystem interface {
	Tick(elapsed time.Duration)
}

// EventSource drains discrete events that are due at a given game time.
// EventQueue-backed sources wrap a queue plus the game logic that handles
// its events.
type EventSource interface {
	// NextEventTime returns the time of the earliest queued event.
	// ok is false when the source has no queued events.
	NextEventTime() (t util.Time, ok bool)

	// RunDue handles every event due at now (event time at or before now).
	// Handling an event may queue new events, including at now.
	RunDue(now util.Time)
}

type Driver struct { ... }

func NewDriver() *Driver
func (d *Driver) RegisterTickSystem(s TickSystem)
func (d *Driver) RegisterEventSource(s EventSource)
func (d *Driver) Now() util.Time
func (d *Driver) RunUntil(target util.Time)
```

Vantage never sees the game's world type. Game-side implementations of
`TickSystem` and `EventSource` capture their `*World` themselves.

### RunUntil semantics

While `Now() < target`:

1. Compute the next stop: the minimum of `target` and every source's
   `NextEventTime()`.
2. Advance the clock to the stop and compute the elapsed duration.
3. Run every tick system, in registration order, with the elapsed duration.
4. Drain events: call `RunDue(Now())` on every event source, in registration
   order. If handling events queued new events due at the current instant
   (in any source, including ones already drained), repeat this step until
   every source is quiet at the current instant. Cascades within one instant
   therefore resolve deterministically before the clock moves.

Registration order is the phase ordering, stated once. The current lockstep
sequence (movement, then action ticks, then effects) becomes: movement
registered as the tick system, the action source registered before the effect
source.

The clock never moves backward. Scheduling an event in the past (before
`Now()`) is a caller bug; such an event is dispatched at the current instant
during the next drain, and the clock is unaffected.

### Clock ownership

The driver owns `CurrentGameTime` and exposes it through `Now()`. Today the
world owns the time field while the time system mutates it, splitting the
responsibility. After migration, the game world holds the driver and reads
time from it.

## Migration sketch

Per game (`lockstep/core` first, then `nrg/rts`):

* `queuedAction` and `queuedEffect` implement `sim.Element` instead of
  `util.ElementWithPriority`. `queuedEffect` gains a source identifier and a
  per-source counter for its key; the world assigns the counter at scheduling
  time.
* `StateSystem` and `EffectSystem` become `sim.EventSource` implementations
  wrapping their queue plus their existing handling logic. `MovementSystem`
  becomes a `sim.TickSystem`.
* `TimeSystem` is deleted; the world constructs a `sim.Driver`, registers
  movement, the action source, and the effect source, and calls
  `driver.RunUntil` where it called `timeSystem.RunUntil`.
* The time-listener notification (`EventSystem.PublishTimeUpdate`) stays
  game-side, invoked from the game's `RunUntil` call site or from a
  registered tick system.

## Testing

* Insertion-order independence: property-style test queueing the same element
  set in shuffled orders and asserting identical dequeue sequences.
* Lexicographic ordering: elements across distinct times, and same-time
  elements ordered by tie-break.
* Driver stop points: the clock stops exactly at each event time and at the
  target, with correct elapsed durations passed to tick systems.
* Same-instant cascades: an event handler that queues another event at the
  current instant, in an already-drained source, still gets handled before
  the clock advances.
* Registration-order draining: two sources with events at the same instant
  drain in registration order.

## Open questions

* Shared ruleset home: whether `nrg` eventually consumes `lockstep/core` as a
  shared deterministic ruleset (with `nrg` as the open-world variant), or the
  two games keep diverging copies. Does not block this design; the queue and
  driver are agnostic either way.
