package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScreenLoggerPrintProfiler(t *testing.T) {
	prev := DebugMode
	DebugMode = true
	defer func() { DebugMode = prev }()

	p := NewProfiler()
	p.Record("move", 10*time.Millisecond)
	p.Record("ai", 5*time.Millisecond)

	var s ScreenLogger
	s.PrintProfiler(p)

	// One buffered line per phase, sorted by total descending (move before ai).
	assert.Len(t, s.messages, 2)
	assert.Contains(t, s.messages[0], "move")
	assert.Contains(t, s.messages[1], "ai")
}
