package sim

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/util"
)

// newEntities allocates n entities in a fresh world. Allocation is monotonic,
// so entities[i].Compare(entities[j]) < 0 for i < j.
func newEntities(n int) []ecs.EntityId {
	w := ecs.NewWorld()
	ids := make([]ecs.EntityId, n)
	for i := range ids {
		ids[i] = w.NewEntity()
	}
	return ids
}

// drain removes every event from q in dequeue order.
func drain(q *EventQueue) []Event {
	var got []Event
	for {
		e, ok := q.Pop()
		if !ok {
			return got
		}
		got = append(got, e)
	}
}

func TestEventQueueLexicographicOrder(t *testing.T) {
	e := newEntities(3) // e[0] < e[1] < e[2] by Compare

	// Ordering priority is Time, then Key, then Entity.
	events := []Event{
		{Time: util.Time(30), Key: 2, Entity: e[0]},
		{Time: util.Time(10), Key: 5, Entity: e[2]},
		{Time: util.Time(30), Key: 1, Entity: e[2]},
		{Time: util.Time(30), Key: 1, Entity: e[0]},
	}
	want := []Event{
		{Time: util.Time(10), Key: 5, Entity: e[2]}, // earliest time
		{Time: util.Time(30), Key: 1, Entity: e[0]}, // key 1 before key 2; entity e[0] before e[2]
		{Time: util.Time(30), Key: 1, Entity: e[2]},
		{Time: util.Time(30), Key: 2, Entity: e[0]},
	}

	q := NewEventQueue()
	for _, ev := range events {
		q.Add(ev)
	}
	require.Equal(t, len(want), q.Len())
	assert.Equal(t, want, drain(q))
	assert.Equal(t, 0, q.Len())
}

func TestEventQueueInsertionOrderIndependent(t *testing.T) {
	e := newEntities(3)
	base := []Event{
		{Time: util.Time(10), Key: 1, Entity: e[0]},
		{Time: util.Time(10), Key: 1, Entity: e[1]},
		{Time: util.Time(10), Key: 2, Entity: e[0]},
		{Time: util.Time(20), Key: 1, Entity: e[2]},
		{Time: util.Time(5), Key: 7, Entity: e[1]},
	}

	q := NewEventQueue()
	for _, ev := range base {
		q.Add(ev)
	}
	want := drain(q)

	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		shuffled := append([]Event(nil), base...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		q := NewEventQueue()
		for _, ev := range shuffled {
			q.Add(ev)
		}
		assert.Equal(t, want, drain(q))
	}
}

func TestEventQueuePeekPopEmpty(t *testing.T) {
	e := newEntities(2)
	q := NewEventQueue()

	_, ok := q.Peek()
	assert.False(t, ok)
	_, ok = q.Pop()
	assert.False(t, ok)

	q.Add(Event{Time: util.Time(40), Key: 1, Entity: e[0]})
	q.Add(Event{Time: util.Time(20), Key: 1, Entity: e[1]})

	peeked, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, util.Time(20), peeked.Time)
	assert.Equal(t, 2, q.Len()) // Peek does not remove.

	popped, ok := q.Pop()
	require.True(t, ok)
	assert.Equal(t, util.Time(20), popped.Time)
	assert.Equal(t, 1, q.Len())
}

func TestEventQueuePeekAhead(t *testing.T) {
	e := newEntities(3)
	q := NewEventQueue()
	q.Add(Event{Time: util.Time(30), Key: 1, Entity: e[0]})
	q.Add(Event{Time: util.Time(10), Key: 1, Entity: e[1]})
	q.Add(Event{Time: util.Time(20), Key: 1, Entity: e[2]})

	ahead := q.PeekAhead(2)
	require.Len(t, ahead, 2)
	assert.Equal(t, util.Time(10), ahead[0].Time)
	assert.Equal(t, util.Time(20), ahead[1].Time)
	assert.Equal(t, 3, q.Len()) // PeekAhead does not remove.

	// n larger than Len returns all in order.
	all := q.PeekAhead(99)
	require.Len(t, all, 3)
	assert.Equal(t, util.Time(30), all[2].Time)
}

func TestEventQueueSnapshotRestoreRoundTrip(t *testing.T) {
	e := newEntities(3)
	q := NewEventQueue()
	q.Add(Event{Time: util.Time(30), Key: 2, Entity: e[0]})
	q.Add(Event{Time: util.Time(10), Key: 5, Entity: e[2]})
	q.Add(Event{Time: util.Time(30), Key: 1, Entity: e[1]})

	want := q.PeekAhead(99) // canonical order, non-destructive

	snap := q.Snapshot()
	assert.Len(t, snap, 3)

	restored := Restore(snap)
	assert.Equal(t, want, drain(restored))
}
