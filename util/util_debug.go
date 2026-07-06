//go:build !race

package util

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

var (
	// DebugMode indicates whether debug mode is enabled.
	DebugMode = true

	// Log is the global screen logger instance.
	Log = ScreenLogger{}
)

// ScreenLogger is a logger that prints debug messages to the screen.
type ScreenLogger struct {
	messages []string
}

// DebugPrintf prints a formatted debug message to the screen.
func (s *ScreenLogger) Printf(format string, a ...interface{}) {
	if DebugMode {
		s.messages = append(s.messages, fmt.Sprintf(format, a...))
	}
}

// DebugPrint prints a debug message to the screen.
func (s *ScreenLogger) Print(m string) {
	if DebugMode {
		s.messages = append(s.messages, m)
	}
}

// Draw draws the debug messages on the screen.
func (s *ScreenLogger) Draw(screen *ebiten.Image) {
	if !DebugMode {
		return
	}
	var sb strings.Builder
	for _, msg := range s.messages {
		sb.WriteString(msg)
		sb.WriteString("\n")
	}
	ebitenutil.DebugPrint(screen, sb.String())
	s.messages = nil
}

// PrintFpsCounter draws the FPS counter on the screen.
func (s *ScreenLogger) PrintFpsCounter() {
	s.Printf("FPS: %f", ebiten.ActualFPS())
}

// PrintProfiler queues one debug line per recorded phase (name, total time,
// average, call count), sorted by total time descending. Like the other prints
// it is gated by DebugMode and rendered by the next Draw.
func (s *ScreenLogger) PrintProfiler(p *Profiler) {
	for _, t := range p.Snapshot() {
		s.Printf("%-16s total %v  avg %v  x%d", t.Name, t.Total, t.Average(), t.Calls)
	}
}
