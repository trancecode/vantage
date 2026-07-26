package scene

import (
	"math"
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

// TestShowcaseGridGeometryIsFixed pins the grid constants as literals. They are
// deliberately independent of any library's art, so a game whose sprites are
// tile sized keeps exactly the layout it has always had.
func TestShowcaseGridGeometryIsFixed(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"column pitch", showcaseColumnPitch, 2.0},
		{"row pitch", showcaseRowPitch, 4.0},
		{"origin X", showcaseOriginX, 1.0},
		{"origin Y", showcaseOriginY, 1.0},
		{"label center X", showcaseLabelCenterX, 0.5},
		{"sprite label Y", showcaseSpriteLabelY, 1.2},
		{"animation label Y", showcaseAnimationLabelY, 1.8},
		{"slot pixels", render.TileSize, 16.0},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	if showcaseColumnsPerRow != 10 {
		t.Errorf("columns per row = %d, want 10", showcaseColumnsPerRow)
	}
}

// TestShowcaseLayoutTileSizedArtUsesTheFixedPitches pins the cell positions and
// fit scales for a library of 16x16 art: exactly the fixed pitches, and no
// scaling at all, since one tile of art is exactly one slot.
func TestShowcaseLayoutTileSizedArtUsesTheFixedPitches(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Character", showcaseTestSprite(render.AnimationIdleDown, render.AnimationMoveDown))
	l.Add("Grass", showcaseTestSprite(render.AnimationDefault))

	want := []showcaseCell{
		{Name: "Character", Animation: render.AnimationIdleDown, X: 1.0, Y: 1.0, FitScale: 1.0},
		{Name: "Character", Animation: render.AnimationMoveDown, X: 3.0, Y: 1.0, FitScale: 1.0},
		{Name: "Grass", Animation: render.AnimationDefault, X: 1.0, Y: 5.0, FitScale: 1.0},
	}
	cells := showcaseLayout(l)
	if len(cells) != len(want) {
		t.Fatalf("len(cells) = %d, want %d", len(cells), len(want))
	}
	for i, w := range want {
		got := cells[i]
		if got.Name != w.Name || got.Animation != w.Animation ||
			got.X != w.X || got.Y != w.Y || got.FitScale != w.FitScale {
			t.Errorf("cell %d = {%q %v X:%v Y:%v FitScale:%v}, want {%q %v X:%v Y:%v FitScale:%v}",
				i, got.Name, got.Animation, got.X, got.Y, got.FitScale,
				w.Name, w.Animation, w.X, w.Y, w.FitScale)
		}
	}
}

// TestShowcaseFitScaleShrinksOversizedArtOnly is the core of the contact-sheet
// layout: art bigger than a slot is scaled down into it, and art at or below a
// slot is left at its natural size rather than magnified. A sprite's own Scale
// counts towards its size, since that is the size the game draws it at.
func TestShowcaseFitScaleShrinksOversizedArtOnly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sprite *render.Sprite
		want   float64
	}{
		{"16x16 art fills a slot exactly", showcaseTestSpriteSized(16, 1.0, render.AnimationDefault), 1.0},
		{"64x64 art shrinks to a quarter", showcaseTestSpriteSized(64, 1.0, render.AnimationDefault), 0.25},
		{"8x8 art is not blown up", showcaseTestSpriteSized(8, 1.0, render.AnimationDefault), 1.0},
		{"16x16 art at scale 4 fits like 64x64", showcaseTestSpriteSized(16, 4.0, render.AnimationDefault), 0.25},
		{"16x16 art at scale 0.5 is not blown up", showcaseTestSpriteSized(16, 0.5, render.AnimationDefault), 1.0},
		{"a sprite with no animations", render.NewSprite(), 1.0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := showcaseFitScale(tc.sprite); got != tc.want {
				t.Fatalf("showcaseFitScale = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestShowcaseFitScaleMeasuresTheLargestFrameOfTheSprite checks that the
// measurement covers every frame of every animation rather than the first one
// it finds, so a sprite whose frames differ in size still fits its slot.
func TestShowcaseFitScaleMeasuresTheLargestFrameOfTheSprite(t *testing.T) {
	s := render.NewSprite()
	s.AddImage(render.AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(render.AnimationIdleDown, ebiten.NewImage(64, 64))
	s.AddImage(render.AnimationMoveDown, ebiten.NewImage(16, 16))

	if got := showcaseFitScale(s); got != 0.25 {
		t.Fatalf("showcaseFitScale = %v, want 0.25 (the 64x64 frame must win)", got)
	}
}

// TestShowcaseLayoutLargeArtKeepsTheSameCellPositions is the point of the fixed
// slots: a 64x64 sprite occupies the cell a 16x16 sprite would, shrunk to fit
// it, instead of pushing the grid apart.
func TestShowcaseLayoutLargeArtKeepsTheSameCellPositions(t *testing.T) {
	small := render.NewSpriteLibrary()
	small.Add("Boss", showcaseTestSpriteSized(16, 1.0, render.AnimationIdleDown, render.AnimationMoveDown))
	small.Add("Minion", showcaseTestSpriteSized(16, 1.0, render.AnimationDefault))

	large := render.NewSpriteLibrary()
	large.Add("Boss", showcaseTestSpriteSized(64, 1.0, render.AnimationIdleDown, render.AnimationMoveDown))
	large.Add("Minion", showcaseTestSpriteSized(64, 1.0, render.AnimationDefault))

	smallCells, largeCells := showcaseLayout(small), showcaseLayout(large)
	if len(smallCells) != 3 || len(largeCells) != 3 {
		t.Fatalf("len(cells) = %d and %d, want 3 each", len(smallCells), len(largeCells))
	}
	for i := range smallCells {
		if largeCells[i].X != smallCells[i].X || largeCells[i].Y != smallCells[i].Y {
			t.Errorf("cell %d moved for large art: (%v, %v), want (%v, %v)",
				i, largeCells[i].X, largeCells[i].Y, smallCells[i].X, smallCells[i].Y)
		}
		if smallCells[i].FitScale != 1.0 {
			t.Errorf("cell %d: 16x16 fit scale = %v, want 1", i, smallCells[i].FitScale)
		}
		if largeCells[i].FitScale != 0.25 {
			t.Errorf("cell %d: 64x64 fit scale = %v, want 0.25", i, largeCells[i].FitScale)
		}
	}
}

// TestShowcaseLayoutFitScaleIsPerCell checks that each cell's fit scale comes
// from its own sprite: in a mixed library one huge sprite shrinks only itself
// and moves nothing, which is what the derived spacing it replaced got wrong.
func TestShowcaseLayoutFitScaleIsPerCell(t *testing.T) {
	l := render.NewSpriteLibrary()
	l.Add("Grass", showcaseTestSpriteSized(16, 1.0, render.AnimationDefault))
	l.Add("Huge", showcaseTestSpriteSized(512, 1.0, render.AnimationDefault))
	l.Add("Pebble", showcaseTestSpriteSized(8, 1.0, render.AnimationDefault))

	cells := showcaseLayout(l)
	if len(cells) != 3 {
		t.Fatalf("len(cells) = %d, want 3", len(cells))
	}

	wantScales := map[string]float64{"Grass": 1.0, "Huge": 16.0 / 512.0, "Pebble": 1.0}
	for i, cell := range cells {
		if got, want := cell.FitScale, wantScales[cell.Name]; got != want {
			t.Errorf("cell %q: FitScale = %v, want %v", cell.Name, got, want)
		}
		// All three share one row at the fixed column pitch, unmoved by the
		// 512-pixel sprite among them.
		wantX := showcaseOriginX + float64(i)*showcaseColumnPitch
		if cell.X != wantX || cell.Y != showcaseOriginY {
			t.Errorf("cell %q: position = (%v, %v), want (%v, %v)",
				cell.Name, cell.X, cell.Y, wantX, showcaseOriginY)
		}
	}
}

// TestShowcaseLayoutWrapsAtTheSameColumnCountForLargeArt checks that sprite
// size changes neither the spacing nor the shape of the grid: the wrap still
// happens after showcaseColumnsPerRow cells, at the fixed row pitch.
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
	if want := showcaseOriginY + showcaseRowPitch; eleventh.Y != want {
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

// TestShowcaseLabelsAreNeverHiddenByZoom guards the reason the labels set an
// infinite MaxZoom: TextWriter hides fixed-size text above
// render.DefaultMaxZoomForText, and zooming in to inspect detailed art is
// exactly when these labels are wanted.
func TestShowcaseLabelsAreNeverHiddenByZoom(t *testing.T) {
	w := showcaseLabelWriter()
	if w.Scaling {
		t.Fatal("labels should keep a fixed pixel size, not scale with zoom")
	}
	if !math.IsInf(w.MaxZoom, 1) {
		t.Fatalf("label MaxZoom = %v, want +Inf so the zoom threshold never hides them", w.MaxZoom)
	}
	if w.MaxZoom <= render.DefaultMaxZoomForText {
		t.Fatalf("label MaxZoom = %v, want above the default %v", w.MaxZoom, render.DefaultMaxZoomForText)
	}
}
