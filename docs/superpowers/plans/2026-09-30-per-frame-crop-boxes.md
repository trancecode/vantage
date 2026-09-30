# Per-frame crop boxes implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `render.LoadSpriteAutoCropped` crop every frame to its own content, with each frame recording its offset in the uncropped cell, so a sheet costs video memory in proportion to what each frame draws.

**Architecture:** `Animation.Images []*ebiten.Image` becomes `Animation.Frames []Frame`, where a `Frame` pairs an image with its `Offset` in frame space (the uncropped cell). The animation keeps one anchor (`ZeroPosition`) in frame space; the anchor a frame is drawn with is `ZeroPosition - Offset`. Task 1 introduces the types and makes every draw and measurement offset-aware while all loaders still emit zero offsets. Task 2 switches `autoCropAtlas` to per-frame boxes.

**Tech Stack:** Go, Ebitengine v2 (`github.com/hajimehoshi/ebiten/v2`), `task` for lint and tests.

**Spec:** [docs/superpowers/specs/2026-09-30-per-frame-crop-boxes-design.md](../specs/2026-09-30-per-frame-crop-boxes-design.md)

## Global constraints

* Before every commit: `export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless`. Never run bare `go test`; the `render` and `scene` packages need the virtual display `task test:headless` provides. A single test can be run with `xvfb-run -a go test ./render/ -run TestName -v`.
* Commit author is `Claude Code <herve.quiroz+claude@gmail.com>` (already configured). Do not add a `Co-Authored-By:` line. End every commit message with `Claude-Session: https://claude.ai/code/session_01JRMCVuKS29RMrESngA7MTU`.
* Commit to `main`; do not push (the controller pushes).
* Follow `docs/styleguide.md`: struct field comments go on the line above the field and start with the field name; doc comments start with the name; errors are `<context>: <reason>`.
* When removing logic, leave no comment about what was removed.
* `LoadSpriteAutoCropped` and `MustLoadSpriteAutoCropped` keep their exact signatures.
* `shelfPack` keeps its one-pixel gutter.
* An empty frame is stored as a 1x1 image taken from its cell's top-left pixel, at offset (0, 0).
* Offsets are `image.Point`; anchors stay `geometry.Vector2`.

## Review focus

1. A mirrored animation (`AnimationIdleLeft` drawn from `AnimationIdleRight`) whose frames have non-zero offsets must keep its anchor pixel on the world position under the flip. Pinned by `TestBuildDrawOpAppliesTheFrameOffset` (Task 1) and the mirrored case in `TestAutoCroppedDrawsWhereTheUncroppedSpriteWould` (Task 2).
2. `SetZeroPosition` called after `LoadSpriteAutoCropped` with the same sheet anchor must change nothing. Pinned by `TestSetZeroPositionAfterAutoCropIsANoOp` (Task 2).
3. An animation whose every frame is empty must load, and each frame must be a transparent 1x1. Pinned by `TestAutoCropStoresAnEmptyFrameAsOnePixel` (Task 2).
4. Frames of mixed sizes must still pack with the gutter and never overlap. Pinned by `TestShelfPackKeepsTheGutterForMixedSizes` (Task 2).
5. `VisibleBounds` and `VisibleTopAboveZero` of an auto-cropped sprite must equal those of the uniform load, since nrg hit-tests with them. Pinned by the extent comparisons added to `render/pixeltest` (Task 2); they can only run inside a game loop, because `ebiten.Image.At` needs one.

---

### Task 1: Frames with offsets, and offset-aware drawing

**Files:**
* Modify: `render/render_sprite.go`
* Modify: `render/render_spriteautocrop.go` (mechanical migration only, behavior unchanged)
* Modify: `scene/scene_spriteshowcase.go:90-100`
* Modify: `render/doc.go`
* Test: `render/render_sprite_test.go`, `render/render_spriteautocrop_test.go`, `render/render_filter_test.go`

**Interfaces:**
* Produces:
  * `type Frame struct { Image *ebiten.Image; Offset image.Point }`
  * `type FrameSpec struct { Rect image.Rectangle; Offset image.Point }`
  * `Animation.Frames []Frame` (replaces `Animation.Images`)
  * `AnimationSpec.Frames []FrameSpec` (replaces `[]image.Rectangle`)
  * `func (a *Animation) FrameAt(elapsed time.Duration) Frame`
  * `func (s *Sprite) buildDrawOp(p geometry.Vector2, a AnimationType, offset image.Point, requiresFlip bool, c *Camera, displayScale float64) *ebiten.DrawImageOptions`
  * `func (s *Sprite) resolveAnimation(a AnimationType) (*Animation, bool)`

- [ ] **Step 1: Write the failing tests**

Add to `render/render_sprite_test.go`:

```go
// TestFrameAtPicksTheFrameForTheElapsedTime covers the frame arithmetic every
// animated draw shares: frames split the duration evenly and the animation
// loops.
func TestFrameAtPicksTheFrameForTheElapsedTime(t *testing.T) {
	animation := &Animation{
		Frames: []Frame{
			{Offset: image.Pt(0, 0)},
			{Offset: image.Pt(1, 0)},
			{Offset: image.Pt(2, 0)},
			{Offset: image.Pt(3, 0)},
		},
		Duration: 400 * time.Millisecond,
	}
	for _, tc := range []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{99 * time.Millisecond, 0},
		{100 * time.Millisecond, 1},
		{350 * time.Millisecond, 3},
		{400 * time.Millisecond, 0},
		{520 * time.Millisecond, 1},
	} {
		if got := animation.FrameAt(tc.elapsed).Offset.X; got != tc.want {
			t.Errorf("FrameAt(%v) = frame %d, want %d", tc.elapsed, got, tc.want)
		}
	}
}

// TestFrameAtWithoutADurationShowsTheFirstFrame covers a zero duration, which
// holds the first frame rather than dividing by zero.
func TestFrameAtWithoutADurationShowsTheFirstFrame(t *testing.T) {
	animation := &Animation{Frames: []Frame{{Offset: image.Pt(7, 0)}, {Offset: image.Pt(8, 0)}}}
	if got := animation.FrameAt(time.Hour).Offset.X; got != 7 {
		t.Fatalf("FrameAt with no duration = frame at offset %d, want the first frame's 7", got)
	}
}

// TestBuildDrawOpAppliesTheFrameOffset covers the core of per-frame geometry:
// a frame stored at an offset inside its uncropped cell is drawn so that the
// animation's anchor, which is in cell coordinates, still lands on the world
// position. In the frame's own pixels that anchor is ZeroPosition - Offset.
// The mirrored case flips about that same point, so it must land there too.
func TestBuildDrawOpAppliesTheFrameOffset(t *testing.T) {
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	zero := geometry.NewVector2(10, 20)
	offset := image.Pt(3, 7)

	s := NewSprite()
	s.AddImage(AnimationIdleRight, ebiten.NewImage(8, 2))
	s.Animations[AnimationIdleRight].ZeroPosition = zero

	want := c.WorldToScreen(p)
	// The anchor pixel in the cropped frame's own coordinates.
	localX, localY := zero.X()-float64(offset.X), zero.Y()-float64(offset.Y)
	const eps = 1e-9
	for _, tc := range []struct {
		a            AnimationType
		requiresFlip bool
	}{
		{AnimationIdleRight, false},
		{AnimationIdleLeft, true},
	} {
		for _, displayScale := range []float64{1.0, 0.5} {
			op := s.buildDrawOp(p, tc.a, offset, tc.requiresFlip, c, displayScale)
			gotX, gotY := op.GeoM.Apply(localX, localY)
			if diff := gotX - want.X(); diff > eps || diff < -eps {
				t.Errorf("%s at scale %v: anchor X = %v, want %v", tc.a, displayScale, gotX, want.X())
			}
			if diff := gotY - want.Y(); diff > eps || diff < -eps {
				t.Errorf("%s at scale %v: anchor Y = %v, want %v", tc.a, displayScale, gotY, want.Y())
			}
		}
	}
}

// TestLoadSpriteAnimationsCarriesFrameOffsets covers that a spec's per-frame
// offsets reach the loaded frames unchanged, alongside the animation anchor.
func TestLoadSpriteAnimationsCarriesFrameOffsets(t *testing.T) {
	img := ebiten.NewImage(64, 64)
	s, err := LoadSpriteAnimations(img, map[AnimationType]AnimationSpec{
		AnimationIdleDown: {
			Frames: []FrameSpec{
				{Rect: image.Rect(0, 0, 8, 12), Offset: image.Pt(4, 2)},
				{Rect: image.Rect(8, 0, 12, 4), Offset: image.Pt(1, 9)},
			},
			Anchor: geometry.NewVector2(8, 16),
		},
	})
	if err != nil {
		t.Fatalf("LoadSpriteAnimations returned error: %v", err)
	}
	frames := s.Animations[AnimationIdleDown].Frames
	if got, want := frames[0].Offset, image.Pt(4, 2); got != want {
		t.Fatalf("frame 0 offset = %v, want %v", got, want)
	}
	if got, want := frames[1].Offset, image.Pt(1, 9); got != want {
		t.Fatalf("frame 1 offset = %v, want %v", got, want)
	}
	if got := frames[1].Image.Bounds().Dx(); got != 4 {
		t.Fatalf("frame 1 width = %d, want 4", got)
	}
	if got, want := s.Anchor(AnimationIdleDown), geometry.NewVector2(8, 16); got != want {
		t.Fatalf("Anchor(IdleDown) = %v, want %v", got, want)
	}
}

// TestAddImageStoresAnUncroppedFrame covers hand-built sprites, such as a
// game's own layered loader: AddImage frames sit at offset zero, so they draw
// exactly as they did before frames had offsets.
func TestAddImageStoresAnUncroppedFrame(t *testing.T) {
	s := NewSprite()
	s.AddImage(AnimationIdleDown, ebiten.NewImage(16, 16))
	if got := s.Animations[AnimationIdleDown].Frames[0].Offset; got != (image.Point{}) {
		t.Fatalf("AddImage frame offset = %v, want zero", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run 'TestFrameAt|TestBuildDrawOpAppliesTheFrameOffset|TestLoadSpriteAnimationsCarriesFrameOffsets|TestAddImageStoresAnUncroppedFrame' -v`
Expected: build failure, `undefined: Frame`, `undefined: FrameSpec`.

- [ ] **Step 3: Change the types in `render/render_sprite.go`**

Replace the `Animation` type with:

```go
// Animation represents a sequence of frames forming an animation.
type Animation struct {
	// Frames are the animation's images in order, each with where it sits in
	// the animation's frame space.
	Frames []Frame

	// Duration is how long one pass through all the frames takes.
	Duration time.Duration

	// ZeroPosition is this animation's anchor: the pixel that sits on the drawn
	// world position, in frame space. Frame space is the coordinate system of
	// one uncropped frame, such as the sheet cell a frame was cut from. Frames of
	// different animations need not share a size or an anchor.
	ZeroPosition geometry.Vector2
}

// Frame is one image of an animation and where that image sits in the
// animation's frame space. Cropping a frame to its content moves its image's
// top-left corner away from frame space's origin, and Offset records by how
// much, so the animation keeps a single anchor however each frame was cropped.
type Frame struct {
	// Image is the frame's pixels.
	Image *ebiten.Image

	// Offset is where Image's top-left pixel sits in frame space: zero for an
	// uncropped frame, the crop box's corner for a cropped one. The anchor in
	// Image's own pixels is the animation's ZeroPosition minus Offset.
	Offset image.Point
}

// FrameAt returns the frame shown once elapsed has passed since the animation
// started, looping. The frames split Duration evenly; a zero Duration holds
// the first frame. It panics on an animation with no frames.
func (a *Animation) FrameAt(elapsed time.Duration) Frame {
	index := 0
	if a.Duration > 0 {
		index = int(elapsed / (a.Duration / time.Duration(len(a.Frames))))
		index %= len(a.Frames)
	}
	return a.Frames[index]
}
```

`AddImage` appends an uncropped frame:

```go
// AddImage adds an uncropped frame, at offset zero, to the specified animation
// type.
func (s *Sprite) AddImage(animationType AnimationType, img *ebiten.Image) {
	if _, ok := s.Animations[animationType]; !ok {
		s.Animations[animationType] = &Animation{}
	}
	s.Animations[animationType].Frames = append(s.Animations[animationType].Frames, Frame{Image: img})
}
```

`Image` reads the first frame's image:

```go
func (s *Sprite) Image(animationType AnimationType) *ebiten.Image {
	if _, ok := s.Animations[animationType]; !ok || len(s.Animations[animationType].Frames) == 0 {
		if !UsePlaceholderSpriteImages {
			panic(fmt.Sprintf("no such animation type: %s", animationType))
		}
		// TODO: return a default image
		return nil
	}
	return s.Animations[animationType].Frames[0].Image
}
```

(Keep its existing doc comment.)

- [ ] **Step 4: Replace the mirror resolution helpers**

Replace `animationFrame` with `resolveAnimation` and `firstFrame`:

```go
// resolveAnimation returns the animation that a is drawn from and whether
// drawing it needs a horizontal flip: a itself when the sprite has it,
// otherwise its mirror from MirroredAnimations. It returns nil when neither
// exists.
func (s *Sprite) resolveAnimation(a AnimationType) (*Animation, bool) {
	if animation, ok := s.Animations[a]; ok {
		return animation, false
	}
	if other, mirrored := MirroredAnimations[a]; mirrored {
		if animation, ok := s.Animations[other]; ok {
			return animation, true
		}
	}
	return nil, false
}

// firstFrame returns the first frame of a, resolving a mirrored animation to
// the animation it is drawn from, and false when neither exists or has frames.
func (s *Sprite) firstFrame(a AnimationType) (Frame, bool) {
	animation, _ := s.resolveAnimation(a)
	if animation == nil || len(animation.Frames) == 0 {
		return Frame{}, false
	}
	return animation.Frames[0], true
}
```

Rewrite `Anchor` on top of it, keeping its doc comment:

```go
func (s *Sprite) Anchor(a AnimationType) geometry.Vector2 {
	animation, _ := s.resolveAnimation(a)
	if animation == nil {
		return geometry.Zero2D()
	}
	return animation.ZeroPosition
}
```

- [ ] **Step 5: Make the draw path offset-aware**

`buildDrawOp` takes the drawn frame's offset. Replace it and its doc comment with:

```go
// buildDrawOp builds the draw options for one frame of the sprite at
// world-tile position p: the tile ratio combined with displayScale, the
// frame's anchor offset, an optional horizontal flip, then the camera
// transform. The frame's anchor is the animation's anchor less the frame's
// offset in frame space.
//
// The scale and the anchor offset use the same factor, so the transform is a
// uniform scale about the anchor: the pixel anchored at the world position p
// stays at p, and only the drawn extent changes. A displayScale of 1 draws the
// sprite at the size the game draws it.
func (s *Sprite) buildDrawOp(p geometry.Vector2, a AnimationType, offset image.Point, requiresFlip bool, c *Camera, displayScale float64) *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.Filter = SpriteFilter
	scale := s.TileRatio() * displayScale
	op.GeoM.Scale(scale, scale)
	anchor := s.Anchor(a).Sub(geometry.NewVector2(offset.X, offset.Y))
	op.GeoM.Translate(-anchor.X()*scale, -anchor.Y()*scale)
	if requiresFlip {
		op.GeoM.Scale(-1, 1)
	}
	c.Adjust(op, p)
	return op
}
```

Replace the body of `Draw` (keep its doc comment):

```go
func (s *Sprite) Draw(screen *ebiten.Image, c *Camera, p geometry.Vector2, a AnimationType) {
	animation, requiresFlip := s.resolveAnimation(a)
	if animation == nil || len(animation.Frames) == 0 {
		if !UsePlaceholderSpriteImages {
			panic(fmt.Sprintf("no such animation type: %s", a))
		}
		return
	}
	frame := animation.Frames[0]
	if frame.Image == nil {
		return
	}
	screen.DrawImage(frame.Image, s.buildDrawOp(p, a, frame.Offset, requiresFlip, c, 1.0))
}
```

Replace the body of `DrawAnimationScaled` (keep its doc comment):

```go
func (s *Sprite) DrawAnimationScaled(screen *ebiten.Image, c *Camera, p geometry.Vector2, a AnimationType, duration time.Duration, displayScale float64) {
	animation, requiresFlip := s.resolveAnimation(a)
	if animation == nil {
		panic(fmt.Sprintf("no such animation type: %s", a))
	}
	if len(animation.Frames) == 0 {
		panic(fmt.Sprintf("no image for this animation type: %s", a))
	}

	frame := animation.FrameAt(duration)
	if frame.Image == nil {
		return
	}
	screen.DrawImage(frame.Image, s.buildDrawOp(p, a, frame.Offset, requiresFlip, c, displayScale))
}
```

- [ ] **Step 6: Make the measurements offset-aware**

In `VisibleBounds`, replace `img := s.animationFrame(a)` / `if img != nil {` with:

```go
	frame, ok := s.firstFrame(a)
	var result image.Rectangle
	if ok && frame.Image != nil {
		img := frame.Image
```

and replace the normalizing block with one that adds the offset:

```go
		if maxX >= minX && maxY >= minY {
			// Normalize to the frame's own origin, then place it in frame space.
			result = image.Rect(
				minX-bounds.Min.X, minY-bounds.Min.Y,
				maxX-bounds.Min.X+1, maxY-bounds.Min.Y+1,
			).Add(frame.Offset)
		}
```

(The existing code names the image bounds `frame`; rename that local to `bounds` since `frame` is now the `Frame`.) Update its doc comment's coordinate sentence to: "expressed in frame space, the same coordinates as Anchor, so a hit test combines the two directly." Keep the mirroring paragraph.

In `VisibleTopAboveZero`, replace `img := s.animationFrame(a)` / `if img != nil {` / `anchorY := s.Anchor(a).Y()` with:

```go
	frame, ok := s.firstFrame(a)
	result := 0.0
	if ok && frame.Image != nil {
		img := frame.Image
		// The anchor in this frame's own pixels, which is what the row index
		// below is measured in.
		anchorY := s.Anchor(a).Y() - float64(frame.Offset.Y)
```

and update the inner comment to say "Both it and anchorY are in the frame's own source pixels".

- [ ] **Step 7: Migrate `AnimationSpec` and the loaders**

Replace `AnimationSpec` and its doc comment with:

```go
// AnimationSpec describes one animation's geometry within an image: where its
// frames are, where each sits in frame space, where the anchor is, and how
// long it runs. It is the load-time description of an animation, where
// Animation is the loaded form; the two differ in that a spec names frames as
// rectangles into a source image while an Animation holds uploaded textures.
//
// Frames need not be the same size as each other or as another animation's,
// which is what lets a sheet be packed with one crop box per frame.
type AnimationSpec struct {
	// Frames are this animation's frames, in order.
	Frames []FrameSpec

	// Anchor is the pixel that sits on the drawn world position, in frame
	// space.
	Anchor geometry.Vector2

	// Duration is how long the whole animation runs. Zero means one second.
	Duration time.Duration
}

// FrameSpec is one frame of an AnimationSpec: its source rectangle and where
// that rectangle's top-left sits in frame space.
type FrameSpec struct {
	// Rect is the frame's source rectangle in the image.
	Rect image.Rectangle

	// Offset is where Rect's top-left pixel sits in frame space, zero for an
	// uncropped frame.
	Offset image.Point
}
```

In `LoadSpriteAnimations`, replace the per-spec loop body with:

```go
		spec := specs[a]
		if len(spec.Frames) == 0 {
			return nil, fmt.Errorf("animation %s: no frames", a)
		}
		frames := make([]Frame, 0, len(spec.Frames))
		for i, f := range spec.Frames {
			if f.Rect.Empty() {
				return nil, fmt.Errorf("animation %s frame %d: empty rectangle %v", a, i, f.Rect)
			}
			if !f.Rect.In(bounds) {
				return nil, fmt.Errorf("animation %s frame %d: rectangle %v is outside the image bounds %v", a, i, f.Rect, bounds)
			}
			frames = append(frames, Frame{Image: img.SubImage(f.Rect).(*ebiten.Image), Offset: f.Offset})
		}

		duration := spec.Duration
		if duration == 0 {
			duration = time.Second
		}
		sprite.Animations[a] = &Animation{Frames: frames, Duration: duration, ZeroPosition: spec.Anchor}
```

Update its doc comment's first paragraph to say each animation carries "its own frame rectangles and offsets, anchor and duration".

In `LoadSprite`, build `[]FrameSpec`:

```go
		frames := make([]FrameSpec, 0, len(animationIndexes))
		for _, index := range animationIndexes {
			x := (index % width) * tileWidth
			y := (index / width) * tileHeight
			frames = append(frames, FrameSpec{Rect: image.Rect(x, y, x+tileWidth, y+tileHeight)})
		}
```

- [ ] **Step 8: Migrate `autoCropAtlas` mechanically**

In `render/render_spriteautocrop.go`, keep the per-animation union box and rebased anchor for now (Task 2 changes the behavior). Only the types change:

```go
		frames := make([]FrameSpec, 0, len(frameIndexes))
		for _, index := range frameIndexes {
			source := box.Add(cellAt(index).Min)
			frames = append(frames, FrameSpec{Rect: source})
			pending = append(pending, placement{source: source})
		}
```

and in the rewrite loop: `spec.Frames[i].Rect = placed[next].dest`.

- [ ] **Step 9: Migrate the showcase**

In `scene/scene_spriteshowcase.go`'s `showcaseFitScale`, iterate frames:

```go
	for _, animation := range sprite.Animations {
		for _, frame := range animation.Frames {
			if frame.Image == nil {
				continue
			}
			bounds := frame.Image.Bounds()
			artPixels = max(artPixels, float64(max(bounds.Dx(), bounds.Dy()))*drawnScale)
		}
	}
```

- [ ] **Step 10: Migrate the existing tests**

* Every `buildDrawOp(p, a, requiresFlip, c, scale)` call in `render/render_sprite_test.go`, `render/render_filter_test.go` and `render/render_spriteautocrop_test.go` gains `image.Point{}` as its third argument, since those sprites' frames are uncropped. (Task 2 rewrites the auto-crop test's call.)
* `.Images[i]` becomes `.Frames[i].Image` and `len(...Images)` becomes `len(...Frames)`.
* In `render_sprite_test.go`, spec literals become `[]FrameSpec{{Rect: image.Rect(...)}, ...}`, and the bad-geometry cases become `AnimationSpec{Frames: []FrameSpec{{Rect: image.Rect(4, 4, 4, 4)}}}` and `AnimationSpec{Frames: []FrameSpec{{Rect: image.Rect(0, 0, 128, 128)}}}`.
* In `render_spriteautocrop_test.go`, `.Frames[0].Dx()` becomes `.Frames[0].Rect.Dx()` (and likewise `.Dy()`, `.Min`, and whole-rectangle comparisons use `.Rect`).
* The existing `TestBuildDrawOpUsesTheAnimationAnchor` keeps its meaning with `image.Point{}`.

- [ ] **Step 11: Update `render/doc.go`**

Replace the sentence starting "The anchor that a sprite is drawn and hit-tested against is per animation:" through "drawn from." with:

```go
// An Animation's frames each carry an Offset: where the frame's image sits in
// the animation's frame space, the uncropped cell it was cut from. The anchor
// a sprite is drawn and hit-tested against is per animation and in frame
// space, so a frame cropped to its content is drawn with the anchor less its
// offset, and VisibleBounds reports frame space too. SetZeroPosition sets one
// anchor across every animation on the sprite, which is what a uniform sheet
// wants, and Anchor reads the resolved value for a given AnimationType,
// resolving a mirrored animation to the animation it is drawn from.
```

- [ ] **Step 12: Run the full checks**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless`
Expected: all pass, including the new tests and the untouched `render/pixeltest` A/B test (every offset is still zero).

- [ ] **Step 13: Commit**

```bash
git add render/ scene/
git commit -m "Give animation frames an offset in frame space

Animation.Images becomes Animation.Frames, a slice of Frame that pairs each
image with where it sits in the uncropped cell, and AnimationSpec.Frames
becomes a slice of FrameSpec. Drawing and the visible-extent measurements
use the anchor less the frame's offset. Every loader still emits zero
offsets, so nothing draws differently yet.

Animation.FrameAt exposes the frame arithmetic the draw path uses.

Callers that read Animation.Images move to Animation.Frames[i].Image.

Claude-Session: https://claude.ai/code/session_01JRMCVuKS29RMrESngA7MTU"
```

---

### Task 2: Crop every frame to its own box

**Files:**
* Modify: `render/render_spriteautocrop.go`
* Modify: `render/pixeltest/pixeltest_test.go`, `render/pixeltest/pixeltest_mask_test.go`, `render/pixeltest/pixeltest_game_test.go`
* Modify: `docs/debugging.md` (the `render/pixeltest` section), `docs/performance_optimization.md` (the auto-crop section)
* Modify: `render/doc.go` (the `LoadSpriteAutoCropped` sentence)
* Test: `render/render_spriteautocrop_test.go`

**Interfaces:**
* Consumes (from Task 1): `Frame`, `FrameSpec`, `Animation.Frames`, `Animation.FrameAt`, `buildDrawOp(p, a, offset, requiresFlip, c, displayScale)`.
* Produces: `autoCropAtlas` specs whose `Anchor` is the sheet anchor unchanged and whose `FrameSpec.Offset` is each frame's crop-box corner in its cell.

- [ ] **Step 1: Rewrite the auto-crop unit tests for per-frame boxes**

In `render/render_spriteautocrop_test.go`:

Replace `TestAutoCropTightensEachAnimation` with:

```go
// TestAutoCropTightensEachFrame covers the core measurement: each frame gets a
// crop box around its own content and records the box's corner as its offset,
// while the animation keeps the sheet-wide anchor, so the drawn result is
// unchanged.
func TestAutoCropTightensEachFrame(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	down := specs[AnimationIdleDown]
	for i, want := range []struct {
		width, height int
		offset        image.Point
	}{
		{8, 8, image.Pt(4, 4)},
		{4, 4, image.Pt(2, 2)},
	} {
		frame := down.Frames[i]
		if frame.Rect.Dx() != want.width || frame.Rect.Dy() != want.height {
			t.Fatalf("frame %d size = %dx%d, want %dx%d", i, frame.Rect.Dx(), frame.Rect.Dy(), want.width, want.height)
		}
		if frame.Offset != want.offset {
			t.Fatalf("frame %d offset = %v, want %v", i, frame.Offset, want.offset)
		}
	}
	if got, want := down.Anchor, geometry.NewVector2(8, 16); got != want {
		t.Fatalf("anchor = %v, want the sheet anchor %v", got, want)
	}

	srcArea := 32 * 32
	atlasArea := atlas.Bounds().Dx() * atlas.Bounds().Dy()
	if atlasArea >= srcArea {
		t.Fatalf("atlas area %d is not smaller than the source area %d", atlasArea, srcArea)
	}
}
```

Delete `TestAutoCropUnionsFramesOfOneAnimation` (its behavior no longer exists) and `TestAutoCropFallsBackForAnAllTransparentAnimation` (replaced below).

Replace `TestAutoCropAnchorRebaseResistsTransposition` with:

```go
// TestAutoCropOffsetResistsTransposition covers the offset recorded for a crop
// box. A box whose width equals its height and whose corner sits on the
// diagonal cannot tell an offset from one with X and Y swapped. This fixture's
// box is 8x2 and starts off the diagonal at (3,7), so a transposed offset
// reads back as (7,3) and fails.
func TestAutoCropOffsetResistsTransposition(t *testing.T) {
	_, specs, err := autoCropAtlas(autoCropAsymmetricTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0},
	}, nil, geometry.NewVector2(20, 30))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	frame := specs[AnimationIdleDown].Frames[0]
	if frame.Rect.Dx() != 8 || frame.Rect.Dy() != 2 {
		t.Fatalf("crop size = %dx%d, want 8x2", frame.Rect.Dx(), frame.Rect.Dy())
	}
	if got, want := frame.Offset, image.Pt(3, 7); got != want {
		t.Fatalf("offset = %v, want %v", got, want)
	}
	if got, want := specs[AnimationIdleDown].Anchor, geometry.NewVector2(20, 30); got != want {
		t.Fatalf("anchor = %v, want the sheet anchor %v", got, want)
	}
}
```

Add:

```go
// TestAutoCropStoresAnEmptyFrameAsOnePixel covers frames with nothing drawn,
// such as the blank tail of a death animation: each becomes a single
// transparent pixel at offset zero rather than a full cell, whether or not the
// rest of its animation has content.
func TestAutoCropStoresAnEmptyFrameAsOnePixel(t *testing.T) {
	atlas, specs, err := autoCropAtlas(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown:  {0, 2},
		AnimationIdleRight: {2, 2},
	}, nil, geometry.NewVector2(8, 16))
	if err != nil {
		t.Fatalf("autoCropAtlas returned error: %v", err)
	}

	for _, frame := range []FrameSpec{
		specs[AnimationIdleDown].Frames[1],
		specs[AnimationIdleRight].Frames[0],
		specs[AnimationIdleRight].Frames[1],
	} {
		if frame.Rect.Dx() != 1 || frame.Rect.Dy() != 1 {
			t.Fatalf("empty frame size = %dx%d, want 1x1", frame.Rect.Dx(), frame.Rect.Dy())
		}
		if frame.Offset != (image.Point{}) {
			t.Fatalf("empty frame offset = %v, want zero", frame.Offset)
		}
		if _, _, _, a := atlas.At(frame.Rect.Min.X, frame.Rect.Min.Y).RGBA(); a != 0 {
			t.Fatalf("empty frame pixel at %v has alpha %d, want transparent", frame.Rect.Min, a)
		}
	}

	// An animation with no content at all still loads.
	if _, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleRight: {2},
	}, nil, geometry.Zero2D()); err != nil {
		t.Fatalf("LoadSpriteAutoCropped of an all-empty animation returned error: %v", err)
	}
}

// TestShelfPackKeepsTheGutterForMixedSizes covers shelving frames whose sizes
// differ, which per-frame crop boxes make the normal case: every frame keeps
// its own size, stays inside the atlas, and never touches another.
func TestShelfPackKeepsTheGutterForMixedSizes(t *testing.T) {
	sizes := []image.Point{{5, 5}, {3, 7}, {1, 1}, {8, 2}, {4, 4}, {6, 3}, {2, 9}}
	frames := make([]placement, len(sizes))
	for i, size := range sizes {
		frames[i] = placement{source: image.Rect(0, 0, size.X, size.Y)}
	}

	atlas, placed := shelfPack(frames)
	for i, p := range placed {
		if p.dest.Size() != sizes[i] {
			t.Fatalf("frame %d placed at size %v, want %v", i, p.dest.Size(), sizes[i])
		}
		if !p.dest.In(atlas.Bounds()) {
			t.Fatalf("frame %d at %v is outside the atlas %v", i, p.dest, atlas.Bounds())
		}
		for j := i + 1; j < len(placed); j++ {
			if p.dest.Inset(-1).Overlaps(placed[j].dest) {
				t.Fatalf("frame %d %v and frame %d %v are adjacent or overlapping", i, p.dest, j, placed[j].dest)
			}
		}
	}
}

// TestSetZeroPositionAfterAutoCropIsANoOp covers the hazard per-frame anchors
// would have created: the auto-cropped anchor is the sheet anchor in cell
// coordinates, so setting that same anchor again afterwards changes neither
// the anchor nor where any frame draws.
func TestSetZeroPositionAfterAutoCropIsANoOp(t *testing.T) {
	anchor := geometry.NewVector2(8, 16)
	s, err := LoadSpriteAutoCropped(autoCropTestSheet(), 2, 2, map[AnimationType][]int{
		AnimationIdleDown: {0, 1},
	}, nil, anchor)
	if err != nil {
		t.Fatalf("LoadSpriteAutoCropped returned error: %v", err)
	}
	c := drawOpTestCamera()
	p := geometry.NewVector2(3, 5)
	frames := s.Animations[AnimationIdleDown].Frames
	before := make([]*ebiten.DrawImageOptions, len(frames))
	for i, frame := range frames {
		before[i] = s.buildDrawOp(p, AnimationIdleDown, frame.Offset, false, c, 1.0)
	}

	s.SetZeroPosition(anchor)

	if got := s.Anchor(AnimationIdleDown); got != anchor {
		t.Fatalf("Anchor after SetZeroPosition = %v, want %v", got, anchor)
	}
	for i, frame := range frames {
		if after := s.buildDrawOp(p, AnimationIdleDown, frame.Offset, false, c, 1.0); !geoMEquals(after, before[i]) {
			t.Fatalf("frame %d draw moved: %v, want %v", i, after.GeoM, before[i].GeoM)
		}
	}
}
```

Rewrite `TestAutoCroppedDrawsWhereTheUncroppedSpriteWould` so it tracks sheet pixels frame by frame, with the crop-box corners still hardcoded from the fixture, and covers the mirrored animation:

```go
// TestAutoCroppedDrawsWhereTheUncroppedSpriteWould is the property the whole
// change has to preserve: a given pixel of the source sheet lands on the same
// screen point whether the sprite was loaded uniformly or auto-cropped, so the
// repack is invisible in the game.
//
// This tracks a sheet pixel through both load paths rather than comparing an
// anchor to itself. In the uniform sprite a frame is the whole cell, so a pixel
// at cell-local (qx, qy) is at frame-local (qx, qy). In the cropped sprite the
// same pixel is at frame-local (qx-originX, qy-originY), where origin is that
// frame's crop box corner in the cell. origin is hardcoded from
// autoCropTestSheet's fixture rather than read back from autoCropAtlas, so a
// wrong offset cannot cancel itself out: cell 0's content is an 8x8 block at
// cell-local (4,4), cell 1's a 4x4 block at cell-local (2,2).
func TestAutoCroppedDrawsWhereTheUncroppedSpriteWould(t *testing.T) {
	sheet := autoCropTestSheet()
	// IdleDown's two frames crop to different boxes; IdleLeft is drawn by
	// flipping IdleRight.
	indexes := map[AnimationType][]int{
		AnimationIdleDown:  {0, 1},
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

	// probes are cell-local points inside the frame's box: its corner, so an
	// offset error shows up, and a second point, so a scale error does too.
	cases := []struct {
		a            AnimationType
		frame        int
		requiresFlip bool
		origin       image.Point
		probes       []image.Point
	}{
		{AnimationIdleDown, 0, false, image.Pt(4, 4), []image.Point{{4, 4}, {11, 11}}},
		{AnimationIdleDown, 1, false, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
		{AnimationIdleRight, 0, false, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
		{AnimationIdleLeft, 0, true, image.Pt(2, 2), []image.Point{{2, 2}, {5, 5}}},
	}

	for _, tc := range cases {
		source, _ := cropped.resolveAnimation(tc.a)
		offset := source.Frames[tc.frame].Offset
		uniformOp := uniform.buildDrawOp(p, tc.a, image.Point{}, tc.requiresFlip, c, 1.0)
		croppedOp := cropped.buildDrawOp(p, tc.a, offset, tc.requiresFlip, c, 1.0)

		for _, q := range tc.probes {
			wantX, wantY := uniformOp.GeoM.Apply(float64(q.X), float64(q.Y))
			gotX, gotY := croppedOp.GeoM.Apply(float64(q.X-tc.origin.X), float64(q.Y-tc.origin.Y))
			if diff := gotX - wantX; diff > eps || diff < -eps {
				t.Errorf("%s frame %d: sheet pixel %v X = %v, want %v", tc.a, tc.frame, q, gotX, wantX)
			}
			if diff := gotY - wantY; diff > eps || diff < -eps {
				t.Errorf("%s frame %d: sheet pixel %v Y = %v, want %v", tc.a, tc.frame, q, gotY, wantY)
			}
		}
	}
}
```

Update `TestAutoCropIsReproducible` to compare every frame (`for i := range spec.Frames { if spec.Frames[i] != firstSpecs[a].Frames[i] ...`), since `FrameSpec` is comparable with `!=`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -run 'TestAutoCrop|TestShelfPack|TestSetZeroPositionAfterAutoCrop' -v`
Expected: `TestAutoCropTightensEachFrame`, `TestAutoCropOffsetResistsTransposition`, `TestAutoCropStoresAnEmptyFrameAsOnePixel` and `TestAutoCroppedDrawsWhereTheUncroppedSpriteWould` fail (frames still share the union box, offsets are zero). `TestShelfPackKeepsTheGutterForMixedSizes` and `TestSetZeroPositionAfterAutoCropIsANoOp` may already pass; that is fine, they pin behavior that must survive.

- [ ] **Step 3: Crop per frame in `autoCropAtlas`**

Replace the per-animation loop body in `render/render_spriteautocrop.go` with:

```go
	for _, a := range sortedAnimationTypes(indexes) {
		frameIndexes := indexes[a]
		if len(frameIndexes) == 0 {
			continue
		}

		frames := make([]FrameSpec, 0, len(frameIndexes))
		for _, index := range frameIndexes {
			if index < 0 || index >= columns*rows {
				return nil, nil, fmt.Errorf("animation %s: frame index %d is outside a %dx%d grid", a, index, columns, rows)
			}
			cell := cellAt(index)
			box, ok := cropBoxIn(alphaAt, cell)
			if !ok {
				// Nothing is drawn in this frame. The cell's corner pixel is
				// transparent, so one pixel of it stands in for the frame at
				// almost no cost, where a full cell would charge a blank frame
				// for padding.
				box = image.Rect(0, 0, 1, 1)
			}
			source := box.Add(cell.Min)
			frames = append(frames, FrameSpec{Rect: source, Offset: box.Min})
			pending = append(pending, placement{source: source})
		}
		specs[a] = AnimationSpec{
			Frames:   frames,
			Anchor:   anchor,
			Duration: durations[a],
		}
	}
```

Update `autoCropAtlas`'s doc comment first paragraph to:

```go
// autoCropAtlas measures a tight crop box per frame over the uniform grid
// described by columns, rows and indexes, and packs every referenced frame into
// a new atlas at its own box's size. Each frame records its box's corner as its
// offset in the cell, and every animation keeps anchor, which is already in
// cell coordinates, unchanged.
```

In `shelfPack`'s doc comment, replace "Frames of one animation all share a size, so they shelf neatly, and the win being chased here is dropping transparent padding rather than the last few percent of packing efficiency." with "Tallest-first shelving keeps each shelf's wasted height down even for frames of mixed sizes, and the win being chased here is dropping transparent padding rather than the last few percent of packing efficiency."

Replace `LoadSpriteAutoCropped`'s doc comment first paragraph with:

```go
// LoadSpriteAutoCropped builds a sprite from a uniform sheet, cropping each
// frame to its own content and repacking the frames into a smaller texture
// before upload. anchor is the sheet-wide anchor in cell-local pixels, which is
// the sprite's frame space: every animation keeps it as its anchor, and each
// frame records where its crop box sat in its cell. SetZeroPosition afterwards
// therefore behaves as it would on the uniform sheet.
```

- [ ] **Step 4: Run the auto-crop tests**

Run: `export GOMODCACHE=/tmp/go-mod-cache && xvfb-run -a go test ./render/ -v -run 'TestAutoCrop|TestShelfPack|TestSetZeroPosition|TestLoadSpriteAutoCropped'`
Expected: PASS.

- [ ] **Step 5: Make the pixel A/B test frame-aware**

`croppedQuadOnScreen` must measure the frame actually drawn at the scenario's elapsed time and include its offset. In `render/pixeltest/pixeltest_mask_test.go` replace it with:

```go
// croppedQuadOnScreen computes the on-screen rectangle
// [render.Sprite.DrawAnimationScaled] draws animation a into at elapsed, for
// the given camera, position and scale. It replicates Sprite.buildDrawOp's
// math, which is unexported, from exported API: the frame
// [render.Animation.FrameAt] picks, its Offset, [render.Sprite.Anchor],
// [render.Sprite.TileRatio] and [render.Camera.Adjust]. A mirrored animation is
// resolved the way DrawAnimationScaled resolves it.
//
// This is what a comparison masks down to for FilterLinear scenarios that
// compare a padded frame against a genuinely smaller cropped one: see
// buildScenarios for why the region outside this quad is expected to differ.
func croppedQuadOnScreen(sprite *render.Sprite, camera *render.Camera, pos geometry.Vector2, a render.AnimationType, elapsed time.Duration, scale float64) image.Rectangle {
	drawFrom := a
	requiresFlip := false
	if !sprite.HasAnimation(a) {
		if mirror, ok := render.MirroredAnimations[a]; ok {
			drawFrom = mirror
			requiresFlip = true
		}
	}
	frame := sprite.Animations[drawFrom].FrameAt(elapsed)
	bounds := frame.Image.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())

	effectiveScale := sprite.TileRatio() * scale
	anchor := sprite.Anchor(a).Sub(geometry.NewVector2(frame.Offset.X, frame.Offset.Y))

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(effectiveScale, effectiveScale)
	op.GeoM.Translate(-anchor.X()*effectiveScale, -anchor.Y()*effectiveScale)
	if requiresFlip {
		op.GeoM.Scale(-1, 1)
	}
	camera.Adjust(op, pos)

	x0, y0 := op.GeoM.Apply(0, 0)
	x1, y1 := op.GeoM.Apply(w, h)
	return image.Rect(
		int(math.Round(min(x0, x1))), int(math.Round(min(y0, y1))),
		int(math.Round(max(x0, x1))), int(math.Round(max(y0, y1))),
	)
}
```

(add `"time"` to its imports) and in `compareMasked` pass `sc.elapsed`: `croppedQuadOnScreen(cropped, camera, pos, sc.animation, sc.elapsed, sc.scale)`.

- [ ] **Step 6: Compare visible extents inside the game loop**

`VisibleBounds` and `VisibleTopAboveZero` read pixels with `ebiten.Image.At`, which only works inside a running game, so compare them in `comparisonGame.Draw`. In `render/pixeltest/pixeltest_game_test.go`:

Add a field to `comparisonGame`:

```go
	// extentMismatches describes every animation whose visible extent differs
	// between the two sprites, filled in by Draw.
	extentMismatches []string
```

At the end of `Draw`, before `g.done = true`:

```go
	// The visible-extent queries are what a game hit-tests and places
	// nameplates with, and they must not notice the crop either. They are in
	// frame space, so the uniform and cropped answers are directly comparable.
	for _, a := range []render.AnimationType{
		render.AnimationIdleDown, render.AnimationIdleRight, render.AnimationIdleLeft,
		render.AnimationAttackDown, render.AnimationAttackRight,
	} {
		if want, got := g.uniform.VisibleBounds(a), g.cropped.VisibleBounds(a); got != want {
			g.extentMismatches = append(g.extentMismatches, fmt.Sprintf("%s VisibleBounds = %v, want %v", a, got, want))
		}
		if want, got := g.uniform.VisibleTopAboveZero(a), g.cropped.VisibleTopAboveZero(a); math.Abs(got-want) > 1e-9 {
			g.extentMismatches = append(g.extentMismatches, fmt.Sprintf("%s VisibleTopAboveZero = %v, want %v", a, got, want))
		}
	}
```

(add `"math"` to its imports). In `TestAutoCroppedSpriteRendersIdenticallyToUniform`, after the scenario loop:

```go
	for _, m := range game.extentMismatches {
		t.Errorf("visible extent differs between uniform and auto-cropped: %s", m)
	}
```

- [ ] **Step 7: Update the fixture and scenario comments**

In `render/pixeltest/pixeltest_test.go` the fixture already gives `AnimationIdleDown`'s two frames content of different sizes at different offsets, which is exactly what per-frame crops need; only the prose is stale. Rewrite:

* In `buildFixtureSheet`'s comment, the first bullet: cells 0 and 1 are two frames of `AnimationIdleDown` "with content of different sizes at different cell-local offsets, so each crops to its own box and the two frames are stored at different sizes and offsets". Drop the union wording.
* The cell 1 inline comment: replace the union sentence with "Frame 0's content is (2,5)-(10,9) and frame 1's is (6,9)-(18,17), so the two frames crop to different sizes at different offsets."
* In `fixtureAnchor`'s comment: "Both loaders rebase it into per-animation coordinates their own way" becomes "The uniform sprite draws every frame against it directly; the cropped sprite draws each frame against it less that frame's offset".
* In `buildScenarios`' comment: "an off-diagonal box, a union of two frames, and a mirrored flip" becomes "an off-diagonal box, two frames of one animation cropped to different boxes, and a mirrored flip". Replace the final paragraph ("AnimationIdleDown's two frames are drawn separately ...") with: "AnimationIdleDown's two frames are drawn separately rather than as one scenario, because they are cropped to different boxes at different offsets: each needs its own quad and its own offset checked."

- [ ] **Step 8: Update the documentation**

* `render/doc.go`: replace "LoadSpriteAutoCropped crops each animation to its own content, repacks the frames into a smaller image before upload, and derives each animation's anchor from a sheet-wide one," with "LoadSpriteAutoCropped crops each frame to its own content, repacks the frames into a smaller image before upload, and records each frame's offset in its cell so the sheet-wide anchor serves every frame,".
* `docs/debugging.md`, `render/pixeltest` section: after the sentence ending "compares them with `visualtest.CompareImages`.", add: "It also compares `VisibleBounds` and `VisibleTopAboveZero` between the two sprites, which must agree because both are reported in frame space; those read pixels back and so can only run inside the game loop." Read the rest of that section and correct anything that still says a crop box is shared across an animation's frames.
* `docs/performance_optimization.md`, "Auto-crop startup scan cost" section: "to find a tight crop box per animation" becomes "to find a tight crop box per frame". The scan still visits each referenced cell once, so the benchmark figures stand.

- [ ] **Step 9: Run the full checks**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless`
Expected: all pass, including `render/pixeltest`'s `TestAutoCroppedSpriteRendersIdenticallyToUniform` with the new extent checks.

- [ ] **Step 10: Commit**

```bash
git add render/ docs/debugging.md docs/performance_optimization.md
git commit -m "Crop each auto-cropped frame to its own box

autoCropAtlas now crops every frame to its own content instead of every
animation to the union of its frames. Each frame records its crop box's
corner as its offset in the cell, and every animation keeps the sheet-wide
anchor unchanged. An empty frame is stored as one transparent pixel.

Because the anchor stays in cell coordinates, SetZeroPosition after
LoadSpriteAutoCropped with the same anchor is now a no-op rather than a
silent misplacement. LoadSpriteAutoCropped's signature is unchanged.

Claude-Session: https://claude.ai/code/session_01JRMCVuKS29RMrESngA7MTU"
```
