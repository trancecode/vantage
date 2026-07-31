package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/trancecode/vantage/util"
)

func TestScreenLoggerPrintProfiler(t *testing.T) {
	prev := util.DebugMode
	util.DebugMode = true
	defer func() { util.DebugMode = prev }()

	p := util.NewProfiler()
	p.Record("move", 10*time.Millisecond)
	p.Record("ai", 5*time.Millisecond)

	var s ScreenLogger
	s.PrintProfiler(p)

	// One buffered line per phase, sorted by total descending (move before ai).
	assert.Len(t, s.messages, 2)
	assert.Contains(t, s.messages[0], "move")
	assert.Contains(t, s.messages[1], "ai")
}
