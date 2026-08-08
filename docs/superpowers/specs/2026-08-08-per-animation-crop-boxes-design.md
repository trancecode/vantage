# Per-animation crop boxes and anchors design

## Purpose

Let each animation in a sprite carry its own frame geometry and its own anchor,
and let the engine crop a uniform sheet down to that geometry at load time, so a
sheet costs video memory in proportion to what it draws rather than to its
widest pose.

A sprite sheet is a grid of equally sized cells, one animation frame per cell.
Because the cells are uniform, the widest pose in the sheet decides the cell
size for every other pose: a sword swung out sideways inflates the cell a
standing-still frame is stored in. The character occupies a small part of most
cells and the rest is transparent padding.

Padding is free on disk, because PNG compresses it away. It is not free in video
memory, because a texture stores every pixel whether or not anything is drawn
there. Measured on the sheets published as [sprite_generation_pipeline
v0.1.0](https://github.com/herve-quiroz/sprite_generation_pipeline/releases/tag/v0.1.0),
one 7296x10624 sheet is 77.5 million pixels of which 4.0% are non-transparent,
and costs 310 MB as an uncompressed RGBA8 texture. Ebitengine uploads
uncompressed, so that is the resident cost.

The engine cannot express the fix today. `Sprite` holds one anchor for all
animations, `LoadSprite` derives one cell size for the whole sheet, and the
visible-extent helpers cache a single answer computed from an arbitrary frame.

## Vocabulary

An *animation* is a run of frames the engine keys by `render.AnimationType`,
such as walking while facing north-east. It is the granularity this design works
at. The sprite pipeline and lockstep call the same thing a *clip*; this document
uses "animation" throughout, since that is the engine's own word.

One animation has three representations across its life, and they are
deliberately named as one family. `AnimationType` names it, `AnimationSpec`
describes it as rectangles into a source image, and `Animation` is the loaded
form holding uploaded textures.

An *anchor* is the pixel inside a frame that sits on the drawn world position.
For a character standing on a tile it is the point between their feet. The
engine calls it `ZeroPosition`. Moving it slides the character relative to their
tile.

An animation's *crop box* is the tight rectangle around every non-transparent
pixel in any of its frames, expressed in cell-local coordinates. One box per
animation, shared by all its frames, which is what keeps the anchor per
animation rather than per frame.

An *atlas* is the repacked image the cropped frames are copied into. It is a
new, smaller image, not a view onto the original.

## Granularity: per animation, not per frame

Measured across the published sheets, per-animation crops take the resident cost
of six sheets from about 2.05 GB to about 285 MB. Per-frame crops would reach
about 29 MB per sheet against 47 MB per sheet for per-animation, a further gain
of roughly 15 MB per sheet, in exchange for every frame carrying its own
geometry and anchor.

Per-animation captures 86% of the available saving for a single geometry per
animation. This design is per-animation. `AnimationSpec.Frames` is a list of
rectangles rather than one rectangle plus a count, so per-frame crops remain
expressible later without a second API, but nothing in this design produces
them.

## Architecture

### The anchor moves onto the animation

`Animation` gains the anchor, as a plain value:

```go
type Animation struct {
    Images       []*ebiten.Image
    Duration     time.Duration
    ZeroPosition geometry.Vector2
}
```

`Sprite.ZeroPosition` is deleted. There is exactly one place an anchor lives, so
there is no optional-anchor pointer, no draw-time fallback, and no second
convention to keep consistent.

`SetZeroPosition` survives as the uniform-sheet convenience it always was: it
writes the anchor to every animation currently on the sprite. That keeps nrg's
`MustLoadSprite(...).SetZeroPosition(v)` reading exactly as it does now, and
meaning the same thing for a sheet where every frame shares an anchor.

Deleting the sprite-level field introduces one ordering hazard.
`NewSprite().SetZeroPosition(v)` followed by `AddImage` would write the anchor
to nothing and then add an animation without one. `SetZeroPosition` therefore
panics when the sprite has no animations, because that call is unambiguously
misordered and the package already panics freely in `Image` and
`DrawAnimationScaled`. Construction order is frames first, anchor second.

Reads go through a new accessor rather than the field:

```go
// Anchor returns the anchor of the given animation, resolving a mirrored
// animation to the animation it is drawn from.
func (s *Sprite) Anchor(a AnimationType) geometry.Vector2
```

The alternative considered was making the anchor a required loader parameter,
which removes the ordering hazard entirely by making the anchor arrive with the
frames. It is rejected because it pushes `MustLoadSprite` to six positional
arguments with the anchor stranded after a large map literal, which reads worse
at every nrg registration site than the chained setter does.

### Sprite.Scale is removed

`Sprite.Scale` and `SetScale` are deleted.

`Scale` is a bare multiplier with no stated reference point, which the sprite
tile size design already identified as the problem `SourceTileSize` exists to
solve. Any fixed multiplier `k` is expressible as a source tile size of
`TileSize / k`, and that version is better because it tracks the configured tile
size instead of going stale when the tile size changes. The only thing lost is
"scale this sprite by k whatever the tile size", which is the thing that design
argues you should not want. Per-draw view scaling is unaffected: it lives in
`DrawAnimationScaled`'s `displayScale`.

Removing it also resolves an ambiguity this design would otherwise have to
encode. `ZeroPosition` is currently measured in *Scale-applied* pixels, because
`buildDrawOp` translates by `-zero * ratio * displayScale` and deliberately
omits `s.Scale`, so the anchored source pixel is `ZeroPosition / Scale`.
Auto-cropping rebases the anchor by a crop origin, which is a plain
source-pixel offset, and the two units only agree when `Scale` is 1. With
`Scale` gone, `buildDrawOp` computes `scale := ratio * displayScale` and the
anchor translate uses that same factor, so anchor units and source pixels agree
by construction.

`Scale` is 1 in both consumers today, so nothing renders differently. nrg's only
`SetScale` call is `SetScale(1.0)` at `sprites/sprites.go:83`, and lockstep
never sets it. Four places inside vantage use it and change here:

* `cmd/showcasedemo/main.go:114` sizes its placeholder sprites with `SetScale`
  so the fixed-slot scale-down-to-fit behaviour is visible. It moves to
  `SetSourceTileSize`, which demonstrates the supported mechanism instead of the
  removed one: a 32 pixel placeholder declaring a source tile size of 8 draws
  four tiles wide.
* `scene/scene_spriteshowcase.go:89` measures a sprite at the size the engine
  draws it, as `sprite.Scale * sprite.TileRatio()`. It becomes
  `sprite.TileRatio()`.
* `render/render_sprite.go:308`, inside `VisibleTopAboveZero`, multiplies the
  row offset by `s.Scale` to reconcile the two unit conventions. With one
  convention the multiply disappears.
* Vantage's own tests seed `SetScale(2)` and `SetScale(3)`, and move to
  `SetSourceTileSize`, which they already use elsewhere for the same purpose.

### The draw path resolves the anchor per animation

`buildDrawOp` takes the animation that supplies the frames, and reads the anchor
from that animation. The distinction matters for mirroring: drawing
`AnimationIdleLeft` from `AnimationIdleRight` must use the right-facing
animation's anchor, because those are the frames on screen.

The existing transform order already mirrors about the anchor, since the
translate puts the anchor at the origin before `Scale(-1, 1)`. Substituting a
per-animation anchor preserves that with no further change.

### Animation-scoped geometry queries

```go
func (s *Sprite) VisibleBounds(a AnimationType) image.Rectangle
func (s *Sprite) VisibleTopAboveZero(a AnimationType) float64
```

Once animations have different crop sizes there is a correct answer per
animation and no correct answer per sprite, so the argument is required rather
than optional. The caches become maps keyed by animation type.
`VisibleTopAboveZero` keeps caching the pre-ratio value and applying `TileRatio`
fresh on every call, so a tile size changed after the first call still takes
effect.

Both resolve through `MirroredAnimations` the way `Draw` does, so asking for a
mirrored animation returns the source animation's geometry rather than failing.
An animation the sprite does not have returns the zero value rather than
panicking; these are queries, not draws.

`DrawNameplate` and `DrawFloatingBar` gain the same parameter and pass it
through. Without it a nameplate would hold its height from whichever animation
happened to populate the cache first, and drift vertically as a character
changed animation.

One limitation is accepted rather than fixed: `VisibleBounds` returns the source
animation's rectangle unmirrored, so a hit test against a flipped sprite is
reflected about the anchor. That is the current behaviour, since today's single
box is not mirrored either, and correcting it is independent of crop boxes.

### The animation descriptor and its loader

```go
// AnimationSpec describes one animation's geometry within an image.
type AnimationSpec struct {
    Frames   []image.Rectangle
    Anchor   geometry.Vector2
    Duration time.Duration
}

func LoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) (*Sprite, error)
func MustLoadSpriteAnimations(img *ebiten.Image, specs map[AnimationType]AnimationSpec) *Sprite
```

One descriptor carrying rectangles, anchor and duration, rather than three
parallel maps keyed by the same animation type. `LoadSprite` already carries a
parallel `durations` map, and adding two more would compound that.

An animation with no frames, a rectangle outside the image bounds, and an empty
rectangle are all errors, phrased per the style guide as `<context>: <reason>`
and naming the animation. A zero duration defaults to one second, matching
`LoadSprite`.

`LoadSprite` keeps its signature and becomes a thin wrapper: it expands its
uniform grid into a `map[AnimationType]AnimationSpec` and calls
`LoadSpriteAnimations`. There is one slicing path underneath, and the uniform
grid is simply the convenient way to describe it.

### Auto-cropping at load

```go
func LoadSpriteAutoCropped(src image.Image, width, height int,
    indexes map[AnimationType][]int,
    durations map[AnimationType]time.Duration,
    anchor geometry.Vector2) (*Sprite, error)
func MustLoadSpriteAutoCropped(...) *Sprite
```

It takes `image.Image` rather than `*ebiten.Image` because the crop must happen
before anything is uploaded. It mirrors `LoadSprite`'s argument order with the
sheet-wide anchor appended; the anchor is a parameter rather than a chained
setter because the computation consumes it. As in `LoadSprite`, `width` and
`height` are column and row counts, not pixel dimensions.

The algorithm, per animation in sorted `AnimationType` order:

* Union the alpha bounding boxes of all the animation's frames, in cell-local
  coordinates, giving one crop box for the animation.
* Shelf-pack every frame of every animation into a new `*image.RGBA`, and copy
  with `draw.Draw`.
* Emit an `AnimationSpec` whose `Frames` are the destination rectangles and
  whose `Anchor` is `anchor.Sub(boxOrigin)`, where `boxOrigin` is the crop box's
  cell-local minimum. With `Scale` removed this rebase is exact.

Then hand the result to `LoadSpriteAnimations`.

Four properties worth stating:

* **Cells that no animation references are never copied.** The sheets lay out
  one animation per row, and a short animation leaves the rest of its row empty;
  one sheet uses 1384 of 2432 cells. Packing only visits rectangles some
  animation's indexes reference, so that 43% is recovered here rather than
  needing a separate pipeline change.
* **The atlas is byte-for-byte reproducible.** Animations are packed in sorted
  `AnimationType` order, never map order, so the output does not depend on Go's
  map iteration.
* **An all-transparent animation falls back to its full cell.** It is
  pathological input, and a safe fallback beats both a crash and a degenerate
  box.
* **The alpha scan takes a fast path.** When `src` is `*image.RGBA` or
  `*image.NRGBA` it reads `Pix` directly rather than going through `At`, which
  matters at roughly 77 million pixels per sheet.

The packer is unexported and tested from inside the package. Keeping it separate
from the loader means it reads a CPU-side `image.Image`, so its tests can assert
real pixel values. That matters more than it sounds: `*ebiten.Image` pixels
cannot be read back outside a running `ebiten.RunGame` loop, which is why the
existing visible-extent test seeds its cache instead of measuring
(`render/render_sprite_test.go:322`). The packer is the one part of this change
whose output can be checked directly rather than inferred.

## The atlas question, settled

Cropping cannot be done by narrowing sub-images. Ebitengine's `SubImage`
(`image.go:1128` in v2.9.9) returns an `Image` sharing the parent's underlying
`i.image` and only narrowing `bounds`, so the parent texture stays fully
resident and a tighter sub-image saves nothing at all. The crop must produce a
genuinely smaller texture, which is why auto-crop allocates and copies into a
new atlas.

`LoadSpriteAnimations` takes explicit rectangles into whatever image it is
handed, so vantage never needs an opinion on whether that image is a padded
sheet carrying crop metadata or an already-repacked atlas. Both load
identically. Only the repacked one saves video memory, and
`LoadSpriteAutoCropped` is how vantage produces one.

Peak memory is not reduced. The full padded image is still decoded and held in
RAM while the crops are copied out of it. What drops is the resident video
memory, which is the pressure this addresses.

## Consumer migration

* **nrg** keeps using `LoadSprite` and `SetZeroPosition` unchanged. Its
  `SetScale(1.0)` at `sprites/sprites.go:83` is dropped. `entityAtScreenPos` at
  `rts/rts_scene.go:229` passes the animation it is testing, which
  `SpriteComponent.Animation` (`rts/rts_sprite.go:14`) already holds, and reads
  the anchor through `Sprite.Anchor(a)` instead of the deleted field. Its
  hand-rolled rectangle currently multiplies the anchor by `Scale` a second
  time, which is wrong under the present convention and correct once `Scale` is
  gone.
* **lockstep** swaps `LoadSprite` plus `SetZeroPosition` for
  `LoadSpriteAutoCropped` in `shell/shell_sprites.go:136`, passing the decoded
  and recoloured image it already has. It needs no manifest change, so the sheet
  format stays at version 2 and the pipeline does not move.

## Non-goals

* **Per-frame crop boxes.** Covered under granularity above.
* **A manifest version 3, or any pipeline change.** Auto-crop derives the
  per-animation geometry from pixels the pipeline already ships. Precomputing
  the boxes offline and calling `LoadSpriteAnimations` directly stays available
  if the startup scan ever proves too slow, and needs no further engine work.
* **Mirroring `VisibleBounds` for flipped sprites.** Pre-existing, independent.
* **Folding `LoadSprite`'s parallel `durations` map into `AnimationSpec`.** A
  real wart, but unrelated to crop boxes, and widening this change to reach it
  is not worth it.
* **Proving the real-sheet memory figures inside vantage.** The mechanism is
  tested here on synthetic sheets; the 310 MB to 47 MB numbers are a claim about
  the published art and are verified in lockstep. See testing.

## Testing

The centrepiece is that cropping is invisible on screen. Build a synthetic
sheet, load it with `LoadSprite` and with `LoadSpriteAutoCropped`, and assert
both put the anchor pixel on the same screen point for every animation.
Comparing the `GeoM` that `buildDrawOp` produces is how the existing tests at
`render/render_sprite_test.go:62` already work.

The narrower regression also goes in: two animations with different crop sizes
and different anchors, drawn at one world position, asserting their anchors
coincide. That is the failure that would otherwise show up as a character
bobbing between the ground and mid-air as its animation changes.

`VisibleTopAboveZero` is covered for a sprite whose animations have genuinely
different heights, asserting each returns its own answer and that no animation's
cached value leaks into another's, alongside the existing property that
`TileRatio` is applied fresh on every call.

The packer is tested as pure CPU code with no Ebitengine involved: a synthetic
sheet with known padding, asserting the per-animation boxes, the derived
anchors, a byte-identical atlas across runs, that cells no animation references
never appear, and the all-transparent-animation fallback. Because it reads an
`image.Image`, these assert real pixel values rather than seeding a cache.

The saving is measured, not asserted. A test comparing packed against padded
pixel counts on a synthetic sheet proves the mechanism. The 310 MB to 47 MB
figure is a claim about real sheets and can only be confirmed in lockstep, so it
stays a lockstep-side verification rather than a vantage test that pretends to
prove it.

Regression and consumers:

* `render_sprite_test.go`, `render_spritelibrary_test.go`,
  `render_filter_test.go` and `scene_spriteshowcase_test.go` pass with only
  mechanical edits for the removed `Scale` and the new animation arguments.
* `task lint`, `task test:headless` and `go vet` all pass.
* nrg builds and renders, checked with an xvfb screenshot.
* `cmd/showcasedemo` is screenshotted, since it is the file whose `SetScale`
  usage is being replaced.

## Documentation and release

* `render/doc.go` describes the anchor as per-animation, drops `Scale`, and
  names `LoadSpriteAnimations` and `LoadSpriteAutoCropped`.
* `docs/debugging.md:279` currently records that the showcase demo uses
  `Sprite.Scale` rather than large frames, and is rewritten for
  `SetSourceTileSize`.
* `docs/performance_optimization.md` records the startup alpha scan and shelf
  pack as a known cost, with precomputed boxes as the documented escape hatch.
* Released as a tag, since lockstep consumes vantage by version.
