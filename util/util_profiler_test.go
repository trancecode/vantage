package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfilerRecordsAndSnapshotsSortedByTotal(t *testing.T) {
	p := NewProfiler()
	p.Record("move", 10*time.Millisecond)
	p.Record("ai", 5*time.Millisecond)
	p.Record("ai", 5*time.Millisecond)
	p.Record("move", 40*time.Millisecond)

	snap := p.Snapshot()
	require.Len(t, snap, 2)

	// Sorted by total time descending: move (50ms) before ai (10ms).
	assert.Equal(t, "move", snap[0].Name)
	assert.Equal(t, 50*time.Millisecond, snap[0].Total)
	assert.Equal(t, int64(2), snap[0].Calls)
	assert.Equal(t, 25*time.Millisecond, snap[0].Average())

	assert.Equal(t, "ai", snap[1].Name)
	assert.Equal(t, 10*time.Millisecond, snap[1].Total)
	assert.Equal(t, int64(2), snap[1].Calls)
	assert.Equal(t, 5*time.Millisecond, snap[1].Average())
}

func TestProfilerAverageZeroCallsIsZero(t *testing.T) {
	assert.Equal(t, time.Duration(0), PhaseTiming{Name: "x"}.Average())
}

func TestProfilerNilIsSafeNoOp(t *testing.T) {
	var p *Profiler // nil
	// Recording into a nil profiler is a no-op, so producers can guard cheaply
	// or record unconditionally without a panic.
	assert.NotPanics(t, func() { p.Record("x", time.Second) })
	assert.Nil(t, p.Snapshot())
}
