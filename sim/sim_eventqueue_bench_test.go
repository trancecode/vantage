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
			for i := range n {
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
