# Sim event queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a vantage `sim` package providing a generic deterministic event queue and a time-advancement driver that games use in place of their duplicated scheduling machinery.

**Architecture:** Two pieces in one new package. `EventQueue[T]` is a min-heap ordered lexicographically by `(EventTime, TieBreak)`, so dequeue order is a pure function of the queued set, independent of insertion order. `Driver` owns the game clock and advances it event by event, running registered tick systems over each elapsed interval and draining registered event sources at each stop until every source is quiet at the current instant.

**Tech Stack:** Go 1.26.4, standard library `container/heap`, `cmp`, `slices`, `time`. Tests use `testing` plus `github.com/stretchr/testify` (already a dependency).

## Global Constraints

- Go version is `1.26.4` (from `go.mod`); do not introduce any other version reference.
- Author Name `Claude Code`, Author Email `herve.quiroz+claude@gmail.com`, and **no** `Co-Authored-By:` line in commits (per `CLAUDE.md`).
- The `sim` package depends only on `github.com/trancecode/vantage/util` (for `util.Time`) plus the standard library. It must **not** import `ecs`, `motion`, or any game package.
- Follow `docs/styleguide.md`: document every exported type/function/field starting with its name; group and alphabetize imports (stdlib, third-party, local); avoid `else` after early return; use `panic()` for unrecoverable errors.
- File naming follows the package-prefix convention seen in `util/` (`util_priorityqueue.go`): source files are `sim_<topic>.go`, tests are `sim_<topic>_test.go`, package doc lives in `doc.go`.
- Before pushing: `export GOMODCACHE=/tmp/go-mod-cache` then run `task lint`, `task test:headless`, and `go vet` — all must pass.

## Out of scope (handled in other repos, do not attempt here)

- `ecs.EntityId.Compare` in `github.com/trancecode/ecs` — a prerequisite for game-side key derivation, being handled in parallel. The `sim` package does not need it.
- Migrating `lockstep/core` and `nrg/rts` (`queuedAction`/`queuedEffect`, `TimeSystem`, etc.) — those live in the game repos.

---

### Task 1: EventQueue

The generic, insertion-order-independent event queue. Mirrors the structure of the existing `util.PriorityQueue` (an `internal…` heap type behind a public wrapper) so the codebase stays consistent, but orders lexicographically by `(EventTime, TieBreak)` instead of a single integer priority.

**Files:**
- Create: `sim/doc.go`
- Create: `sim/sim_eventqueue.go`
- Test: `sim/sim_eventqueue_test.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/util` — `util.Time` (a `time.Duration`-backed comparable ordered type; `<`, `==` work directly).
- Produces (later tasks and games rely on these exact names/types):
  - `type Element[T any] interface { EventTime() util.Time; TieBreak(other T) int }`
  - `type EventQueue[T Element[T]] struct { … }`
  - `func NewEventQueue[T Element[T]]() *EventQueue[T]`
  - `func (q *EventQueue[T]) Add(element T)`
  - `func (q *EventQueue[T]) Peek() (T, bool)`
  - `func (q *EventQueue[T]) Next() (T, bool)`
  - `func (q *EventQueue[T]) Len() int`

- [ ] **Step 1: Write the failing tests**

Create `sim/sim_eventqueue_test.go`:

```go
package sim

import (
	"cmp"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/vantage/util"
)

// testEvent is a minimal Element used to exercise the queue. key provides a
// strict tie-break among events sharing an EventTime.
type testEvent struct {
	at  util.Time
	key int
}

func (e testEvent) EventTime() util.Time { return e.at }

func (e testEvent) TieBreak(other testEvent) int { return cmp.Compare(e.key, other.key) }

// drain removes every element from q in order and returns them.
func drain(q *EventQueue[testEvent]) []testEvent {
	var got []testEvent
	for {
		e, ok := q.Next()
		if !ok {
			return got
		}
		got = append(got, e)
	}
}

func TestEventQueueLexicographicOrder(t *testing.T) {
	// Distinct times sort by time; equal times sort by tie-break key.
	events := []testEvent{
		{at: util.Time(30), key: 2},
		{at: util.Time(10), key: 5},
		{at: util.Time(30), key: 1},
		{at: util.Time(20), key: 9},
	}
	want := []testEvent{
		{at: util.Time(10), key: 5},
		{at: util.Time(20), key: 9},
		{at: util.Time(30), key: 1},
		{at: util.Time(30), key: 2},
	}

	q := NewEventQueue[testEvent]()
	for _, e := range events {
		q.Add(e)
	}

	require.Equal(t, len(want), q.Len())
	assert.Equal(t, want, drain(q))
	assert.Equal(t, 0, q.Len())
}

func TestEventQueueInsertionOrderIndependent(t *testing.T) {
	base := []testEvent{
		{at: util.Time(10), key: 1},
		{at: util.Time(10), key: 2},
		{at: util.Time(10), key: 3},
		{at: util.Time(20), key: 1},
		{at: util.Time(5), key: 7},
		{at: util.Time(30), key: 4},
	}

	q := NewEventQueue[testEvent]()
	for _, e := range base {
		q.Add(e)
	}
	want := drain(q)

	// Every shuffled insertion order must produce the identical dequeue order.
	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		shuffled := append([]testEvent(nil), base...)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		q := NewEventQueue[testEvent]()
		for _, e := range shuffled {
			q.Add(e)
		}
		assert.Equal(t, want, drain(q))
	}
}

func TestEventQueuePeekAndEmpty(t *testing.T) {
	q := NewEventQueue[testEvent]()

	_, ok := q.Peek()
	assert.False(t, ok)
	_, ok = q.Next()
	assert.False(t, ok)

	q.Add(testEvent{at: util.Time(40), key: 1})
	q.Add(testEvent{at: util.Time(20), key: 1})

	peeked, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, testEvent{at: util.Time(20), key: 1}, peeked)
	// Peek does not remove.
	assert.Equal(t, 2, q.Len())

	next, ok := q.Next()
	require.True(t, ok)
	assert.Equal(t, testEvent{at: util.Time(20), key: 1}, next)
	assert.Equal(t, 1, q.Len())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestEventQueue -v`
Expected: FAIL to compile — `undefined: NewEventQueue` / `undefined: EventQueue` / package `sim` has no Go files.

- [ ] **Step 3: Write the package doc**

Create `sim/doc.go`:

```go
// Package sim provides deterministic event scheduling for games built on
// vantage. It holds only scheduling and time-advancement machinery; game
// content (what an event does) stays in the consuming game.
//
// Key exports:
//   - EventQueue: a generic min-heap whose dequeue order is a pure function of
//     the queued set, ordered lexicographically by event time then a
//     caller-supplied tie-break, so insertion order cannot change outcomes.
//   - Driver: owns the game clock and advances it event by event, running
//     registered TickSystems over each elapsed interval and draining registered
//     EventSources at each stop until every source is quiet at that instant.
package sim
```

- [ ] **Step 4: Write the EventQueue implementation**

Create `sim/sim_eventqueue.go`:

```go
package sim

import (
	"container/heap"
	"slices"

	"github.com/trancecode/vantage/util"
)

// Element is implemented by values stored in an EventQueue.
type Element[T any] interface {
	// EventTime is the game time at which the element is due.
	EventTime() util.Time

	// TieBreak defines a strict total order among elements that share the same
	// EventTime. It returns a negative value if the receiver sorts before other,
	// positive if after. It must never return 0 for two distinct queued
	// elements: the queue does not fall back to insertion order, so a duplicate
	// key leaves the relative order of those elements unspecified.
	TieBreak(other T) int
}

// eventLess reports whether a sorts before b under the queue's hardwired
// lexicographic order: EventTime first, then TieBreak among same-time peers.
func eventLess[T Element[T]](a, b T) bool {
	if at, bt := a.EventTime(), b.EventTime(); at != bt {
		return at < bt
	}
	return a.TieBreak(b) < 0
}

type internalEventQueue[T Element[T]] struct {
	elements []T
}

func (q *internalEventQueue[T]) Len() int { return len(q.elements) }

func (q *internalEventQueue[T]) Less(i, j int) bool {
	return eventLess(q.elements[i], q.elements[j])
}

func (q *internalEventQueue[T]) Swap(i, j int) {
	q.elements[i], q.elements[j] = q.elements[j], q.elements[i]
}

func (q *internalEventQueue[T]) Push(element any) {
	q.elements = append(q.elements, element.(T))
}

func (q *internalEventQueue[T]) Pop() any {
	old := q.elements
	n := len(old)
	element := old[n-1]
	q.elements = slices.Delete(old, n-1, n) // Avoid memory leak.
	return element
}

// EventQueue is a generic deterministic min-heap of scheduled events. Elements
// dequeue in lexicographic order of (EventTime, TieBreak); the order is a pure
// function of the queued set, independent of insertion order.
type EventQueue[T Element[T]] struct {
	internal *internalEventQueue[T]
}

// NewEventQueue returns an empty EventQueue.
func NewEventQueue[T Element[T]]() *EventQueue[T] {
	return &EventQueue[T]{internal: &internalEventQueue[T]{}}
}

// Len returns the number of queued elements.
func (q *EventQueue[T]) Len() int { return q.internal.Len() }

// Add inserts element into the queue.
func (q *EventQueue[T]) Add(element T) { heap.Push(q.internal, element) }

// Peek returns the earliest element without removing it. The second return
// value is false if the queue is empty.
func (q *EventQueue[T]) Peek() (T, bool) {
	if len(q.internal.elements) == 0 {
		var empty T
		return empty, false
	}
	return q.internal.elements[0], true
}

// Next removes and returns the earliest element. The second return value is
// false if the queue is empty.
func (q *EventQueue[T]) Next() (T, bool) {
	if len(q.internal.elements) == 0 {
		var empty T
		return empty, false
	}
	return heap.Pop(q.internal).(T), true
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestEventQueue -v`
Expected: PASS (all three `TestEventQueue…` tests).

- [ ] **Step 6: Lint and vet**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go vet ./sim/ && task lint`
Expected: no errors, no warnings.

- [ ] **Step 7: Commit**

```bash
git add sim/doc.go sim/sim_eventqueue.go sim/sim_eventqueue_test.go
git commit -m "Add sim.EventQueue: deterministic lexicographic event queue"
```

---

### Task 2: Driver

The time-advancement driver generalizing `TimeSystem.RunUntil`. It owns the clock, runs tick systems over each elapsed interval, and drains event sources at each stop, resolving same-instant cascades before the clock moves.

**Files:**
- Create: `sim/sim_driver.go`
- Test: `sim/sim_driver_test.go`

**Interfaces:**
- Consumes: `util.Time`; `EventQueue[T]` and `Element[T]` from Task 1 (used only in the test's event-source helper).
- Produces:
  - `type TickSystem interface { Tick(elapsed time.Duration) }`
  - `type EventSource interface { NextEventTime() (t util.Time, ok bool); RunDue(now util.Time) }`
  - `type Driver struct { … }`
  - `func NewDriver() *Driver`
  - `func (d *Driver) RegisterTickSystem(s TickSystem)`
  - `func (d *Driver) RegisterEventSource(s EventSource)`
  - `func (d *Driver) Now() util.Time`
  - `func (d *Driver) RunUntil(target util.Time)`

- [ ] **Step 1: Write the failing tests**

Create `sim/sim_driver_test.go`:

```go
package sim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/trancecode/vantage/util"
)

// recordingTick records the elapsed duration of every Tick call.
type recordingTick struct {
	elapsed []time.Duration
}

func (r *recordingTick) Tick(elapsed time.Duration) {
	r.elapsed = append(r.elapsed, elapsed)
}

// testSource is an EventSource backed by an EventQueue. Each dispatched event
// is appended (with the source's label) to the shared log, so cross-source
// ordering is observable. onRun, if set, runs after each dispatch and may queue
// follow-up events to exercise cascades.
type testSource struct {
	label string
	queue *EventQueue[testEvent]
	log   *[]string
	onRun func(now util.Time, s *testSource)
}

func newTestSource(label string, log *[]string) *testSource {
	return &testSource{label: label, queue: NewEventQueue[testEvent](), log: log}
}

func (s *testSource) NextEventTime() (util.Time, bool) {
	if e, ok := s.queue.Peek(); ok {
		return e.EventTime(), true
	}
	return 0, false
}

func (s *testSource) RunDue(now util.Time) {
	for {
		e, ok := s.queue.Peek()
		if !ok || e.EventTime() > now {
			return
		}
		s.queue.Next()
		*s.log = append(*s.log, s.label)
		if s.onRun != nil {
			s.onRun(now, s)
		}
	}
}

func TestDriverStopPointsAndElapsed(t *testing.T) {
	var log []string
	src := newTestSource("a", &log)
	src.queue.Add(testEvent{at: util.Time(3), key: 1})
	src.queue.Add(testEvent{at: util.Time(7), key: 1})

	tick := &recordingTick{}

	d := NewDriver()
	d.RegisterTickSystem(tick)
	d.RegisterEventSource(src)

	d.RunUntil(util.Time(10))

	// Stops at 3 (event), 7 (event), 10 (target): elapsed 3, 4, 3.
	assert.Equal(t, []time.Duration{3, 4, 3}, tick.elapsed)
	assert.Equal(t, util.Time(10), d.Now())
	assert.Equal(t, []string{"a", "a"}, log)
}

func TestDriverSameInstantCascadeAcrossDrainedSource(t *testing.T) {
	var log []string
	first := newTestSource("first", &log)
	second := newTestSource("second", &log)

	// first has an event at 5 whose handler queues an event at 5 in second,
	// which is registered BEFORE first and so is drained before first each pass.
	first.queue.Add(testEvent{at: util.Time(5), key: 1})
	first.onRun = func(now util.Time, _ *testSource) {
		second.queue.Add(testEvent{at: now, key: 1})
	}

	d := NewDriver()
	d.RegisterEventSource(second)
	d.RegisterEventSource(first)

	d.RunUntil(util.Time(10))

	// The cascaded "second" event must be handled at instant 5, before the
	// clock advances to the target.
	assert.Equal(t, []string{"first", "second"}, log)
	assert.Equal(t, util.Time(10), d.Now())
}

func TestDriverDrainsInRegistrationOrder(t *testing.T) {
	var log []string
	src1 := newTestSource("src1", &log)
	src2 := newTestSource("src2", &log)

	src1.queue.Add(testEvent{at: util.Time(5), key: 1})
	src2.queue.Add(testEvent{at: util.Time(5), key: 1})

	d := NewDriver()
	d.RegisterEventSource(src1)
	d.RegisterEventSource(src2)

	d.RunUntil(util.Time(10))

	assert.Equal(t, []string{"src1", "src2"}, log)
}

func TestDriverPastEventDoesNotRewindClock(t *testing.T) {
	var log []string
	src := newTestSource("a", &log)
	tick := &recordingTick{}

	d := NewDriver()
	d.RegisterTickSystem(tick)
	d.RegisterEventSource(src)

	// Advance to 10 first.
	d.RunUntil(util.Time(10))
	// Now queue an event in the past and advance to 20.
	src.queue.Add(testEvent{at: util.Time(4), key: 1})
	d.RunUntil(util.Time(20))

	// The past event is dispatched, but the clock only ever moved forward.
	assert.Equal(t, []string{"a"}, log)
	assert.Equal(t, util.Time(20), d.Now())
	for _, e := range tick.elapsed {
		assert.GreaterOrEqual(t, e, time.Duration(0))
	}
}

func TestDriverNoEventsAdvancesToTarget(t *testing.T) {
	tick := &recordingTick{}
	d := NewDriver()
	d.RegisterTickSystem(tick)

	d.RunUntil(util.Time(8))

	assert.Equal(t, util.Time(8), d.Now())
	assert.Equal(t, []time.Duration{8}, tick.elapsed)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestDriver -v`
Expected: FAIL to compile — `undefined: NewDriver` / `undefined: Driver` / `undefined: TickSystem`.

- [ ] **Step 3: Write the Driver implementation**

Create `sim/sim_driver.go`:

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

// EventSource drains discrete events that are due at a given game time.
// EventQueue-backed sources wrap a queue plus the game logic that handles its
// events.
type EventSource interface {
	// NextEventTime returns the time of the earliest queued event. ok is false
	// when the source has no queued events.
	NextEventTime() (t util.Time, ok bool)

	// RunDue handles every event due at now (event time at or before now).
	// Handling an event may queue new events, including at now.
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
		// A past event (t <= now) must not rewind the clock; it is dispatched
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ -run TestDriver -v`
Expected: PASS (all `TestDriver…` tests).

- [ ] **Step 5: Run the full package test suite, lint, and vet**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./sim/ && go vet ./sim/ && task lint`
Expected: all pass, no warnings.

- [ ] **Step 6: Commit**

```bash
git add sim/sim_driver.go sim/sim_driver_test.go
git commit -m "Add sim.Driver: event-by-event time advancement with cascade drain"
```

---

### Task 3: Verify headless suite and finalize

Confirm the new package passes the repository's required headless test target and there are no version-skew or documentation regressions.

**Files:**
- Modify (only if a package map exists and lists packages): none expected — there is no `ARCHITECTURE.md`; `sim` is self-documenting via `sim/doc.go`.

- [ ] **Step 1: Run the required headless suite**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task test:headless`
Expected: PASS, including the new `sim` package (`sim` does not touch Ebiten/GLFW, so it also passes under `go test ./sim/`, but the repo's required gate is `task test:headless`).

- [ ] **Step 2: Confirm no version-skew or lint regressions repo-wide**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...`
Expected: PASS. (No new Go-version reference was introduced, so the version-sync check is unaffected.)

- [ ] **Step 3: Final commit if anything remains uncommitted**

Only if `git status` shows uncommitted changes from lint auto-formatting:

```bash
git status
git add -A
git commit -m "Format sim package"
```

---

## Notes for the implementer

- `util.Time` is `type Time time.Duration`; the comparison operators (`<`, `==`) work directly on `util.Time` values, and `Time.Sub(other) time.Duration` returns the elapsed duration. That is why `stop.Sub(d.now)` yields the `time.Duration` a `TickSystem.Tick` expects.
- The `EventQueue` deliberately mirrors `util.PriorityQueue`'s shape (private `internal…` heap type behind a public wrapper) for codebase consistency. The only real difference is `eventLess`, which compares lexicographically instead of by a single `Priority()`.
- Do not add insertion-order fallback in `eventLess`. The determinism contract requires that duplicate keys stay unspecified rather than silently first-in-first-out.
