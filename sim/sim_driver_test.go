package sim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/vantage/util"
)

// recordingHandler records handled events in order. onHandle, if set, runs after
// each event and may schedule follow-up events.
type recordingHandler struct {
	handled  []Event
	onHandle func(now util.Time, e Event)
}

func (h *recordingHandler) HandleEvent(now util.Time, e Event) {
	h.handled = append(h.handled, e)
	if h.onHandle != nil {
		h.onHandle(now, e)
	}
}

// recordingTick records the elapsed duration of every Tick.
type recordingTick struct {
	elapsed []time.Duration
}

func (r *recordingTick) Tick(elapsed time.Duration) { r.elapsed = append(r.elapsed, elapsed) }

// labeledTick appends its label to a shared log on each Tick.
type labeledTick struct {
	label string
	log   *[]string
}

func (t *labeledTick) Tick(_ time.Duration) { *t.log = append(*t.log, t.label) }

func TestDriverStopPointsAndElapsed(t *testing.T) {
	e := newEntities(2)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)
	d.Queue().Add(Event{Time: util.Time(3), Key: 1, Entity: e[0]})
	d.Queue().Add(Event{Time: util.Time(7), Key: 1, Entity: e[1]})

	d.RunUntil(util.Time(10))

	assert.Equal(t, []time.Duration{3, 4, 3}, tick.elapsed)
	assert.Equal(t, util.Time(10), d.Now())
	require.Len(t, h.handled, 2)
	assert.Equal(t, util.Time(3), h.handled[0].Time)
	assert.Equal(t, util.Time(7), h.handled[1].Time)
}

func TestDriverSameInstantCascade(t *testing.T) {
	e := newEntities(2)
	h := &recordingHandler{}

	d := NewDriver(h)
	// Handling the key-1 event schedules a key-2 event at the same instant.
	h.onHandle = func(now util.Time, ev Event) {
		if ev.Key == 1 {
			d.Queue().Add(Event{Time: now, Key: 2, Entity: e[1]})
		}
	}
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	d.RunUntil(util.Time(10))

	require.Len(t, h.handled, 2)
	assert.Equal(t, uint64(1), h.handled[0].Key)
	assert.Equal(t, uint64(2), h.handled[1].Key)
	assert.Equal(t, util.Time(5), h.handled[1].Time) // cascaded event handled at instant 5
	assert.Equal(t, util.Time(10), d.Now())
}

func TestDriverPastEventDoesNotRewindClock(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)

	d.RunUntil(util.Time(10))
	// Schedule an event in the past, then advance to 20.
	d.Queue().Add(Event{Time: util.Time(4), Key: 1, Entity: e[0]})
	d.RunUntil(util.Time(20))

	require.Len(t, h.handled, 1)
	assert.Equal(t, util.Time(20), d.Now())
	assert.Equal(t, []time.Duration{10, 0, 10}, tick.elapsed)
}

func TestDriverNoEventsAdvancesToTarget(t *testing.T) {
	tick := &recordingTick{}
	d := NewDriver(&recordingHandler{})
	d.RegisterTickSystem(tick)

	d.RunUntil(util.Time(8))

	assert.Equal(t, util.Time(8), d.Now())
	assert.Equal(t, []time.Duration{8}, tick.elapsed)
}

func TestDriverRunsTickSystemsInRegistrationOrder(t *testing.T) {
	e := newEntities(1)
	var log []string
	d := NewDriver(&recordingHandler{})
	d.RegisterTickSystem(&labeledTick{label: "first", log: &log})
	d.RegisterTickSystem(&labeledTick{label: "second", log: &log})
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	d.RunUntil(util.Time(10))

	assert.Equal(t, []string{"first", "second", "first", "second"}, log)
}

func TestDriverRestoreNow(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	tick := &recordingTick{}

	d := NewDriver(h)
	d.RegisterTickSystem(tick)
	d.RestoreNow(util.Time(100))
	assert.Equal(t, util.Time(100), d.Now())

	d.Queue().Add(Event{Time: util.Time(103), Key: 1, Entity: e[0]})
	d.RunUntil(util.Time(105))

	require.Len(t, h.handled, 1)
	assert.Equal(t, util.Time(103), h.handled[0].Time)
	assert.Equal(t, util.Time(105), d.Now())
	assert.Equal(t, []time.Duration{3, 2}, tick.elapsed) // 100->103, 103->105
}

func TestDriverDispatchesEventExactlyAtTarget(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	d := NewDriver(h)
	d.Queue().Add(Event{Time: util.Time(10), Key: 1, Entity: e[0]})

	d.RunUntil(util.Time(10))

	require.Len(t, h.handled, 1)
	assert.Equal(t, util.Time(10), h.handled[0].Time) // event at exactly the target is dispatched
	assert.Equal(t, util.Time(10), d.Now())
}

func TestDriverRestoreQueue(t *testing.T) {
	e := newEntities(2)
	snap := []Event{
		{Time: util.Time(5), Key: 1, Entity: e[0]},
		{Time: util.Time(8), Key: 1, Entity: e[1]},
	}
	h := &recordingHandler{}
	d := NewDriver(h)
	d.RestoreQueue(Restore(snap))

	d.RunUntil(util.Time(10))

	require.Len(t, h.handled, 2)
	assert.Equal(t, util.Time(5), h.handled[0].Time)
	assert.Equal(t, util.Time(8), h.handled[1].Time)
}

func TestDriverProfilesTickSystemsAndDrain(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	tick := &recordingTick{}
	p := util.NewProfiler()

	d := NewDriver(h)
	d.SetProfiler(p)
	assert.Same(t, p, d.Profiler())
	d.RegisterTickSystem(tick)
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	// Two stops (advance to 5, advance to 10): the tick system and the drain
	// are each measured once per stop.
	d.RunUntil(util.Time(10))

	byName := map[string]int64{}
	for _, pt := range p.Snapshot() {
		byName[pt.Name] = pt.Calls
	}
	assert.Equal(t, int64(2), byName["*sim.recordingTick"], "tick system timed per stop, labeled by type")
	assert.Equal(t, int64(2), byName["sim.drain"], "event drain timed per stop")
}

func TestDriverWithoutProfilerRecordsNothing(t *testing.T) {
	e := newEntities(1)
	h := &recordingHandler{}
	d := NewDriver(h) // no profiler attached
	d.RegisterTickSystem(&recordingTick{})
	d.Queue().Add(Event{Time: util.Time(5), Key: 1, Entity: e[0]})

	assert.Nil(t, d.Profiler())
	assert.NotPanics(t, func() { d.RunUntil(util.Time(10)) })
	assert.Equal(t, util.Time(10), d.Now())
}
