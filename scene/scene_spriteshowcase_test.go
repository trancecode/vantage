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

func TestShowcaseLayoutEmptyLibraryProducesNoCells(t *testing.T) {
	cells := showcaseLayout(render.NewSpriteLibrary())
	if len(cells) != 0 {
		t.Fatalf("len(cells) = %d, want 0", len(cells))
	}
}

func TestShowcaseLayoutWrapsSingleAnimationRows(t *testing.T) {
	l := render.NewSpriteLibrary()
	// More than showcaseColumnsPerRow entries, to exercise the row wrap.
	names := []string{
		"T01", "T02", "T03", "T04", "T05", "T06",
		"T07", "T08", "T09", "T10", "T11", "T12",
	}
	for _, name := range names {
		l.Add(name, showcaseTestSprite(render.AnimationDefault))
	}

	cells := showcaseLayout(l)
	if len(cells) != len(names) {
		t.Fatalf("len(cells) = %d, want %d", len(cells), len(names))
	}

	// The first showcaseColumnsPerRow cells share one row: one Y, strictly
	// increasing X.
	firstRow := cells[:showcaseColumnsPerRow]
	for i, cell := range firstRow {
		if cell.Y != showcaseOriginY {
			t.Errorf("cell %d: Y = %v, want %v", i, cell.Y, showcaseOriginY)
		}
		if i > 0 && cell.X <= firstRow[i-1].X {
			t.Errorf("cell %d: X = %v, want > previous X %v", i, cell.X, firstRow[i-1].X)
		}
	}

	// The 11th cell (index 10) starts a new row: X returns to the origin, Y
	// drops one row pitch below the 10th cell's (index 9) Y.
	tenth := cells[showcaseColumnsPerRow-1]
	eleventh := cells[showcaseColumnsPerRow]
	if eleventh.X != showcaseOriginX {
		t.Errorf("11th cell: X = %v, want %v (origin)", eleventh.X, showcaseOriginX)
	}
	if eleventh.Y != tenth.Y+showcaseRowPitch {
		t.Errorf("11th cell: Y = %v, want %v (10th Y + row pitch)", eleventh.Y, tenth.Y+showcaseRowPitch)
	}
}

func TestShowcaseLayoutMultiAnimationSpriteSharesOneRow(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))

	cells := showcaseLayout(l)
	if len(cells) != 2 {
		t.Fatalf("len(cells) = %d, want 2", len(cells))
	}

	// AnimationIdleDown.String() < AnimationMoveDown.String(), so
	// AnimationIdleDown comes first.
	if cells[0].Animation != render.AnimationIdleDown {
		t.Errorf("cells[0].Animation = %v, want %v", cells[0].Animation, render.AnimationIdleDown)
	}
	if cells[1].Animation != render.AnimationMoveDown {
		t.Errorf("cells[1].Animation = %v, want %v", cells[1].Animation, render.AnimationMoveDown)
	}

	if cells[0].Y != showcaseOriginY || cells[1].Y != showcaseOriginY {
		t.Errorf("cells Y = %v, %v, want both %v", cells[0].Y, cells[1].Y, showcaseOriginY)
	}
	if cells[0].X != showcaseOriginX {
		t.Errorf("cells[0].X = %v, want %v", cells[0].X, showcaseOriginX)
	}
	if cells[1].X != showcaseOriginX+showcaseColumnPitch {
		t.Errorf("cells[1].X = %v, want %v", cells[1].X, showcaseOriginX+showcaseColumnPitch)
	}
}

func TestShowcaseLayoutMultiAnimationSpritesComeBeforeSingle(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))

	cells := showcaseLayout(l)
	if len(cells) != 3 {
		t.Fatalf("len(cells) = %d, want 3", len(cells))
	}

	if cells[0].Name != "Character" || cells[1].Name != "Character" {
		t.Errorf("cells[0], cells[1] = %q, %q, want both %q", cells[0].Name, cells[1].Name, "Character")
	}
	if cells[2].Name != "Grass" {
		t.Errorf("cells[2] = %q, want %q", cells[2].Name, "Grass")
	}
	// The single-animation sprite's row starts below the multi-animation row.
	if cells[2].Y != cells[0].Y+showcaseRowPitch {
		t.Errorf("cells[2].Y = %v, want %v (multi-animation row Y + row pitch)", cells[2].Y, cells[0].Y+showcaseRowPitch)
	}
}

func TestSpriteShowcaseSceneDrawIsNoOpWhenHidden(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	s := NewSpriteShowcaseSceneFor(l)
	s.Init(640, 480)

	s.SetVisible(false)
	s.Draw(ebiten.NewImage(640, 480))
	if s.cellsDrawn != 0 {
		t.Fatalf("cellsDrawn = %d after a hidden Draw, want 0", s.cellsDrawn)
	}

	s.SetVisible(true)
	s.Draw(ebiten.NewImage(640, 480))
	if s.cellsDrawn == 0 {
		t.Fatal("cellsDrawn = 0 after a visible Draw, want > 0")
	}
}
