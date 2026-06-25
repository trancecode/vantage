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

// screenshotCapturer tracks simulated time and decides when to capture frames.
type screenshotCapturer struct {
	path      string
	delay     time.Duration
	frequency time.Duration
	sequence  bool

	totalSimulatedTime time.Duration
	captureCount       int
	shouldCapture      bool
	done               bool
}

func newScreenshotCapturer(path string, delay, frequency time.Duration) *screenshotCapturer {
	return &screenshotCapturer{
		path:      path,
		delay:     delay,
		frequency: frequency,
		sequence:  strings.Contains(path, "%"),
	}
}

// tick advances simulated time and sets shouldCapture when a frame is due.
func (s *screenshotCapturer) tick(duration time.Duration) {
	s.totalSimulatedTime += duration
	if s.done || s.totalSimulatedTime < s.delay {
		return
	}
	if s.sequence {
		if s.frequency <= 0 {
			return
		}
		timeSinceDelay := s.totalSimulatedTime - s.delay
		expectedCaptures := int(timeSinceDelay/s.frequency) + 1
		if expectedCaptures > s.captureCount {
			s.shouldCapture = true
		}
	} else if s.captureCount == 0 {
		s.shouldCapture = true
	}
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
