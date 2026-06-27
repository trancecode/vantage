package app

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// screenshotCapturer schedules screenshots at exact game-time targets. It works
// in game time — the duration by which the engine advances the simulation each
// frame — not wall-clock time: delay and frequency are game-time offsets.
type screenshotCapturer struct {
	path      string
	delay     time.Duration
	frequency time.Duration
	sequence  bool

	gameTime      time.Duration // game time advanced so far
	carry         time.Duration // game time deferred from a clamped frame
	captureCount  int
	shouldCapture bool
	done          bool
}

func newScreenshotCapturer(path string, delay, frequency time.Duration) *screenshotCapturer {
	return &screenshotCapturer{
		path:      path,
		delay:     delay,
		frequency: frequency,
		sequence:  strings.Contains(path, "%"),
	}
}

// advance reports how much game time the engine should actually advance this
// frame, given the proposed frameDuration. When advancing by the full amount
// would cross the next screenshot's target game time, it clamps the advance so
// game time lands exactly on the target, defers the remainder to the next frame
// (carry), and flags a capture. While that capture is pending (set here,
// consumed by capture in Draw) it returns 0, holding the simulation at the
// target until the frame is grabbed. So screenshots always land on exact
// game-time targets, regardless of how Ebiten interleaves Update and Draw.
func (s *screenshotCapturer) advance(frameDuration time.Duration) time.Duration {
	if s.done {
		return frameDuration
	}
	if s.shouldCapture {
		// Hold the simulation until the pending capture has been drawn.
		return 0
	}

	d := frameDuration + s.carry
	s.carry = 0

	target := s.nextTarget()
	if target < 0 || s.gameTime+d < target {
		s.gameTime += d
		return d
	}

	// Clamp so game time lands exactly on the target; carry the remainder.
	advance := target - s.gameTime
	s.carry = d - advance
	s.gameTime = target
	s.shouldCapture = true
	return advance
}

// nextTarget returns the game time of the next screenshot, or a negative value
// if none remains (a single shot already taken, or a sequence with no positive
// frequency).
func (s *screenshotCapturer) nextTarget() time.Duration {
	if !s.sequence {
		if s.captureCount > 0 {
			return -1
		}
		return s.delay
	}
	if s.frequency <= 0 {
		return -1
	}
	return s.delay + time.Duration(s.captureCount)*s.frequency
}

// capture writes a screenshot of screen if one is due this frame.
func (s *screenshotCapturer) capture(screen *ebiten.Image) {
	if !s.shouldCapture {
		return
	}
	s.shouldCapture = false
	s.captureCount++

	path := s.path
	if s.sequence {
		path = fmt.Sprintf(s.path, s.captureCount)
	} else {
		s.done = true
	}

	if err := SaveScreenshot(screen, path); err != nil {
		logger.Error().Err(err).Msgf("Failed to save screenshot to %s", path)
	} else {
		logger.Info().Msgf("Screenshot saved: %s", path)
	}
}

// SaveScreenshot encodes img as a PNG at filePath, creating parent directories.
func SaveScreenshot(img *ebiten.Image, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]byte, width*height*4)
	img.ReadPixels(pixels)

	rgbaImg := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		pixelIndex := i / 4
		x := pixelIndex % width
		y := pixelIndex / width
		rgbaImg.Set(x, y, color.RGBA{R: pixels[i], G: pixels[i+1], B: pixels[i+2], A: pixels[i+3]})
	}

	if err := png.Encode(file, rgbaImg); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode screenshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close screenshot file: %w", err)
	}
	return nil
}
