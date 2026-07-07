package render

import (
	"sort"
)

// DrawList collects drawable payloads and iterates them in painter's order:
// ascending layer first, then ascending Y within a layer. This draws entries
// further back (lower Y) before those in front, which is the canonical
// back-to-front ordering for 2D top-down games.
//
// The list is generic over the payload type, so callers keep their own
// notion of what a drawable is (a sprite, an animation, a callback). The
// state-to-payload mapping and the meaning of layer values live in the caller.
//
// A DrawList is meant to be reused across frames: Clear it, Add the current
// frame's entries, then iterate with Each. The zero value is an empty,
// ready-to-use list.
type DrawList[T any] struct {
	entries []drawEntry[T]
}

// drawEntry pairs a payload with the sort key used to order it.
type drawEntry[T any] struct {
	// layer is the coarse depth bucket; lower layers draw first.
	layer int

	// y is the world Y coordinate used to order entries within a layer;
	// lower values draw first (further back).
	y float64

	// payload is the caller-supplied value to iterate over.
	payload T
}

// Add appends payload keyed on its layer and Y coordinate. Entries added with
// equal keys keep their insertion order when iterated (see Each).
func (l *DrawList[T]) Add(layer int, y float64, payload T) {
	l.entries = append(l.entries, drawEntry[T]{layer: layer, y: y, payload: payload})
}

// Len returns the number of entries currently in the list.
func (l *DrawList[T]) Len() int {
	return len(l.entries)
}

// Clear removes all entries while retaining the underlying capacity, so the
// list can be refilled each frame without reallocating.
func (l *DrawList[T]) Clear() {
	l.entries = l.entries[:0]
}

// Each sorts the entries into painter's order (ascending layer, then ascending
// Y) and invokes visit for each payload in that order. The sort is stable, so
// entries sharing a (layer, Y) key are visited in insertion order. Sorting
// happens in place, so repeated calls without intervening mutations reuse the
// already-ordered slice.
func (l *DrawList[T]) Each(visit func(payload T)) {
	sort.SliceStable(l.entries, func(i, j int) bool {
		a, b := l.entries[i], l.entries[j]
		if a.layer != b.layer {
			return a.layer < b.layer
		}
		return a.y < b.y
	})
	for i := range l.entries {
		visit(l.entries[i].payload)
	}
}
