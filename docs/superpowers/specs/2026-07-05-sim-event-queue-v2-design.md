# Sim package design v2: single serializable event queue

Supersedes `2026-07-02-sim-event-queue-design.md` and the `sim` API shipped in
vantage `v0.1.4` (the generic `Element[T]` / `EventQueue[T]` and the
`EventSource` abstraction). Nothing consumes `v0.1.4` yet, so the API is free to
change; this design will ship as `v0.1.5`.

## Context

The v1 design used a generic `EventQueue[T Element[T]]` where each game supplied
its own element struct and `TieBreak`. Two requirements surfaced afterward that
v1 serves poorly:

* Savegames (needed in nrg) must persist the pending event queue. A queue of
  arbitrary generic game structs has no snapshot API and mixes in queue-only
  state (the per-source effect counter) that is not reconstructable from
  components, so serialization would leak into engine internals.
* The UI (needed in lockstep) must display upcoming events so the player can see
  who acts after whom, which requires ordered read-ahead into the queue's
  future, not just peeking the head.

Both point at the same simplification: store lightweight, fixed-shape events that
reference entities, keep meaning in the entity's components, and let the engine
own a single ordered queue it can snapshot and read ahead.

## Goals

* One concrete, serializable event type and a single event queue per world,
  replacing the per-kind action and effect queues.
* Ordered, non-destructive read-ahead for the UI.
* An in-memory whole-queue snapshot and rebuild, shaped so byte serialization
  for savegames drops in later without redesign.
* Dequeue order that is a pure function of the queued set, independent of
  insertion order.

## Phasing

* Phase 1 (this work): the `Event` type, the `EventQueue` (add, peek, pop, len,
  `PeekAhead`, `Snapshot`, `Restore`), and the `Driver`. Depends only on
  `EntityId.Compare` (already shipped in ecs `v0.1.1`), so no ecs change and no
  vantage save-format decision are needed. Ships as vantage `v0.1.5`.
* Phase 2 (deferred): byte serialization (`EventQueue.MarshalBinary` /
  `UnmarshalBinary`), which needs `ecs.EntityId` marshaling and the chosen
  vantage save format. Not blocking phase 1.

## Non-goals

* Parallel producers. Keying on entity identity (including freshly-allocated
  effect entities) is deterministic single-threaded; parallel scheduling would
  first need deterministic entity allocation, out of scope here.
* Moving game rules into vantage. Dispatch (which handler runs for an event)
  stays game-side.
* Full world serialization. Making `ecs.EntityId` marshalable is in scope;
  serializing all entities, components, and the id allocator counter is a
  separate ecs and game concern that savegames also need.

## Event

```go
// Event is a scheduled occurrence about a single entity.
type Event struct {
	// Time is the game time at which the event is due.
	Time util.Time

	// Entity is the entity the event concerns. The game resolves meaning from
	// the entity's components and/or from Key.
	Entity ecs.EntityId

	// Key is a client-defined discriminator. It typically holds the event type,
	// but the client may pack a type, subtype, counter, or other metadata into
	// its 64 bits. It participates in ordering and uniqueness.
	Key uint64
}
```

The event is fixed-shape and carries no payload beyond these three fields. Any
data an event's handler needs is looked up from the entity's components at
handling time, exactly as scheduled effects already work (the queue references
the effect entity; the effect data lives in its component).

### Ordering and uniqueness

Ordering is lexicographic and hardwired: `Time`, then `Key`, then `Entity`
(compared with `EntityId.Compare`). Rationale:

* `Time` is the schedule, the engine's domain.
* `Key` is the client's control over intra-instant priority. Using the event
  type as the key makes a cross-entity phase order fall out directly (for
  example an effect key sorting before an action key resolves all effects before
  all actions at a shared instant).
* `Entity` is the final, always-unique deterministic tie-break.

Uniqueness rule: two distinct queued events must never share the same
`(Time, Key, Entity)`. This is almost always automatic. Each entity has at most
one action per instant, and effects each get their own entity, so
`(T, EFFECT, E_a)` and `(T, EFFECT, E_b)` differ by entity. The only way to
collide is scheduling the same key for the same entity at the same instant; that
is a caller bug. The queue does not fall back to insertion order for duplicates,
so a silent first-in-first-out dependence cannot creep in. The earlier
per-source counter is unnecessary: entity identity provides the tie-break.

### Entity identifier and ecs dependency

`Event.Entity` is an `ecs.EntityId`. Ordering relies only on `EntityId.Compare`
(shipped in ecs `v0.1.1`), so the phase 1 core needs no ecs change. Byte
serialization of `EntityId` (`MarshalBinary`/`UnmarshalBinary`) is a phase 2
concern: it is deferred until the vantage save format is chosen, because
freezing a byte layout on ecs before that is premature. See Phasing.

## Event queue

```go
type EventQueue struct { /* binary min-heap of Event */ }

func NewEventQueue() *EventQueue
func (q *EventQueue) Add(e Event)
func (q *EventQueue) Peek() (Event, bool)          // earliest, no removal
func (q *EventQueue) Pop() (Event, bool)           // remove earliest
func (q *EventQueue) Len() int

// PeekAhead returns the next n events in dequeue order without removing them
// (all of them when n >= Len). For UI read-ahead; not on the hot path.
func (q *EventQueue) PeekAhead(n int) []Event

// Snapshot returns every queued event, order unspecified. For serialization
// and for rebuilding a queue in memory.
func (q *EventQueue) Snapshot() []Event

// Restore rebuilds a queue from a snapshot (phase 1: in-memory reconstruction).
func Restore(events []Event) *EventQueue

// Phase 2 (deferred, needs the save format and ecs.EntityId marshaling):
//   func (q *EventQueue) MarshalBinary() ([]byte, error)
//   func (q *EventQueue) UnmarshalBinary(data []byte) error
```

### Backing structure and read-ahead cost

The queue is a binary min-heap, so `Add` and `Pop` stay O(log n). `PeekAhead`
and `Snapshot` copy the backing slice (and `PeekAhead` pops from the copy),
which is off the hot path. Because dequeue order is a pure function of the set,
`Snapshot`/`UnmarshalBinary` need not preserve heap array layout: restore rebuilds
the heap from the event set. If ordered read-ahead ever shows up in a profile
(large queue re-read every frame), cache a sorted view invalidated on mutation,
or switch to an order-statistics structure; not now.

## Driver

The driver owns the clock, runs tick systems over each interval, and drains the
single event queue at each stop through one game-provided handler.

```go
// TickSystem consumes elapsed game time continuously (for example movement).
type TickSystem interface {
	Tick(elapsed time.Duration)
}

// EventHandler dispatches a due event. The game switches on Event.Key (and/or
// inspects Entity's components) to run the right logic. It may schedule new
// events, including at now.
type EventHandler interface {
	HandleEvent(now util.Time, e Event)
}

type Driver struct { /* clock, tick systems, queue, handler */ }

func NewDriver(handler EventHandler) *Driver
func (d *Driver) RegisterTickSystem(s TickSystem)
func (d *Driver) Queue() *EventQueue      // schedule, UI read-ahead, snapshot
func (d *Driver) Now() util.Time
func (d *Driver) RestoreNow(t util.Time)  // set the clock at load only
func (d *Driver) RunUntil(target util.Time)
```

`Queue()` exposes the single queue for scheduling (`driver.Queue().Add(...)`),
UI read-ahead (`PeekAhead`), and serialization (`MarshalBinary`).

### RunUntil semantics

While `Now() < target`:

1. Next stop is the minimum of `target` and the queue head's `Time`.
2. Clamp the stop up to `Now()` so a past-scheduled event never rewinds the
   clock; advance the clock to the stop and compute the elapsed duration.
3. Run every tick system, in registration order, with the elapsed duration.
4. Drain: while the queue head is due (`Time <= Now()`), pop it and call
   `HandleEvent(Now(), event)`. Because handling may schedule new events at
   `Now()`, the re-check catches same-instant cascades before the clock moves.

Draining pops one event at a time in strict `(Time, Key, Entity)` order, so a
same-instant event scheduled by a handler is interleaved at its correct ordered
position within the instant.

### Clock ownership and restore

The driver owns the clock; the world reads it through `Now()`. The clock starts
at `util.Time(0)`, is monotonic, and advances only via `RunUntil`. `RestoreNow`
exists solely to reseat the clock when loading a savegame, before any
`RunUntil` call.

## Serialization (savegame) — phase 2, deferred

Byte-level serialization is deferred until the vantage save format is chosen and
`ecs.EntityId` marshaling exists (see Phasing). The phase 1 shape already makes
it straightforward when the time comes: a savegame persists the driver clock
(`Now()`, a `util.Time`) and the event set (`Queue().Snapshot()`), and load
reseats the clock (`RestoreNow`) and rebuilds the queue (`Restore`). Because
dequeue order is a pure function of the set, no heap layout needs preserving.
Tick systems and the handler are code, not state, and are re-registered on load.
Broader world state (entities, components, the id allocator counter) is
serialized by the game and ecs and is out of scope here; the event queue's
entity references stay valid because load restores entity ids unchanged.

## Migration sketch

Per game (`lockstep/core` first, then `nrg/rts`):

* Replace `ActionQueue`/`EffectQueue` (`util.PriorityQueue`) with one
  `sim.EventQueue` reached through the driver.
* Scheduling a next action becomes `driver.Queue().Add(sim.Event{Time: t,
  Entity: entityId, Key: keyAction})`; scheduling an effect uses the effect
  entity and `keyEffect`. Choose key constants so effects sort before actions if
  that phase order is desired.
* `StateSystem` and `EffectSystem` merge into one `EventHandler` that switches on
  `Key` and runs the existing handling logic, resolving components from
  `Event.Entity`.
* `MovementSystem` is a `sim.TickSystem` (being reworked in vantage to update the
  position of all entities with a movement component).
* `TimeSystem` is deleted; the world holds the `sim.Driver`, registers movement
  and the handler, and calls `driver.RunUntil` where it called
  `timeSystem.RunUntil`.
* The time-listener notification stays game-side, at the `RunUntil` call site.

## Testing

* Insertion-order independence: shuffle the same event set and assert identical
  dequeue order.
* Lexicographic ordering: events across distinct times, and same-time events
  ordered by `Key` then `Entity`.
* Read-ahead: `PeekAhead(n)` returns the correct ordered window without mutating
  the queue.
* Snapshot round-trip: `Restore(q.Snapshot())` yields a queue with the same
  dequeue sequence (byte-level `MarshalBinary` round-trip is a phase 2 test).
* Driver stop points and elapsed durations, same-instant cascades (a handler
  scheduling an event at the current instant is handled before the clock moves),
  and the past-event clamp.

## Open questions

* Whether ecs should expose only `EntityId` marshaling now, or a broader
  `World` snapshot/restore (entities plus the allocator counter) that a full
  savegame needs anyway. This design needs only the former; the latter can
  follow as its own ecs work.
* Coordinate the `sim.TickSystem` interface with the in-progress vantage
  movement-system rework so the two agree on the tick signature.
