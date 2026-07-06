package sim

import (
	"container/heap"
	"encoding/binary"
	"fmt"
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

// MarshalBinary encodes the queued events as a uint32 count followed by 24 bytes
// per event: Time (8, big-endian), Entity (8, via EntityId marshaling), Key
// (8, big-endian). The order is Snapshot order; because dequeue order is a pure
// function of the set, it need not be preserved.
func (q *EventQueue) MarshalBinary() ([]byte, error) {
	events := q.Snapshot()
	buf := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(events)*24), uint32(len(events)))
	for _, e := range events {
		buf = binary.BigEndian.AppendUint64(buf, uint64(e.Time))
		ent, err := e.Entity.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshaling event queue: %w", err)
		}
		buf = append(buf, ent...) // 8 bytes
		buf = binary.BigEndian.AppendUint64(buf, e.Key)
	}
	return buf, nil
}

// UnmarshalBinary replaces the queue's contents with events decoded from data as
// written by MarshalBinary.
func (q *EventQueue) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("sim.EventQueue.UnmarshalBinary: short header (%d bytes)", len(data))
	}
	n := binary.BigEndian.Uint32(data)
	data = data[4:]
	if len(data) != int(n)*24 {
		return fmt.Errorf("sim.EventQueue.UnmarshalBinary: expected %d event bytes, got %d", int(n)*24, len(data))
	}
	events := make([]Event, 0, n)
	for range n {
		var e Event
		e.Time = util.Time(binary.BigEndian.Uint64(data[0:8]))
		if err := e.Entity.UnmarshalBinary(data[8:16]); err != nil {
			return fmt.Errorf("unmarshaling event queue: %w", err)
		}
		e.Key = binary.BigEndian.Uint64(data[16:24])
		events = append(events, e)
		data = data[24:]
	}
	*q = *Restore(events)
	return nil
}

// indexOf returns the heap position of the queued event matching (entity, key),
// or -1 if none is queued. It assumes at most one queued event per
// (entity, key), consistent with how callers that reschedule or cancel key
// their events.
func (q *EventQueue) indexOf(entity ecs.EntityId, key uint64) int {
	for i, e := range q.internal.elements {
		if e.Key == key && e.Entity == entity {
			return i
		}
	}
	return -1
}

// Cancel removes and returns the queued event matching (entity, key). ok is
// false if no such event is queued. It assumes at most one event per
// (entity, key). Cancel scans the heap, so it is O(n); it is meant for
// occasional use (interrupting or cancelling a pending event), not the hot path.
func (q *EventQueue) Cancel(entity ecs.EntityId, key uint64) (Event, bool) {
	i := q.indexOf(entity, key)
	if i < 0 {
		return Event{}, false
	}
	return heap.Remove(q.internal, i).(Event), true
}

// Reschedule changes the time of the queued event matching (entity, key) to
// newTime and restores heap order, returning whether such an event was found.
// It assumes at most one event per (entity, key). Like Cancel it scans the heap
// (O(n)) and is meant for occasional use, such as delaying a pending event when
// its owner is staggered.
func (q *EventQueue) Reschedule(entity ecs.EntityId, key uint64, newTime util.Time) bool {
	i := q.indexOf(entity, key)
	if i < 0 {
		return false
	}
	q.internal.elements[i].Time = newTime
	heap.Fix(q.internal, i)
	return true
}
