package sim

import (
	"testing"

	"github.com/trancecode/vantage/util"
)

// benchEvent is a minimal Element with an integer tie-break, mirroring the
// per-entity key shape the games use.
type benchEvent struct {
	at  util.Time
	key int
}

func (e benchEvent) EventTime() util.Time          { return e.at }
func (e benchEvent) TieBreak(other benchEvent) int { return e.key - other.key }

// BenchmarkEventQueueAdd measures raw insertion into a growing queue.
func BenchmarkEventQueueAdd(b *testing.B) {
	q := NewEventQueue[benchEvent]()
	for i := 0; i < b.N; i++ {
		// Pseudo-random-ish times to avoid always inserting at the tail.
		q.Add(benchEvent{at: util.Time((i * 2654435761) & 0xffffff), key: i})
	}
}

// BenchmarkEventQueueSteadyState measures the real scheduler pattern at a fixed
// occupancy: pop the earliest event and immediately schedule a new future one,
// so the queue stays at size n across the loop.
func BenchmarkEventQueueSteadyState(b *testing.B) {
	for _, n := range []int{100, 1000, 10000, 100000} {
		b.Run(sizeLabel(n), func(b *testing.B) {
			q := NewEventQueue[benchEvent]()
			for i := 0; i < n; i++ {
				q.Add(benchEvent{at: util.Time((i * 2654435761) & 0xffffff), key: i})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e, _ := q.Next()
				// Reschedule further out so it does not re-pop immediately.
				q.Add(benchEvent{at: e.at + 0x1000, key: e.key})
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
