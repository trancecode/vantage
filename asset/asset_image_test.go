package asset

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// encodeTestPNG returns the bytes of a 2×1 PNG with a red and a green pixel.
func encodeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLoadImageDecodesPNG(t *testing.T) {
	img, err := LoadImage(encodeTestPNG(t))
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 2 || h != 1 {
		t.Fatalf("image is %d×%d, want 2×1", w, h)
	}
}

func TestLoadImageRejectsGarbage(t *testing.T) {
	if _, err := LoadImage([]byte("not an image")); err == nil {
		t.Fatal("LoadImage must fail on undecodable bytes")
	}
}

func TestMustLoadImagePanicsOnGarbage(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustLoadImage must panic on undecodable bytes")
		}
	}()
	MustLoadImage([]byte("not an image"))
}

func TestImageCacheLoadsOnceAndReuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tile.png")
	if err := os.WriteFile(path, encodeTestPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := NewImageCache()
	first, err := cache.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	second, err := cache.Load(path)
	if err != nil {
		t.Fatalf("Load (cached): %v", err)
	}
	if first != second {
		t.Fatal("a second Load of the same path must return the cached image")
	}
}

func TestImageCacheMissingFile(t *testing.T) {
	cache := NewImageCache()
	if _, err := cache.Load(filepath.Join(t.TempDir(), "absent.png")); err == nil {
		t.Fatal("Load must fail for a missing file")
	}
}
