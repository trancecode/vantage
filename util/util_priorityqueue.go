package util

import (
	"container/heap"
	"slices"
)

type ElementWithPriority interface {
	Priority() int64
}

type internalPriorityQueue[T ElementWithPriority] struct {
	elements []T
}

func (pq *internalPriorityQueue[T]) Push(element any) {
	pq.elements = append(pq.elements, element.(T))
}

func (pq *internalPriorityQueue[T]) Pop() any {
	old := pq.elements
	n := len(old)
	element := old[n-1]
	pq.elements = slices.Delete(old, len(old)-1, len(old)) // Avoid memory leak
	return element
}

func (pq *internalPriorityQueue[T]) Len() int {
	return len(pq.elements)
}

func (pq *internalPriorityQueue[T]) Less(i, j int) bool {
	return pq.elements[i].Priority() < pq.elements[j].Priority()
}

func (pq *internalPriorityQueue[T]) Swap(i, j int) {
	pq.elements[i], pq.elements[j] = pq.elements[j], pq.elements[i]
}

type PriorityQueue[T ElementWithPriority] struct {
	internal *internalPriorityQueue[T]
}

func NewPriorityQueue[T ElementWithPriority]() *PriorityQueue[T] {
	return &PriorityQueue[T]{
		internal: &internalPriorityQueue[T]{},
	}
}

func (pq *PriorityQueue[T]) Len() int {
	return pq.internal.Len()
}

func (pq *PriorityQueue[T]) Add(element T) {
	heap.Push(pq.internal, element)
}

func (pq *PriorityQueue[T]) Peek() (T, bool) {
	if len(pq.internal.elements) == 0 {
		var empty T
		return empty, false
	}

	return pq.internal.elements[0], true
}

func (pq *PriorityQueue[T]) Next() (T, bool) {
	if len(pq.internal.elements) == 0 {
		var empty T
		return empty, false
	}

	return heap.Pop(pq.internal).(T), true
}
