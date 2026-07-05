# Sim event queue v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the generic `sim.Element[T]`/`EventQueue[T]`/`EventSource` model (shipped in vantage v0.1.4) with a single concrete, serialization-ready event queue keyed by `(Time, Key, Entity)` plus a handler-driven `Driver`.

**Architecture:** One concrete `Event{Time, Entity, Key}` value type, one `EventQueue` (a binary min-heap ordered lexicographically by Time, then Key, then Entity via `ecs.EntityId.Compare`), and a `Driver` that owns the clock, slices tick systems over intervals, and drains the single queue at each stop through one game-provided `EventHandler`. Dequeue order is a pure function of the queued set.

**Tech Stack:** Go 1.26.4, standard library `container/heap`/`slices`/`time`, `github.com/trancecode/ecs/ecs` (for `EntityId`), `github.com/trancecode/vantage/util` (for `util.Time`). Tests use `github.com/stretchr/testify`.

## Global Constraints

- Go version is `1.26.4` (from `go.mod`); introduce no other version reference.
- Commit author `Claude Code <herve.quiroz+claude@gmail.com>`, and **no** `Co-Authored-By:` line.
- The `sim` package depends only on `github.com/trancecode/vantage/util`, `github.com/trancecode/ecs/ecs`, and the standard library. It must **not** import `motion`, `tilemap`, or any game package.
- ecs must be bumped to `v0.1.1`, which provides `func (e EntityId) Compare(other EntityId) int`. The currently pinned `v0.1.1-0.20260620052537-953afc80bc40` does not have it.
- Event ordering is hardwired lexicographic: `Time`, then `Key`, then `Entity` (via `Entity.Compare`). No insertion-order fallback. Two distinct queued events with equal `(Time, Key, Entity)` are a caller bug with unspecified order.
- `Event` fields are exactly `Time util.Time`, `Entity ecs.EntityId`, `Key uint64`.
- This work **replaces** the v0.1.4 sim files (`sim_eventqueue.go`, `sim_driver.go`, and their tests/bench). It ships as vantage **v0.1.6** (`v0.1.5` is taken by `motion.System`). Byte serialization (`MarshalBinary`) is out of scope (phase 2); phase 1 provides in-memory `Snapshot`/`Restore` only.
- File naming: source `sim_<topic>.go`, tests `sim_<topic>_test.go`, package doc `doc.go`.
- Before pushing: `export GOMODCACHE=/tmp/go-mod-cache` then `task lint`, `task test:headless`, and `go vet ./...` all pass. The `sim` package needs no display, so `go test ./sim/` also works directly.

## Out of scope

- Byte serialization (`EventQueue.MarshalBinary`/`UnmarshalBinary`) and `ecs.EntityId` marshaling — deferred to phase 2, pending the vantage save format.
- The `lockstep/core` and `nrg/rts` migrations — separate repos.

---

### Task 1: Event type and EventQueue

Replace the generic queue with the concrete `Event` + `EventQueue`, bump ecs, and update the benchmark.

**Files:**
- Modify: `go.mod`, `go.sum` (bump ecs to v0.1.1)
- Rewrite: `sim/sim_eventqueue.go`
- Rewrite: `sim/sim_eventqueue_test.go`
- Rewrite: `sim/sim_eventqueue_bench_test.go`

**Interfaces:**
- Consumes: `util.Time`; `ecs.EntityId` with `Compare(other) int`; `ecs.NewWorld() *World` and `world.NewEntity() ecs.EntityId` (tests only).
- Produces (Task 2 and games rely on these exact names/types):
  - `type Event struct { Time util.Time; Entity ecs.EntityId; Key uint64 }`
  - `func NewEventQueue() *EventQueue`
  - `func Restore(events []Event) *EventQueue`
  - `func (q *EventQueue) Add(e Event)`
  - `func (q *EventQueue) Peek() (Event, bool)`
  - `func (q *EventQueue) Pop() (Event, bool)`
  - `func (q *EventQueue) PeekAhead(n int) []Event`
  - `func (q *EventQueue) Snapshot() []Event`
  - `func (q *EventQueue) Len() int`

- [ ] **Step 1: Bump ecs to v0.1.1**

Run:
```bash
cd ~/src/vantage
export GOMODCACHE=/tmp/go-mod-cache
go get github.com/trancecode/ecs@v0.1.1
go build ./...
```
Expected: `go.mod` now pins `github.com/trancecode/ecs v0.1.1`; full module builds (the existing `motion`/`tilemap` packages still compile against the newer ecs).

- [ ] **Step 2: Write the failing tests**

Rewrite `sim/sim_eventqueue_test.go`:

```go
package sim

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// newEntities allocates n entities in a fresh world. Allocation is monotonic,
// so entities[i].Compare(entities[j]) < 0 for i < j.
func newEntities(n int) []ecs.EntityId {
	w := ecs.NewWorld()
	ids := make([]ecs.EntityId, n)
	for i := range ids {
		ids[i] = w.NewEntity()
	}
	return ids
}

// drain removes every event from q in dequeue order.
func drain(q *EventQueue) []Event {
	var got []Event
	for {
		e, ok := q.Pop()
		if !ok {
			return got
		}
		got = append(got, e)
	}
}

func TestEventQueueLexicographicOrder(t *testing.T) {
	e := newEntities(3) // e[0] < e[1] < e[2] by Compare

	// Ordering priority is Time, then Key, then Entity.
	events := []Event{
		{Time: util.Time(30), Key: 2, Entity: e[0]},
		{Time: util.Time(10), Key: 5, Entity: e[2]},
		{Time: util.Time(30), Key: 1, Entity: e[2]},
		{Time: util.Time(30), Key: 1, Entity: e[0]},
	}
	want := []Event{
		{Time: util.Time(10), Key: 5, Entity: e[2]}, // earliest time
		{Time: util.Time(30), Key: 1, Entity: e[0]}, // key 1 before key 2; entity e[0] before e[2]
		{Time: util.Time(30), Key: 1, Entity: e[2]},
		{Time: util.Time(30), Key: 2, Entity: e[0]},
	}

	q := NewEventQueue()
	for _, ev := range events {
		q.Add(ev)
	}
	require.Equal(t, len(want), q.Len())
	assert.Equal(t, want, drain(q))
	assert.Equal(t, 0, q.Len())
}

func TestEventQueueInsertionOrderIndependent(t *testing.T) {
	e := newEntities(3)
	base := []Event{
		{Time: util.Time(10), Key: 1, Entity: e[0]},
		{Time: util.Time(10), Key: 1, Entity: e[1]},
		{Time: util.Time(10), Key: 2, Entity: e[0]},
		{Time: util.Time(20), Key: 1, Entity: e[2]},
		{Time: util.Time(5), Key: 7, Entity: e[1]},
	}

	q := NewEventQueue()
	for _, ev := range base {
		q.Add(ev)
	}
	want := drain(q)

	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		shuffled := append([]Event(nil), base...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		q := NewEventQueue()
		for _, ev := range shuffled {
			q.Add(ev)
		}
		assert.Equal(t, want, drain(q))
	}
}

func TestEventQueuePeekPopEmpty(t *testing.T) {
	e := newEntities(2)
	q := NewEventQueue()

	_, ok := q.Peek()
	assert.False(t, ok)
	_, ok = q.Pop()
	assert.False(t, ok)

	q.Add(Event{Time: util.Time(40), Key: 1, Entity: e[0]})
	q.Add(Event{Time: util.Time(20), Key: 1, Entity: e[1]})

	peeked, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, util.Time(20), peeked.Time)
	assert.Equal(t, 2, q.Len()) // Peek does not remove.

	popped, ok := q.Pop()
	require.True(t, ok)
	assert.Equal(t, util.Time(20), popped.Time)
	assert.Equal(t, 1, q.Len())
}

func TestEventQueuePeekAhead(t *testing.T) {
	e := newEntities(3)
	q := NewEventQueue()
	q.Add(Event{Time: util.Time(30), Key: 1, Entity: e[0]})
	q.Add(Event{Time: util.Time(10), Key: 1, Entity: e[1]})
	q.Add(Event{Time: util.Time(20), Key: 1, Entity: e[2]})

	ahead := q.PeekAhead(2)
	require.Len(t, ahead, 2)
	assert.Equal(t, util.Time(10), ahead[0].Time)
	assert.Equal(t, util.Time(20), ahead[1].Time)
	assert.Equal(t, 3, q.Len()) // PeekAhead does not remove.

	// n larger than Len returns all in order.
	all := q.PeekAhead(99)
	require.Len(t, all, 3)
	assert.Equal(t, util.Time(30), all[2].Time)
}

func TestEventQueueSnapshotRestoreRoundTrip(t *testing.T) {
	e := newEntities(3)
	q := NewEventQueue()
	q.Add(Event{Time: util.Time(30), Key: 2, Entity: e[0]})
	q.Add(Event{Time: util.Time(10), Key: 5, Entity: e[2]})
	q.Add(Event{Time: util.Time(30), Key: 1, Entity: e[1]})

	want := q.PeekAhead(99) // canonical order, non-destructive

	snap := q.Snapshot()
	assert.Len(t, snap, 3)

	restored := Restore(snap)
	assert.Equal(t, want, drain(restored))
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestEventQueue -v`
Expected: FAIL to compile — the old generic `NewEventQueue[T]` no longer matches `NewEventQueue()`, and `Event`/`Restore`/`PeekAhead`/`Snapshot`/`Pop` are undefined.

- [ ] **Step 4: Rewrite the implementation**

Replace the entire contents of `sim/sim_eventqueue.go`:

```go
package sim

import (
	"container/heap"
	"slices"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// Event is a scheduled occurrence about a single entity. It carries no payload
// beyond these fields; a handler resolves what to do from the entity's
// components and from Key.
type Event struct {
	// Time is the game time at which the event is due.
	Time util.Time

	// Entity is the entity the event concerns.
	Entity ecs.EntityId

	// Key is a client-defined discriminator, typically the event type. It may
	// pack a type, subtype, or counter into its bits. It participates in
	// ordering and uniqueness.
	Key uint64
}

// eventLess reports whether a sorts before b under the queue's hardwired
// lexicographic order: Time first, then Key, then Entity.
func eventLess(a, b Event) bool {
	if a.Time != b.Time {
		return a.Time < b.Time
	}
	if a.Key != b.Key {
		return a.Key < b.Key
	}
	return a.Entity.Compare(b.Entity) < 0
}

type internalEventQueue struct {
	elements []Event
}

func (q *internalEventQueue) Len() int { return len(q.elements) }

func (q *internalEventQueue) Less(i, j int) bool {
	return eventLess(q.elements[i], q.elements[j])
}

func (q *internalEventQueue) Swap(i, j int) {
	q.elements[i], q.elements[j] = q.elements[j], q.elements[i]
}

func (q *internalEventQueue) Push(x any) {
	q.elements = append(q.elements, x.(Event))
}

func (q *internalEventQueue) Pop() any {
	old := q.elements
	n := len(old)
	e := old[n-1]
	q.elements = slices.Delete(old, n-1, n) // Avoid memory leak.
	return e
}

// EventQueue is a deterministic min-heap of scheduled events. Dequeue order is a
// pure function of the queued set (lexicographic by Time, then Key, then
// Entity), independent of insertion order.
type EventQueue struct {
	internal *internalEventQueue
}

// NewEventQueue returns an empty EventQueue.
func NewEventQueue() *EventQueue {
	return &EventQueue{internal: &internalEventQueue{}}
}

// Restore rebuilds a queue from a snapshot (as returned by Snapshot). Input
// order does not matter; the result dequeues in the queue's canonical order.
func Restore(events []Event) *EventQueue {
	q := NewEventQueue()
	q.internal.elements = append(q.internal.elements, events...)
	heap.Init(q.internal)
	return q
}

// Len returns the number of queued events.
func (q *EventQueue) Len() int { return q.internal.Len() }

// Add inserts e into the queue.
func (q *EventQueue) Add(e Event) { heap.Push(q.internal, e) }

// Peek returns the earliest event without removing it. ok is false if the queue
// is empty.
func (q *EventQueue) Peek() (Event, bool) {
	if len(q.internal.elements) == 0 {
		return Event{}, false
	}
	return q.internal.elements[0], true
}

// Pop removes and returns the earliest event. ok is false if the queue is empty.
func (q *EventQueue) Pop() (Event, bool) {
	if len(q.internal.elements) == 0 {
		return Event{}, false
	}
	return heap.Pop(q.internal).(Event), true
}

// PeekAhead returns the next n events in dequeue order without removing them,
// clamped to the number queued. It is for UI read-ahead and is not on the hot
// path; it copies the backing heap.
func (q *EventQueue) PeekAhead(n int) []Event {
	if n > q.Len() {
		n = q.Len()
	}
	if n <= 0 {
		return nil
	}
	scratch := &internalEventQueue{elements: append([]Event(nil), q.internal.elements...)}
	result := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, heap.Pop(scratch).(Event))
	}
	return result
}

// Snapshot returns every queued event in unspecified order, for serialization
// and for rebuilding a queue with Restore.
func (q *EventQueue) Snapshot() []Event {
	return append([]Event(nil), q.internal.elements...)
}
```

- [ ] **Step 5: Rewrite the benchmark**

Replace the entire contents of `sim/sim_eventqueue_bench_test.go`:

```go
package sim

import (
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// benchEntities pre-allocates a pool of entities to key benchmark events on.
func benchEntities(n int) []ecs.EntityId {
	w := ecs.NewWorld()
	ids := make([]ecs.EntityId, n)
	for i := range ids {
		ids[i] = w.NewEntity()
	}
	return ids
}

// BenchmarkEventQueueAdd measures raw insertion into a growing queue.
func BenchmarkEventQueueAdd(b *testing.B) {
	pool := benchEntities(1024)
	q := NewEventQueue()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Add(Event{Time: util.Time((i * 2654435761) & 0xffffff), Entity: pool[i&1023], Key: uint64(i)})
	}
}

// BenchmarkEventQueueSteadyState measures the scheduler pattern at fixed
// occupancy: pop the earliest event and schedule a new future one.
func BenchmarkEventQueueSteadyState(b *testing.B) {
	pool := benchEntities(1024)
	for _, n := range []int{100, 1000, 10000, 100000} {
		b.Run(sizeLabel(n), func(b *testing.B) {
			q := NewEventQueue()
			for i := 0; i < n; i++ {
				q.Add(Event{Time: util.Time((i * 2654435761) & 0xffffff), Entity: pool[i&1023], Key: uint64(i)})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e, _ := q.Pop()
				q.Add(Event{Time: e.Time + 0x1000, Entity: e.Entity, Key: e.Key})
			}
		})
	}
}

func sizeLabel(n int) string {
	switch {
	case n >= 100000:
		return "n=100k"
	case n >= 10000:
		return "n=10k"
	case n >= 1000:
		return "n=1k"
	default:
		return "n=100"
	}
}
```

- [ ] **Step 6: Run tests and benchmark smoke to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestEventQueue -v && go test ./sim/ -run '^$' -bench BenchmarkEventQueueAdd -benchtime=10x`
Expected: PASS for all `TestEventQueue…`; the benchmark compiles and runs.

- [ ] **Step 7: Lint and vet**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go vet ./sim/ && task lint`
Expected: no errors or warnings.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum sim/sim_eventqueue.go sim/sim_eventqueue_test.go sim/sim_eventqueue_bench_test.go
git commit -m "Replace generic sim.EventQueue with concrete Event queue keyed by (Time, Key, Entity)"
```

---

### Task 2: Driver

Replace the `EventSource`-based driver with a single-queue, handler-driven `Driver`.

**Files:**
- Rewrite: `sim/sim_driver.go`
- Rewrite: `sim/sim_driver_test.go`

**Interfaces:**
- Consumes: `Event`, `EventQueue`, `NewEventQueue`, `Add`, `Peek`, `Pop` from Task 1; `util.Time`; `ecs` (tests).
- Produces:
  - `type TickSystem interface { Tick(elapsed time.Duration) }`
  - `type EventHandler interface { HandleEvent(now util.Time, e Event) }`
  - `func NewDriver(handler EventHandler) *Driver`
  - `func (d *Driver) RegisterTickSystem(s TickSystem)`
  - `func (d *Driver) Queue() *EventQueue`
  - `func (d *Driver) Now() util.Time`
  - `func (d *Driver) RestoreNow(t util.Time)`
  - `func (d *Driver) RunUntil(target util.Time)`

- [ ] **Step 1: Write the failing tests**

Replace the entire contents of `sim/sim_driver_test.go`:

```go
package sim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// recordingHandler records handled events in order. onHandle, if set, runs after
// each event and may schedule follow-up events.
type recordingHandler struct {
	handled  []Event
	onHandle func(now util.Time, e Event)
}

func (h *recordingHandler) HandleEvent(now util.Time, e Event) {
	h.handled = append(h.handled, e)
	if h.onHandle != nil {
		h.onHandle(now, e)
	}
}

// recordingTick records the elapsed duration of every Tick.
type recordingTick struct {
	elapsed []time.Duration
}

func (r *recordingTick) Tick(elapsed time.Duration) { r.elapsed = append(r.elapsed, elapsed) }

// labeledTick appends its label to a shared log on each Tick.
type labeledTick struct {
	label string
	log   *[]string
}

func (t *labeledTick) Tick(_ time.Duration) { *t.log = append(*t.log, t.label) }

func TestDriverStopPointsAndElapsed(t *testing.T) {
	e := newEntities(2)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)
	d.Queue().Add(Event{Time: util.Time(3), Key: 1, Entity: e[0]})
	d.Queue().Add(Event{Time: util.Time(7), Key: 1, Entity: e[1]})

	d.RunUntil(util.Time(10))

	assert.Equal(t, []time.Duration{3, 4, 3}, tick.elapsed)
	assert.Equal(t, util.Time(10), d.Now())
	require.Len(t, h.handled, 2)
	assert.Equal(t, util.Time(3), h.handled[0].Time)
	assert.Equal(t, util.Time(7), h.handled[1].Time)
}

func TestDriverSameInstantCascade(t *testing.T) {
	e := newEntities(2)
	h := &recordingHandler{}

	d := NewDriver(h)
	// Handling the key-1 event schedules a key-2 event at the same instant.
	h.onHandle = func(now util.Time, ev Event) {
		if ev.Key == 1 {
			d.Queue().Add(Event{Time: now, Key: 2, Entity: e[1]})
		}
	}
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	d.RunUntil(util.Time(10))

	require.Len(t, h.handled, 2)
	assert.Equal(t, uint64(1), h.handled[0].Key)
	assert.Equal(t, uint64(2), h.handled[1].Key)
	assert.Equal(t, util.Time(5), h.handled[1].Time) // cascaded event handled at instant 5
	assert.Equal(t, util.Time(10), d.Now())
}

func TestDriverPastEventDoesNotRewindClock(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)

	d.RunUntil(util.Time(10))
	// Schedule an event in the past, then advance to 20.
	d.Queue().Add(Event{Time: util.Time(4), Key: 1, Entity: e[0]})
	d.RunUntil(util.Time(20))

	require.Len(t, h.handled, 1)
	assert.Equal(t, util.Time(20), d.Now())
	assert.Equal(t, []time.Duration{10, 0, 10}, tick.elapsed)
}

func TestDriverNoEventsAdvancesToTarget(t *testing.T) {
	tick := &recordingTick{}
	d := NewDriver(&recordingHandler{})
	d.RegisterTickSystem(tick)

	d.RunUntil(util.Time(8))

	assert.Equal(t, util.Time(8), d.Now())
	assert.Equal(t, []time.Duration{8}, tick.elapsed)
}

func TestDriverRunsTickSystemsInRegistrationOrder(t *testing.T) {
	e := newEntities(1)
	var log []string
	d := NewDriver(&recordingHandler{})
	d.RegisterTickSystem(&labeledTick{label: "first", log: &log})
	d.RegisterTickSystem(&labeledTick{label: "second", log: &log})
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	d.RunUntil(util.Time(10))

	assert.Equal(t, []string{"first", "second", "first", "second"}, log)
}

func TestDriverRestoreNow(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)
	d.RestoreNow(util.Time(100))
	assert.Equal(t, util.Time(100), d.Now())

	d.Queue().Add(Event{Time: util.Time(103), Key: 1, Entity: e[0]})
	d.RunUntil(util.Time(105))

	require.Len(t, h.handled, 1)
	assert.Equal(t, util.Time(103), h.handled[0].Time)
	assert.Equal(t, util.Time(105), d.Now())
	assert.Equal(t, []time.Duration{3, 2}, tick.elapsed) // 100->103, 103->105
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestDriver -v`
Expected: FAIL to compile — `NewDriver` now takes a handler, and `EventHandler`/`Queue`/`RestoreNow` are undefined (the old `EventSource`/`RegisterEventSource` API is gone).

- [ ] **Step 3: Rewrite the implementation**

Replace the entire contents of `sim/sim_driver.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestDriver -v`
Expected: PASS for all `TestDriver…`.

- [ ] **Step 5: Run the full package suite, lint, and vet**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ && go vet ./sim/ && task lint`
Expected: all pass, no warnings.

- [ ] **Step 6: Commit**

```bash
git add sim/sim_driver.go sim/sim_driver_test.go
git commit -m "Rewrite sim.Driver: single event queue drained through one EventHandler"
```

---

### Task 3: Package doc and full verification

Update the package doc to v2 and confirm the repository's required gates pass.

**Files:**
- Rewrite: `sim/doc.go`

- [ ] **Step 1: Rewrite the package doc**

Replace the entire contents of `sim/doc.go`:

```go
// Package sim provides deterministic event scheduling for games built on
// vantage. It holds only scheduling and time-advancement machinery; game
// content (what an event does) stays in the consuming game.
//
// Key exports:
//   - Event: a scheduled occurrence about a single entity, ordered
//     lexicographically by Time, then Key (a client-defined discriminator),
//     then Entity, so dequeue order is a pure function of the queued set.
//   - EventQueue: a min-heap of Events with ordered read-ahead (PeekAhead) and
//     in-memory snapshot and rebuild (Snapshot, Restore).
//   - Driver: owns the game clock and advances it event by event, running
//     registered TickSystems over each interval and draining the event queue at
//     each stop through a single EventHandler, resolving same-instant cascades
//     before the clock moves.
package sim
```

- [ ] **Step 2: Run the required headless suite**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task test:headless`
Expected: PASS, including the `sim` package.

- [ ] **Step 3: Repo-wide lint and vet**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...`
Expected: PASS. The ecs bump in Task 1 is the only dependency change; the full module builds against ecs v0.1.1.

- [ ] **Step 4: Commit**

```bash
git add sim/doc.go
git commit -m "Document sim v2 package: Event, EventQueue, Driver"
```

---

## Notes for the implementer

- `util.Time` is `type Time time.Duration`; `<`/`==` work directly, and `Time.Sub(other) time.Duration` gives the elapsed duration passed to `TickSystem.Tick`.
- `ecs.EntityId` is opaque: create entities in tests via `ecs.NewWorld()` then `world.NewEntity()`. Allocation is monotonic, so an earlier-allocated entity sorts before a later one under `Compare`.
- Do not add an insertion-order fallback in `eventLess`; duplicate `(Time, Key, Entity)` keys stay unspecified by contract.
- The `EventQueue` mirrors the structure of `util.PriorityQueue` (a private `internal…` heap behind a public wrapper) for codebase consistency; the difference is the concrete `Event` element and the three-level `eventLess`.
