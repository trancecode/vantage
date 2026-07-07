package render

import (
	"testing"
)

// collect drains the list into a slice of payloads in iteration order.
func collect[T any](l *DrawList[T]) []T {
	got := make([]T, 0, l.Len())
	l.Each(func(payload T) {
		got = append(got, payload)
	})
	return got
}

func TestDrawListOrdersByLayerThenY(t *testing.T) {
	var list DrawList[string]
	// Add in a deliberately scrambled order so the sort has to do real work.
	list.Add(1, 5.0, "layer1-y5")
	list.Add(0, 9.0, "layer0-y9")
	list.Add(1, 2.0, "layer1-y2")
	list.Add(0, 1.0, "layer0-y1")

	got := collect(&list)
	want := []string{"layer0-y1", "layer0-y9", "layer1-y2", "layer1-y5"}
	if len(got) != len(want) {
		t.Fatalf("iterated %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDrawListIsStableForEqualKeys(t *testing.T) {
	var list DrawList[string]
	// All entries share the same (layer, Y) key, so insertion order must win.
	list.Add(3, 4.0, "first")
	list.Add(3, 4.0, "second")
	list.Add(3, 4.0, "third")

	got := collect(&list)
	want := []string{"first", "second", "third"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDrawListLayerBeatsY(t *testing.T) {
	var list DrawList[string]
	// A high-Y entry on a lower layer must still draw before a low-Y entry on a
	// higher layer: layer dominates Y.
	list.Add(1, 0.0, "layer1-y0")
	list.Add(0, 100.0, "layer0-y100")

	got := collect(&list)
	want := []string{"layer0-y100", "layer1-y0"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

func TestDrawListClearResetsEntries(t *testing.T) {
	var list DrawList[int]
	list.Add(0, 1.0, 10)
	list.Add(0, 2.0, 20)
	if list.Len() != 2 {
		t.Fatalf("Len after adds = %d, want 2", list.Len())
	}

	list.Clear()
	if list.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", list.Len())
	}

	// The list must be usable again after clearing.
	list.Add(0, 3.0, 30)
	got := collect(&list)
	if len(got) != 1 || got[0] != 30 {
		t.Fatalf("after refill = %v, want [30]", got)
	}
}

func TestDrawListEachIsRepeatable(t *testing.T) {
	var list DrawList[int]
	list.Add(2, 1.0, 2)
	list.Add(0, 1.0, 0)
	list.Add(1, 1.0, 1)

	first := collect(&list)
	second := collect(&list)
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("repeated Each differs at %d: %v vs %v", i, first, second)
		}
	}
	want := []int{0, 1, 2}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("order[%d] = %d, want %d", i, first[i], want[i])
		}
	}
}
