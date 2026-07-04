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
