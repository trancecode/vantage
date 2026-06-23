package app

import (
	"testing"
	"time"
)

func TestSingleCaptureAfterDelay(t *testing.T) {
	c := newScreenshotCapturer(ScreenshotConfig{Path: "/tmp/shot.png", Delay: time.Second})
	c.tick(500 * time.Millisecond)
	if c.shouldCapture {
		t.Fatal("should not capture before delay")
	}
	c.tick(600 * time.Millisecond) // total 1.1s >= 1s delay
	if !c.shouldCapture {
		t.Fatal("should capture once after delay")
	}
}

func TestSequenceCapturesAtFrequency(t *testing.T) {
	c := newScreenshotCapturer(ScreenshotConfig{
		Path:      "/tmp/frame-%d.png",
		Delay:     0,
		Frequency: time.Second,
	})
	if !c.sequence {
		t.Fatal("expected sequence mode for path containing percent-d verb")
	}
	c.tick(time.Second)
	if !c.shouldCapture {
		t.Fatal("expected first sequence capture")
	}
	c.shouldCapture = false
	c.captureCount = 1
	c.tick(time.Second) // total 2s, expectedCaptures = 3 > 1
	if !c.shouldCapture {
		t.Fatal("expected next sequence capture")
	}
}
