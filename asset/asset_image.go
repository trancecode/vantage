package asset

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png" // the format game sprites ship in; games blank-import others
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// LoadImage decodes image bytes (typically go:embed sprite data) into an
// ebiten.Image.
func LoadImage(b []byte) (*ebiten.Image, error) {
	raw, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	return ebiten.NewImageFromImage(raw), nil
}

// MustLoadImage decodes image bytes and panics on failure, for package
// initialization of embedded images.
func MustLoadImage(b []byte) *ebiten.Image {
	img, err := LoadImage(b)
	if err != nil {
		panic(fmt.Sprintf("loading image: %v", err))
	}
	return img
}

// ImageCache loads images from files and caches them by path, so repeated
// lookups of the same sprite sheet decode it once. It is safe for concurrent
// use.
type ImageCache struct {
	mu     sync.RWMutex
	images map[string]*ebiten.Image
}

// NewImageCache returns an empty cache.
func NewImageCache() *ImageCache {
	return &ImageCache{images: make(map[string]*ebiten.Image)}
}

// Load returns the image for path, decoding and caching it on first use.
// Concurrent loads of the same path may decode more than once but always
// return the same cached image.
func (c *ImageCache) Load(path string) (*ebiten.Image, error) {
	c.mu.RLock()
	img, ok := c.images[path]
	c.mu.RUnlock()
	if ok {
		return img, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image %s: %w", path, err)
	}
	img, err = LoadImage(b)
	if err != nil {
		return nil, fmt.Errorf("loading image %s: %w", path, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.images[path]; ok {
		return existing, nil
	}
	c.images[path] = img
	return img, nil
}

// MustLoad returns the image for path and panics on failure, for load-time
// setup of assets the game cannot run without.
func (c *ImageCache) MustLoad(path string) *ebiten.Image {
	img, err := c.Load(path)
	if err != nil {
		panic(fmt.Sprintf("loading image %s: %v", path, err))
	}
	return img
}
