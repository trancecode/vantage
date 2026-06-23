package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// Logger is the global logger instance
	Logger zerolog.Logger
)

// gameTimeConsoleWriter is a custom writer that formats game logs with game time 
// positioned right after the level and before the message, as requested in issue #37
type gameTimeConsoleWriter struct {
	out     io.Writer
	noColor bool
}

func (w *gameTimeConsoleWriter) Write(p []byte) (n int, err error) {
	// Parse the JSON log entry
	var logEntry map[string]interface{}
	if err := json.Unmarshal(p, &logEntry); err != nil {
		// If it's not JSON, just pass through
		return w.out.Write(p)
	}
	
	// Format manually in the desired format: timestamp level [game_time] message fields
	var output bytes.Buffer
	
	// Timestamp
	if timestamp, ok := logEntry["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			output.WriteString(t.Format(time.DateTime))
		}
	}
	
	// Level
	if level, ok := logEntry["level"].(string); ok {
		output.WriteString(" ")
		// Convert level to uppercase short form to match zerolog ConsoleWriter style
		switch level {
		case "trace":
			output.WriteString("TRC")
		case "debug":
			output.WriteString("DBG")
		case "info":
			output.WriteString("INF")
		case "warn":
			output.WriteString("WRN")
		case "error":
			output.WriteString("ERR")
		case "fatal":
			output.WriteString("FTL")
		case "panic":
			output.WriteString("PNC")
		default:
			output.WriteString(level)
		}
	}
	
	// Game time (right after level) - this is the key fix for issue #37
	if gameTimeRaw, ok := logEntry["game_time"]; ok {
		var gameTimeStr string
		
		// Convert the raw value to a proper duration string using DurationString
		switch v := gameTimeRaw.(type) {
		case json.Number:
			// JSON numbers are stored as json.Number
			if ns, err := v.Int64(); err == nil {
				gameTimeStr = DurationString(time.Duration(ns))
			}
		case float64:
			// Sometimes JSON numbers become float64
			ns := int64(v)
			gameTimeStr = DurationString(time.Duration(ns))
		case int64:
			gameTimeStr = DurationString(time.Duration(v))
		case Time:
			// If it's our custom Time type
			gameTimeStr = v.String()
		case time.Duration:
			gameTimeStr = DurationString(v)
		default:
			// Fallback to string representation
			gameTimeStr = fmt.Sprintf("%v", v)
		}
		
		// Handle zero duration specially for display
		if gameTimeStr == "" {
			gameTimeStr = "0s"
		}
		
		output.WriteString(" [")
		output.WriteString(gameTimeStr)
		output.WriteString("]")
	}
	
	// Message
	if message, ok := logEntry["message"].(string); ok {
		output.WriteString(" ")
		output.WriteString(message)
	}
	
	// Other fields (excluding standard ones and game_time)
	for key, value := range logEntry {
		if key == "time" || key == "level" || key == "message" || key == "game_time" {
			continue
		}
		output.WriteString(fmt.Sprintf(" %s=%v", key, value))
	}
	
	output.WriteString("\n")
	return w.out.Write(output.Bytes())
}

// NewConsoleWriter creates a zerolog console writer with standardized formatting
func NewConsoleWriter() io.Writer {
	// Check if running inside VS Code Studio (when RUNNING_IN_VSCODE is set)
	// and disable color output to avoid ANSI escape codes in the output
	noColor := os.Getenv("RUNNING_IN_VSCODE") != ""
	
	return &gameTimeConsoleWriter{
		out:     os.Stdout,
		noColor: noColor,
	}
}

// InitLogging initializes the zerolog console writer and sets up the global logger
func InitLogging() {
	// Create console writer for pretty output
	consoleWriter := NewConsoleWriter()

	// Set the global logger
	Logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	// Also set the global zerolog logger
	log.Logger = Logger
}
