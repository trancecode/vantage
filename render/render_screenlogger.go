package render

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/trancecode/vantage/util"
)

// Log is the global screen logger instance.
var Log = ScreenLogger{}

// ScreenLogger is a logger that prints debug messages to the screen.
type ScreenLogger struct {
	messages []string
}

// Printf prints a formatted debug message to the screen.
func (s *ScreenLogger) Printf(format string, a ...any) {
	if util.DebugMode {
		s.messages = append(s.messages, fmt.Sprintf(format, a...))
	}
}

// Print prints a debug message to the screen.
func (s *ScreenLogger) Print(m string) {
	if util.DebugMode {
		s.messages = append(s.messages, m)
	}
}

// Draw draws the debug messages on the screen.
func (s *ScreenLogger) Draw(screen *ebiten.Image) {
	if !util.DebugMode {
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
// it is gated by util.DebugMode and rendered by the next Draw.
func (s *ScreenLogger) PrintProfiler(p *util.Profiler) {
	for _, t := range p.Snapshot() {
		s.Printf("%-16s total %v  avg %v  x%d", t.Name, t.Total, t.Average(), t.Calls)
	}
}
