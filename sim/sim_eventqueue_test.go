package sim

import (
	"cmp"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/vantage/util"
)

// testEvent is a minimal Element used to exercise the queue. key provides a
// strict tie-break among events sharing an EventTime.
type testEvent struct {
	at  util.Time
	key int
}

func (e testEvent) EventTime() util.Time { return e.at }

func (e testEvent) TieBreak(other testEvent) int { return cmp.Compare(e.key, other.key) }

// drain removes every element from q in order and returns them.
func drain(q *EventQueue[testEvent]) []testEvent {
	var got []testEvent
	for {
		e, ok := q.Next()
		if !ok {
			return got
		}
		got = append(got, e)
	}
}

func TestEventQueueLexicographicOrder(t *testing.T) {
	// Distinct times sort by time; equal times sort by tie-break key.
	events := []testEvent{
		{at: util.Time(30), key: 2},
		{at: util.Time(10), key: 5},
		{at: util.Time(30), key: 1},
		{at: util.Time(20), key: 9},
	}
	want := []testEvent{
		{at: util.Time(10), key: 5},
		{at: util.Time(20), key: 9},
		{at: util.Time(30), key: 1},
		{at: util.Time(30), key: 2},
	}

	q := NewEventQueue[testEvent]()
	for _, e := range events {
		q.Add(e)
	}

	require.Equal(t, len(want), q.Len())
	assert.Equal(t, want, drain(q))
	assert.Equal(t, 0, q.Len())
}

func TestEventQueueInsertionOrderIndependent(t *testing.T) {
	base := []testEvent{
		{at: util.Time(10), key: 1},
		{at: util.Time(10), key: 2},
		{at: util.Time(10), key: 3},
		{at: util.Time(20), key: 1},
		{at: util.Time(5), key: 7},
		{at: util.Time(30), key: 4},
	}

	q := NewEventQueue[testEvent]()
	for _, e := range base {
		q.Add(e)
	}
	want := drain(q)

	// Every shuffled insertion order must produce the identical dequeue order.
	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		shuffled := append([]testEvent(nil), base...)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		q := NewEventQueue[testEvent]()
		for _, e := range shuffled {
			q.Add(e)
		}
		assert.Equal(t, want, drain(q))
	}
}

func TestEventQueuePeekAndEmpty(t *testing.T) {
	q := NewEventQueue[testEvent]()

	_, ok := q.Peek()
	assert.False(t, ok)
	_, ok = q.Next()
	assert.False(t, ok)

	q.Add(testEvent{at: util.Time(40), key: 1})
	q.Add(testEvent{at: util.Time(20), key: 1})

	peeked, ok := q.Peek()
	require.True(t, ok)
	assert.Equal(t, testEvent{at: util.Time(20), key: 1}, peeked)
	// Peek does not remove.
	assert.Equal(t, 2, q.Len())

	next, ok := q.Next()
	require.True(t, ok)
	assert.Equal(t, testEvent{at: util.Time(20), key: 1}, next)
	assert.Equal(t, 1, q.Len())
}
