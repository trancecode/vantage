// Package capture provides a fixed-step frame-capture helper for deterministic
// visual-regression testing. A [StepCapturer] advances a game-supplied
// simulation by a fixed game-time step once per frame and saves a screenshot
// every N frames, producing a frame sequence to diff (with the sibling
// visualtest package) against a golden set. It depends on Ebitengine, so it is
// kept separate from the display-free diff library.
package capture

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

// StepCaptureConfig configures a [StepCapturer].
type StepCaptureConfig struct {
	// Advance advances the game simulation by step of game time. It is called
	// once per frame from [StepCapturer.Draw]. A game using a StepCapturer must
	// not advance its own simulation elsewhere, so that the captured sequence
	// is a pure function of the step count. Required.
	Advance func(step time.Duration)

	// Step is the fixed game-time increment applied each frame. Must be
	// positive.
	Step time.Duration

	// Every captures a screenshot every Every frames, on frames 0, Every,
	// 2*Every, and so on. Must be positive.
	Every int

	// Count is the number of screenshots to capture before [StepCapturer.Done]
	// reports true. A value of zero or less captures indefinitely.
	Count int

	// PathPattern is a printf pattern receiving the 1-based capture index used
	// to build each frame's file path, e.g. "frames/frame_%03d.png". Must
	// contain a format verb. Parent directories are created as needed.
	PathPattern string

	// Save writes screen as a PNG at path. It defaults to an internal PNG
	// encoder ([SavePNG]); override it for tests or custom encoding.
	Save func(screen *ebiten.Image, path string) error
}

// StepCapturer advances a game-supplied simulation by a fixed game-time step
// once per frame and saves a screenshot every N frames, producing a
// deterministic frame sequence for visual-regression testing. Wire its Draw
// into the game's Draw (after the game has drawn the screen) and let its
// Advance hook be the only thing that advances the simulation.
type StepCapturer struct {
	config   StepCaptureConfig
	frame    int
	captured int
}

// NewStepCapturer validates config and returns a StepCapturer. It fails if the
// simulation hook is missing, the step or frame interval is not positive, or
// the path pattern has no format verb.
func NewStepCapturer(config StepCaptureConfig) (*StepCapturer, error) {
	if config.Advance == nil {
		return nil, fmt.Errorf("StepCaptureConfig.Advance must be set")
	}
	if config.Step <= 0 {
		return nil, fmt.Errorf("StepCaptureConfig.Step must be positive, got %v", config.Step)
	}
	if config.Every <= 0 {
		return nil, fmt.Errorf("StepCaptureConfig.Every must be positive, got %d", config.Every)
	}
	// A real verb must survive stripping escaped "%%", so "frame%%.png" (no
	// verb, would format to a fixed path every frame) is rejected.
	if !strings.Contains(strings.ReplaceAll(config.PathPattern, "%%", ""), "%") {
		return nil, fmt.Errorf("StepCaptureConfig.PathPattern %q must contain a format verb", config.PathPattern)
	}
	if config.Save == nil {
		config.Save = SavePNG
	}
	return &StepCapturer{config: config}, nil
}

// Draw saves the current screen when the frame is a capture frame, then
// advances the simulation by the configured step. Call it once per game Draw,
// after the game has rendered to screen. It is a no-op once [StepCapturer.Done]
// reports true, and it does not advance the simulation past the final capture.
func (c *StepCapturer) Draw(screen *ebiten.Image) error {
	if c.Done() {
		return nil
	}

	if c.frame%c.config.Every == 0 {
		c.captured++
		path := fmt.Sprintf(c.config.PathPattern, c.captured)
		if err := c.config.Save(screen, path); err != nil {
			return fmt.Errorf("saving capture %d to %q: %w", c.captured, path, err)
		}
		if c.Done() {
			return nil
		}
	}

	c.config.Advance(c.config.Step)
	c.frame++
	return nil
}

// Done reports whether the configured number of screenshots has been captured.
// It is always false when Count is zero or less (capture indefinitely).
func (c *StepCapturer) Done() bool {
	return c.config.Count > 0 && c.captured >= c.config.Count
}

// SavePNG encodes screen as a PNG at path, creating parent directories.
func SavePNG(screen *ebiten.Image, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %q: %w", dir, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return err // os.PathError already includes operation and filename
	}

	if err := png.Encode(file, imageFromScreen(screen)); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode png: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing %q: %w", path, err)
	}
	return nil
}

// imageFromScreen copies screen's pixels into an *image.RGBA for encoding.
func imageFromScreen(screen *ebiten.Image) *image.RGBA {
	bounds := screen.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	pixels := make([]byte, width*height*4)
	screen.ReadPixels(pixels)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		pixel := i / 4
		img.Set(pixel%width, pixel/width, color.RGBA{R: pixels[i], G: pixels[i+1], B: pixels[i+2], A: pixels[i+3]})
	}
	return img
}
