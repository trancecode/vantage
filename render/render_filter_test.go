package render

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
)

func TestParseFilterAcceptsBothNames(t *testing.T) {
	for name, want := range map[string]ebiten.Filter{
		FilterNameNearest: ebiten.FilterNearest,
		FilterNameLinear:  ebiten.FilterLinear,
	} {
		got, err := ParseFilter(name)
		if err != nil {
			t.Fatalf("ParseFilter(%q) returned error: %v", name, err)
		}
		if got != want {
			t.Fatalf("ParseFilter(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseFilterRejectsUnknownNameAndListsTheValidOnes(t *testing.T) {
	_, err := ParseFilter("bilinear")
	if err == nil {
		t.Fatal("ParseFilter accepted an unknown filter name")
	}
	for _, want := range []string{"bilinear", FilterNameNearest, FilterNameLinear} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestFilterNameRoundTrips(t *testing.T) {
	for _, name := range []string{FilterNameNearest, FilterNameLinear} {
		filter, err := ParseFilter(name)
		if err != nil {
			t.Fatalf("ParseFilter(%q) returned error: %v", name, err)
		}
		if got := FilterName(filter); got != name {
			t.Fatalf("FilterName round trip = %q, want %q", got, name)
		}
	}
}

func TestSpriteFilterDefaultsToNearest(t *testing.T) {
	// Pixel art is the engine's default assumption, so scaling must not blur it
	// unless a game opts in.
	if SpriteFilter != ebiten.FilterNearest {
		t.Fatalf("SpriteFilter = %v, want FilterNearest", SpriteFilter)
	}
}

func TestBuildDrawOpAppliesSpriteFilter(t *testing.T) {
	original := SpriteFilter
	t.Cleanup(func() { SpriteFilter = original })

	s := NewSprite()
	camera := NewCamera(640, 480)

	SpriteFilter = ebiten.FilterLinear
	if op := s.buildDrawOp(geometry.Zero2D(), false, camera, 1.0); op.Filter != ebiten.FilterLinear {
		t.Fatalf("buildDrawOp Filter = %v, want FilterLinear", op.Filter)
	}

	SpriteFilter = ebiten.FilterNearest
	if op := s.buildDrawOp(geometry.Zero2D(), false, camera, 1.0); op.Filter != ebiten.FilterNearest {
		t.Fatalf("buildDrawOp Filter = %v, want FilterNearest", op.Filter)
	}
}

func TestCameraDrawImageOptionsAppliesSpriteFilter(t *testing.T) {
	original := SpriteFilter
	t.Cleanup(func() { SpriteFilter = original })

	SpriteFilter = ebiten.FilterLinear
	camera := NewCamera(640, 480)
	if op := camera.DrawImageOptions(geometry.Zero2D()); op.Filter != ebiten.FilterLinear {
		t.Fatalf("DrawImageOptions Filter = %v, want FilterLinear", op.Filter)
	}
}
