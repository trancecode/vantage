package util

import (
	"sort"
	"time"
)

// Profiler accumulates wall-clock time spent in named code paths, for debug and
// monitoring only. It is write-only from the simulation's point of view:
// nothing in game logic reads it, so recorded timings never influence behavior
// and never make a run non-deterministic. Producers that measure with the
// wall clock should skip measurement entirely when their profiler is nil, so
// profiling has zero cost when off.
//
// The zero-value Profiler is not ready for use; call NewProfiler. A nil
// *Profiler is a valid no-op receiver (Record does nothing, Snapshot returns
// nil), so callers can hold an optional profiler without nil checks at every
// record site.
type Profiler struct {
	phases map[string]*phaseStat
}

type phaseStat struct {
	total time.Duration
	calls int64
}

// NewProfiler returns an empty Profiler.
func NewProfiler() *Profiler {
	return &Profiler{phases: make(map[string]*phaseStat)}
}

// Record adds one timing observation to the named phase. It is a no-op on a nil
// Profiler.
func (p *Profiler) Record(name string, d time.Duration) {
	if p == nil {
		return
	}
	s, ok := p.phases[name]
	if !ok {
		s = &phaseStat{}
		p.phases[name] = s
	}
	s.total += d
	s.calls++
}

// PhaseTiming is a snapshot of one phase's accumulated timing.
type PhaseTiming struct {
	// Name is the phase's stable identifier (for example "ai_tick").
	Name string

	// Total is the summed wall time recorded for the phase.
	Total time.Duration

	// Calls is the number of observations recorded for the phase.
	Calls int64
}

// Average returns the mean time per call, or zero when there are no calls.
func (t PhaseTiming) Average() time.Duration {
	if t.Calls == 0 {
		return 0
	}
	return t.Total / time.Duration(t.Calls)
}

// Snapshot returns every recorded phase, sorted by total time descending (the
// order hotspot reports usually want). It returns nil on a nil Profiler.
func (p *Profiler) Snapshot() []PhaseTiming {
	if p == nil {
		return nil
	}
	out := make([]PhaseTiming, 0, len(p.phases))
	for name, s := range p.phases {
		out = append(out, PhaseTiming{Name: name, Total: s.total, Calls: s.calls})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}
