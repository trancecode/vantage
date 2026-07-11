//go:build race

package util

import (
	"fmt"
)

var (
	DebugMode = true

	Log = ScreenLogger{}
)

// ScreenLogger is a logger that prints debug messages (stub for race builds)
type ScreenLogger struct {
	messages []string
}

// Printf stub for race builds
func (s *ScreenLogger) Printf(format string, a ...any) {
	if DebugMode {
		s.messages = append(s.messages, fmt.Sprintf(format, a...))
	}
}

// Print stub for race builds
func (s *ScreenLogger) Print(m string) {
	if DebugMode {
		s.messages = append(s.messages, m)
	}
}

// Draw stub for race builds - accepts any value to avoid type conflicts
func (s *ScreenLogger) Draw(screen any) {
	// Do nothing in race builds
	s.messages = nil
}

// PrintFpsCounter stub for race builds
func (s *ScreenLogger) PrintFpsCounter() {
	s.Printf("FPS: %f", 60.0) // Mock FPS value
}

// PrintProfiler queues one debug line per recorded phase (name, total time,
// average, call count), sorted by total time descending. Like the other prints
// it is gated by DebugMode and rendered by the next Draw.
func (s *ScreenLogger) PrintProfiler(p *Profiler) {
	for _, t := range p.Snapshot() {
		s.Printf("%-16s total %v  avg %v  x%d", t.Name, t.Total, t.Average(), t.Calls)
	}
}