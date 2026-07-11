package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestLoadSpriteEmptyIndexesDoesNotPanic guards against a regression where an
// animation type mapped to an empty index slice (e.g. a not-yet-populated,
// data-driven frame list) left no entry in sprite.Animations, causing
// LoadSprite to dereference a nil *Animation when setting the duration.
func TestLoadSpriteEmptyIndexesDoesNotPanic(t *testing.T) {
	img := ebiten.NewImage(4, 4)
	indexes := map[AnimationType][]int{
		AnimationIdleUp: {},
	}

	sprite, err := LoadSprite(img, 2, 2, indexes, nil)
	if err != nil {
		t.Fatalf("LoadSprite returned error: %v", err)
	}
	if sprite.HasAnimation(AnimationIdleUp) {
		t.Fatalf("expected no animation entry for an empty index list")
	}
}
