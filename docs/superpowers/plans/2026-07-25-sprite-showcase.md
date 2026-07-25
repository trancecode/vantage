# Sprite showcase implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move sprite naming and lookup into the engine as `render.SpriteLibrary`, add a `scene.SpriteShowcaseScene` that draws every registered sprite with its animations, and add a general `--scene` flag that forces which scenes are shown.

**Architecture:** Three additions in three existing packages. `render` gains a name-to-sprite library plus the package-level default `render.Sprites`, which games register into at `init()` time. `scene` gains a read-only showcase scene that walks a library and draws each sprite's animations in a labelled grid, ported from `nrg/rts/rts_showcase.go`. `app` gains a repeatable `--scene` flag driving `Manager.ShowOnly` and `Manager.SetExclusiveFocus`, and registers the showcase on demand when it is the scene requested.

**Tech Stack:** Go, Ebitengine (`github.com/hajimehoshi/ebiten/v2`), `github.com/spf13/pflag` for flags, `go-task` for the build.

**Design spec:** `docs/superpowers/specs/2026-07-25-sprite-showcase-design.md`

## Global constraints

* Set `export GOMODCACHE=/tmp/go-mod-cache` before any Go command. The default module cache is not writable.
* Run tests with `task test:headless`, never bare `go test`. The `render`, `ui`, `scene` and `app` packages initialize Ebiten/GLFW and need the virtual display that target provides.
* Before any commit: `task lint`, `task test:headless` and `go vet ./...` must all pass. If they cannot be run, do not commit; report the exact error.
* Commit author is `Claude Code <herve.quiroz+claude@gmail.com>`. Do not add a `Co-Authored-By:` line.
* Work directly on `main`. This is an interactive session; do not create branches.
* Follow `docs/styleguide.md`: Java-style camel case for acronyms (`HTTPPort` is pre-existing, but new code uses e.g. `FpsCounter` style), grouped and alphabetized imports (standard library, third party, then `github.com/trancecode/...`), `panic()` for unrecoverable errors.
* File naming follows the package prefix convention: files in `render` are `render_*.go`, in `scene` are `scene_*.go`.
* Every exported type, function, constant and struct field gets a doc comment.
* No em dashes in documentation prose.

---

### Task 1: `render.SpriteLibrary`

The name-to-sprite collection and the package-level default. No dependency on any other task.

**Files:**
- Create: `render/render_spritelibrary.go`
- Test: `render/render_spritelibrary_test.go`

**Interfaces:**
- Consumes: `render.Sprite` and `render.NewSprite` from `render/render_sprite.go`.
- Produces:
  - `type SpriteLibrary struct` with unexported field `sprites map[string]*Sprite`
  - `func NewSpriteLibrary() *SpriteLibrary`
  - `func (l *SpriteLibrary) Add(name string, s *Sprite) *Sprite`
  - `func (l *SpriteLibrary) Get(name string) (*Sprite, bool)`
  - `func (l *SpriteLibrary) Names() []string`
  - `func (l *SpriteLibrary) Len() int`
  - `var Sprites = NewSpriteLibrary()`

- [ ] **Step 1: Write the failing tests**

Create `render/render_spritelibrary_test.go`:

```go
package render

import (
	"testing"
)

func TestSpriteLibraryAddAndGet(t *testing.T) {
	l := NewSpriteLibrary()
	s := NewSprite()
	l.Add("Character", s)

	got, ok := l.Get("Character")
	if !ok {
		t.Fatal("Get did not find the added sprite")
	}
	if got != s {
		t.Fatal("Get returned a different sprite than the one added")
	}
}

func TestSpriteLibraryGetUnknownName(t *testing.T) {
	l := NewSpriteLibrary()
	if _, ok := l.Get("Missing"); ok {
		t.Fatal("Get reported an unknown name as present")
	}
}

func TestSpriteLibraryAddReturnsSpriteForChaining(t *testing.T) {
	l := NewSpriteLibrary()
	s := NewSprite()
	if got := l.Add("Character", s); got != s {
		t.Fatal("Add did not return the sprite it was given")
	}
}

func TestSpriteLibraryNamesAreSorted(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("Zebra", NewSprite())
	l.Add("Apple", NewSprite())
	l.Add("Mango", NewSprite())

	names := l.Names()
	want := []string{"Apple", "Mango", "Zebra"}
	if len(names) != len(want) {
		t.Fatalf("Names returned %d entries, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("Names()[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestSpriteLibraryLen(t *testing.T) {
	l := NewSpriteLibrary()
	if l.Len() != 0 {
		t.Fatalf("new library Len = %d, want 0", l.Len())
	}
	l.Add("A", NewSprite())
	l.Add("B", NewSprite())
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}
}

func TestSpriteLibraryAddEmptyNamePanics(t *testing.T) {
	l := NewSpriteLibrary()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty sprite name")
		}
	}()
	l.Add("", NewSprite())
}

func TestSpriteLibraryAddNilSpritePanics(t *testing.T) {
	l := NewSpriteLibrary()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil sprite")
		}
	}()
	l.Add("Character", nil)
}

func TestSpriteLibraryAddDuplicateNamePanics(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("Character", NewSprite())
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate sprite name")
		}
	}()
	l.Add("Character", NewSprite())
}

func TestNewSpriteLibraryIsIndependentOfDefault(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("OnlyInLocalLibrary", NewSprite())
	if _, ok := Sprites.Get("OnlyInLocalLibrary"); ok {
		t.Fatal("adding to a new library leaked into the default Sprites library")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: FAIL to build the `render` package, with errors like `undefined: NewSpriteLibrary` and `undefined: Sprites`.

- [ ] **Step 3: Write the implementation**

Create `render/render_spritelibrary.go`:

```go
package render

import (
	"fmt"
	"sort"
)

// SpriteLibrary maps display names to sprites. Games register their sprites
// into the package-level Sprites library at init time, and engine tooling such
// as the sprite showcase scene reads them back.
//
// A SpriteLibrary is not safe for concurrent use: registration happens at init
// time and reads happen while drawing, both on the same goroutine.
type SpriteLibrary struct {
	sprites map[string]*Sprite
}

// NewSpriteLibrary returns an empty SpriteLibrary. Most games register into the
// package-level Sprites library instead; this is for tests and for tooling that
// needs a library of its own.
func NewSpriteLibrary() *SpriteLibrary {
	return &SpriteLibrary{sprites: map[string]*Sprite{}}
}

// Add registers a sprite under a display name and returns it, so that
// registration composes with the chainable Sprite setters:
//
//	Sprites.Add("Character", MustLoadSprite(img, 6, 10, indexes, nil)).
//		SetType(SpriteTypeActor)
//
// It panics on an empty name, a nil sprite, or a name that is already
// registered. All three are load-time programming errors, and registration
// runs from init, so failing loudly beats yielding a silently wrong catalog.
func (l *SpriteLibrary) Add(name string, s *Sprite) *Sprite {
	if name == "" {
		panic("sprite name must not be empty")
	}
	if s == nil {
		panic(fmt.Sprintf("sprite %q must not be nil", name))
	}
	if _, ok := l.sprites[name]; ok {
		panic(fmt.Sprintf("duplicate sprite name: %s", name))
	}
	l.sprites[name] = s
	return s
}

// Get returns the sprite registered under a name. Unlike Add, a miss is a
// plausible runtime condition rather than a programming error, so it reports
// absence instead of panicking.
func (l *SpriteLibrary) Get(name string) (*Sprite, bool) {
	s, ok := l.sprites[name]
	return s, ok
}

// Names returns every registered name, sorted lexicographically, so callers
// that iterate the library get a stable order without sorting themselves.
func (l *SpriteLibrary) Names() []string {
	names := make([]string, 0, len(l.sprites))
	for name := range l.sprites {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Len returns the number of registered sprites.
func (l *SpriteLibrary) Len() int {
	return len(l.sprites)
}

// Sprites is the default sprite library. Games register their sprites here, and
// engine tooling reads them back.
var Sprites = NewSpriteLibrary()
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, all `TestSpriteLibrary*` and `TestNewSpriteLibraryIsIndependentOfDefault` tests included.

- [ ] **Step 5: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

Expected: both succeed with no warnings.

- [ ] **Step 6: Commit**

```bash
git add render/render_spritelibrary.go render/render_spritelibrary_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add an engine-level sprite library

render.SpriteLibrary maps display names to sprites, with a package-level
Sprites default that games register into at init time. Add returns the sprite
so registration composes with the chainable setters, and panics on an empty
name, a nil sprite or a duplicate, all of which are load-time errors."
```

---

### Task 2: `scene.SpriteShowcaseScene`

The scene that draws a library. Depends on Task 1.

**Files:**
- Create: `scene/scene_spriteshowcase.go`
- Test: `scene/scene_spriteshowcase_test.go`
- Modify: `scene/scene.go:11-14` (the `SceneName` doc comment)

**Interfaces:**
- Consumes: `render.SpriteLibrary`, `render.Sprites`, `render.NewSpriteLibrary` (Task 1); `render.Camera`, `render.NewCamera`, `render.NewCameraController`, `render.TextDefault`, `render.AlignCenter`, `render.Sprite.AllAnimations`, `render.Sprite.DrawAnimation`, `render.AnimationType.String`; `scene.BaseScene` and its `Camera *render.Camera` field.
- Produces:
  - `const SpriteShowcaseSceneName SceneName = "sprite_showcase"`
  - `type SpriteShowcaseScene struct`
  - `func NewSpriteShowcaseScene() *SpriteShowcaseScene`
  - `func NewSpriteShowcaseSceneFor(library *render.SpriteLibrary) *SpriteShowcaseScene`

- [ ] **Step 1: Write the failing tests**

Create `scene/scene_spriteshowcase_test.go`. The helper builds sprites from a
blank in-memory image, which needs no asset files. `AnimationDefault`,
`AnimationIdleDown` and `AnimationMoveDown` are real values from
`render/render_animation.go`.

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: FAIL to build the `scene` package, with `undefined: NewSpriteShowcaseScene`, `undefined: SpriteShowcaseSceneName` and `undefined: showcaseColumnsPerRow`.

- [ ] **Step 3: Write the implementation**

Create `scene/scene_spriteshowcase.go`:

```go
package scene

import (
	"image/color"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/util"
)

// SpriteShowcaseSceneName is the SceneName used by the engine's
// SpriteShowcaseScene.
const SpriteShowcaseSceneName SceneName = "sprite_showcase"

// Grid geometry for the showcase, in tiles.
const (
	// showcaseColumnPitch is the horizontal distance between two cells.
	showcaseColumnPitch = 2.0
	// showcaseRowPitch is the vertical distance between two rows, leaving room
	// for the sprite and both labels.
	showcaseRowPitch = 4.0
	// showcaseOriginX is the left edge of the grid.
	showcaseOriginX = 1.0
	// showcaseOriginY is the top edge of the grid.
	showcaseOriginY = 1.0
	// showcaseLabelCenterX offsets a label to the horizontal center of its cell.
	showcaseLabelCenterX = 0.5
	// showcaseSpriteLabelY offsets the sprite name below the sprite.
	showcaseSpriteLabelY = 1.2
	// showcaseAnimationLabelY offsets the animation name below the sprite name.
	showcaseAnimationLabelY = 1.8
	// showcaseColumnsPerRow caps how many single-animation sprites share a row,
	// so their labels do not overlap.
	showcaseColumnsPerRow = 10
)

// showcaseLabelBackground is the semi-transparent backdrop that keeps labels
// readable over sprite art.
var showcaseLabelBackground = color.RGBA{R: 0, G: 0, B: 0, A: 180}

// SpriteShowcaseScene draws every sprite in a render.SpriteLibrary, animating,
// labelled with the sprite name and the name of each animation. It is a
// read-only inspection surface for checking art without building a level around
// it, reached with the --scene sprite_showcase flag.
type SpriteShowcaseScene struct {
	BaseScene

	// library is the sprite library to display.
	library *render.SpriteLibrary

	// durationSinceInit accumulates elapsed time and drives the animation phase.
	durationSinceInit time.Duration

	// cameraController maps W/A/S/D and Q/E onto the scene camera.
	cameraController *render.CameraController
}

// NewSpriteShowcaseScene returns a showcase for the package-level
// render.Sprites library, which is what a game registers its sprites into.
func NewSpriteShowcaseScene() *SpriteShowcaseScene {
	return NewSpriteShowcaseSceneFor(render.Sprites)
}

// NewSpriteShowcaseSceneFor returns a showcase for an explicit library. Tests
// use this so they can exercise the layout without mutating render.Sprites.
func NewSpriteShowcaseSceneFor(library *render.SpriteLibrary) *SpriteShowcaseScene {
	return &SpriteShowcaseScene{library: library}
}

// SceneName returns the name of the sprite showcase scene.
func (s *SpriteShowcaseScene) SceneName() SceneName {
	return SpriteShowcaseSceneName
}

// Init builds a camera anchored at the top-left corner so the grid starts there,
// and resets the animation clock.
func (s *SpriteShowcaseScene) Init(screenWidth, screenHeight int) {
	s.durationSinceInit = 0
	s.Camera = render.NewCamera(screenWidth, screenHeight)
	s.Camera.SetZeroAsTopLeft()
	s.cameraController = render.NewCameraController(s.Camera)
	util.Logger.Debug().Msg(s.Camera.CameraDebugInfo())
}

// Update advances the animation clock and, when the scene has focus, applies
// camera panning and zoom.
func (s *SpriteShowcaseScene) Update(duration time.Duration) error {
	s.durationSinceInit += duration
	if s.HasFocus() {
		s.cameraController.HandleInput()
	}
	return nil
}

// Draw renders the sprite grid.
func (s *SpriteShowcaseScene) Draw(screen *ebiten.Image) {
	if !s.IsVisible() {
		return
	}
	s.drawAllSprites(screen)
}

// LayerIndex returns the bottom layer. The showcase is a full-screen scene, not
// an overlay.
func (s *SpriteShowcaseScene) LayerIndex() int {
	return 0
}

// drawAllSprites lays out the library: sprites with several animations get a
// row each with one column per animation, then single-animation sprites pack
// into a grid wrapping at showcaseColumnsPerRow.
func (s *SpriteShowcaseScene) drawAllSprites(screen *ebiten.Image) {
	var multiAnimation, singleAnimation []string
	for _, name := range s.library.Names() {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		if len(sprite.AllAnimations()) > 1 {
			multiAnimation = append(multiAnimation, name)
		} else {
			singleAnimation = append(singleAnimation, name)
		}
	}

	y := showcaseOriginY

	for _, name := range multiAnimation {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		sort.Slice(animations, func(i, j int) bool {
			return animations[i].String() < animations[j].String()
		})

		x := showcaseOriginX
		for _, animation := range animations {
			s.drawCell(screen, sprite, name, animation, x, y)
			x += showcaseColumnPitch
		}
		y += showcaseRowPitch
	}

	x := showcaseOriginX
	column := 0
	for _, name := range singleAnimation {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		if len(animations) == 0 {
			continue
		}

		s.drawCell(screen, sprite, name, animations[0], x, y)

		x += showcaseColumnPitch
		column++
		if column >= showcaseColumnsPerRow {
			x = showcaseOriginX
			y += showcaseRowPitch
			column = 0
		}
	}
}

// drawCell draws one animation at the given tile position, with the sprite name
// and the animation name centered beneath it.
func (s *SpriteShowcaseScene) drawCell(
	screen *ebiten.Image,
	sprite *render.Sprite,
	name string,
	animation render.AnimationType,
	x, y float64,
) {
	sprite.DrawAnimation(screen, s.Camera, geometry.NewVector2(x, y), animation, s.durationSinceInit)

	labelX := x + showcaseLabelCenterX
	s.drawLabel(screen, name, labelX, y+showcaseSpriteLabelY)

	animationName, _ := strings.CutPrefix(animation.String(), "Animation")
	s.drawLabel(screen, animationName, labelX, y+showcaseAnimationLabelY)
}

// drawLabel draws centered text on a dark backdrop at a tile position.
func (s *SpriteShowcaseScene) drawLabel(screen *ebiten.Image, msg string, x, y float64) {
	render.TextDefault.
		WithAlignment(render.AlignCenter).
		WithBackground(showcaseLabelBackground).
		Text(msg).
		Draw(screen, s.Camera, geometry.NewVector2(x, y))
}
```

- [ ] **Step 4: Update the `SceneName` doc comment**

The engine now reserves two names. In `scene/scene.go`, replace:

```go
// SceneName identifies a scene within a Manager. Each game defines its own
// SceneName constants; the engine reserves only DialogSceneName.
type SceneName string
```

with:

```go
// SceneName identifies a scene within a Manager. Each game defines its own
// SceneName constants; the engine reserves DialogSceneName and
// SpriteShowcaseSceneName.
type SceneName string
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, all `TestSpriteShowcaseScene*` tests included.

If `render.TextDefault` turns out to be shared mutable state (the chained
`With*` calls returning the same pointer rather than a copy), the labels will
still draw and the tests will still pass; note it in the commit body and move
on. Do not restructure `render_text.go` in this task.

- [ ] **Step 6: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

Expected: both succeed with no warnings.

- [ ] **Step 7: Commit**

```bash
git add scene/scene_spriteshowcase.go scene/scene_spriteshowcase_test.go scene/scene.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add the sprite showcase scene

SpriteShowcaseScene draws every sprite in a render.SpriteLibrary with its
animations, labelled, in a grid: multi-animation sprites get a row each, then
single-animation sprites pack ten to a row. Ported from nrg's ShowcaseScene
with the game-specific catalog replaced by a library, so the engine now
reserves a second scene name."
```

---

### Task 3: Scene selection with `--scene`

The flag, its validation, and on-demand registration of the showcase. Depends on Tasks 1 and 2.

**Files:**
- Modify: `app/settings.go` (add `SceneSettings`, the `Scene` field, and the flag)
- Modify: `app/settings.toml` (add the `[scene]` section)
- Modify: `app/app.go` (add `applySceneSelection`, call it from `Run`)
- Test: `app/settings_test.go` (append), `app/app_scene_test.go` (create)

**Interfaces:**
- Consumes: `scene.SpriteShowcaseSceneName` and `scene.NewSpriteShowcaseScene` (Task 2); `render.Sprites` (Task 1); `scene.Manager.Scene`, `scene.Manager.AddScene`, `scene.Manager.ShowOnly`, `scene.Manager.SetExclusiveFocus`.
- Produces:
  - `type SceneSettings struct { Show []string }`
  - `Settings.Scene SceneSettings`
  - `func (a *App) applySceneSelection() error`

- [ ] **Step 1: Write the failing tests**

Append to `app/settings_test.go`:

```go
func TestLoadSettingsSceneShowDefaultsEmpty(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 0 {
		t.Fatalf("scene.show default = %v, want empty", s.Scene.Show)
	}
}

func TestSceneFlagRepeats(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene=rts", "--scene=dialog"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 2 || s.Scene.Show[0] != "rts" || s.Scene.Show[1] != "dialog" {
		t.Fatalf("scene.show = %v, want [rts dialog]", s.Scene.Show)
	}
}

func TestSceneFlagOverridesLoadedValue(t *testing.T) {
	s, err := LoadSettings("", []string{"scene.show=rts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 1 || s.Scene.Show[0] != "rts" {
		t.Fatalf("scene.show from override = %v, want [rts]", s.Scene.Show)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene=sprite_showcase"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Scene.Show) != 1 || s.Scene.Show[0] != "sprite_showcase" {
		t.Fatalf("flag did not override scene.show: %v", s.Scene.Show)
	}
}
```

Create `app/app_scene_test.go`:

```go
package app

import (
	"strings"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/scene"
)

// stubScene is a minimal Scene for exercising scene selection.
type stubScene struct {
	scene.BaseScene
	name scene.SceneName
}

func (s *stubScene) SceneName() scene.SceneName   { return s.name }
func (s *stubScene) Init(w, h int)                {}
func (s *stubScene) LayerIndex() int              { return 0 }
func (s *stubScene) Update(d time.Duration) error { return nil }
func (s *stubScene) Draw(screen *ebiten.Image)    {}

// newTestApp returns an App whose Scene.Show is set to the given names.
func newTestApp(t *testing.T, show ...string) *App {
	t.Helper()
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Scene.Show = show
	return New(s)
}

func TestApplySceneSelectionEmptyLeavesVisibilityAlone(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	first.SetVisible(true)
	second := &stubScene{name: "second"}
	second.SetVisible(false)
	a.Manager().AddScene(first)
	a.Manager().AddScene(second)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	if !first.IsVisible() {
		t.Fatal("empty selection hid a visible scene")
	}
	if second.IsVisible() {
		t.Fatal("empty selection showed a hidden scene")
	}
}

func TestApplySceneSelectionShowsOnlyRequestedAndFocusesFirst(t *testing.T) {
	a := newTestApp(t, "first", "second")
	first := &stubScene{name: "first"}
	second := &stubScene{name: "second"}
	third := &stubScene{name: "third"}
	third.SetVisible(true)
	a.Manager().AddScene(first)
	a.Manager().AddScene(second)
	a.Manager().AddScene(third)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	if !first.IsVisible() || !second.IsVisible() {
		t.Fatal("requested scenes are not visible")
	}
	if third.IsVisible() {
		t.Fatal("unrequested scene is still visible")
	}
	if !first.HasFocus() {
		t.Fatal("first requested scene does not have focus")
	}
	if second.HasFocus() || third.HasFocus() {
		t.Fatal("focus is not exclusive to the first requested scene")
	}
}

func TestApplySceneSelectionUnknownNameIsAnError(t *testing.T) {
	a := newTestApp(t, "nosuchscene")
	a.Manager().AddScene(&stubScene{name: "first"})

	err := a.applySceneSelection()
	if err == nil {
		t.Fatal("expected an error for an unknown scene name")
	}
	if !strings.Contains(err.Error(), "nosuchscene") {
		t.Fatalf("error does not name the unknown scene: %v", err)
	}
	if !strings.Contains(err.Error(), "first") {
		t.Fatalf("error does not list the registered scenes: %v", err)
	}
}

func TestApplySceneSelectionRegistersShowcaseOnDemand(t *testing.T) {
	a := newTestApp(t, string(scene.SpriteShowcaseSceneName))

	if _, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName); ok {
		t.Fatal("showcase should not be registered before selection")
	}
	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	registered, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName)
	if !ok {
		t.Fatal("showcase was not registered on demand")
	}
	if !registered.IsVisible() {
		t.Fatal("on-demand showcase is not visible")
	}
	if !registered.HasFocus() {
		t.Fatal("on-demand showcase does not have focus")
	}
}

func TestApplySceneSelectionKeepsGameRegisteredShowcase(t *testing.T) {
	a := newTestApp(t, string(scene.SpriteShowcaseSceneName))
	own := &stubScene{name: scene.SpriteShowcaseSceneName}
	a.Manager().AddScene(own)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	registered, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName)
	if !ok {
		t.Fatal("scene disappeared")
	}
	if registered != scene.Scene(own) {
		t.Fatal("engine replaced the game's own scene registered under the showcase name")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: FAIL to build the `app` package, with `s.Scene undefined` and `a.applySceneSelection undefined`.

- [ ] **Step 3: Add the settings**

In `app/settings.go`, add the field to `Settings`, keeping the existing order and adding `Scene` after `Camera`:

```go
type Settings struct {
	Window     WindowSettings     `toml:"window"`
	Camera     CameraSettings     `toml:"camera"`
	Scene      SceneSettings      `toml:"scene"`
	Debug      DebugSettings      `toml:"debug"`
	Screenshot ScreenshotSettings `toml:"screenshot"`
	Run        RunSettings        `toml:"run"`
	Log        LogSettings        `toml:"log"`
	Render     RenderSettings     `toml:"render"`
}
```

Add the type next to the other section types:

```go
// SceneSettings forces which registered scenes are shown at startup. An empty
// Show leaves scene visibility entirely to the game.
type SceneSettings struct {
	Show []string `toml:"show"`
}
```

Add the flag in `RegisterFlags`, after the camera-related flags and before the
debug flags. `StringSliceVar` comes from `github.com/spf13/pflag`, already
aliased to `flag` in this file, and accepts both repetition and comma-separated
values:

```go
	fs.StringSliceVar(&s.Scene.Show, "scene", s.Scene.Show,
		"Show only the named scenes and focus the first (repeatable)")
```

In `app/settings.toml`, add the section after `[camera]`:

```toml
[scene]
show = []
```

- [ ] **Step 4: Add the selection logic**

In `app/app.go`, add the method. It needs `scene` (already imported) and
`render`:

```go
// applySceneSelection honours the [scene] show setting and the --scene flag: it
// shows exactly the named scenes and focuses the first. An empty selection
// leaves visibility to the game.
//
// It registers the engine's own SpriteShowcaseScene on demand, since no game
// registers it, and validates the requested names, because Manager.ShowOnly and
// Manager.SetExclusiveFocus silently ignore names that match nothing: a typo
// would otherwise blank the window with no diagnostic.
func (a *App) applySceneSelection() error {
	show := a.settings.Scene.Show
	if len(show) == 0 {
		return nil
	}

	names := make([]scene.SceneName, 0, len(show))
	for _, name := range show {
		names = append(names, scene.SceneName(name))
	}

	for _, name := range names {
		if name != scene.SpriteShowcaseSceneName {
			continue
		}
		if _, ok := a.manager.Scene(name); !ok {
			a.manager.AddScene(scene.NewSpriteShowcaseScene())
		}
	}

	for _, name := range names {
		if _, ok := a.manager.Scene(name); !ok {
			return fmt.Errorf("unknown scene %q, registered scenes: %s",
				name, strings.Join(a.registeredSceneNames(), ", "))
		}
	}

	a.manager.ShowOnly(names...)
	a.manager.SetExclusiveFocus(names[0])

	if _, ok := a.manager.Scene(scene.SpriteShowcaseSceneName); ok && render.Sprites.Len() == 0 {
		util.Logger.Warn().Msg(
			"Sprite showcase requested but no sprites are registered; register them with render.Sprites.Add")
	}

	return nil
}

// registeredSceneNames returns every registered scene name, sorted, for error
// messages.
func (a *App) registeredSceneNames() []string {
	names := a.manager.SceneNames()
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, string(n))
	}
	return out
}
```

This needs `Manager.SceneNames`, which does not exist yet. Add it to
`scene/scene_manager.go`, next to `Scene`:

```go
// SceneNames returns every registered scene name, sorted lexicographically.
func (m *Manager) SceneNames() []SceneName {
	names := make([]SceneName, 0, len(m.scenes))
	for name := range m.scenes {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}
```

`sort` is already imported in `scene/scene_manager.go`.

Call it from `Run`, between the window title and the manager `Init`, so it runs
after the game has registered its scenes and before every scene is initialized.
Replace this part of `app/app.go`:

```go
	ebiten.SetWindowTitle(a.settings.Window.Title)

	a.screenWidth, a.screenHeight = ebiten.Monitor().Size()
	a.manager.Init(a.screenWidth, a.screenHeight)
```

with:

```go
	ebiten.SetWindowTitle(a.settings.Window.Title)

	if err := a.applySceneSelection(); err != nil {
		return err
	}

	a.screenWidth, a.screenHeight = ebiten.Monitor().Size()
	a.manager.Init(a.screenWidth, a.screenHeight)
```

Add `"strings"` to the standard-library import group in `app/app.go`, and
`"github.com/trancecode/vantage/render"` to the local group, if either is
missing.

- [ ] **Step 5: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, including the new `TestApplySceneSelection*`, `TestSceneFlag*`
and `TestLoadSettingsSceneShowDefaultsEmpty` tests.

- [ ] **Step 6: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

Expected: both succeed with no warnings.

- [ ] **Step 7: Commit**

```bash
git add app/settings.go app/settings.toml app/app.go app/settings_test.go \
  app/app_scene_test.go scene/scene_manager.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Force which scenes are shown with a --scene flag

The repeatable --scene flag and its [scene] show equivalent show exactly the
named scenes and focus the first, with the sprite showcase registered on demand
since no game registers it. Requested names are validated against the manager,
because ShowOnly and SetExclusiveFocus silently ignore names that match
nothing, so a typo would otherwise render a black window with no diagnostic."
```

---

### Task 4: Documentation

Package docs, architecture map, debugging guide and the performance log. Depends on Tasks 1 to 3.

**Files:**
- Modify: `render/doc.go`, `scene/doc.go`, `app/doc.go`
- Modify: `ARCHITECTURE.md:34`, `ARCHITECTURE.md:36`, `ARCHITECTURE.md:37`
- Modify: `docs/debugging.md`
- Modify: `docs/performance_optimization.md`

**Interfaces:**
- Consumes: everything from Tasks 1 to 3. Produces no code.

- [ ] **Step 1: Update the package docs**

In `render/doc.go`, add after the `Sprite` sentence:

```
// SpriteLibrary maps display names to sprites, with the package-level Sprites
// as the default library a game registers into at init time.
```

In `scene/doc.go`, replace the final sentence so both reserved names are named,
and mention the showcase:

```
// BaseScene is an embeddable default implementation, DialogScene is the
// engine's built-in modal dialog overlay, and SpriteShowcaseScene draws every
// sprite in a render.SpriteLibrary for visual inspection. Scenes are identified
// by a typed-string SceneName; each game defines its own names, and the engine
// reserves DialogSceneName and SpriteShowcaseSceneName.
```

In `app/doc.go`, add before the final sentence:

```
// The [scene] show setting and the --scene flag force which registered scenes
// are shown at startup, and register the engine's sprite showcase on demand.
```

- [ ] **Step 2: Update ARCHITECTURE.md**

In the package table, extend the three affected rows so they mention the new
surface. Line 34 becomes:

```
| `render` | Graphics layer: camera, sprites, sprite library, text | `asset`, `geometry` |
```

Line 36 becomes:

```
| `scene` | `Scene` interface, the `Manager` that drives scenes, and the sprite showcase | `render`, `ui` |
```

Line 37 becomes:

```
| `app` | Top-level `App` (implements `ebiten.Game`), window, scene selection, screenshots | `config`, `render`, `scene`, `util` |
```

- [ ] **Step 3: Add the debugging docs**

In `docs/debugging.md`, add two sections after "Debug mode" and before "Screen
logger (on-screen overlay)":

````markdown
## Forcing scenes

`[scene] show` in the game's TOML settings, or the `--scene` command-line flag,
overrides which registered scenes are visible at startup. The named scenes are
shown, every other scene is hidden, and the first name listed gets exclusive
focus.

```bash
# Show one scene
./game --scene rts

# Show two, focusing the first. Repetition and commas both work.
./game --scene rts --scene dialog
./game --scene rts,dialog
```

The equivalent settings entry:

```toml
[scene]
show = ["rts", "dialog"]
```

An empty `show`, which is the default, leaves scene visibility entirely to the
game. A name that is not registered is a startup error listing the scenes that
are, since a silently ignored typo would otherwise render a black window.

The flag selects among scenes the game has already registered; it does not
construct them. The one exception is the engine's own sprite showcase below,
which `app` registers on demand.

## Sprite showcase

`--scene sprite_showcase` displays every sprite in the engine's sprite library,
animating, labelled with the sprite name and the name of each animation. Use it
to inspect art without building a level around it.

Sprites with more than one animation get a row each, with one column per
animation. Sprites with a single animation pack into a grid below them, ten to
a row.

```bash
./game --scene sprite_showcase
```

The scene needs no wiring: `app` registers it when the flag names it. What it
shows is whatever the game registered into `render.Sprites`:

```go
render.Sprites.Add("Character", render.MustLoadSprite(
    data.ImagePlayer, 6, 10,
    map[render.AnimationType][]int{
        render.AnimationIdleDown:  {0, 1, 2, 3, 4, 5},
        render.AnimationIdleRight: {6, 7, 8, 9, 10, 11},
    },
    nil,
)).SetZeroPosition(geometry.NewVector2(24, 40)).SetType(render.SpriteTypeActor)
```

`Add` returns the sprite, so registration chains onto the sprite setters. It
panics on an empty name, a nil sprite, or a duplicate name, since all three are
load-time mistakes. If the library is empty when the showcase is shown, the
engine logs a warning rather than leaving a blank screen unexplained.

Camera controls, from `render.CameraController`:

* `W` / `A` / `S` / `D` pan.
* `Q` / `E` zoom out and in.
````

- [ ] **Step 4: Record the performance note**

Per `CLAUDE.md`, potential optimizations get recorded rather than made. Add to
`docs/performance_optimization.md`, matching the surrounding format:

```markdown
## Sprite showcase draws every sprite every frame

`scene.SpriteShowcaseScene.drawAllSprites` walks the whole sprite library each
frame and draws every cell, with no culling against the camera viewport and no
caching of the layout. On a large library most cells are off screen. This is
acceptable for a debug scene reached by an explicit flag, and it is deliberately
not optimized, but a library of thousands of sprites would need viewport culling.
```

Before adding it, read `docs/performance_optimization.md` and check whether any
existing entry has been made irrelevant by this work. `CLAUDE.md` requires that
cleanup as part of the change. If none has, say so and change nothing else.

- [ ] **Step 5: Verify the build is still clean**

Documentation-only, but `doc.go` files are Go source:

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all three succeed.

- [ ] **Step 6: Commit**

```bash
git add render/doc.go scene/doc.go app/doc.go ARCHITECTURE.md \
  docs/debugging.md docs/performance_optimization.md
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Document the sprite library, showcase scene and scene selection

Add debugging guide sections for --scene and --scene sprite_showcase, extend
the package docs and the architecture table, and record the showcase's
uncalled-for redraw as a known optimization rather than making it."
```

---

## Self-review notes

Spec coverage, section by section:

* `render.SpriteLibrary`, package-level `Sprites`, chainable `Add`, panics, sorted `Names`, comma-ok `Get`, pure `LoadSprite`: Task 1.
* `scene.SpriteShowcaseScene`, both constructors, scene lifecycle, layout, label handling, `SceneName` doc update: Task 2.
* `SceneSettings`, `--scene` flag, validation, on-demand showcase registration, placement in `Run`, empty-library warning: Task 3.
* Documentation and the performance note: Task 4.

Deviations from the spec, both discovered while checking the code and folded
into the spec before this plan was written:

* The spec originally called for a custom `flag.Value`. The engine uses
  `github.com/spf13/pflag`, so `StringSliceVar` covers it.
* The spec originally gave the scene its own `camera` field. `BaseScene`
  already has one.

One addition this plan makes that the spec did not name: `Manager.SceneNames`,
needed so the validation error can list the registered scenes. It is a
read-only accessor in the same shape as the existing `Scene` method.
