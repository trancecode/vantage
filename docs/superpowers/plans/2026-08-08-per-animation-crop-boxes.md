# Per-animation crop boxes and anchors implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give each animation in a `render.Sprite` its own frame geometry and its
own anchor, and add a loader that crops a uniform sheet down to that geometry
before upload, so a sheet costs video memory in proportion to what it draws.

**Architecture:** The anchor moves from `Sprite` onto `Animation`, and
`Sprite.Scale` is deleted because it is what forces the anchor into a second unit
convention. A new `AnimationSpec` descriptor expresses per-animation rectangles,
anchor and duration; `LoadSpriteAnimations` consumes it and `LoadSprite` becomes a
thin wrapper that expands its uniform grid into one. `LoadSpriteAutoCropped` scans
a CPU-side image for per-animation alpha bounding boxes, repacks the frames into a
smaller atlas, rebases the anchors, and hands the result to
`LoadSpriteAnimations`.

**Tech Stack:** Go, Ebitengine v2.9.9, `image`/`image/draw` from the standard
library, Taskfile for lint and test targets.

**Design spec:** `docs/superpowers/specs/2026-08-08-per-animation-crop-boxes-design.md`

## Global constraints

* Set `export GOMODCACHE=/tmp/go-mod-cache` in every shell; the default module
  cache is not writable.
* On a fresh checkout run `task install:tools` once. `task lint` fails hard rather
  than skipping when staticcheck or golangci-lint is missing.
* Verification for every task: `task lint`, `task test:headless`, `go vet ./...`.
  Never bare `go test`: the `render`, `ui` and `scene` packages initialise
  Ebitengine and GLFW from package-level `init`, which needs a display.
* Never run a command that opens a window. Any `go run ./cmd/...` is wrapped in
  `xvfb-run -a` with explicit `--width`, `--height`, `--screenshot_path`,
  `--screenshot_delay` and `--run_for`.
* This is an interactive session, so commit directly to `main`. Do not create
  branches or pull requests. Commit author is already configured as
  `Claude Code <herve.quiroz+claude@gmail.com>`; never add a `Co-Authored-By:`
  line.
* Vantage's exported API is not a stability commitment. Prefer the cleaner model
  and migrate the consumers, per the "API Compatibility Is Not a Constraint"
  section of `CLAUDE.md`.
* `*ebiten.Image` pixels cannot be read back outside a running `ebiten.RunGame`
  loop. Tests that need pixel values must either use a CPU-side `image.Image` or
  seed the cache the way `render/render_sprite_test.go:322` does. Do not write a
  test that calls `At` on an `*ebiten.Image` and asserts on the result; it will
  not work.
* Documentation style: sentence case headings, `*` for list bullets, no em
  dashes, noun phrases for section titles.

---

### Task 1: Remove Sprite.Scale

Deleting `Scale` first means every later task works in one unit convention.
`ZeroPosition` stops being measured in Scale-applied pixels and becomes plain
source pixels, which is what makes the auto-crop anchor rebase exact.

**Files:**
- Modify: `render/render_sprite.go` (`Sprite` struct, `buildDrawOp`,
  `VisibleTopAboveZero`, `SetScale`, `NewSprite`)
- Modify: `scene/scene_spriteshowcase.go:89`
- Modify: `cmd/showcasedemo/main.go` (`demoSprite`, `registerSprites`, package doc)
- Test: `render/render_sprite_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `buildDrawOp(p geometry.Vector2, requiresFlip bool, c *Camera,
  displayScale float64) *ebiten.DrawImageOptions` with signature unchanged but
  `s.Scale` gone from its body. `Sprite.ZeroPosition` still exists and is now in
  source pixels. `Sprite.Scale` and `Sprite.SetScale` no longer exist.

- [ ] **Step 1: Update the tests to the new expectation**

In `render/render_sprite_test.go`, five tests seed `SetScale`. Rewrite them so
they no longer reference it.

`TestDrawAnimationGeometryIsUnchangedByTheDisplayScale` (line 59):

```go
func TestDrawAnimationGeometryIsUnchangedByTheDisplayScale(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	s := NewSprite().SetZeroPosition(geometry.NewVector2(8, 24))

	for _, requiresFlip := range []bool{false, true} {
		// The pre-display-scale transform, rebuilt here independently.
		want := &ebiten.DrawImageOptions{}
		want.GeoM.Translate(-s.ZeroPosition.X(), -s.ZeroPosition.Y())
		if requiresFlip {
			want.GeoM.Scale(-1, 1)
		}
		c.Adjust(want, p)

		got := s.buildDrawOp(p, requiresFlip, c, 1.0)
		if !geoMEquals(got, want) {
			t.Fatalf("flip=%v: buildDrawOp at display scale 1 = %v, want %v", requiresFlip, got.GeoM, want.GeoM)
		}
	}
}
```

`TestDisplayScaleShrinksAboutTheZeroPosition` (line 93): drop `const scale`, drop
both `SetScale(scale)` calls, and replace the anchored-source-pixel comment and
computation. The anchored source pixel is now the anchor itself:

```go
	for _, tc := range []struct {
		name   string
		sprite *Sprite
	}{
		{
			"no source tile size",
			NewSprite().SetZeroPosition(zero),
		},
		{
			"source tile size 64 at tile size 32, ratio 0.5",
			NewSprite().SetZeroPosition(zero).SetSourceTileSize(64),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := drawOpTestCamera()
			p := geometry.NewVector2(3, 5)
			s := tc.sprite

			// ZeroPosition is in source pixels, so it is the anchored pixel.
			anchorSourceX, anchorSourceY := zero.X(), zero.Y()
```

The rest of that test body is unchanged.

`TestBuildDrawOpUnchangedWithoutASourceTileSize` (line 222) loses its scale:

```go
func TestBuildDrawOpUnchangedWithoutASourceTileSize(t *testing.T) {
	c := drawOpTestCamera()
	s := NewSprite().SetZeroPosition(geometry.NewVector2(8, 24))

	got := s.buildDrawOp(geometry.NewVector2(3, 4), false, c, 1.0)

	want := &ebiten.DrawImageOptions{}
	want.GeoM.Translate(-8, -24)
	c.Adjust(want, geometry.NewVector2(3, 4))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}
```

`TestBuildDrawOpComposesScaleRatioAndDisplayScale` (line 262) becomes a
two-factor test. Rename it and update its comment:

```go
// TestBuildDrawOpComposesRatioAndDisplayScale covers that the tile ratio and the
// display scale multiply into one uniform scale rather than only one of them
// taking effect.
func TestBuildDrawOpComposesRatioAndDisplayScale(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })
	TileSize = 32

	c := drawOpTestCamera()
	s := NewSprite().SetSourceTileSize(64) // ratio 0.5

	got := s.buildDrawOp(geometry.NewVector2(0, 0), false, c, 2.0)

	// 0.5 * 2 = 1
	want := &ebiten.DrawImageOptions{}
	want.GeoM.Scale(1, 1)
	c.Adjust(want, geometry.NewVector2(0, 0))

	if !geoMEquals(got, want) {
		t.Fatalf("GeoM = %v, want %v", got.GeoM, want.GeoM)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ 2>&1 | head -20
```

Expected: compile failure, `s.Scale undefined` is not yet true, so instead expect
`TestBuildDrawOpUnchangedWithoutASourceTileSize` and its neighbours to FAIL on
mismatched geometry, because the implementation still multiplies by `Scale` of 1
while the tests no longer set 2 or 3.

- [ ] **Step 3: Delete Scale from the sprite**

In `render/render_sprite.go`, remove the `Scale float64` field from `Sprite`,
remove `Scale: 1.0` from `NewSprite`, and delete `SetScale` entirely.

Update `buildDrawOp`:

```go
// buildDrawOp builds the draw options for the sprite at world-tile position p:
// the tile ratio combined with displayScale, the zero-position offset, an
// optional horizontal flip, then the camera transform.
//
// The scale and the anchor offset use the same factor, so the transform is a
// uniform scale about the zero position: the pixel anchored at the world
// position p stays at p, and only the drawn extent changes. A displayScale of 1
// draws the sprite at the size the game draws it.
func (s *Sprite) buildDrawOp(p geometry.Vector2, requiresFlip bool, c *Camera, displayScale float64) *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.Filter = SpriteFilter
	scale := s.TileRatio() * displayScale
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(-s.ZeroPosition.X()*scale, -s.ZeroPosition.Y()*scale)
	if requiresFlip {
		op.GeoM.Scale(-1, 1)
	}
	c.Adjust(op, p)
	return op
}
```

Update `VisibleTopAboveZero`'s row arithmetic at line 308, which currently
reconciles the two conventions:

```go
				// (y - bounds.Min.Y) is the row index within the frame. Both it
				// and ZeroPosition are in source pixels, so the first visible
				// pixel sits ZeroPosition.Y() - rowIndex source pixels above
				// ZeroPosition.
				result = s.ZeroPosition.Y() - float64(y-bounds.Min.Y)
```

Also update its doc comment: replace "the frame is scaled by Scale before the
zero-position offset is applied (see buildDrawOp), so the row offset is
multiplied by Scale to match what ends up on screen" with "the row offset and
ZeroPosition are both in source pixels, and the tile ratio maps them to what ends
up on screen".

Update `DrawAnimationScaled`'s doc comment to drop "multiplies Sprite.Scale for
this call only; SetScale would change the sprite everywhere it is drawn", leaving
the display scale described on its own terms.

- [ ] **Step 4: Update the showcase scene**

`scene/scene_spriteshowcase.go:89`:

```go
	drawnScale := sprite.TileRatio()
```

Update the doc comment above `showcaseFitScale` (lines 68 to 70), replacing "The
drawn size is the raw frame dimension multiplied by the sprite's own Scale and by
its render.Sprite.TileRatio" with "The drawn size is the raw frame dimension
multiplied by its render.Sprite.TileRatio".

- [ ] **Step 5: Update showcasedemo**

In `cmd/showcasedemo/main.go`, `demoSprite` drops its scale parameter:

```go
// demoSprite builds a sprite whose frames are all size pixels square, with one
// animation per requested type.
func demoSprite(size int, fill color.RGBA, animations ...render.AnimationType) *render.Sprite {
	s := render.NewSprite()
	for i, a := range animations {
		for f := range framesPerAnimation {
			s.AddImage(a, frame(size, shaded(fill, 0.45+0.15*float64((i+f)%framesPerAnimation))))
		}
	}
	return s
}
```

In `registerSprites`, drop the scale argument from every call and convert the one
scaled sprite to a source tile size. `GiantScaled16x4` was 16 pixel art at scale
4, drawn 64 pixels wide; a source tile size of 4 gives a tile ratio of 16/4 = 4 at
the default tile size, so it draws identically and now tracks the configured tile
size:

```go
	// Several animations each: one row per sprite, one column per animation.
	render.Sprites.Add("HeroNormal16", demoSprite(16, red,
		render.AnimationIdleDown, render.AnimationIdleRight,
		render.AnimationMoveDown, render.AnimationMoveRight))
	render.Sprites.Add("BossHuge64", demoSprite(64, blue,
		render.AnimationIdleDown, render.AnimationMoveDown, render.AnimationAttackDown))
	// 16 pixel art drawn for 4 pixel tiles, so it covers four tiles.
	render.Sprites.Add("GiantSourceTile16x4", demoSprite(16, amber,
		render.AnimationIdleDown, render.AnimationMoveDown).SetSourceTileSize(4))

	// A 64 pixel frame declaring the tile size it was drawn for, so it is two
	// tiles at the default tile size, with directional animations carrying
	// registered names instead of the generated AnimationType placeholder.
	render.Sprites.Add("DirectionalTiles64x2", demoSprite(64, purple,
		animationSouth, animationEast, animationNorth, animationWest,
	).SetSourceTileSize(32))

	// One animation each: packed ten to a row, so twelve of them wrap.
	for i, size := range []int{16, 16, 32, 8, 16, 64, 16, 24, 16, 48, 16, 16} {
		render.Sprites.Add(
			fmt.Sprintf("Tile%02d_%dpx", i+1, size),
			demoSprite(size, green, render.AnimationDefault),
		)
	}
```

In the package doc comment at the top of the file, replace "one uses Sprite.Scale
rather than large frames" with "one declares a source tile size smaller than a
tile rather than using large frames".

- [ ] **Step 6: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass. If `grep -rn "SetScale\|\.Scale\b" --include=*.go .` finds any
remaining `Sprite.Scale` use outside `GeoM.Scale` and `Vector2.Scale`, fix it.

- [ ] **Step 7: Screenshot the showcase to confirm nothing moved**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go run ./cmd/showcasedemo \
    --width 800 --height 600 \
    --screenshot_path /tmp/showcase-task1.png \
    --screenshot_delay 500ms \
    --run_for 1000ms
```

Read `/tmp/showcase-task1.png`. Expected: the grid renders, and the sprite
formerly named `GiantScaled16x4` still fills its slot at the same size.

- [ ] **Step 8: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Remove the sprite scale in favour of the source tile size

Sprite.Scale is a bare multiplier with no reference point, which SourceTileSize
already replaced: any fixed multiplier k is a source tile size of TileSize/k, and
that version tracks the configured tile size instead of going stale.

It is also why ZeroPosition is measured in Scale-applied pixels while everything
else is in source pixels. With Scale gone the scale and the anchor offset use one
factor, so the two agree by construction.
EOF
)"
```

---

### Task 2: Move the anchor onto the animation

**Files:**
- Modify: `render/render_sprite.go` (`Animation`, `Sprite`, `NewSprite`,
  `AddImage`, `SetZeroPosition`, `buildDrawOp`, `VisibleTopAboveZero`,
  `LoadSprite`)
- Test: `render/render_sprite_test.go`

**Interfaces:**
- Consumes: `buildDrawOp` from Task 1.
- Produces: `Animation.ZeroPosition geometry.Vector2`;
  `func (s *Sprite) Anchor(a AnimationType) geometry.Vector2`;
  `buildDrawOp(p geometry.Vector2, a AnimationType, requiresFlip bool, c *Camera,
  displayScale float64) *ebiten.DrawImageOptions`. `Sprite.ZeroPosition` no
  longer exists. `SetZeroPosition` panics on a sprite with no animations.

- [ ] **Step 1: Write the failing test**

Add to `render/render_sprite_test.go`:

```go
// TestAnchorIsPerAnimation covers the core of per-animation geometry: two
// animations on one sprite carry different anchors, and each is used for its
// own animation rather than one of them winning for both.
func TestAnchorIsPerAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))
	s.Animations[AnimationIdleDown].ZeroPosition = geometry.NewVector2(4, 8)
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(10, 2)

	if got, want := s.Anchor(AnimationIdleDown), geometry.NewVector2(4, 8); got != want {
		t.Fatalf("Anchor(IdleDown) = %v, want %v", got, want)
	}
	if got, want := s.Anchor(AnimationIdleRight), geometry.NewVector2(10, 2); got != want {
		t.Fatalf("Anchor(IdleRight) = %v, want %v", got, want)
	}
}

// TestAnchorResolvesAMirroredAnimation covers that a left-facing animation drawn
// by flipping its right-facing mirror uses the mirror's anchor, since those are
// the frames on screen.
func TestAnchorResolvesAMirroredAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(10, 2)

	if got, want := s.Anchor(AnimationIdleLeft), geometry.NewVector2(10, 2); got != want {
		t.Fatalf("Anchor(IdleLeft) = %v, want %v", got, want)
	}
}

// TestAnchorOfAnUnknownAnimationIsZero covers that Anchor is a query and returns
// the zero vector rather than panicking, unlike the draw path.
func TestAnchorOfAnUnknownAnimationIsZero(t *testing.T) {
	if got := NewSprite().Anchor(AnimationIdleDown); !got.IsZero() {
		t.Fatalf("Anchor of an unknown animation = %v, want the zero vector", got)
	}
}

// TestSetZeroPositionWritesEveryAnimation covers the uniform-sheet convenience:
// one call sets the anchor on all animations currently on the sprite.
func TestSetZeroPositionWritesEveryAnimation(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(16, 16))

	zero := geometry.NewVector2(8, 24)
	if got := s.SetZeroPosition(zero); got != s {
		t.Fatal("SetZeroPosition did not return the sprite")
	}
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		if got := s.Anchor(a); got != zero {
			t.Fatalf("Anchor(%v) = %v, want %v", a, got, zero)
		}
	}
}

// TestSetZeroPositionPanicsWithoutAnimations covers the ordering hazard left by
// deleting the sprite-level anchor: a misordered call is loud rather than
// silently writing the anchor to nothing.
func TestSetZeroPositionPanicsWithoutAnimations(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("SetZeroPosition on a sprite with no animations did not panic")
		}
	}()
	NewSprite().SetZeroPosition(geometry.NewVector2(1, 2))
}

// TestBuildDrawOpUsesTheAnimationAnchor covers that the draw path reads the
// anchor of the animation being drawn. Two animations with different anchors,
// drawn at one world position, must both land their own anchor pixel on that
// position: this is the regression that would otherwise appear in game as a
// character bobbing between the ground and mid-air as its animation changes.
func TestBuildDrawOpUsesTheAnimationAnchor(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)

	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	s.AddImage(AnimationIdleRight, ebiten.NewImage(64, 64))
	s.Animations[AnimationIdleDown].ZeroPosition = geometry.NewVector2(8, 16)
	s.Animations[AnimationIdleRight].ZeroPosition = geometry.NewVector2(32, 60)

	want := c.WorldToScreen(p)
	const eps = 1e-9
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		anchor := s.Anchor(a)
		op := s.buildDrawOp(p, a, false, c, 1.0)
		gotX, gotY := op.GeoM.Apply(anchor.X(), anchor.Y())
		if diff := gotX - want.X(); diff > eps || diff < -eps {
			t.Errorf("animation %v: anchor X = %v, want %v", a, gotX, want.X())
		}
		if diff := gotY - want.Y(); diff > eps || diff < -eps {
			t.Errorf("animation %v: anchor Y = %v, want %v", a, gotY, want.Y())
		}
	}
}
```

Every existing call to `buildDrawOp` in the test files also needs the new
argument. In `render/render_sprite_test.go` pass `AnimationDefault`; in
`render/render_filter_test.go` (lines 66 and 71) pass `AnimationDefault` as well.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ 2>&1 | head -20
```

Expected: compile failure, `s.Anchor undefined` and `too many arguments in call
to s.buildDrawOp`.

- [ ] **Step 3: Implement the per-animation anchor**

In `render/render_sprite.go`:

```go
// Animation represents a sequence of images forming an animation.
type Animation struct {
	Images   []*ebiten.Image
	Duration time.Duration

	// ZeroPosition is this animation's anchor: the pixel inside its frames that
	// sits on the drawn world position, in its own frames' local pixels. Frames
	// of different animations need not share a size or an anchor, which is what
	// lets a sheet be cropped per animation.
	ZeroPosition geometry.Vector2
}
```

Remove the `ZeroPosition geometry.Vector2` field from `Sprite`, and remove
`ZeroPosition: geometry.Zero2D()` from `NewSprite`.

Add the accessor and rewrite the setter:

```go
// Anchor returns the anchor of the given animation, resolving a mirrored
// animation to the animation it is drawn from, since those are the frames that
// end up on screen. An animation the sprite does not have returns the zero
// vector: this is a query, not a draw.
func (s *Sprite) Anchor(a AnimationType) geometry.Vector2 {
	if animation, ok := s.Animations[a]; ok {
		return animation.ZeroPosition
	}
	if other, mirrored := MirroredAnimations[a]; mirrored {
		if animation, ok := s.Animations[other]; ok {
			return animation.ZeroPosition
		}
	}
	return geometry.Zero2D()
}

// SetZeroPosition sets one anchor on every animation the sprite currently has,
// which is the right thing for a uniform sheet where every frame shares an
// anchor. Animations added afterwards do not get it, so it panics on a sprite
// with no animations: that call is misordered rather than merely early.
func (s *Sprite) SetZeroPosition(pos geometry.Vector2) *Sprite {
	if len(s.Animations) == 0 {
		panic("SetZeroPosition on a sprite with no animations: add frames first")
	}
	for _, animation := range s.Animations {
		animation.ZeroPosition = pos
	}
	return s
}
```

Update `buildDrawOp` to take the animation and read its anchor. `Anchor` already
resolves mirrors, so callers pass the animation they were asked for:

```go
func (s *Sprite) buildDrawOp(p geometry.Vector2, a AnimationType, requiresFlip bool, c *Camera, displayScale float64) *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.Filter = SpriteFilter
	scale := s.TileRatio() * displayScale
	op.GeoM.Scale(scale, scale)
	anchor := s.Anchor(a)
	op.GeoM.Translate(-anchor.X()*scale, -anchor.Y()*scale)
	if requiresFlip {
		op.GeoM.Scale(-1, 1)
	}
	c.Adjust(op, p)
	return op
}
```

Update the two call sites: `Draw` (line 128) becomes
`op := s.buildDrawOp(p, a, requiresFlip, c, 1.0)` and `DrawAnimationScaled`
(line 205) becomes `op := s.buildDrawOp(p, a, requiresFlip, c, displayScale)`.

In `VisibleTopAboveZero`, replace `s.ZeroPosition.Y()` with the anchor of
whichever animation supplied the frame. For this task, keep the existing
"any available frame" behaviour and use `geometry.Zero2D()` semantics by reading
the anchor from the same animation the loop picked:

```go
	var img *ebiten.Image
	anchorY := 0.0
	for a, anim := range s.Animations {
		if len(anim.Images) > 0 {
			img = anim.Images[0]
			anchorY = s.Anchor(a).Y()
			break
		}
	}
```

and use `anchorY` in place of `s.ZeroPosition.Y()` at the assignment. Task 3
replaces this whole arbitrary-frame scheme; this step only keeps it compiling and
self-consistent.

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Move the sprite anchor onto the animation

Once animations can carry different crop sizes, a single sprite-level anchor
cannot place them all, and getting it wrong shows up as a character jumping
between the ground and mid-air as its animation changes.

Animation gains ZeroPosition and Sprite loses it, so there is one place an anchor
lives and no fallback to keep consistent. SetZeroPosition stays as the
uniform-sheet convenience, writing one anchor to every animation, and panics on a
sprite with no animations rather than silently writing to nothing.
EOF
)"
```

---

### Task 3: Scope the visible-extent queries to an animation

**Files:**
- Modify: `render/render_sprite.go` (`Sprite` caches, `VisibleBounds`,
  `VisibleTopAboveZero`)
- Modify: `render/render_overlay.go` (`DrawNameplate`, `DrawFloatingBar`)
- Test: `render/render_sprite_test.go`

**Interfaces:**
- Consumes: `Sprite.Anchor` from Task 2.
- Produces: `func (s *Sprite) VisibleBounds(a AnimationType) image.Rectangle`;
  `func (s *Sprite) VisibleTopAboveZero(a AnimationType) float64`;
  `func DrawNameplate(screen *ebiten.Image, camera *Camera, worldPos
  geometry.Vector2, sprite *Sprite, a AnimationType, writer *TextWriter,
  gapPixels float64)`; `func DrawFloatingBar(screen *ebiten.Image, camera
  *Camera, worldPos geometry.Vector2, sprite *Sprite, a AnimationType, fraction
  float64, style FloatingBarStyle)`.

- [ ] **Step 1: Write the failing test**

Replace `TestVisibleTopAboveZeroScalesWithTheTileRatio` in
`render/render_sprite_test.go` and add a per-animation test beside it. Both seed
the cache rather than measuring, because `*ebiten.Image` pixels cannot be read
outside a `RunGame` loop:

```go
// TestVisibleTopAboveZeroScalesWithTheTileRatio covers that the tile ratio is
// applied on every return path from the cache, so a tile size change after the
// sprite's visible extent was first measured is still picked up.
//
// Seeded rather than measured: VisibleTopAboveZero scans pixels with img.At,
// which cannot run outside an ebiten.RunGame loop. The cache holds the
// pre-ratio value, so seeding it is exactly what a prior measurement would have
// left behind.
func TestVisibleTopAboveZeroScalesWithTheTileRatio(t *testing.T) {
	original := TileSize
	t.Cleanup(func() { TileSize = original })

	s := NewSprite()
	s.cachedVisibleTopAboveZero[AnimationIdleDown] = 12.0

	if got := s.VisibleTopAboveZero(AnimationIdleDown); got != 12.0 {
		t.Fatalf("VisibleTopAboveZero = %v without a source tile size, want 12", got)
	}

	s.SetSourceTileSize(32)
	TileSize = 16 // ratio 0.5

	if got, want := s.VisibleTopAboveZero(AnimationIdleDown), 6.0; got != want {
		t.Fatalf("VisibleTopAboveZero = %v at ratio 0.5, want %v", got, want)
	}
}

// TestVisibleExtentIsPerAnimation covers that each animation keeps its own
// measurement, so a sheet whose animations have different heights does not
// return one animation's answer for all of them. Without this a nameplate would
// drift vertically as a character changed animation.
func TestVisibleExtentIsPerAnimation(t *testing.T) {
	s := NewSprite()
	s.cachedVisibleTopAboveZero[AnimationIdleDown] = 12.0
	s.cachedVisibleTopAboveZero[AnimationIdleRight] = 40.0
	s.cachedVisibleBounds[AnimationIdleDown] = image.Rect(0, 0, 8, 12)
	s.cachedVisibleBounds[AnimationIdleRight] = image.Rect(0, 0, 32, 40)

	if got, want := s.VisibleTopAboveZero(AnimationIdleDown), 12.0; got != want {
		t.Fatalf("VisibleTopAboveZero(IdleDown) = %v, want %v", got, want)
	}
	if got, want := s.VisibleTopAboveZero(AnimationIdleRight), 40.0; got != want {
		t.Fatalf("VisibleTopAboveZero(IdleRight) = %v, want %v", got, want)
	}
	if got, want := s.VisibleBounds(AnimationIdleDown), image.Rect(0, 0, 8, 12); got != want {
		t.Fatalf("VisibleBounds(IdleDown) = %v, want %v", got, want)
	}
	if got, want := s.VisibleBounds(AnimationIdleRight), image.Rect(0, 0, 32, 40); got != want {
		t.Fatalf("VisibleBounds(IdleRight) = %v, want %v", got, want)
	}
}

// TestVisibleExtentOfAnUnknownAnimationIsEmpty covers that these are queries:
// an animation the sprite does not have returns the zero value rather than
// panicking or measuring an unrelated frame.
func TestVisibleExtentOfAnUnknownAnimationIsEmpty(t *testing.T) {
	s := NewSprite()
	if got := s.VisibleBounds(AnimationIdleDown); !got.Empty() {
		t.Fatalf("VisibleBounds of an unknown animation = %v, want empty", got)
	}
	if got := s.VisibleTopAboveZero(AnimationIdleDown); got != 0 {
		t.Fatalf("VisibleTopAboveZero of an unknown animation = %v, want 0", got)
	}
}
```

Add `"image"` to the test file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ 2>&1 | head -20
```

Expected: compile failure, `too many arguments in call to s.VisibleTopAboveZero`
and `cannot assign to struct field ... in map` style errors on the caches.

- [ ] **Step 3: Implement the animation-scoped queries**

In `render/render_sprite.go`, replace the two cache fields:

```go
	// cachedVisibleTopAboveZero caches VisibleTopAboveZero per animation, holding
	// the pre-ratio value so a tile size changed later still takes effect.
	cachedVisibleTopAboveZero map[AnimationType]float64

	// cachedVisibleBounds caches VisibleBounds per animation.
	cachedVisibleBounds map[AnimationType]image.Rectangle
```

Initialise both in `NewSprite`:

```go
func NewSprite() *Sprite {
	return &Sprite{
		Animations:                make(map[AnimationType]*Animation),
		cachedVisibleTopAboveZero: make(map[AnimationType]float64),
		cachedVisibleBounds:       make(map[AnimationType]image.Rectangle),
	}
}
```

Add a frame resolver next to `Anchor`:

```go
// animationFrame returns the first frame of a, resolving a mirrored animation to
// the animation it is drawn from, and nil when neither exists or has frames.
func (s *Sprite) animationFrame(a AnimationType) *ebiten.Image {
	animation, ok := s.Animations[a]
	if !ok {
		other, mirrored := MirroredAnimations[a]
		if !mirrored {
			return nil
		}
		if animation, ok = s.Animations[other]; !ok {
			return nil
		}
	}
	if len(animation.Images) == 0 {
		return nil
	}
	return animation.Images[0]
}
```

Rewrite `VisibleBounds` to take the animation, use `animationFrame`, and cache per
animation. Its doc comment replaces "in one frame" with "in the first frame of the
given animation", and drops the sentence claiming frames across animations share a
size and layout. Add: "The rectangle is in the source animation's own
coordinates and is not mirrored, so a hit test against a flipped sprite is
reflected about the anchor."

Rewrite `VisibleTopAboveZero` the same way, reading the anchor with
`s.Anchor(a).Y()` and caching by `a`. Keep the pre-ratio caching and the fresh
`* s.TileRatio()` on both return paths. Replace the doc sentence about frames
sharing a layout with "measured for the given animation, since animations may
have different crop sizes and anchors".

Both return the zero value when `animationFrame` gives nil, and both must record
that zero in the cache so a repeated call does not rescan.

- [ ] **Step 4: Update the overlay helpers**

In `render/render_overlay.go`, add an `a AnimationType` parameter after `sprite`
to both `DrawNameplate` and `DrawFloatingBar`, and pass it through:

```go
	bottom := overlayBottomScreen(camera, worldPos, sprite.VisibleTopAboveZero(a), gapPixels)
```

```go
	bottom := overlayBottomScreen(camera, worldPos, sprite.VisibleTopAboveZero(a), style.GapPixels)
```

Add to both doc comments: "The animation is required because a sprite's visible
top depends on which animation is drawn; passing the animation currently playing
keeps the label at a constant gap as the character changes animation."

- [ ] **Step 5: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Scope the visible-extent queries to an animation

VisibleBounds and VisibleTopAboveZero computed one answer from an arbitrary frame
and cached it for the whole sprite, documented as safe because frames across
animations share a size and layout. Per-animation crop boxes break exactly that
invariant.

Both now take the animation they are asked about and cache per animation, and the
overlay helpers pass through the animation being drawn, so a nameplate keeps its
gap instead of holding the height of whichever animation populated the cache
first.
EOF
)"
```

---

### Task 4: Add AnimationSpec and LoadSpriteAnimations

**Files:**
- Modify: `render/render_sprite.go` (new type, new loaders, `LoadSprite`)
- Test: `render/render_sprite_test.go`

**Interfaces:**
- Consumes: `Animation.ZeroPosition` from Task 2.
- Produces: `type AnimationSpec struct { Frames []image.Rectangle; Anchor
  geometry.Vector2; Duration time.Duration }`;
  `func LoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) (*Sprite, error)`;
  `func MustLoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) *Sprite`;
  `func sortedAnimationTypes[V any](m map[AnimationType]V) []AnimationType`.

- [ ] **Step 1: Write the failing test**

```go
// TestLoadSpriteAnimationsBuildsPerAnimationGeometry covers the primitive
// loader: each animation gets its own frames, its own anchor and its own
// duration, with no uniform grid involved.
func TestLoadSpriteAnimationsBuildsPerAnimationGeometry(t *testing.T) {
	img := ebiten.NewImage(64, 64)
	specs := map[AnimationType]AnimationSpec{
		AnimationIdleDown: {
			Frames:   []image.Rectangle{image.Rect(0, 0, 8, 12), image.Rect(8, 0, 16, 12)},
			Anchor:   geometry.NewVector2(4, 12),
			Duration: 200 * time.Millisecond,
		},
		AnimationIdleRight: {
			Frames: []image.Rectangle{image.Rect(0, 16, 32, 48)},
			Anchor: geometry.NewVector2(16, 32),
		},
	}

	s, err := LoadSpriteAnimations(img, specs)
	if err != nil {
		t.Fatalf("LoadSpriteAnimations returned error: %v", err)
	}

	if got := len(s.Animations[AnimationIdleDown].Images); got != 2 {
		t.Fatalf("IdleDown frame count = %d, want 2", got)
	}
	if got, want := s.Anchor(AnimationIdleDown), geometry.NewVector2(4, 12); got != want {
		t.Fatalf("Anchor(IdleDown) = %v, want %v", got, want)
	}
	if got, want := s.Anchor(AnimationIdleRight), geometry.NewVector2(16, 32); got != want {
		t.Fatalf("Anchor(IdleRight) = %v, want %v", got, want)
	}
	if got, want := s.Animations[AnimationIdleDown].Duration, 200*time.Millisecond; got != want {
		t.Fatalf("IdleDown duration = %v, want %v", got, want)
	}
	// A zero duration defaults to one second, matching LoadSprite.
	if got, want := s.Animations[AnimationIdleRight].Duration, time.Second; got != want {
		t.Fatalf("IdleRight duration = %v, want %v", got, want)
	}
	// Frames of different animations may differ in size.
	if got := s.Animations[AnimationIdleRight].Images[0].Bounds().Dx(); got != 32 {
		t.Fatalf("IdleRight frame width = %d, want 32", got)
	}
}

// TestLoadSpriteAnimationsRejectsBadGeometry covers the three ways a spec can be
// wrong, each of which is a programming error worth naming rather than drawing
// something wrong.
func TestLoadSpriteAnimationsRejectsBadGeometry(t *testing.T) {
	img := ebiten.NewImage(64, 64)
	for _, tc := range []struct {
		name string
		spec AnimationSpec
	}{
		{"no frames", AnimationSpec{}},
		{"empty rectangle", AnimationSpec{Frames: []image.Rectangle{image.Rect(4, 4, 4, 4)}}},
		{"outside the image", AnimationSpec{Frames: []image.Rectangle{image.Rect(0, 0, 128, 128)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadSpriteAnimations(img, map[AnimationType]AnimationSpec{
				AnimationIdleDown: tc.spec,
			}); err == nil {
				t.Fatal("LoadSpriteAnimations accepted an invalid spec")
			}
		})
	}
}

// TestLoadSpriteMatchesLoadSpriteAnimations covers that the uniform grid is just
// a convenient way to describe the same thing, since LoadSprite is a wrapper.
func TestLoadSpriteMatchesLoadSpriteAnimations(t *testing.T) {
	img := ebiten.NewImage(32, 32)

	fromGrid, err := LoadSprite(img, 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil)
	if err != nil {
		t.Fatalf("LoadSprite returned error: %v", err)
	}

	fromSpecs, err := LoadSpriteAnimations(img, map[AnimationType]AnimationSpec{
		AnimationIdleDown: {Frames: []image.Rectangle{
			image.Rect(0, 0, 16, 16), image.Rect(16, 0, 32, 16),
		}},
	})
	if err != nil {
		t.Fatalf("LoadSpriteAnimations returned error: %v", err)
	}

	for i := range fromGrid.Animations[AnimationIdleDown].Images {
		got := fromGrid.Animations[AnimationIdleDown].Images[i].Bounds()
		want := fromSpecs.Animations[AnimationIdleDown].Images[i].Bounds()
		if got != want {
			t.Fatalf("frame %d bounds = %v, want %v", i, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ 2>&1 | head -20
```

Expected: compile failure, `undefined: AnimationSpec` and
`undefined: LoadSpriteAnimations`.

- [ ] **Step 3: Implement the descriptor and loader**

In `render/render_sprite.go`, add `"sort"` to the imports and:

```go
// AnimationSpec describes one animation's geometry within an image: where its
// frames are, where its anchor sits inside them, and how long it runs. It is the
// load-time description of an animation, where Animation is the loaded form; the
// two differ in that a spec names frames as rectangles into a source image while
// an Animation holds uploaded textures.
//
// Frames need not be the same size as each other or as another animation's,
// which is what lets a sheet be packed with one crop box per animation.
type AnimationSpec struct {
	// Frames are the source rectangles of this animation's frames, in order.
	Frames []image.Rectangle

	// Anchor is the pixel inside this animation's frames that sits on the drawn
	// world position, in frame-local pixels.
	Anchor geometry.Vector2

	// Duration is how long the whole animation runs. Zero means one second.
	Duration time.Duration
}

// sortedAnimationTypes returns m's keys in ascending order, so anything built
// from a map of animations is reproducible rather than depending on Go's map
// iteration order.
func sortedAnimationTypes[V any](m map[AnimationType]V) []AnimationType {
	types := make([]AnimationType, 0, len(m))
	for a := range m {
		types = append(types, a)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}

// LoadSpriteAnimations builds a sprite whose animations each carry their own
// frame rectangles, anchor and duration. Frames are sub-images of img, so the
// whole sprite costs one texture however many animations it has.
//
// Use it when a sheet is not a uniform grid, or when animations need different
// anchors. LoadSprite is the convenience for the uniform case and is built on
// this.
func LoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) (*Sprite, error) {
	sprite := NewSprite()
	bounds := img.Bounds()

	for _, a := range sortedAnimationTypes(specs) {
		spec := specs[a]
		if len(spec.Frames) == 0 {
			return nil, fmt.Errorf("animation %s: no frames", a)
		}
		for i, rect := range spec.Frames {
			if rect.Empty() {
				return nil, fmt.Errorf("animation %s frame %d: empty rectangle %v", a, i, rect)
			}
			if !rect.In(bounds) {
				return nil, fmt.Errorf("animation %s frame %d: rectangle %v is outside the image bounds %v", a, i, rect, bounds)
			}
			sprite.AddImage(a, img.SubImage(rect).(*ebiten.Image))
		}

		animation := sprite.Animations[a]
		animation.ZeroPosition = spec.Anchor
		animation.Duration = spec.Duration
		if animation.Duration == 0 {
			animation.Duration = time.Second
		}
	}

	return sprite, nil
}

// MustLoadSpriteAnimations is like LoadSpriteAnimations but panics on error.
func MustLoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) *Sprite {
	sprite, err := LoadSpriteAnimations(img, specs)
	if err != nil {
		panic(fmt.Sprintf("loading sprite animations: %v", err))
	}
	return sprite
}
```

Rewrite `LoadSprite` as a wrapper. An empty index list still creates no animation,
which is what `TestLoadSpriteEmptyIndexesDoesNotPanic` pins:

```go
// LoadSprite slices img into a grid of width columns by height rows, assigns the
// frame indexes in indexes to each AnimationType, and sets per-animation
// durations (defaulting to one second when unspecified). Note: width and height
// are column/row counts, not pixel dimensions.
//
// It is the convenience for a uniform sheet, expressed on top of
// LoadSpriteAnimations. Anchors are left at zero, since a uniform sheet's anchor
// is normally set once for the whole sprite with SetZeroPosition.
func LoadSprite(img *ebiten.Image, width, height int, indexes map[AnimationType][]int, durations map[AnimationType]time.Duration) (*Sprite, error) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	tileWidth := w / width
	tileHeight := h / height

	specs := make(map[AnimationType]AnimationSpec, len(indexes))
	for animationType, animationIndexes := range indexes {
		// An empty index list creates no animation at all, so it must not reach
		// LoadSpriteAnimations, which rejects a spec with no frames.
		if len(animationIndexes) == 0 {
			continue
		}
		frames := make([]image.Rectangle, 0, len(animationIndexes))
		for _, index := range animationIndexes {
			x := (index % width) * tileWidth
			y := (index / width) * tileHeight
			frames = append(frames, image.Rect(x, y, x+tileWidth, y+tileHeight))
		}
		specs[animationType] = AnimationSpec{Frames: frames, Duration: durations[animationType]}
	}

	return LoadSpriteAnimations(img, specs)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Add AnimationSpec and the loader that consumes it

LoadSprite bakes one uniform grid into its signature, so a sheet packed with a
different crop box per animation cannot be described to it at all.

AnimationSpec carries an animation's frame rectangles, anchor and duration in one
descriptor rather than three maps keyed alike, and LoadSpriteAnimations builds a
sprite from a map of them. LoadSprite keeps its signature and becomes a wrapper
that expands its grid into specs, so there is one slicing path underneath.
EOF
)"
```

---

### Task 5: Add the auto-crop packer

The packer is unexported and pure CPU. It reads an `image.Image`, so unlike the
rest of this change its output can be asserted directly on real pixel values.

**Files:**
- Create: `render/render_spriteautocrop.go`
- Test: `render/render_spriteautocrop_test.go`

**Interfaces:**
- Consumes: `AnimationSpec` and `sortedAnimationTypes` from Task 4.
- Produces:
  `func autoCropAtlas(src image.Image, columns, rows int, indexes map[AnimationType][]int, durations map[AnimationType]time.Duration, anchor geometry.Vector2) (*image.RGBA, map[AnimationType]AnimationSpec, error)`.

- [ ] **Step 1: Write the failing test**

Create `render/render_spriteautocrop_test.go`:

```go
package render

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
)

// autoCropTestSheet builds a 2x2 grid of 16 pixel cells where cell 0 has an 8x8
// opaque block at (4,4), cell 1 has a 4x4 block at (2,2), cell 2 is entirely
// transparent, and cell 3 is untouched by any animation.
func autoCropTestSheet() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fill := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
			}
		}
	}
	fill(4, 4, 12, 12)   // cell 0, cell-local (4,4)-(12,12)
	fill(16+2, 2, 16+6, 6) // cell 1, cell-local (2,2)-(6,6)
	// cell 2 at (0,16) stays transparent; cell 3 at (16,16) is never referenced.
	return img
}

// TestAutoCropTightensEachAnimation covers the core measurement: each animation
// gets a crop box around its own content, and its anchor is rebased into that
// box so the drawn result is unchanged.
func TestAutoCropTightensEachAnimation(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	if got := down.Frames[0].Dx(); got != 8 {
		t.Fatalf("IdleDown crop width = %d, want 8", got)
	}
	if got := down.Frames[0].Dy(); got != 8 {
		t.Fatalf("IdleDown crop height = %d, want 8", got)
	}
	// The box starts at cell-local (4,4), so the anchor moves by that much.
	if got, want := down.Anchor, geometry.NewVector2(4, 12); got != want {
		t.Fatalf("IdleDown anchor = %v, want %v", got, want)
	}

	right := specs[AnimationIdleRight]
	if got := right.Frames[0].Dx(); got != 4 {
		t.Fatalf("IdleRight crop width = %d, want 4", got)
	}
	if got, want := right.Anchor, geometry.NewVector2(6, 14); got != want {
		t.Fatalf("IdleRight anchor = %v, want %v", got, want)
	}

	// The packed atlas is smaller than the source it came from.
	srcArea := 32 * 32
	atlasArea := atlas.Bounds().Dx() * atlas.Bounds().Dy()
	if atlasArea >= srcArea {
		t.Fatalf("atlas area %d is not smaller than the source area %d", atlasArea, srcArea)
	}
}

// TestAutoCropCopiesThePixels covers that the crop is a real copy into a new
// image rather than a narrower view: the packed frame must carry the content.
func TestAutoCropCopiesThePixels(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	rect := specs[AnimationIdleDown].Frames[0]
	// Every pixel of a tight crop around a solid block is opaque.
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if _, _, _, a := atlas.At(x, y).RGBA(); a == 0 {
				t.Fatalf("packed pixel at (%d,%d) is transparent", x, y)
			}
		}
	}
}

// TestAutoCropIsReproducible covers that the atlas does not depend on Go's map
// iteration order, which it would if animations were packed as they were ranged.
func TestAutoCropIsReproducible(t *testing.T) {
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}
	first, firstSpecs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, indexes, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	for range 8 {
		next, nextSpecs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, indexes, nil, geometry.Zero2D())
		if err != nil {
			t.Fatalf("autoCropAtlas returned error: %v", err)
		}
		if next.Bounds() != first.Bounds() {
			t.Fatalf("atlas bounds = %v, want %v", next.Bounds(), first.Bounds())
		}
		for a, spec := range nextSpecs {
			if spec.Frames[0] != firstSpecs[a].Frames[0] {
				t.Fatalf("animation %s frame = %v, want %v", a, spec.Frames[0], firstSpecs[a].Frames[0])
			}
		}
		if string(next.Pix) != string(first.Pix) {
			t.Fatal("atlas pixels differ between runs")
		}
	}
}

// TestAutoCropFallsBackForAnAllTransparentAnimation covers the pathological
// case: an animation with nothing drawn keeps its full cell rather than
// producing a degenerate box or an error.
func TestAutoCropFallsBackForAnAllTransparentAnimation(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {2},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	if got := specs[AnimationIdleDown].Frames[0].Dx(); got != 16 {
		t.Fatalf("all-transparent crop width = %d, want the full cell width 16", got)
	}
}

// TestAutoCropCarriesDurations covers that durations survive the repack, with
// the same one-second default the other loaders use.
func TestAutoCropCarriesDurations(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2,
		map[AnimationType][]int{AnimationIdleDown: {0}, AnimationIdleRight: {1}},
		map[AnimationType]time.Duration{AnimationIdleDown: 250 * time.Millisecond},
		geometry.Zero2D())
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}
	if got, want := specs[AnimationIdleDown].Duration, 250*time.Millisecond; got != want {
		t.Fatalf("IdleDown duration = %v, want %v", got, want)
	}
	if got := specs[AnimationIdleRight].Duration; got != 0 {
		t.Fatalf("IdleRight duration = %v, want 0 so the loader defaults it", got)
	}
}

// TestAutoCropRejectsABadGrid covers that a grid that cannot describe the image
// is named rather than producing a zero-sized cell.
func TestAutoCropRejectsABadGrid(t *testing.T) {
	for _, tc := range []struct{ columns, rows int }{{0, 2}, {2, 0}, {64, 2}} {
		if _, _, err := autoCropAtlas(autoCropTestSheet(), tc.columns, tc.rows,
			map[AnimationType][]int{AnimationIdleDown: {0}}, nil, geometry.Zero2D()); err == nil {
			t.Fatalf("autoCropAtlas accepted a %dx%d grid", tc.columns, tc.rows)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run AutoCrop 2>&1 | head -20
```

Expected: compile failure, `undefined: autoCropAtlas`.

- [ ] **Step 3: Implement the packer**

Create `render/render_spriteautocrop.go`:

```go
package render

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sort"
	"time"

	"github.com/trancecode/vantage/geometry"
)

// alphaReaderFor returns a function reading the alpha at a pixel of src, taking a
// direct Pix path for the concrete types sheets decode to. The generic At path
// allocates a color.Color per pixel, which matters when a sheet is tens of
// millions of pixels.
func alphaReaderFor(src image.Image) func(x, y int) uint32 {
	switch img := src.(type) {
	case *image.RGBA:
		return func(x, y int) uint32 { return uint32(img.Pix[img.PixOffset(x, y)+3]) }
	case *image.NRGBA:
		return func(x, y int) uint32 { return uint32(img.Pix[img.PixOffset(x, y)+3]) }
	}
	return func(x, y int) uint32 {
		_, _, _, a := src.At(x, y).RGBA()
		return a
	}
}

// cropBoxIn returns the tight rectangle around non-transparent pixels of cell,
// in coordinates local to cell's origin, and false when the cell is empty.
func cropBoxIn(alphaAt func(x, y int) uint32, cell image.Rectangle) (image.Rectangle, bool) {
	minX, minY := cell.Dx(), cell.Dy()
	maxX, maxY := -1, -1
	for y := cell.Min.Y; y < cell.Max.Y; y++ {
		for x := cell.Min.X; x < cell.Max.X; x++ {
			if alphaAt(x, y) == 0 {
				continue
			}
			lx, ly := x-cell.Min.X, y-cell.Min.Y
			minX, minY = min(minX, lx), min(minY, ly)
			maxX, maxY = max(maxX, lx), max(maxY, ly)
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

// placement is one frame waiting to be copied into the atlas.
type placement struct {
	source image.Rectangle
	dest   image.Rectangle
}

// autoCropAtlas measures a tight crop box per animation over the uniform grid
// described by columns, rows and indexes, packs every referenced frame into a new
// atlas at that box's size, and rebases anchor into each box.
//
// Cells no animation references are never visited, so a sheet laid out one
// animation per row does not pay for the empty tail of a short row. The result is
// deterministic: animations are processed in sorted order rather than map order.
//
// It takes an image.Image rather than an *ebiten.Image because the whole point is
// to crop before anything reaches the GPU. Cropping an uploaded texture by
// sub-imaging saves nothing, since a sub-image shares its parent's storage.
func autoCropAtlas(
	src image.Image,
	columns, rows int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) (*image.RGBA, map[AnimationType]AnimationSpec, error) {
	if columns <= 0 || rows <= 0 {
		return nil, nil, fmt.Errorf("grid is %dx%d cells, want both positive", columns, rows)
	}
	bounds := src.Bounds()
	cellWidth, cellHeight := bounds.Dx()/columns, bounds.Dy()/rows
	if cellWidth <= 0 || cellHeight <= 0 {
		return nil, nil, fmt.Errorf("a %dx%d grid over a %dx%d image gives %dx%d cells, want both positive",
			columns, rows, bounds.Dx(), bounds.Dy(), cellWidth, cellHeight)
	}

	alphaAt := alphaReaderFor(src)
	cellAt := func(index int) image.Rectangle {
		x := bounds.Min.X + (index%columns)*cellWidth
		y := bounds.Min.Y + (index/columns)*cellHeight
		return image.Rect(x, y, x+cellWidth, y+cellHeight)
	}

	specs := make(map[AnimationType]AnimationSpec, len(indexes))
	var pending []placement

	for _, a := range sortedAnimationTypes(indexes) {
		frameIndexes := indexes[a]
		if len(frameIndexes) == 0 {
			continue
		}

		// One box per animation: the union over its frames, so every frame is
		// stored at the same size and the anchor stays per animation.
		box := image.Rectangle{}
		for _, index := range frameIndexes {
			if index < 0 || index >= columns*rows {
				return nil, nil, fmt.Errorf("animation %s: frame index %d is outside a %dx%d grid", a, index, columns, rows)
			}
			if frameBox, ok := cropBoxIn(alphaAt, cellAt(index)); ok {
				box = box.Union(frameBox)
			}
		}
		if box.Empty() {
			// Nothing is drawn in any frame. Keeping the full cell is safe and
			// costs only what the uncropped sheet already cost.
			box = image.Rect(0, 0, cellWidth, cellHeight)
		}

		frames := make([]image.Rectangle, 0, len(frameIndexes))
		for _, index := range frameIndexes {
			source := box.Add(cellAt(index).Min)
			frames = append(frames, source)
			pending = append(pending, placement{source: source})
		}
		specs[a] = AnimationSpec{
			Frames:   frames,
			Anchor:   anchor.Sub(geometry.NewVector2(box.Min.X, box.Min.Y)),
			Duration: durations[a],
		}
	}

	atlas, placed := shelfPack(pending)

	// Rewrite each animation's frames to where they actually landed. pending was
	// built in the same sorted order, so a running index matches them up.
	next := 0
	for _, a := range sortedAnimationTypes(specs) {
		spec := specs[a]
		for i := range spec.Frames {
			spec.Frames[i] = placed[next].dest
			next++
		}
		specs[a] = spec
	}

	for _, p := range placed {
		draw.Draw(atlas, p.dest, src, p.source.Min, draw.Src)
	}

	return atlas, specs, nil
}

// shelfPack lays the given frames out in rows no wider than a target width,
// tallest first, and returns the atlas to copy them into along with where each
// one goes. The order of the returned placements matches the input.
//
// Shelf packing is deliberately simple. Frames of one animation all share a size,
// so they shelf neatly, and the win being chased here is dropping transparent
// padding rather than the last few percent of packing efficiency.
func shelfPack(frames []placement) (*image.RGBA, []placement) {
	if len(frames) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	}

	// A roughly square atlas, never narrower than the widest frame.
	area, widest := 0, 0
	for _, f := range frames {
		area += f.source.Dx() * f.source.Dy()
		widest = max(widest, f.source.Dx())
	}
	targetWidth := max(widest, int(math.Ceil(math.Sqrt(float64(area)))))

	// Tallest first keeps each shelf's wasted height down. Ties break on the
	// input position, so the layout does not depend on sort stability.
	order := make([]int, len(frames))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := frames[order[i]], frames[order[j]]
		if a.source.Dy() != b.source.Dy() {
			return a.source.Dy() > b.source.Dy()
		}
		return order[i] < order[j]
	})

	placed := make([]placement, len(frames))
	penX, penY, shelfHeight, atlasWidth := 0, 0, 0, 0
	for _, i := range order {
		w, h := frames[i].source.Dx(), frames[i].source.Dy()
		if penX > 0 && penX+w > targetWidth {
			penX, penY = 0, penY+shelfHeight
			shelfHeight = 0
		}
		placed[i] = placement{
			source: frames[i].source,
			dest:   image.Rect(penX, penY, penX+w, penY+h),
		}
		penX += w
		shelfHeight = max(shelfHeight, h)
		atlasWidth = max(atlasWidth, penX)
	}

	return image.NewRGBA(image.Rect(0, 0, atlasWidth, penY+shelfHeight)), placed
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run AutoCrop -v 2>&1 | tail -30
```

Expected: every `AutoCrop` test PASSes.

- [ ] **Step 5: Run the full checks**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Add the sprite sheet auto-crop packer

Measures a tight crop box per animation over a uniform grid, packs every
referenced frame into a new atlas at that box's size, and rebases the sheet-wide
anchor into each box.

Cropping has to copy into a new image rather than narrow a sub-image, because an
Ebitengine sub-image shares its parent's storage and so saves no video memory at
all. Cells no animation references are never visited, so a sheet laid out one
animation per row stops paying for the empty tail of a short row.

Unexported and reading an image.Image, so its tests assert real pixel values
rather than seeding a cache the way the ebiten.Image paths have to.
EOF
)"
```

---

### Task 6: Add LoadSpriteAutoCropped

**Files:**
- Modify: `render/render_spriteautocrop.go`
- Test: `render/render_spriteautocrop_test.go`

**Interfaces:**
- Consumes: `autoCropAtlas` from Task 5, `LoadSpriteAnimations` from Task 4.
- Produces:
  `func LoadSpriteAutoCropped(src image.Image, width, height int, indexes map[AnimationType][]int, durations map[AnimationType]time.Duration, anchor geometry.Vector2) (*Sprite, error)`;
  `func MustLoadSpriteAutoCropped(...) *Sprite`.

- [ ] **Step 1: Write the failing test**

This is the centrepiece: cropping must be invisible on screen.

```go
// TestAutoCroppedDrawsWhereTheUncroppedSpriteWould is the property the whole
// change has to preserve. The same sheet loaded uniformly and auto-cropped must
// put the anchor pixel on the same screen point for every animation, so the
// repack is invisible in the game.
func TestAutoCroppedDrawsWhereTheUncroppedSpriteWould(t *testing.T) {
	sheet := autoCropTestSheet()
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0},
		AnimationIdleRight: {1},
	}
	anchor := geometry.NewVector2(8, 16)

	uniform, err := LoadSprite(ebiten.NewImageFromImage(sheet), 2, 2, indexes, nil)
	if err != nil {
		t.Fatalf("LoadSprite returned error: %v", err)
	}
	uniform.SetZeroPosition(anchor)

	cropped, err := LoadSpriteAutoCropped(sheet, 2, 2, indexes, nil, anchor)
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}

	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	const eps = 1e-9
	for _, a := range []AnimationType{AnimationIdleDown, AnimationIdleRight} {
		wantAnchor := uniform.Anchor(a)
		wantOp := uniform.buildDrawOp(p, a, false, c, 1.0)
		wantX, wantY := wantOp.GeoM.Apply(wantAnchor.X(), wantAnchor.Y())

		gotAnchor := cropped.Anchor(a)
		gotOp := cropped.buildDrawOp(p, a, false, c, 1.0)
		gotX, gotY := gotOp.GeoM.Apply(gotAnchor.X(), gotAnchor.Y())

		if diff := gotX - wantX; diff > eps || diff < -eps {
			t.Errorf("animation %s: anchor X = %v, want %v", a, gotX, wantX)
		}
		if diff := gotY - wantY; diff > eps || diff < -eps {
			t.Errorf("animation %s: anchor Y = %v, want %v", a, gotY, wantY)
		}
	}
}

// TestLoadSpriteAutoCroppedShrinksTheFrames covers that the loaded sprite really
// carries the tight frames rather than the padded cells.
func TestLoadSpriteAutoCroppedShrinksTheFrames(t *testing.T) {
	s, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.Zero2D())
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}
	if got := s.Animations[AnimationIdleDown].Images[0].Bounds().Dx(); got != 8 {
		t.Fatalf("frame width = %d, want the cropped 8 rather than the 16 pixel cell", got)
	}
}
```

Add `"github.com/hajimehoshi/ebiten/v2"` to the test file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run AutoCropped 2>&1 | head -20
```

Expected: compile failure, `undefined: LoadSpriteAutoCropped`.

- [ ] **Step 3: Implement the loaders**

Append to `render/render_spriteautocrop.go`, adding
`"github.com/hajimehoshi/ebiten/v2"` to its imports:

```go
// LoadSpriteAutoCropped builds a sprite from a uniform sheet, cropping each
// animation to its own content and repacking the frames into a smaller texture
// before upload. anchor is the sheet-wide anchor in cell-local pixels; each
// animation's anchor is derived from it and its own crop box, so no per-animation
// anchor has to be supplied.
//
// It takes an image.Image, not an *ebiten.Image, because the crop must happen
// before the sheet is uploaded: a sheet is mostly transparent padding, and
// padding costs nothing on disk but a full texture in video memory.
//
// As in LoadSprite, width and height are column and row counts, not pixel
// dimensions.
func LoadSpriteAutoCropped(
	src image.Image,
	width, height int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) (*Sprite, error) {
	atlas, specs, err := autoCropAtlas(src, width, height, indexes, durations, anchor)
	if err != nil {
		return nil, fmt.Errorf("cropping sprite sheet: %w", err)
	}
	return LoadSpriteAnimations(ebiten.NewImageFromImage(atlas), specs)
}

// MustLoadSpriteAutoCropped is like LoadSpriteAutoCropped but panics on error.
func MustLoadSpriteAutoCropped(
	src image.Image,
	width, height int,
	indexes map[AnimationType][]int,
	durations map[AnimationType]time.Duration,
	anchor geometry.Vector2,
) *Sprite {
	sprite, err := LoadSpriteAutoCropped(src, width, height, indexes, durations, anchor)
	if err != nil {
		panic(fmt.Sprintf("loading auto-cropped sprite: %v", err))
	}
	return sprite
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all pass, including
`TestAutoCroppedDrawsWhereTheUncroppedSpriteWould`.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Add LoadSpriteAutoCropped

Wires the packer to the per-animation loader, so a game passes the decoded sheet
it already has and gets back a sprite drawing from a repacked texture, with the
per-animation anchors derived rather than declared.

Covered by the property that matters: the same sheet loaded uniformly and
auto-cropped puts the anchor pixel on the same screen point for every animation,
so the repack is invisible where it counts.
EOF
)"
```

---

### Task 7: Measure the auto-crop cost

The spec asserts the startup scan is acceptable by analogy with work lockstep
already does per sheet. That is an inference, and it is the one number that could
push the design toward precomputed boxes. Measure it before claiming it.

**Files:**
- Modify: `render/render_spriteautocrop_test.go`
- Modify: `docs/performance_optimization.md`

**Interfaces:**
- Consumes: `autoCropAtlas` from Task 5.
- Produces: nothing other tasks use.

- [ ] **Step 1: Write the benchmark**

```go
// BenchmarkAutoCropAtlas measures the scan and repack on a sheet shaped like the
// real ones: a large grid where most of each cell is transparent. The published
// sheets are 7296x10624 with a 38x64 grid, which is too large to allocate in a
// benchmark loop, so this uses the same cell size and sparsity at a tenth of the
// area and the result scales linearly with pixel count.
func BenchmarkAutoCropAtlas(b *testing.B) {
	const columns, rows, cell = 38, 6, 192
	src := image.NewRGBA(image.Rect(0, 0, columns*cell, rows*cell))
	// A block covering roughly 4% of each cell, matching the measured fill.
	for row := range rows {
		for col := range columns {
			x0 := col*cell + cell/2
			y0 := row*cell + cell/2
			for y := y0; y < y0+cell*2/10; y++ {
				for x := x0; x < x0+cell*2/10; x++ {
					src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
				}
			}
		}
	}
	indexes := map[AnimationType][]int{}
	for row := range rows {
		frames := make([]int, columns)
		for col := range columns {
			frames[col] = row*columns + col
		}
		indexes[AnimationGameBase+AnimationType(row)] = frames
	}

	b.ResetTimer()
	for b.Loop() {
		if _, _, err := autoCropAtlas(src, columns, rows, indexes, nil, geometry.Zero2D()); err != nil {
			b.Fatalf("autoCropAtlas returned error: %v", err)
		}
	}
}
```

Add `"image/color"` to the test file imports if Task 5 did not already.

- [ ] **Step 2: Run the benchmark and record the number**

```bash
export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run '^$' -bench AutoCropAtlas -benchtime 3x 2>&1 | tail -10
```

Record nanoseconds per operation and the pixel count
(`38*192 * 6*192` = 7296x1152 = 8.4 million pixels). Scale to the real sheet's
77.5 million pixels by multiplying by roughly 9.2, then by six sheets.

- [ ] **Step 3: Record the finding**

Add a section to `docs/performance_optimization.md` stating the measured
per-megapixel cost, the extrapolation to six real sheets, and the escape hatch:
precomputing the crop boxes offline and calling `LoadSpriteAnimations` directly
needs no further engine work.

If the extrapolated total exceeds roughly two seconds, stop and report it rather
than continuing. That is the threshold where a visible startup delay makes
precomputed boxes the better default, and it is a decision for the user rather
than something to absorb silently.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Measure the auto-crop scan and repack

The design assumed the startup scan was acceptable by analogy with a full CPU
pass a consumer already makes per sheet. That was an inference, and it is the one
number that could argue for precomputed crop boxes instead.

Adds a benchmark shaped like the real sheets and records the extrapolated cost,
with precomputing the boxes offline noted as the escape hatch that needs no
further engine work.
EOF
)"
```

---

### Task 8: Update the package and engine documentation

**Files:**
- Modify: `render/doc.go`
- Modify: `docs/debugging.md`
- Modify: `ARCHITECTURE.md` if it describes sprite loading

**Interfaces:**
- Consumes: everything from Tasks 1 to 6.
- Produces: nothing other tasks use.

- [ ] **Step 1: Update render/doc.go**

Rewrite the sprite sentences of the package comment. It currently describes
`Sprite` as wrapping directional animations from sprite sheets and mentions
`DrawAnimationScaled`. Add, in the same voice:

* the anchor is per animation, with `SetZeroPosition` setting one anchor across a
  uniform sheet and `Anchor` reading the resolved value
* `AnimationSpec` describes an animation's frames, anchor and duration at load
  time, consumed by `LoadSpriteAnimations`, with `LoadSprite` the uniform-grid
  convenience built on it
* `LoadSpriteAutoCropped` crops each animation to its own content and repacks
  before upload, deriving per-animation anchors
* remove any mention of `Sprite.Scale`

- [ ] **Step 2: Update docs/debugging.md**

Line 279 records that the showcase demo uses `Sprite.Scale` rather than large
frames. Rewrite it for the source tile size, matching the change made in Task 1,
and check lines 75 to 82 for the same claim about a sprite's `Scale` in the
fixed-slot description.

- [ ] **Step 3: Check ARCHITECTURE.md**

```bash
grep -n -i "scale\|zeroposition\|loadsprite" ARCHITECTURE.md
```

Update anything describing the old model. If nothing matches, make no change.

- [ ] **Step 4: Verify the documentation builds and reads**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go doc ./render | head -40 && task lint
```

Expected: the package synopsis reflects the new loaders, and lint passes.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "$(cat <<'EOF'
Document the per-animation anchor and the cropping loaders

Covers the anchor moving onto the animation, AnimationSpec and
LoadSpriteAnimations, and LoadSpriteAutoCropped, and drops the references to the
removed sprite scale from the package doc and the debugging guide.
EOF
)"
```

---

### Task 9: Verify nrg against the local build

nrg is the other consumer and must keep working. Verify it before tagging, so a
problem is fixed in vantage rather than worked around in nrg.

**Files:**
- Modify (temporarily, never committed): `~/src/nrg/go.mod`
- Modify: `~/src/nrg/rts/rts_scene.go`, `~/src/nrg/sprites/sprites.go`

**Interfaces:**
- Consumes: `Sprite.Anchor` and `VisibleBounds` from Tasks 2 and 3.
- Produces: nothing vantage depends on.

- [ ] **Step 1: Point nrg at the local vantage**

```bash
cd ~/src/nrg && export GOMODCACHE=/tmp/go-mod-cache && \
    go mod edit -replace github.com/trancecode/vantage=/home/exedev/src/vantage && \
    go build ./... 2>&1 | head -20
```

Expected: failures at `sprites/sprites.go:83` for `SetScale` and at
`rts/rts_scene.go` for `VisibleBounds` and `ZeroPosition`.

- [ ] **Step 2: Fix the nrg call sites**

In `sprites/sprites.go:83`, drop `.SetScale(1.0)`, leaving
`.SetType(render.SpriteTypeTerrain)`.

In `rts/rts_scene.go`, `entityAtScreenPos` reads the sprite's visible bounds and
anchor. Pass the animation the component already holds, and read the anchor
through the accessor:

```go
		visible := spriteComp.Sprite.VisibleBounds(spriteComp.Animation)
		if visible.Empty() {
			continue
		}
		scale := spriteComp.Sprite.TileRatio() * zoom
		anchor := spriteComp.Sprite.Anchor(spriteComp.Animation)
		entityScreen := s.camera.WorldToScreen(posComp.Position)
		// Sprite frame top-left in screen coords.
		frameLeft := entityScreen.X() - anchor.X()*scale
		frameTop := entityScreen.Y() - anchor.Y()*scale
```

The rest of the function is unchanged. Note that `scale` was
`spriteComp.Sprite.Scale * zoom`, which multiplied the anchor by the sprite scale
a second time; with the scale gone the tile ratio is the correct factor and the
latent error goes away.

- [ ] **Step 3: Build and test nrg**

```bash
cd ~/src/nrg && export GOMODCACHE=/tmp/go-mod-cache && \
    go build ./... && xvfb-run -a go test ./... 2>&1 | grep -v "^ok\|no test files" | head -20
```

Expected: build succeeds and tests pass. If anything fails because of a vantage
design problem rather than a missed call site, fix it in vantage and return to
Task 1's verification.

- [ ] **Step 4: Screenshot nrg to confirm it still renders**

Find nrg's runnable command and run it headless:

```bash
cd ~/src/nrg && ls cmd/ && export GOMODCACHE=/tmp/go-mod-cache && \
    xvfb-run -a go run ./cmd/<name> \
    --width 800 --height 600 \
    --screenshot_path /tmp/nrg-after.png \
    --screenshot_delay 500ms \
    --run_for 1000ms
```

Read `/tmp/nrg-after.png`. Expected: characters stand on their tiles rather than
floating or sinking, which is what a broken anchor would look like.

If nrg's command does not accept those flags, report that and fall back to
confirming the build and tests only.

- [ ] **Step 5: Remove the replace directive**

```bash
cd ~/src/nrg && go mod edit -dropreplace github.com/trancecode/vantage && \
    git diff --stat
```

Leave nrg's source edits uncommitted and report them. nrg is a different
repository, so committing there is a separate decision for the user; the edits
are migrated properly in Task 10 once a tag exists.

---

### Task 10: Release and migrate the consumers

**Files:**
- Modify: `~/src/nrg/go.mod`, plus the edits from Task 9
- Modify: `~/src/lockstep/go.mod`, `~/src/lockstep/shell/shell_sprites.go`

**Interfaces:**
- Consumes: the whole change.
- Produces: a released tag.

- [ ] **Step 1: Confirm vantage is green and pushed**

```bash
cd /home/exedev/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && \
    task lint && task test:headless && go vet ./... && \
    git status --short && git log --oneline -8 | cat
```

Expected: clean tree, all checks pass.

- [ ] **Step 2: Push, rebasing if the remote moved**

```bash
cd /home/exedev/src/vantage && git fetch origin && \
    git rebase origin/main && git push origin main
```

An autonomous agent pushes to this repository, so rebase rather than force-push.

- [ ] **Step 3: Tag the release**

```bash
cd /home/exedev/src/vantage && git tag v0.1.19 && git push origin v0.1.19
```

Confirm `v0.1.18` is the current highest tag first with
`git tag --sort=-creatordate | head -3`, and pick the next patch number if it is
not.

- [ ] **Step 4: Migrate nrg to the tag**

```bash
cd ~/src/nrg && export GOMODCACHE=/tmp/go-mod-cache && \
    go get github.com/trancecode/vantage@v0.1.19 && go mod tidy && \
    go build ./... && xvfb-run -a go test ./... 2>&1 | grep -v "^ok\|no test files" | head
```

Then commit in nrg with the same author configuration, describing the anchor and
scale migration.

- [ ] **Step 5: Migrate lockstep to auto-cropping**

In `~/src/lockstep/shell/shell_sprites.go:136`, replace the uniform load and the
anchor setter with the cropping loader. The recoloured CPU-side image is already
in hand, so no extra decode is needed:

```go
	sprite, err := render.LoadSpriteAutoCropped(
		recolored,
		manifest.Grid.Columns, manifest.Grid.Rows,
		indexes, durations,
		geometry.NewVector2(manifest.Anchor.X, manifest.Anchor.Y),
	)
	if err != nil {
		return sheetSprite{}, fmt.Errorf("building sprite for sheet %q: %w", stem, err)
	}

	// SourceTileSize is the authorial pixels-per-tile the art was rendered at,
	// which vantage uses to scale it against whatever TileSize the game runs
	// with. The anchor is no longer set here: the cropping loader derives one
	// per animation from the sheet-wide anchor and each animation's crop box.
	sprite.SetSourceTileSize(manifest.Source.PixelsPerUnit).
		SetType(render.SpriteTypeActor)
```

`ebiten.NewImageFromImage(recolored)` is no longer needed at that call site.

```bash
cd ~/src/lockstep && export GOMODCACHE=/tmp/go-mod-cache && \
    go get github.com/trancecode/vantage@v0.1.19 && go mod tidy && \
    go build ./... && xvfb-run -a go test ./... 2>&1 | grep -v "^ok\|no test files" | head
```

- [ ] **Step 6: Verify the saving in lockstep and report**

lockstep is where the real sheets live, so it is the only place the 310 MB to
47 MB claim can be checked. Run its arena headless with the sprite sheets enabled
and confirm characters render correctly and stand on their tiles.

Report to the user: the measured atlas sizes against the published sheets, the
benchmark number from Task 7, and the fact that nrg and lockstep both build and
render. Do not commit in lockstep or nrg without confirming with the user, since
those are separate repositories.

---

## Self-review notes

Spec coverage checked against
`docs/superpowers/specs/2026-08-08-per-animation-crop-boxes-design.md`:

* Anchor onto `Animation`, `Sprite.ZeroPosition` deleted, `SetZeroPosition`
  fan-out and panic, `Anchor` accessor: Task 2.
* `Sprite.Scale` removed with all four in-repo call sites: Task 1.
* Animation-scoped `VisibleBounds` and `VisibleTopAboveZero`, per-animation
  caches, mirror resolution, zero value for unknown, overlay parameters: Task 3.
* `AnimationSpec`, `LoadSpriteAnimations`, `LoadSprite` as wrapper, validation
  errors, one-second default: Task 4.
* Union box per animation, sorted determinism, all-transparent fallback, alpha
  fast path, unreferenced cells skipped, shelf packing, anchor rebase: Task 5.
* `LoadSpriteAutoCropped` and the cropping-is-invisible property: Task 6.
* The unmeasured startup cost the spec flags: Task 7.
* `render/doc.go`, `docs/debugging.md`, `docs/performance_optimization.md`: Tasks
  7 and 8.
* nrg and lockstep migration: Tasks 9 and 10.

Two things deliberately deferred rather than dropped, both listed as non-goals in
the spec: per-frame crop boxes, and mirroring `VisibleBounds` for flipped sprites.
