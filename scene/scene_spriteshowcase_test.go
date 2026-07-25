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

// showcaseTestSpriteSized returns a sprite with one frame of the given pixel
// size per requested animation, drawn at the given scale.
func showcaseTestSpriteSized(size int, scale float64, animations ...render.AnimationType) *render.Sprite {
	s := render.NewSprite().SetScale(scale)
	for _, a := range animations {
		s.AddImage(a, ebiten.NewImage(size, size))
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
	if eleventh.Y != tenth.Y+showcaseMinRowPitch {
		t.Errorf("11th cell: Y = %v, want %v (10th Y + row pitch)", eleventh.Y, tenth.Y+showcaseMinRowPitch)
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
	if cells[1].X != showcaseOriginX+showcaseMinColumnPitch {
		t.Errorf("cells[1].X = %v, want %v", cells[1].X, showcaseOriginX+showcaseMinColumnPitch)
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
	if cells[2].Y != cells[0].Y+showcaseMinRowPitch {
		t.Errorf("cells[2].Y = %v, want %v (multi-animation row Y + row pitch)", cells[2].Y, cells[0].Y+showcaseMinRowPitch)
	}
}

// TestShowcaseMetricsForTileSizedArtMatchesTheMinimums pins the layout for art
// that fits in one tile, which is what the showcaseMin constants describe: it
// must be unchanged by the measuring pass, so existing art keeps its layout.
func TestShowcaseMetricsForTileSizedArtMatchesTheMinimums(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))

	for _, tc := range []struct {
		name    string
		library *render.SpriteLibrary
	}{
		{"16x16 sprites", l},
		{"empty library", render.NewSpriteLibrary()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := showcaseMetricsFor(tc.library)
			want := showcaseMetrics{
				ColumnPitch:     2.0,
				RowPitch:        4.0,
				LabelCenterX:    0.5,
				SpriteLabelY:    1.2,
				AnimationLabelY: 1.8,
			}
			if got != want {
				t.Fatalf("showcaseMetricsFor = %+v, want %+v", got, want)
			}
		})
	}
}

// TestShowcaseMetricsForScalesToTheLargestFrame checks that art larger than a
// tile widens the grid proportionally, whether it is large in pixels or drawn
// at a scale that makes it large, and that both labels still land below the
// sprite rather than on top of it.
func TestShowcaseMetricsForScalesToTheLargestFrame(t *testing.T) {
	for _, tc := range []struct {
		name  string
		large *render.Sprite
	}{
		{"64x64 frames", showcaseTestSpriteSized(64, 1.0, render.AnimationDefault)},
		{"16x16 frames drawn at scale 4", showcaseTestSpriteSized(16, 4.0, render.AnimationDefault)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := render.NewSpriteLibrary()
			l.Add("Grass", showcaseTestSprite(render.AnimationDefault)) // small art must not win
			l.Add("Boss", tc.large)

			// The largest frame covers four tiles per side, so every metric is
			// four times its one-tile value.
			got := showcaseMetricsFor(l)
			want := showcaseMetrics{
				ColumnPitch:     4 * showcaseMinColumnPitch,
				RowPitch:        4 * showcaseMinRowPitch,
				LabelCenterX:    4 * showcaseMinLabelCenterX,
				SpriteLabelY:    4 * showcaseMinSpriteLabelY,
				AnimationLabelY: 4 * showcaseMinAnimationLabelY,
			}
			if got != want {
				t.Fatalf("showcaseMetricsFor = %+v, want %+v", got, want)
			}

			// The sprite occupies four tiles below its cell position, so both
			// labels must sit lower than that, and inside the row.
			const spriteHeightTiles = 4.0
			if got.SpriteLabelY <= spriteHeightTiles || got.AnimationLabelY <= got.SpriteLabelY {
				t.Fatalf("labels are not stacked below the sprite: %+v", got)
			}
			if got.AnimationLabelY >= got.RowPitch {
				t.Fatalf("animation label at %v spills into the next row (pitch %v)", got.AnimationLabelY, got.RowPitch)
			}
		})
	}
}

// TestShowcaseLayoutSpacesLargeArtProportionally checks that the measured
// metrics reach the cell positions, not just the metrics function.
func TestShowcaseLayoutSpacesLargeArtProportionally(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Boss", showcaseTestSpriteSized(64, 1.0, render.AnimationIdleDown, render.AnimationMoveDown))
	l.Add("Minion", showcaseTestSpriteSized(64, 1.0, render.AnimationDefault))

	cells := showcaseLayout(l)
	if len(cells) != 3 {
		t.Fatalf("len(cells) = %d, want 3", len(cells))
	}
	if want := showcaseOriginX + 4*showcaseMinColumnPitch; cells[1].X != want {
		t.Errorf("cells[1].X = %v, want %v", cells[1].X, want)
	}
	if want := cells[0].Y + 4*showcaseMinRowPitch; cells[2].Y != want {
		t.Errorf("cells[2].Y = %v, want %v", cells[2].Y, want)
	}
}

// TestShowcaseLayoutWrapsAtTheSameColumnCountForLargeArt checks that sprite
// size changes the spacing but not the shape of the grid: the wrap still
// happens after showcaseColumnsPerRow cells.
func TestShowcaseLayoutWrapsAtTheSameColumnCountForLargeArt(t *testing.T) {
	l := render.NewSpriteLibrary()
	names := []string{
		"T01", "T02", "T03", "T04", "T05", "T06",
		"T07", "T08", "T09", "T10", "T11", "T12",
	}
	for _, name := range names {
		l.Add(name, showcaseTestSpriteSized(64, 1.0, render.AnimationDefault))
	}

	cells := showcaseLayout(l)
	if len(cells) != len(names) {
		t.Fatalf("len(cells) = %d, want %d", len(cells), len(names))
	}
	for i, cell := range cells[:showcaseColumnsPerRow] {
		if cell.Y != showcaseOriginY {
			t.Errorf("cell %d: Y = %v, want %v (still on the first row)", i, cell.Y, showcaseOriginY)
		}
	}
	eleventh := cells[showcaseColumnsPerRow]
	if eleventh.X != showcaseOriginX {
		t.Errorf("11th cell: X = %v, want %v (origin)", eleventh.X, showcaseOriginX)
	}
	if want := showcaseOriginY + 4*showcaseMinRowPitch; eleventh.Y != want {
		t.Errorf("11th cell: Y = %v, want %v", eleventh.Y, want)
	}
}

// TestSpriteShowcaseRegisteredSpriteReachesTheDrawnCells covers the whole path
// a game's art takes: a sprite registered in a library, a showcase built for
// that library, and a drawn frame whose cells name that sprite.
func TestSpriteShowcaseRegisteredSpriteReachesTheDrawnCells(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))

	s := NewSpriteShowcaseSceneFor(l)
	drawShowcase(t, s)

	drawn := map[string]bool{}
	for _, cell := range s.cellsToDraw() {
		drawn[cell.Name] = true
	}
	for _, name := range l.Names() {
		if !drawn[name] {
			t.Errorf("sprite %q registered in the library is missing from the drawn cells", name)
		}
	}
}

// TestSpriteShowcaseSceneUpdateBeforeInitIsClockOnly checks the guard on the
// camera controller, which Init is what builds: an update arriving first must
// still advance the animation clock rather than panic.
func TestSpriteShowcaseSceneUpdateBeforeInitIsClockOnly(t *testing.T) {
	s := NewSpriteShowcaseSceneFor(render.NewSpriteLibrary())
	s.SetFocus(true)
	if err := s.Update(100 * time.Millisecond); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if s.durationSinceInit != 100*time.Millisecond {
		t.Fatalf("durationSinceInit = %v, want 100ms", s.durationSinceInit)
	}
}

func TestSpriteShowcaseSceneDrawIsNoOpWhenHidden(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))
	s := NewSpriteShowcaseSceneFor(l)
	s.Init(640, 480)

	s.SetVisible(false)
	s.Draw(ebiten.NewImage(640, 480)) // keeps the draw path smoke-covered
	if got := s.cellsToDraw(); len(got) != 0 {
		t.Fatalf("cellsToDraw() = %d cells while hidden, want 0", len(got))
	}

	s.SetVisible(true)
	s.Draw(ebiten.NewImage(640, 480))
	if got := s.cellsToDraw(); len(got) == 0 {
		t.Fatal("cellsToDraw() = 0 cells while visible, want > 0")
	}
}
