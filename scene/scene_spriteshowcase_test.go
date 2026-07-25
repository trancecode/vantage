package scene

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/render"
)

// showcaseTestSprite returns a sprite with one frame per requested animation,
// built from a blank image so the test needs no asset files.
func showcaseTestSprite(animations ...render.AnimationType) *render.Sprite {
	s := render.NewSprite()
	for _, a := range animations {
		s.AddImage(a, ebiten.NewImage(16, 16))
	}
	return s
}

// drawShowcase runs a full Init/Update/Draw cycle against an offscreen image.
func drawShowcase(t *testing.T, s *SpriteShowcaseScene) {
	t.Helper()
	s.Init(640, 480)
	s.SetVisible(true)
	if err := s.Update(100 * time.Millisecond); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	s.Draw(ebiten.NewImage(640, 480))
}

func TestSpriteShowcaseSceneName(t *testing.T) {
	s := NewSpriteShowcaseScene()
	if s.SceneName() != SpriteShowcaseSceneName {
		t.Fatalf("SceneName = %q, want %q", s.SceneName(), SpriteShowcaseSceneName)
	}
	if SpriteShowcaseSceneName != "sprite_showcase" {
		t.Fatalf("SpriteShowcaseSceneName = %q, want %q", SpriteShowcaseSceneName, "sprite_showcase")
	}
}

func TestSpriteShowcaseSceneLayerIndexIsBottom(t *testing.T) {
	if got := NewSpriteShowcaseScene().LayerIndex(); got != 0 {
		t.Fatalf("LayerIndex = %d, want 0", got)
	}
}

func TestSpriteShowcaseSceneDefaultsToPackageLibrary(t *testing.T) {
	if NewSpriteShowcaseScene().library != render.Sprites {
		t.Fatal("NewSpriteShowcaseScene did not default to render.Sprites")
	}
}

func TestSpriteShowcaseSceneDrawsEmptyLibrary(t *testing.T) {
	drawShowcase(t, NewSpriteShowcaseSceneFor(render.NewSpriteLibrary()))
}

func TestSpriteShowcaseSceneDrawsSingleAnimationSprites(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	l.Add("Dirt", showcaseTestSprite(render.AnimationDefault))
	drawShowcase(t, NewSpriteShowcaseSceneFor(l))
}

func TestSpriteShowcaseSceneDrawsMultiAnimationSprites(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))
	drawShowcase(t, NewSpriteShowcaseSceneFor(l))
}

func TestSpriteShowcaseSceneDrawsMixedLibrary(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	drawShowcase(t, NewSpriteShowcaseSceneFor(l))
}

func TestSpriteShowcaseSceneWrapsSingleAnimationRows(t *testing.T) {
	l := render.NewSpriteLibrary()
	// More than showcaseColumnsPerRow entries, to exercise the row wrap.
	for _, name := range []string{
		"T01", "T02", "T03", "T04", "T05", "T06",
		"T07", "T08", "T09", "T10", "T11", "T12",
	} {
		l.Add(name, showcaseTestSprite(render.AnimationDefault))
	}
	drawShowcase(t, NewSpriteShowcaseSceneFor(l))
}

func TestSpriteShowcaseSceneDrawIsNoOpWhenHidden(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	s := NewSpriteShowcaseSceneFor(l)
	s.Init(640, 480)
	s.SetVisible(false)
	// Must not panic, and must not require a camera transform to have run.
	s.Draw(ebiten.NewImage(640, 480))
}
