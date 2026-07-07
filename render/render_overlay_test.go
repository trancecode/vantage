package render

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

const overlayEps = 1e-9

// TestOverlayBottomScreenGapConstantAcrossZoom verifies that the screen-pixel
// gap between the sprite's visible top and the overlay bottom stays exactly
// gapPixels regardless of camera zoom, while the visible top itself tracks the
// sprite under zoom.
func TestOverlayBottomScreenGapConstantAcrossZoom(t *testing.T) {
	const (
		visibleTop = 10.0 // sprite pixels above the zero position
		gap        = 4.0  // constant screen-pixel gap
	)
	worldPos := geometry.NewVector2(3.0, -2.0)

	var prevVisibleTopY float64
	for i, zoom := range []float64{1.0, 3.0} {
		camera := NewCamera(800, 600)
		camera.SetZeroAsCenter()
		camera.SetZoom(zoom)

		bottom := overlayBottomScreen(camera, worldPos, visibleTop, gap)

		// The overlay bottom must be gap screen-pixels above the visible top,
		// independent of zoom.
		visibleTopY := camera.WorldToScreen(worldPos).Y() - visibleTop*camera.EffectiveZoom()
		if diff := (visibleTopY - bottom.Y()) - gap; diff > overlayEps || diff < -overlayEps {
			t.Fatalf("zoom %v: gap = %v, want %v", zoom, visibleTopY-bottom.Y(), gap)
		}

		// The overlay stays horizontally centered on the sprite.
		if diff := bottom.X() - camera.WorldToScreen(worldPos).X(); diff > overlayEps || diff < -overlayEps {
			t.Fatalf("zoom %v: center X = %v, want %v", zoom, bottom.X(), camera.WorldToScreen(worldPos).X())
		}

		// The visible top must move as zoom changes, proving the offset is
		// zoom-scaled rather than a fixed screen offset.
		if diff := visibleTopY - prevVisibleTopY; i > 0 && diff <= overlayEps && diff >= -overlayEps {
			t.Fatalf("visible top did not move with zoom: %v", visibleTopY)
		}
		prevVisibleTopY = visibleTopY
	}
}

// TestOverlayBottomScreenZeroGap checks that with no gap the overlay bottom
// lands exactly on the sprite's visible top.
func TestOverlayBottomScreenZeroGap(t *testing.T) {
	camera := NewCamera(800, 600)
	camera.SetZeroAsCenter()
	worldPos := geometry.NewVector2(1.5, 2.5)
	const visibleTop = 7.0

	bottom := overlayBottomScreen(camera, worldPos, visibleTop, 0)
	visibleTopY := camera.WorldToScreen(worldPos).Y() - visibleTop*camera.EffectiveZoom()
	if diff := bottom.Y() - visibleTopY; diff > overlayEps || diff < -overlayEps {
		t.Fatalf("bottom Y = %v, want %v", bottom.Y(), visibleTopY)
	}
}

func TestBarFillWidth(t *testing.T) {
	tests := []struct {
		name     string
		width    float64
		fraction float64
		want     float64
	}{
		{"empty", 100, 0, 0},
		{"half", 100, 0.5, 50},
		{"full", 100, 1, 100},
		{"below clamps to zero", 100, -0.3, 0},
		{"above clamps to full", 100, 1.7, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := barFillWidth(tt.width, tt.fraction); got != tt.want {
				t.Fatalf("barFillWidth(%v, %v) = %v, want %v", tt.width, tt.fraction, got, tt.want)
			}
		})
	}
}
