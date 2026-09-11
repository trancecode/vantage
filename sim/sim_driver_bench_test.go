package sim

import (
	"fmt"
	"testing"
	"time"

	"github.com/trancecode/vantage/util"
)

// rescheduleHandler schedules every event it handles again one beat later, so
// the queue holds the same events every beat.
type rescheduleHandler struct {
	queue *EventQueue
}

func (h *rescheduleHandler) HandleEvent(now util.Time, e Event) {
	e.Time = e.Time.Add(time.Second)
	h.queue.Add(e)
}

// noopTickSystem is a registered tick system that does nothing, so each stop
// pays the dispatch a game's own tick systems add to.
type noopTickSystem struct{}

func (noopTickSystem) Tick(time.Duration) {}

// BenchmarkDriverRunUntil measures one beat of event dispatch: one RunUntil
// advancing the clock by a second, dispatching every event due in it through a
// handler that schedules each again one beat later, with one registered no-op
// tick system. Events sit at distinct times spread across the beat, so each is
// its own stop where tick systems run. ns-per-event/op is the beat's cost
// divided by the events it dispatched.
func BenchmarkDriverRunUntil(b *testing.B) {
	pool := benchEntities(1024)
	for _, events := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("events=%d", events), func(b *testing.B) {
			handler := &rescheduleHandler{}
			driver := NewDriver(handler)
			handler.queue = driver.Queue()
			driver.RegisterTickSystem(noopTickSystem{})
			for i := range events {
				offset := time.Duration(i)*time.Second/time.Duration(events) + time.Nanosecond
				driver.Queue().Add(Event{Time: util.Time(0).Add(offset), Entity: pool[i%len(pool)], Key: uint64(i)})
			}

			beat := util.Time(0)
			driver.RunUntil(beat.Add(time.Second))
			beat = beat.Add(time.Second)
			if got := driver.Queue().Len(); got != events {
				b.Fatalf("after one beat the queue holds %d events, want %d", got, events)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				beat = beat.Add(time.Second)
				driver.RunUntil(beat)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns-per-event/op")
		})
	}
}
