# Showcase slot size and animation names implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the sprite showcase usable for judging art: a configurable slot size so character-sized sprites are not shrunk to one tile, and registrable display names so a game's own animation types do not label as `AnimationType(64)`.

**Architecture:** `render` gains an animation name registry with a fallback to the generated `String()`. `scene` gains a package-level `ShowcaseSlotTiles` that the grid geometry derives from, replacing constants that assumed a one-tile slot. `app` gains the setting, flag and validation.

**Tech Stack:** Go, Ebitengine, `github.com/spf13/pflag`, go-task.

**Design spec:** `docs/superpowers/specs/2026-07-26-showcase-slot-and-animation-names-design.md`

## Global constraints

* `export GOMODCACHE=/tmp/go-mod-cache` before any Go command.
* Run tests with `task test:headless`, never bare `go test`.
* **Never run `go run ./cmd/...` without `xvfb-run -a`.** It opens a window on the user's desktop. This applies to one-off probes too. See `CLAUDE.md`.
* Before any commit: `task lint`, `task test:headless` and `go vet ./...` must all pass.
* Commit author `Claude Code <herve.quiroz+claude@gmail.com>` via `git -c user.name=... -c user.email=...`. No `Co-Authored-By:` line.
* Work directly on `main`. No branches.
* Every exported identifier needs a doc comment. Import groups: stdlib, third party, then `github.com/trancecode/...`.
* Documentation style: sentence case titles, stars not dashes for list items, no em dashes, noun-phrase headings.

## Compatibility guarantee

At a slot of one tile, every derived grid quantity must equal the constant it replaces, so the existing layout tests pass unchanged: column pitch 2.0, row pitch 4.0, label centre X 0.5, sprite label Y 1.2, animation label Y 1.8. Task 2 pins this.

## The no-capture rule

Nothing may capture `ShowcaseSlotTiles` or `render.TileSize` at construction or into a package-level derived value. Settings are applied in `App.Run`, after games have registered sprites from `init()`. Read them where used.

---

### Task 1: The animation name registry

**Files:**
- Create: `render/render_animationname.go`
- Test: `render/render_animationname_test.go`

**Interfaces:**
- Consumes: `render.AnimationType` and its generated `String()`.
- Produces: `func RegisterAnimationName(a AnimationType, name string)`; `func AnimationName(a AnimationType) string`.

- [ ] **Step 1: Write the failing tests**

Create `render/render_animationname_test.go`:

```go
package render

import "testing"

// testAnimationType returns a game-range animation type well clear of both the
// engine's own values and of other tests' registrations, since the registry is
// package-level.
func testAnimationType(offset int) AnimationType {
	return AnimationGameBase + 1000 + AnimationType(offset)
}

func TestAnimationNameUsesTheRegisteredName(t *testing.T) {
	a := testAnimationType(0)
	RegisterAnimationName(a, "South-east")
	if got := AnimationName(a); got != "South-east" {
		t.Fatalf("AnimationName = %q, want %q", got, "South-east")
	}
}

func TestAnimationNameTrimsTheEnginePrefixWhenUnregistered(t *testing.T) {
	// The engine's own names all start with "Animation", which is noise in a
	// label.
	if got := AnimationName(AnimationIdleDown); got != "IdleDown" {
		t.Fatalf("AnimationName = %q, want %q", got, "IdleDown")
	}
	if got := AnimationName(AnimationDefault); got != "Default" {
		t.Fatalf("AnimationName = %q, want %q", got, "Default")
	}
}

func TestAnimationNameFallsBackToStringForUnknownTypes(t *testing.T) {
	// A game type with no registration has no prefix to trim, so it reads as
	// the stringer's placeholder. Ugly, which is why registering is worth it.
	a := testAnimationType(1)
	want := a.String()
	if got := AnimationName(a); got != want {
		t.Fatalf("AnimationName = %q, want %q", got, want)
	}
}

func TestRegisterAnimationNameAcceptsAnEngineType(t *testing.T) {
	// A game may prefer its own wording for the engine's types.
	RegisterAnimationName(AnimationAttackUp, "Attack up")
	defer unregisterAnimationName(AnimationAttackUp)
	if got := AnimationName(AnimationAttackUp); got != "Attack up" {
		t.Fatalf("AnimationName = %q, want %q", got, "Attack up")
	}
}

func TestRegisterAnimationNameEmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on an empty animation name")
		}
	}()
	RegisterAnimationName(testAnimationType(2), "")
}

func TestRegisterAnimationNameDuplicatePanics(t *testing.T) {
	a := testAnimationType(3)
	RegisterAnimationName(a, "First")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on a duplicate animation type")
		}
	}()
	RegisterAnimationName(a, "Second")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: build failure in `render` with `undefined: RegisterAnimationName`, `AnimationName` and `unregisterAnimationName`.

- [ ] **Step 3: Write the implementation**

Create `render/render_animationname.go`:

```go
package render

import (
	"fmt"
	"strings"
)

// enginePrefix is the prefix the generated String() gives the engine's own
// animation type names, which is noise in a label.
const enginePrefix = "Animation"

// animationNames holds display names registered for animation types. Not safe
// for concurrent use: registration happens at init time and reads happen while
// drawing, both on the same goroutine, matching SpriteLibrary.
var animationNames = map[AnimationType]string{}

// RegisterAnimationName gives an animation type a display name. It exists for
// the types the engine's generated String() cannot know: a game defining its
// own types at or above AnimationGameBase gets "AnimationType(64)" and so on,
// which is unreadable in a sprite showcase label.
//
// Registering one of the engine's own types is allowed, for a game that prefers
// its own wording.
//
// It panics on an empty name or a type that is already registered. Both are
// load-time programming errors, and registration runs from init, so failing
// loudly beats a catalog that depends on package initialization order.
func RegisterAnimationName(a AnimationType, name string) {
	if name == "" {
		panic(fmt.Sprintf("animation name for %v must not be empty", a))
	}
	if existing, ok := animationNames[a]; ok {
		panic(fmt.Sprintf("duplicate animation name for %v: %q and %q", a, existing, name))
	}
	animationNames[a] = name
}

// AnimationName returns an animation type's display name: the registered one if
// there is one, and otherwise its String() with the engine's "Animation" prefix
// trimmed, so AnimationIdleDown reads as IdleDown. An unregistered game type has
// no prefix to trim and reads as the stringer's placeholder.
func AnimationName(a AnimationType) string {
	if name, ok := animationNames[a]; ok {
		return name
	}
	trimmed, _ := strings.CutPrefix(a.String(), enginePrefix)
	return trimmed
}

// unregisterAnimationName removes a registration. It exists only so tests can
// clean up after registering one of the engine's own types, since the registry
// is package-level.
func unregisterAnimationName(a AnimationType) {
	delete(animationNames, a)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, all packages.

- [ ] **Step 5: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 6: Commit**

```bash
git add render/render_animationname.go render/render_animationname_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Let games name their own animation types

A game defining animation types at or above AnimationGameBase gets
AnimationType(64) from the generated stringer, which is unreadable as a showcase
label. RegisterAnimationName gives them display names, and AnimationName falls
back to the generated name with the engine's Animation prefix trimmed, moving
that trim out of the showcase and into one place."
```

---

### Task 2: A configurable showcase slot

**Files:**
- Modify: `scene/scene_spriteshowcase.go` (the const block, `showcaseFitScale`, `showcaseLayout`, `drawCell`)
- Test: `scene/scene_spriteshowcase_test.go`

**Interfaces:**
- Consumes: `render.TileSize`; `render.AnimationName` (Task 1).
- Produces: `var scene.ShowcaseSlotTiles float64 = 1`.

- [ ] **Step 1: Write the failing tests**

Append to `scene/scene_spriteshowcase_test.go`. Read the file's existing helpers first (`showcaseTestSprite`, `showcaseTestSpriteSized`) and reuse them.

```go
func TestShowcaseSlotTilesDefaultsToOne(t *testing.T) {
	// The compatibility guarantee: at one tile every derived quantity equals
	// the constant it replaced, so the existing layout tests hold.
	if ShowcaseSlotTiles != 1 {
		t.Fatalf("ShowcaseSlotTiles = %v, want 1", ShowcaseSlotTiles)
	}
	if got := showcaseColumnPitch(); got != 2.0 {
		t.Fatalf("column pitch = %v, want 2", got)
	}
	if got := showcaseRowPitch(); got != 4.0 {
		t.Fatalf("row pitch = %v, want 4", got)
	}
	if got := showcaseLabelCenterX(); got != 0.5 {
		t.Fatalf("label centre X = %v, want 0.5", got)
	}
	if got := showcaseSpriteLabelY(); got != 1.2 {
		t.Fatalf("sprite label Y = %v, want 1.2", got)
	}
	if got := showcaseAnimationLabelY(); got != 1.8 {
		t.Fatalf("animation label Y = %v, want 1.8", got)
	}
}

func TestShowcaseGeometryDerivesFromTheSlot(t *testing.T) {
	original := ShowcaseSlotTiles
	t.Cleanup(func() { ShowcaseSlotTiles = original })
	ShowcaseSlotTiles = 3

	// The gap stays one tile and the label space two, because labels are drawn
	// at a fixed pixel size and do not grow with the slot.
	if got := showcaseColumnPitch(); got != 4.0 {
		t.Fatalf("column pitch = %v, want 4", got)
	}
	if got := showcaseRowPitch(); got != 6.0 {
		t.Fatalf("row pitch = %v, want 6", got)
	}
	if got := showcaseLabelCenterX(); got != 1.5 {
		t.Fatalf("label centre X = %v, want 1.5", got)
	}
	if got := showcaseSpriteLabelY(); got != 3.2 {
		t.Fatalf("sprite label Y = %v, want 3.2", got)
	}
	if got := showcaseAnimationLabelY(); got != 3.8 {
		t.Fatalf("animation label Y = %v, want 3.8", got)
	}
}

func TestShowcaseFitScaleUsesTheSlotSize(t *testing.T) {
	original := ShowcaseSlotTiles
	t.Cleanup(func() { ShowcaseSlotTiles = original })

	// Art two tiles tall: shrunk by half into a one tile slot, left alone in a
	// three tile slot.
	sprite := showcaseTestSpriteSized(32, 1.0, render.AnimationDefault)

	ShowcaseSlotTiles = 1
	if got := showcaseFitScale(sprite); got != 0.5 {
		t.Fatalf("fit scale at a one tile slot = %v, want 0.5", got)
	}

	ShowcaseSlotTiles = 3
	if got := showcaseFitScale(sprite); got != 1.0 {
		t.Fatalf("fit scale at a three tile slot = %v, want 1", got)
	}
}

func TestShowcaseLayoutSpacesCellsBySlot(t *testing.T) {
	original := ShowcaseSlotTiles
	t.Cleanup(func() { ShowcaseSlotTiles = original })
	ShowcaseSlotTiles = 3

	l := render.NewSpriteLibrary()
	l.Add("A", showcaseTestSprite(render.AnimationDefault))
	l.Add("B", showcaseTestSprite(render.AnimationDefault))

	cells := showcaseLayout(l)
	if len(cells) != 2 {
		t.Fatalf("len(cells) = %d, want 2", len(cells))
	}
	if got := cells[1].X - cells[0].X; got != 4.0 {
		t.Fatalf("cell spacing = %v, want the 4 tile column pitch", got)
	}
}

func TestShowcaseLayoutWrapsAtTheSameColumnCountForAnySlot(t *testing.T) {
	original := ShowcaseSlotTiles
	t.Cleanup(func() { ShowcaseSlotTiles = original })
	ShowcaseSlotTiles = 5

	l := render.NewSpriteLibrary()
	for _, n := range []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11"} {
		l.Add(n, showcaseTestSprite(render.AnimationDefault))
	}

	cells := showcaseLayout(l)
	if len(cells) != 11 {
		t.Fatalf("len(cells) = %d, want 11", len(cells))
	}
	if cells[10].X != showcaseOriginX {
		t.Fatalf("11th cell X = %v, want the origin %v", cells[10].X, showcaseOriginX)
	}
	if got := cells[10].Y - cells[9].Y; got != showcaseRowPitch() {
		t.Fatalf("11th cell dropped %v, want one row pitch %v", got, showcaseRowPitch())
	}
}

func TestShowcaseLayoutOrdersAnimationsByValueNotName(t *testing.T) {
	// Once a game can name its types, alphabetical order scrambles them: eight
	// facings named S, SE, E, NE would sort E, N, NE, S. Value order keeps the
	// order the game declared.
	first := render.AnimationGameBase + 2000
	second := render.AnimationGameBase + 2001
	render.RegisterAnimationName(first, "S")
	render.RegisterAnimationName(second, "E")

	l := render.NewSpriteLibrary()
	l.Add("Facings", showcaseTestSprite(first, second))

	cells := showcaseLayout(l)
	if len(cells) != 2 {
		t.Fatalf("len(cells) = %d, want 2", len(cells))
	}
	if cells[0].Animation != first || cells[1].Animation != second {
		t.Fatalf("animations ordered %v then %v, want value order %v then %v",
			cells[0].Animation, cells[1].Animation, first, second)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: build failure in `scene` with `undefined: ShowcaseSlotTiles`, `showcaseColumnPitch` and the other accessors, which are currently constants rather than functions.

- [ ] **Step 3: Replace the geometry constants with slot-derived functions**

In `scene/scene_spriteshowcase.go`, keep `showcaseOriginX`, `showcaseOriginY` and `showcaseColumnsPerRow` as constants. Replace the five slot-dependent constants with the variable and accessor functions:

```go
// ShowcaseSlotTiles is how many tiles square a slot the sprite showcase gives
// each cell's art. Engine configuration, set through [scene]
// showcase_slot_tiles. Art larger than the slot is scaled down to fit it; art
// at or below it is left at its natural size.
//
// A game whose characters are taller than a tile wants a slot big enough to
// show them, since a one tile slot shrinks them until they cannot be judged.
var ShowcaseSlotTiles float64 = 1

// Grid geometry, derived from the slot size so a bigger slot gets a bigger
// cell. The gap stays one tile and the label space two, whatever the slot,
// because labels are drawn at a fixed pixel size and do not grow with it. At a
// slot of one tile every one of these reproduces the constant it replaced.

// showcaseColumnPitch is the horizontal distance between two cells.
func showcaseColumnPitch() float64 { return ShowcaseSlotTiles + 1 }

// showcaseRowPitch is the vertical distance between two rows, leaving room for
// the sprite and both labels.
func showcaseRowPitch() float64 { return ShowcaseSlotTiles + 3 }

// showcaseLabelCenterX offsets a label to the horizontal center of its cell.
func showcaseLabelCenterX() float64 { return ShowcaseSlotTiles / 2 }

// showcaseSpriteLabelY offsets the sprite name below the sprite.
func showcaseSpriteLabelY() float64 { return ShowcaseSlotTiles + 0.2 }

// showcaseAnimationLabelY offsets the animation name below the sprite name.
func showcaseAnimationLabelY() float64 { return ShowcaseSlotTiles + 0.8 }
```

Then update every use. `grep -n "showcaseColumnPitch\|showcaseRowPitch\|showcaseLabelCenterX\|showcaseSpriteLabelY\|showcaseAnimationLabelY" scene/` finds them; each becomes a call. They are in `showcaseLayout` and `drawCell`.

- [ ] **Step 4: Make the fit scale use the slot**

`showcaseFitScale` currently divides `render.TileSize` into the drawn art size. The slot is now `ShowcaseSlotTiles` tiles, so the pixel slot is `render.TileSize * ShowcaseSlotTiles`. Change the final line accordingly and update the doc comment, which says the slot is one tile.

Do not capture either value; read both where used.

- [ ] **Step 5: Order animations by value and use registered names**

In `showcaseLayout`, the animations of a multi-animation sprite are sorted with `sort.Slice` comparing `String()`. Compare the `AnimationType` values instead. Update the doc comment on `showcaseLayout`, which says "sorted by String()".

In `drawCell`, the animation label is built with `strings.CutPrefix(cell.Animation.String(), "Animation")`. Replace that with `render.AnimationName(cell.Animation)`, which does the same trimming and honours registrations. Drop the `strings` import if nothing else in the file uses it.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS. The pre-existing layout tests pin concrete positions computed for a one-tile slot and must pass unchanged, which is the compatibility guarantee. One exception: any existing test asserting animations sort by name will need its expectation updated to value order. Change the expectation, not the assertion's strength, and say in your report which tests you touched and why.

- [ ] **Step 7: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 8: Commit**

```bash
git add scene/scene_spriteshowcase.go scene/scene_spriteshowcase_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Make the showcase slot size configurable

Every cell gave its art exactly one tile, so a game whose characters are taller
than a tile could not judge them. ShowcaseSlotTiles sets the slot, and the grid
geometry derives from it: the gap stays one tile and the label space two, since
labels are a fixed pixel size. At a slot of one tile every quantity reproduces
the constant it replaced.

Animations now order by AnimationType value rather than by name, because once a
game names its own types alphabetical order scrambles them, and labels use the
registered name."
```

---

### Task 3: The slot setting

**Files:**
- Modify: `app/settings.go` (`SceneSettings`, `RegisterFlags`, `Validate`, `Apply`)
- Modify: `app/settings.toml`
- Test: `app/settings_test.go` (append)

**Interfaces:**
- Consumes: `scene.ShowcaseSlotTiles` (Task 2).
- Produces: `SceneSettings.ShowcaseSlotTiles float64`.

- [ ] **Step 1: Write the failing tests**

Append to `app/settings_test.go`. Check the file's imports first; it needs `scene`.

```go
func TestLoadSettingsShowcaseSlotTilesDefaultsToOne(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Scene.ShowcaseSlotTiles != 1 {
		t.Fatalf("scene.showcase_slot_tiles default = %v, want 1", s.Scene.ShowcaseSlotTiles)
	}
}

func TestShowcaseSlotTilesOverrideAndFlag(t *testing.T) {
	s, err := LoadSettings("", []string{"scene.showcase_slot_tiles=3"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Scene.ShowcaseSlotTiles != 3 {
		t.Fatalf("scene.showcase_slot_tiles = %v, want 3", s.Scene.ShowcaseSlotTiles)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--scene_showcase_slot_tiles=8"}); err != nil {
		t.Fatal(err)
	}
	if s.Scene.ShowcaseSlotTiles != 8 {
		t.Fatalf("flag did not override the slot size: %v", s.Scene.ShowcaseSlotTiles)
	}
}

func TestValidateRejectsANonPositiveSlotSize(t *testing.T) {
	for _, size := range []float64{0, -3} {
		s, err := LoadSettings("", nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Scene.ShowcaseSlotTiles = size
		err = s.Validate()
		if err == nil {
			t.Fatalf("Validate accepted a slot size of %v", size)
		}
		if !strings.Contains(err.Error(), "showcase_slot_tiles") {
			t.Fatalf("error does not name the setting: %v", err)
		}
	}
}

func TestApplySetsShowcaseSlotTiles(t *testing.T) {
	original := scene.ShowcaseSlotTiles
	t.Cleanup(func() { scene.ShowcaseSlotTiles = original })

	s, err := LoadSettings("", []string{"scene.showcase_slot_tiles=4"})
	if err != nil {
		t.Fatal(err)
	}
	s.Apply()
	if scene.ShowcaseSlotTiles != 4 {
		t.Fatalf("Apply did not set scene.ShowcaseSlotTiles: %v", scene.ShowcaseSlotTiles)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: build failure in `app` with `s.Scene.ShowcaseSlotTiles undefined`.

- [ ] **Step 3: Add the setting**

In `app/settings.go`, add to `SceneSettings`:

```go
	// ShowcaseSlotTiles is how many tiles square a slot the sprite showcase
	// gives each cell's art. Larger art is scaled down to fit.
	ShowcaseSlotTiles float64 `toml:"showcase_slot_tiles"`
```

Register the flag beside the `--scene` one:

```go
	fs.Float64Var(&s.Scene.ShowcaseSlotTiles, "scene_showcase_slot_tiles", s.Scene.ShowcaseSlotTiles,
		"Tiles square of the slot the sprite showcase gives each cell's art")
```

Extend `Validate`, keeping the existing tile size check:

```go
	if s.Scene.ShowcaseSlotTiles <= 0 {
		return fmt.Errorf("scene.showcase_slot_tiles must be positive, got %v", s.Scene.ShowcaseSlotTiles)
	}
```

Set it in `Apply`, beside the render globals:

```go
	scene.ShowcaseSlotTiles = s.Scene.ShowcaseSlotTiles
```

`app/settings.go` will need to import `github.com/trancecode/vantage/scene`. Confirm that introduces no import cycle: `scene` does not import `app`.

In `app/settings.toml`, add to the `[scene]` section:

```toml
showcase_slot_tiles = 1
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

- [ ] **Step 5: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 6: Commit**

```bash
git add app/settings.go app/settings.toml app/settings_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add the showcase slot size setting

[scene] showcase_slot_tiles and --scene_showcase_slot_tiles set how many tiles
square a slot the showcase gives each cell's art, defaulting to the one tile it
used to hardcode. A non-positive value is a startup error."
```

---

### Task 4: Demo coverage and documentation

**Files:**
- Modify: `cmd/showcasedemo/main.go`
- Modify: `docs/debugging.md`
- Modify: `render/doc.go`, `scene/doc.go`

**Interfaces:**
- Consumes: everything from Tasks 1 to 3.

- [ ] **Step 1: Give the demo a sprite that exercises both features**

The demo currently registers no sprite with a `SourceTileSize` and no registered animation names, so neither the tile ratio path nor the naming path has visual coverage in this repository.

Add one sprite representing the case these features exist for: art larger than a tile that declares the tile size it was drawn for, with several animations carrying registered names. Something like a 64 pixel frame declaring `SetSourceTileSize(32)`, so it is two tiles, with four directional animations at `render.AnimationGameBase + 0..3` named "S", "E", "N", "W" through `render.RegisterAnimationName`.

Register the names from an `init` function or at the top of `registerSprites`, before the sprites are built. Keep the existing sprites exactly as they are, since the documentation describes their sizes.

Update the command's doc comment to say what the new sprite demonstrates.

- [ ] **Step 2: Verify it headlessly and look at the result**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go run ./cmd/showcasedemo \
    --scene_showcase_slot_tiles 3 \
    --screenshot_path /tmp/slot3.png \
    --screenshot_delay 500ms \
    --run_for 1000ms
```

Read the PNG. Confirm the cells are further apart, the art is bigger, and the new sprite's labels read as its registered names rather than `AnimationType(64)`. Capture a default-slot screenshot too and confirm it still matches the documented description. Report what you saw.

Never run this without `xvfb-run -a`.

- [ ] **Step 3: Document both features**

In `docs/debugging.md`'s sprite showcase section:

* Add the slot setting beside the existing "Pixels per tile" section, in the same shape: what it is, the default of 1, that a non-positive value is a startup error, the flag, and a runnable `xvfb-run` example. Explain why a game would raise it: art taller than a tile is otherwise shrunk until it cannot be judged.
* Add a short section on naming animations, with a worked example of a game registering eight facings, and note that unregistered game types label as `AnimationType(64)`. Mention that animations are ordered by their type value, so the order a game declares them in is the order shown.

In `render/doc.go`, mention the animation name registry. In `scene/doc.go`, mention the configurable slot.

Also update the paragraph in `docs/debugging.md` that says each cell gives its art "a slot one tile square", which is now the default rather than a fixed fact.

- [ ] **Step 4: Verify the build is clean**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/showcasedemo/main.go docs/debugging.md render/doc.go scene/doc.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Demonstrate and document the slot size and animation names

showcasedemo gains a two tile sprite that declares its source tile size and
carries named directional animations, so both paths have visual coverage in this
repository rather than unit coverage alone."
```

---

## Self-review notes

Spec coverage:

* The animation name registry, its fallback and its panics: Task 1.
* The configurable slot, the derived geometry, the fit scale and the ordering change: Task 2.
* The setting, flag and validation: Task 3.
* Demo coverage and documentation: Task 4.

The compatibility guarantee is pinned by `TestShowcaseSlotTilesDefaultsToOne`, which asserts all five derived quantities equal their former constants, and by the pre-existing layout tests that compute concrete positions for a one-tile slot.

One thing to watch during Task 2: the existing tests include animation-ordering assertions written when ordering was by name. Their expectations change to value order. That is the intended behavior change, not a regression, and the plan asks the implementer to report which tests they touched.
