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
