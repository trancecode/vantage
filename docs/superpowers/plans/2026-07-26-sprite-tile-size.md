# Sprite tile size implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a game configure its tile size, and let each sprite declare the tile size its art was drawn for, so the engine scales art to the game.

**Architecture:** `render.TileSize` becomes a configurable variable instead of a compile-time constant, and `Camera` stops freezing it at construction. `render.Sprite` gains a `SourceTileSize`, and the draw path applies `TileSize / SourceTileSize` exactly where it already applies `displayScale`. `app` gains the `[render] tile_size` setting, a `--tile_size` flag, and startup validation.

**Tech Stack:** Go, Ebitengine, `github.com/spf13/pflag`, go-task.

**Design spec:** `docs/superpowers/specs/2026-07-26-sprite-tile-size-design.md`

## Global constraints

* `export GOMODCACHE=/tmp/go-mod-cache` before any Go command. The default module cache is not writable.
* Run tests with `task test:headless`, never bare `go test`. The `render`, `ui`, `scene` and `app` packages initialize Ebiten/GLFW and need the virtual display that target provides.
* **Never run `go run ./cmd/...` without `xvfb-run -a`.** It opens a window on the user's desktop. This applies to one-off probes as much as documented commands. See the "Running the Engine Headlessly" section of `CLAUDE.md`.
* Before any commit: `task lint`, `task test:headless` and `go vet ./...` must all pass.
* Commit author is `Claude Code <herve.quiroz+claude@gmail.com>` via `git -c user.name=... -c user.email=...`. No `Co-Authored-By:` line.
* Work directly on `main`. No branches.
* Every exported type, function, constant and struct field needs a doc comment.
* Import groups: standard library, then third party, then `github.com/trancecode/...`, alphabetized within each.
* No em dashes in prose. Sentence case headings, stars for list items.

## Compatibility guarantee

Every change here must be invisible to a game that does not opt in. `TileSize` keeps its default of 16, `SourceTileSize` defaults to zero meaning "no correction", and a sprite that sets neither must produce a byte-identical `GeoM` to today. Several tasks below pin that explicitly.

---

### Task 1: A configurable tile size, and unfreezing the camera

**Files:**
- Modify: `render/render_camera.go` (the `TileSize` declaration, the `Camera` struct, `NewCamera`, `NewScreenCamera`, `EffectiveZoom`)
- Modify: `scene/scene_spriteshowcase.go:46` (the `const` that breaks)
- Test: `render/render_camera_test.go` (append)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `var render.TileSize float64` (was an untyped constant of 16); `Camera` no longer stores a multiplier.

- [ ] **Step 1: Write the failing tests**

Append to `render/render_camera_test.go`:

```go
func TestTileSizeDefaultsTo16(t *testing.T) {
	// A change to the shipped default silently rescales every consuming game,
	// so it must be a deliberate act.
	if TileSize != 16 {
		t.Fatalf("TileSize = %v, want 16", TileSize)
	}
}

func TestEffectiveZoomTracksATileSizeChangedAfterTheCameraWasBuilt(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	c := NewCamera(640, 640)
	before := c.EffectiveZoom()

	TileSize = 32
	after := c.EffectiveZoom()

	if after == before {
		t.Fatal("EffectiveZoom ignored a tile size changed after construction")
	}
	if want := before / 2; after != want {
		t.Fatalf("EffectiveZoom = %v after doubling TileSize, want %v", after, want)
	}
}

func TestEffectiveZoomUnchangedAtTheDefaultTileSize(t *testing.T) {
	// The compatibility guarantee: 640/(20*16) = 2, times a user zoom of 1.
	c := NewCamera(640, 640)
	if got := c.EffectiveZoom(); got != 2 {
		t.Fatalf("EffectiveZoom = %v, want 2", got)
	}
}

func TestScreenCameraIgnoresTileSize(t *testing.T) {
	// A screen-space camera is an identity transform and must not be affected
	// by the world tile size.
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	c := NewScreenCamera(640, 480)
	before := c.EffectiveZoom()

	TileSize = 64
	if after := c.EffectiveZoom(); after != before {
		t.Fatalf("screen camera EffectiveZoom changed with TileSize: %v then %v", before, after)
	}
	if before != 1 {
		t.Fatalf("screen camera EffectiveZoom = %v, want 1", before)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: `TestEffectiveZoomTracksATileSizeChangedAfterTheCameraWasBuilt` fails to build with "cannot assign to TileSize (neither addressable nor a map index expression)", because it is still a constant.

- [ ] **Step 3: Make the tile size a variable**

In `render/render_camera.go`, replace the const block:

```go
const (
	TileSize                 = 16
	defaultVerticalTileCount = 20.0
)
```

with:

```go
// defaultVerticalTileCount is how many tiles a camera shows vertically at a
// user zoom of 1.
const defaultVerticalTileCount = 20.0

// TileSize is how many pixels one world tile occupies before camera zoom. It is
// engine configuration, set through the [render] tile_size setting, and is read
// where it is used rather than captured, so a game that changes it takes effect
// everywhere. Games register sprites from init(), before settings are applied,
// so nothing may freeze this value at construction.
var TileSize float64 = 16
```

- [ ] **Step 4: Stop the camera freezing it**

`NewScreenCamera` needs an identity multiplier that the tile size cannot affect, so the field becomes an override rather than the computed value. In the `Camera` struct, replace:

```go
	screenMultiplier          float64 // Multiplier to normalize zoom across different screen sizes
```

with:

```go
	// fixedScreenMultiplier overrides the tile-derived multiplier when non-zero,
	// which is what makes a screen-space camera an identity transform. Zero means
	// derive it from the screen height and the current TileSize.
	fixedScreenMultiplier float64
```

In `NewCamera`, drop the `screenMultiplier` computation and the field from the literal, leaving `fixedScreenMultiplier` at its zero value. In `NewScreenCamera`, replace `screenMultiplier: 1.0,` with `fixedScreenMultiplier: 1.0,`.

Add the accessor next to `EffectiveZoom`:

```go
// screenMultiplier normalizes zoom across screen sizes, so a camera shows
// defaultVerticalTileCount tiles vertically at a user zoom of 1. It is computed
// on demand rather than stored, so a tile size configured after the camera was
// built still applies.
func (c *Camera) screenMultiplier() float64 {
	if c.fixedScreenMultiplier != 0 {
		return c.fixedScreenMultiplier
	}
	return float64(c.screenHeight) / (defaultVerticalTileCount * TileSize)
}
```

Change `EffectiveZoom` to call it:

```go
func (c *Camera) EffectiveZoom() float64 {
	return c.zoom * c.screenMultiplier()
}
```

Then check the rest of the file for any other reader of the old field and route it through the method. `grep -n "screenMultiplier" render/render_camera.go` must show only the field, the method, and `EffectiveZoom`.

- [ ] **Step 5: Fix the showcase constant**

`scene/scene_spriteshowcase.go:46` declares `showcaseSlotPixels = render.TileSize` inside a `const` block, which no longer compiles. Do not simply move it to a `var`: a slot frozen at load time would ignore a configured tile size, for the same reason the camera must not freeze it.

Remove `showcaseSlotPixels` from the const block, along with its comment, and read the tile size where the slot size is used. The only use is at `scene/scene_spriteshowcase.go:80`, inside `showcaseFitScale`: replace it with `render.TileSize`. Also update the reference in `showcaseFitScale`'s doc comment at line 51, so it says the slot is one tile rather than naming a removed constant.

One test also pins it. `scene/scene_spriteshowcase_test.go:204` has a table entry `{"slot pixels", showcaseSlotPixels, 16.0}` which no longer compiles. Replace that entry so it asserts `render.TileSize` is 16, keeping the intent: the slot is one tile, and one tile is 16 pixels by default. Do not delete the assertion.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, including the four new camera tests and every existing `scene` test. The showcase layout tests pin concrete numbers computed against a 16 pixel tile; they must still pass unchanged, since the default tile size is still 16.

- [ ] **Step 7: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 8: Commit**

```bash
git add render/render_camera.go render/render_camera_test.go scene/scene_spriteshowcase.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Make the tile size configurable and stop the camera freezing it

render.TileSize becomes a variable so a game can choose it. NewCamera used to
compute screenMultiplier once at construction, so a camera built before the tile
size was configured kept scaling by the old value with no error; it is now
computed on demand. A screen-space camera keeps its identity transform through an
explicit override rather than a captured number."
```

---

### Task 2: Source tile size on the sprite

**Files:**
- Modify: `render/render_sprite.go` (the `Sprite` struct, `buildDrawOp`, `VisibleTopAboveZero`, a new setter and helper)
- Test: `render/render_sprite_test.go` (append)

**Interfaces:**
- Consumes: `render.TileSize` as a variable (Task 1).
- Produces: `Sprite.SourceTileSize float64`; `func (s *Sprite) SetSourceTileSize(size float64) *Sprite`; unexported `func (s *Sprite) tileRatio() float64`.

**Use the existing test helpers.** `render/render_sprite_test.go` already has `drawOpTestCamera()` and `geoMEquals(got, want)`; the tests below are written with `NewCamera` and `!=` for brevity, so adapt them to those helpers rather than introducing a second style. Read the top of that file first.

- [ ] **Step 1: Write the failing tests**

Append to `render/render_sprite_test.go`:

```go
func TestTileRatioIsOneWithoutASourceTileSize(t *testing.T) {
	for _, source := range []float64{0, -1, -64} {
		s := NewSprite()
		s.SourceTileSize = source
		if got := s.tileRatio(); got != 1.0 {
			t.Fatalf("tileRatio with SourceTileSize %v = %v, want 1", source, got)
		}
	}
}

func TestTileRatioScalesArtToTheGameTileSize(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	TileSize = 32
	for _, tc := range []struct {
		source float64
		want   float64
	}{
		{32, 1.0},  // drawn for the game's own tile size
		{64, 0.5},  // finer art, drawn down
		{16, 2.0},  // coarser art, drawn up
	} {
		s := NewSprite().SetSourceTileSize(tc.source)
		if got := s.tileRatio(); got != tc.want {
			t.Fatalf("tileRatio for source %v at TileSize 32 = %v, want %v", tc.source, got, tc.want)
		}
	}
}

func TestSetSourceTileSizeChains(t *testing.T) {
	s := NewSprite()
	if got := s.SetSourceTileSize(64); got != s {
		t.Fatal("SetSourceTileSize did not return the sprite")
	}
	if s.SourceTileSize != 64 {
		t.Fatalf("SourceTileSize = %v, want 64", s.SourceTileSize)
	}
}

func TestBuildDrawOpUnchangedWithoutASourceTileSize(t *testing.T) {
	// The compatibility guarantee: a sprite that does not opt in draws exactly
	// as it did before source tile sizes existed.
	c := NewCamera(640, 480)
	s := NewSprite().SetScale(2).SetZeroPosition(geometry.NewVector2(8, 24))

	got := s.buildDrawOp(geometry.NewVector2(3, 4), false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(2, 2)
	want.GeoM.Translate(-8, -24)
	c.Adjust(want, geometry.NewVector2(3, 4))

	if got.GeoM != want.GeoM {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

func TestBuildDrawOpAppliesTheTileRatio(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := NewCamera(640, 480)
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if got.GeoM != want.GeoM {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

func TestBuildDrawOpComposesScaleRatioAndDisplayScale(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := NewCamera(640, 480)
	s := NewSprite().SetScale(3).SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), false, c, 2.0)

	// 3 * 0.5 * 2 = 3
	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(3, 3)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if got.GeoM != want.GeoM {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}

func TestTileRatioScalesTheAnchor(t *testing.T) {
	// The anchored point must land on the same world position whatever the
	// ratio, so the offset scales with it exactly as it does with displayScale.
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := NewCamera(640, 480)
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	want.GeoM.Translate(-0*0.5, -0*0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if got.GeoM != want.GeoM {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}

	// With a real anchor, the translate is the anchor times the ratio.
	s = NewSprite().SetSourceTileSize(64).SetZeroPosition(geometry.NewVector2(10, 20))
	got = s.buildDrawOp(geometry.NewVector2(0, 0), false, c, 1.0)

	want = &ebiten.DrawImageOptions{}
	want.GeoM.Scale(0.5, 0.5)
	want.GeoM.Translate(-10*0.5, -20*0.5)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if got.GeoM != want.GeoM {
		t.Fatalf("anchored GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: build failure in `render` with `undefined: tileRatio` and `SetSourceTileSize`.

- [ ] **Step 3: Add the field, setter and ratio**

In `render/render_sprite.go`, add to the `Sprite` struct after `Scale`:

```go
	// SourceTileSize is the tile size this sprite's art was drawn for. Zero
	// means it was drawn for whatever the game's tile size is, so no correction
	// applies. It is authorial metadata and is never inferred from the image: a
	// 64 pixel image drawn for 128 pixel tiles and a 64 pixel image drawn for 32
	// pixel tiles are different sprites that happen to share a file size.
	SourceTileSize float64
```

Add the setter beside the other chainable setters:

```go
// SetSourceTileSize sets the tile size this sprite's art was drawn for and
// returns the sprite, so it chains with the other setters.
func (s *Sprite) SetSourceTileSize(size float64) *Sprite {
	s.SourceTileSize = size
	return s
}
```

And the ratio:

```go
// tileRatio is the scale that maps art drawn for SourceTileSize onto the game's
// current TileSize. It is computed on every call rather than stored, because
// sprites are built from init() before settings are applied.
func (s *Sprite) tileRatio() float64 {
	if s.SourceTileSize <= 0 {
		return 1.0
	}
	return TileSize / s.SourceTileSize
}
```

- [ ] **Step 4: Fold the ratio into the draw**

Replace the two transform lines in `buildDrawOp`:

```go
	op.GeoM.Scale(s.Scale*displayScale, s.Scale*displayScale)
	op.GeoM.Translate(-s.ZeroPosition.X()*displayScale, -s.ZeroPosition.Y()*displayScale)
```

with:

```go
	// The tile ratio is an engine-applied uniform scale about the anchor, so it
	// multiplies in exactly where displayScale does. ZeroPosition keeps its
	// existing meaning, in Scale-applied pixels, which is why the translate does
	// not include Scale.
	ratio := s.tileRatio()
	scale := s.Scale * ratio * displayScale
	op.GeoM.Scale(scale, scale)
	anchor := ratio * displayScale
	op.GeoM.Translate(-s.ZeroPosition.X()*anchor, -s.ZeroPosition.Y()*anchor)
```

Update `buildDrawOp`'s doc comment to mention the tile ratio alongside the sprite scale.

- [ ] **Step 5: Account for the ratio in the visible extent**

`VisibleTopAboveZero` reports drawn pixels and currently accounts for `Scale` only, so a nameplate on a rescaled sprite would sit at the wrong height.

Do NOT clear caches in the setter, and do NOT cache a ratio-applied value. Cache the pre-ratio result exactly as now, and multiply by the ratio on return. That way a tile size configured after first use is picked up too, and no cache invalidation is needed anywhere:

```go
func (s *Sprite) VisibleTopAboveZero() float64 {
	if s.cachedVisibleTopAboveZero != nil {
		return *s.cachedVisibleTopAboveZero * s.tileRatio()
	}
	// ... existing computation unchanged, storing the pre-ratio value in the
	// cache ...
	return result * s.tileRatio()
}
```

Adapt that shape to the function's real structure: the rule is that the cached value stays pre-ratio and every return path multiplies by `s.tileRatio()`.

Update the doc comment to say the result is in drawn pixels including the tile ratio, and that the cache holds the pre-ratio value.

Leave `VisibleBounds` alone. It is documented as frame-local pixel coordinates, which is source pixels, and the ratio does not apply.

- [ ] **Step 6: Add a test for the visible extent**

`VisibleTopAboveZero` scans pixels with `img.At`, and this package's tests cannot read ebiten pixels back, since nothing runs an `ebiten.RunGame` loop. The existing file says so explicitly in the comment above `TestDrawAnimationGeometryIsUnchangedByTheDisplayScale`. Calling it on a real image from a test would panic.

Seed the cache instead. The test is in package `render`, so it can set the unexported cache field directly, which exercises the ratio arithmetic on the return path without scanning anything:

```go
func TestVisibleTopAboveZeroScalesWithTheTileRatio(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	// Seeded rather than measured: VisibleTopAboveZero scans pixels with
	// img.At, which cannot run outside an ebiten.RunGame loop. The cache holds
	// the pre-ratio value, so seeding it is exactly what a prior measurement
	// would have left behind.
	cached := 12.0
	s := NewSprite()
	s.cachedVisibleTopAboveZero = &cached

	if got := s.VisibleTopAboveZero(); got != cached {
		t.Fatalf("VisibleTopAboveZero = %v without a source tile size, want %v", got, cached)
	}

	s.SetSourceTileSize(32)
	TileSize = 16 // ratio 0.5

	if got, want := s.VisibleTopAboveZero(), cached/2; got != want {
		t.Fatalf("VisibleTopAboveZero = %v at ratio 0.5, want %v", got, want)
	}
}
```

If the field name differs from `cachedVisibleTopAboveZero`, use the real one; do not rename it.

- [ ] **Step 7: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, all packages.

- [ ] **Step 8: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 9: Commit**

```bash
git add render/render_sprite.go render/render_sprite_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Let a sprite declare the tile size its art was drawn for

Sprite.SourceTileSize states what the art was authored against, and the engine
applies TileSize/SourceTileSize at draw time. Unlike Scale, which is a bare
multiplier with no reference point, this stays correct when the game's tile size
changes. Zero means no correction, so existing sprites are untouched.

The ratio is an engine-applied uniform scale about the anchor, so it multiplies
in where displayScale already does, and VisibleTopAboveZero caches its pre-ratio
value so no cache needs invalidating."
```

---

### Task 3: The tile size setting

**Files:**
- Modify: `app/settings.go` (`RenderSettings`, `RegisterFlags`, `Apply`, a new `validate`)
- Modify: `app/settings.toml`
- Modify: `app/app.go` (call `validate` from `Run`)
- Test: `app/settings_test.go` (append)

**Interfaces:**
- Consumes: `render.TileSize` as a variable (Task 1).
- Produces: `RenderSettings.TileSize float64`; unexported `func (s *Settings) validate() error`.

- [ ] **Step 1: Write the failing tests**

Append to `app/settings_test.go`:

```go
func TestLoadSettingsTileSizeDefaultsTo16(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Render.TileSize != 16 {
		t.Fatalf("render.tile_size default = %v, want 16", s.Render.TileSize)
	}
}

func TestTileSizeOverrideAndFlag(t *testing.T) {
	s, err := LoadSettings("", []string{"render.tile_size=32"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Render.TileSize != 32 {
		t.Fatalf("render.tile_size = %v, want 32", s.Render.TileSize)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	s.RegisterFlags(fs)
	if err := fs.Parse([]string{"--tile_size=64"}); err != nil {
		t.Fatal(err)
	}
	if s.Render.TileSize != 64 {
		t.Fatalf("flag did not override render.tile_size: %v", s.Render.TileSize)
	}
}

func TestValidateRejectsANonPositiveTileSize(t *testing.T) {
	for _, size := range []float64{0, -16} {
		s, err := LoadSettings("", nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Render.TileSize = size
		err = s.validate()
		if err == nil {
			t.Fatalf("validate accepted a tile size of %v", size)
		}
		if !strings.Contains(err.Error(), "tile_size") {
			t.Fatalf("error does not name the setting: %v", err)
		}
	}
}

func TestValidateAcceptsThePositiveDefault(t *testing.T) {
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.validate(); err != nil {
		t.Fatalf("validate rejected the default settings: %v", err)
	}
}

func TestApplySetsTileSize(t *testing.T) {
	original := render.TileSize
	t.Cleanup(func() { render.TileSize = original })

	s, err := LoadSettings("", []string{"render.tile_size=32"})
	if err != nil {
		t.Fatal(err)
	}
	s.Apply()
	if render.TileSize != 32 {
		t.Fatalf("Apply did not set render.TileSize: %v", render.TileSize)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: build failure in `app` with `s.Render.TileSize undefined` and `s.validate undefined`.

- [ ] **Step 3: Add the setting**

In `app/settings.go`, add to `RenderSettings`:

```go
	TileSize float64 `toml:"tile_size"`
```

Register the flag in `RegisterFlags`, next to the existing render flag:

```go
	fs.Float64Var(&s.Render.TileSize, "tile_size", s.Render.TileSize,
		"Pixels per world tile before camera zoom")
```

Add the validation. A plain number needs a range check rather than the `UnmarshalText` treatment `FilterSetting` gets, and `App.Run` already returns startup errors for bad scene names:

```go
// validate reports a setting whose value cannot be used. Call it before Apply,
// so a bad value is a startup error rather than a global set to nonsense.
func (s *Settings) validate() error {
	if s.Render.TileSize <= 0 {
		return fmt.Errorf("render.tile_size must be positive, got %v", s.Render.TileSize)
	}
	return nil
}
```

Set the global in `Apply`, beside the existing two:

```go
	render.TileSize = s.Render.TileSize
```

In `app/settings.toml`, add to the `[render]` section:

```toml
tile_size = 16
```

- [ ] **Step 4: Call validate from Run**

In `app/app.go`, `Run` currently opens with `a.settings.Apply()`. Put validation first, so a bad value never reaches a global:

```go
func (a *App) Run() error {
	if err := a.settings.validate(); err != nil {
		return err
	}
	a.settings.Apply()
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task test:headless
```

Expected: PASS, all packages. Existing `app` tests that call `LoadSettings` must be unaffected, since the default is still 16.

- [ ] **Step 6: Run lint and vet**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && go vet ./...
```

- [ ] **Step 7: Commit**

```bash
git add app/settings.go app/settings.toml app/app.go app/settings_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add the render tile size setting

[render] tile_size and --tile_size choose how many pixels a world tile occupies,
defaulting to the 16 the engine used to hardcode. A non-positive value is a
startup error rather than a global set to nonsense, checked before Apply."
```

---

### Task 4: Documentation

**Files:**
- Modify: `render/doc.go`
- Modify: `ARCHITECTURE.md` (the `render` row)
- Modify: `docs/performance_optimization.md` (review only, per `CLAUDE.md`)

**Interfaces:**
- Consumes: everything from Tasks 1 to 3. Produces no code.

- [ ] **Step 1: Update the package doc**

In `render/doc.go`, the existing sentence reads "TileSize (16px) defines the base tile dimension used across the rendering pipeline." That is no longer accurate. Replace it with wording covering both halves of this change: that the tile size is engine configuration with a default of 16, and that a sprite may declare the tile size its art was drawn for, which the engine corrects for at draw time.

- [ ] **Step 2: Update ARCHITECTURE.md**

The `render` row currently reads:

```
| `render` | Graphics layer: camera, sprites, sprite library, text | `asset`, `geometry` |
```

Extend the description to mention tile scaling. Do not change the dependency column; this task adds no imports.

- [ ] **Step 3: Review the performance log**

`CLAUDE.md` requires reviewing `docs/performance_optimization.md` for entries made irrelevant by a change, and recording new opportunities rather than acting on them.

Read it. `tileRatio` is now computed per draw call rather than stored, and `screenMultiplier` likewise, both of which are divisions on a hot path that were previously constant folded or cached. Record that as an entry, noting it was a deliberate correctness choice: caching either one reintroduces the freezing bug this change exists to remove. State plainly in your report whether any existing entry became irrelevant, and change nothing else if none did.

- [ ] **Step 4: Verify the build is still clean**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

- [ ] **Step 5: Commit**

```bash
git add render/doc.go ARCHITECTURE.md docs/performance_optimization.md
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Document the configurable tile size and source tile size

Also record the per-draw cost of computing the tile ratio and the screen
multiplier, which is deliberate: caching either reintroduces the freezing this
change removes."
```

---

## Self-review notes

Spec coverage:

* Configurable `render.TileSize`, the variable-not-accessor decision, and the showcase `const` it forces: Task 1.
* The camera capture fix and its two regression tests: Task 1.
* `Sprite.SourceTileSize`, the chainable setter, `tileRatio`, and folding it into `buildDrawOp`: Task 2.
* The visible-extent helpers: Task 2.
* The settings surface and startup validation: Task 3.
* Documentation: Task 4.

Two deliberate deviations from the spec, both narrowing work rather than adding it:

* The spec says `SetSourceTileSize` clears the visible-extent caches. Task 2 instead caches the pre-ratio value and applies the ratio on return, which needs no invalidation and additionally survives a tile size changed after first use. The spec's approach would leave a stale cache in exactly that case.
* The spec leaves open whether `VisibleBounds` needs the ratio. It does not: it is documented as frame-local pixel coordinates, which is source pixels.

The compatibility guarantee is pinned in three places: `TestTileSizeDefaultsTo16`, `TestEffectiveZoomUnchangedAtTheDefaultTileSize`, and `TestBuildDrawOpUnchangedWithoutASourceTileSize`.
