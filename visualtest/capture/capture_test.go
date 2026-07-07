package capture

import (
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// runToDone drives Draw until the capturer reports Done, guarding against a
// scheduling bug that would loop forever. It returns the number of Draw calls
// made.
func runToDone(t *testing.T, c *StepCapturer) int {
	t.Helper()
	const maxFrames = 1000
	for frame := range maxFrames {
		if c.Done() {
			return frame
		}
		if err := c.Draw(nil); err != nil {
			t.Fatalf("Draw: %v", err)
		}
	}
	t.Fatalf("capturer did not finish within %d frames", maxFrames)
	return 0
}

func TestStepCapturerSchedule(t *testing.T) {
	const step = 10 * time.Millisecond
	var savedPaths []string
	advances := 0

	capturer, err := NewStepCapturer(StepCaptureConfig{
		Advance: func(s time.Duration) {
			if s != step {
				t.Errorf("Advance step = %v, want %v", s, step)
			}
			advances++
		},
		Step:        step,
		Every:       2,
		Count:       3,
		PathPattern: "frame_%03d.png",
		Save: func(_ *ebiten.Image, path string) error {
			savedPaths = append(savedPaths, path)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewStepCapturer: %v", err)
	}

	runToDone(t, capturer)

	wantPaths := []string{"frame_001.png", "frame_002.png", "frame_003.png"}
	if len(savedPaths) != len(wantPaths) {
		t.Fatalf("saved paths = %v, want %v", savedPaths, wantPaths)
	}
	for i, want := range wantPaths {
		if savedPaths[i] != want {
			t.Errorf("saved path %d = %q, want %q", i, savedPaths[i], want)
		}
	}

	// Captures land on frames 0, 2, 4; the simulation advances once per frame
	// but not past the final capture, so 4 advances over frames 0..3.
	if advances != 4 {
		t.Errorf("advances = %d, want 4", advances)
	}
	if !capturer.Done() {
		t.Error("Done() = false after capturing all frames")
	}
}

func TestStepCapturerDrawIsNoOpWhenDone(t *testing.T) {
	capturer, err := NewStepCapturer(StepCaptureConfig{
		Advance:     func(time.Duration) {},
		Step:        time.Millisecond,
		Every:       1,
		Count:       1,
		PathPattern: "f%d.png",
		Save:        func(*ebiten.Image, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("NewStepCapturer: %v", err)
	}

	if err := capturer.Draw(nil); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if !capturer.Done() {
		t.Fatal("Done() = false after the single capture")
	}

	extraSaves := 0
	capturer.config.Save = func(*ebiten.Image, string) error { extraSaves++; return nil }
	if err := capturer.Draw(nil); err != nil {
		t.Fatalf("Draw after Done: %v", err)
	}
	if extraSaves != 0 {
		t.Errorf("Draw after Done saved %d extra frames, want 0", extraSaves)
	}
}

func TestNewStepCapturerValidation(t *testing.T) {
	valid := StepCaptureConfig{
		Advance:     func(time.Duration) {},
		Step:        time.Millisecond,
		Every:       1,
		Count:       1,
		PathPattern: "f%d.png",
	}

	tests := []struct {
		name   string
		mutate func(*StepCaptureConfig)
	}{
		{"missing advance", func(c *StepCaptureConfig) { c.Advance = nil }},
		{"non-positive step", func(c *StepCaptureConfig) { c.Step = 0 }},
		{"non-positive every", func(c *StepCaptureConfig) { c.Every = 0 }},
		{"pattern without verb", func(c *StepCaptureConfig) { c.PathPattern = "frame.png" }},
		{"pattern with only an escaped percent", func(c *StepCaptureConfig) { c.PathPattern = "frame%%.png" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := valid
			tc.mutate(&config)
			if _, err := NewStepCapturer(config); err == nil {
				t.Fatalf("NewStepCapturer with %s = nil error, want error", tc.name)
			}
		})
	}
}

func TestNewStepCapturerDefaultsSave(t *testing.T) {
	capturer, err := NewStepCapturer(StepCaptureConfig{
		Advance:     func(time.Duration) {},
		Step:        time.Millisecond,
		Every:       1,
		PathPattern: "f%d.png",
	})
	if err != nil {
		t.Fatalf("NewStepCapturer: %v", err)
	}
	if capturer.config.Save == nil {
		t.Error("Save was not defaulted")
	}
}
