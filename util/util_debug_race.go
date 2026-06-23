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
func (s *ScreenLogger) Printf(format string, a ...interface{}) {
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

// Draw stub for race builds - accepts any interface{} to avoid type conflicts
func (s *ScreenLogger) Draw(screen interface{}) {
	// Do nothing in race builds
	s.messages = nil
}

// PrintFpsCounter stub for race builds
func (s *ScreenLogger) PrintFpsCounter() {
	s.Printf("FPS: %f", 60.0) // Mock FPS value
}