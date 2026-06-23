package util

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewConsoleWriter_DisablesColorInVSCode(t *testing.T) {
	// Test when RUNNING_IN_VSCODE is not set - should have colors enabled
	_ = os.Unsetenv("RUNNING_IN_VSCODE")
	writer1 := NewConsoleWriter()
	if writer1 == nil {
		t.Error("Expected writer to be created")
	}

	// Test when RUNNING_IN_VSCODE is set - should have colors disabled
	_ = os.Setenv("RUNNING_IN_VSCODE", "true")
	writer2 := NewConsoleWriter()
	if writer2 == nil {
		t.Error("Expected writer to be created")
	}

	// Test when RUNNING_IN_VSCODE is set to empty string - should have colors enabled (empty string means not set)
	_ = os.Setenv("RUNNING_IN_VSCODE", "")
	writer3 := NewConsoleWriter()
	if writer3 == nil {
		t.Error("Expected writer to be created")
	}

	// Clean up
	_ = os.Unsetenv("RUNNING_IN_VSCODE")
}

func TestNewConsoleWriter_OtherProperties(t *testing.T) {
	writer := NewConsoleWriter()

	// Verify that writer is created
	if writer == nil {
		t.Error("Expected writer to be created")
	}
}

func TestNewConsoleWriter_GameTimeFormatting(t *testing.T) {
	// Test with actual util.Time type (which will be used in practice)
	gameTime := Time(12*time.Second + 34*time.Millisecond)
	
	// Capture the output
	var buf bytes.Buffer
	testWriter := &gameTimeConsoleWriter{
		out:     &buf,
		noColor: true,
	}
	
	testLogger := zerolog.New(testWriter).With().Timestamp().Logger()
	gameLogger := testLogger.With().Interface("game_time", gameTime).Logger()
	
	gameLogger.Info().
		Str("component", "game").
		Str("entity", "103").
		Int("from_state", 2).
		Int("to_state", 3).
		Msg("Changing entity state")
	
	output := buf.String()
	
	// Verify the output contains the game time in the correct format and position
	if !strings.Contains(output, "[12s34ms]") {
		t.Errorf("Expected output to contain '[12s34ms]', got: %s", output)
	}
	
	// Verify the game time appears before the message (not at the end)
	gameTimePos := strings.Index(output, "[12s34ms]")
	messagePos := strings.Index(output, "Changing entity state")
	
	if gameTimePos == -1 || messagePos == -1 {
		t.Errorf("Could not find game time or message in output: %s", output)
	}
	
	if gameTimePos >= messagePos {
		t.Errorf("Expected game time to appear before message, got: %s", output)
	}
	
	// Verify that "game_time=" does NOT appear in the output (should be excluded)
	if strings.Contains(output, "game_time=") {
		t.Errorf("Expected 'game_time=' field to be excluded from output, got: %s", output)
	}
}

func TestNewConsoleWriter_GameTimeFormattingWithUtilTime(t *testing.T) {
	// Test with zero time
	var buf bytes.Buffer
	testWriter := &gameTimeConsoleWriter{
		out:     &buf,
		noColor: true,
	}
	
	testLogger := zerolog.New(testWriter).With().Timestamp().Logger()
	
	// Test with zero time
	zeroTime := Time(0)
	gameLogger := testLogger.With().Interface("game_time", zeroTime).Logger()
	gameLogger.Info().Msg("test message")
	
	output := buf.String()
	
	// The zero time should display as "[0s]"
	if !strings.Contains(output, "[0s]") {
		t.Errorf("Expected output to contain '[0s]' for zero time, got: %s", output)
	}
}