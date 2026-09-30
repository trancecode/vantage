# Per-frame crop boxes design

## Purpose

Let `LoadSpriteAutoCropped` crop every frame to its own content rather than every
animation to the union of its frames, so a sheet costs video memory in
proportion to what each frame draws.

[Per-animation crop boxes](2026-08-08-per-animation-crop-boxes-design.md) took
the published sheets from about 2 GB to about 285 MB and deliberately stopped
there. That design kept `AnimationSpec.Frames` a list of rectangles so per-frame
crops stayed expressible, and this design builds on it.

The request comes from the sprite pipeline for lockstep. A weapon swing or a
death fall has a union box that is mostly empty in any one frame. Measured with
a port of `autoCropAtlas` and `shelfPack` (`driver/measure_vram.py` in
sprite_generation_pipeline, against the pitched-v1 sheets):

* Sergeant sheet: 46.6 MiB per animation, 33.1 MiB per frame.
* lockstep's roster, 6 sheets x 8 clips x 8 directions: 356 MiB per animation,
  213 MiB per frame, a 40% saving.
* Layered sprites, where outfit, weapon and shield are separate sheets stacked
  at draw time, become viable. Five sergeant layers in 4 directions cost
  53.3 MiB per animation, 19.8 MiB per frame, against 24.0 MiB baked into one
  sheet. A weapon layer's per-animation box spans its whole swing.

## Vocabulary

An *animation* is a run of frames keyed by `render.AnimationType`, as in the
per-animation design. `AnimationSpec` describes it at load time and `Animation`
is the loaded form.

An *anchor* is the pixel that sits on the drawn world position; the engine calls
it `ZeroPosition`. For a character, it is the point between their feet.

A frame's *crop box* is the tight rectangle around its non-transparent pixels,
in cell-local coordinates.

*Frame space* is the coordinate system of one uncropped frame: for a frame cut
from a uniform sheet, the cell it came from. A frame's *offset* is where its
stored image's top-left corner sits in frame space. An uncropped frame has
offset (0, 0); a cropped frame's offset is its crop box's corner.

## Design: an offset per frame, an anchor per animation

The obvious encoding gives each frame its own anchor, rebased into its own crop
box. This design instead keeps one anchor per animation, expressed in frame
space, and gives each frame an offset. The anchor a frame is drawn with is the
animation's anchor minus the frame's offset. Texture packers encode trimmed
sprites the same way.

The two encodings draw the same pixels. The offset encoding wins on the
contracts around the draw:

* **`SetZeroPosition` stays meaningful.** It sets an anchor in frame space,
  which is the space `LoadSpriteAutoCropped` receives its sheet-wide anchor in.
  Calling it after auto-cropping with the sheet's anchor changes nothing, and
  calling it with a different anchor moves every frame consistently. Under
  per-frame anchors it would silently overwrite every derived value, the hazard
  lockstep's loader comment warns about today.
* **Consumer code keeps its meaning.** nrg sets `Animation.ZeroPosition` by hand
  for its LPC sprites and hit-tests by combining `VisibleBounds` with `Anchor`.
  Both stay correct because `VisibleBounds` reports frame space too.
* **Mirroring is unchanged.** A mirrored animation still flips about its anchor,
  and `Sprite.Anchor` still resolves a mirrored animation to its source.

## Types

`Animation` holds frames instead of bare images:

```go
type Animation struct {
    Frames       []Frame
    Duration     time.Duration
    ZeroPosition geometry.Vector2 // in frame space
}

type Frame struct {
    Image  *ebiten.Image
    Offset image.Point // where Image's top-left sits in frame space
}
```

A slice of `Frame` replaces `Images []*ebiten.Image` rather than sitting beside
it as a parallel `Offsets` slice. Two slices that must stay the same length is a
footgun the repository's compatibility policy says not to keep.

`AnimationSpec` follows the same shape:

```go
type AnimationSpec struct {
    Frames   []FrameSpec
    Anchor   geometry.Vector2 // in frame space
    Duration time.Duration
}

type FrameSpec struct {
    Rect   image.Rectangle // source rectangle in the image
    Offset image.Point     // where Rect's top-left sits in frame space
}
```

`Offset` is an `image.Point` because crop boxes are whole pixels; the anchor
stays a `geometry.Vector2` because it can be fractional.

`AddImage` appends a frame at offset (0, 0), so hand-built sprites such as nrg's
behave as before. `LoadSprite` and `LoadSpriteAnimations` keep their
signatures; `LoadSprite` emits zero offsets. `LoadSpriteAutoCropped` and
`MustLoadSpriteAutoCropped` keep theirs.

## Cropping

`autoCropAtlas` measures each referenced frame's crop box and packs it at that
size. Every animation's anchor is the sheet-wide anchor unchanged, and each
frame's offset is its box's corner.

An empty frame is stored as a 1x1 image copied from its cell's top-left pixel,
which is transparent because the cell is empty, at offset (0, 0). The
per-animation design fell back to the full cell. That fallback was cheap there
because it only applied when a whole animation was empty; per frame, it would
charge a full cell for every blank frame at the tail of a death clip. An
animation that is entirely empty gets the same treatment on every frame.

`shelfPack` is unchanged, including the one-pixel gutter that keeps
Ebitengine's linear filter from sampling a neighbouring frame. Its comment that
frames of one animation share a size stops being true and is rewritten.
Tallest-first shelving still suits frames of mixed sizes.

## Drawing and measurement

A frame's anchor is `ZeroPosition - Offset`. Every consumer of the anchor reads
it for the specific frame it handles:

* `buildDrawOp` takes the frame's offset. `Draw` uses the first frame and
  `DrawAnimationScaled` the frame chosen by elapsed time.
* `VisibleBounds` still measures the first frame and now adds its offset, so it
  reports frame space. For an uncropped frame this is the same rectangle as
  today.
* `VisibleTopAboveZero` still measures the first frame and uses that frame's
  anchor.
* `Anchor` keeps returning the animation's frame-space anchor.

The two caches stay per animation, since both measure only the first frame.

## Migration

Inside vantage, the showcase scene's fit scale reads `Frames[i].Image` instead
of `Images[i]`, and `render/doc.go` describes offsets. The debugging and
performance documents that describe per-animation cropping are updated.

Outside vantage:

* lockstep only bumps the version; drawing is unchanged and memory drops. Its
  loader comment in `shell/shell_sprites.go` describing per-animation crops,
  derived per-animation anchors, and `SetZeroPosition` overwriting them is
  stale and should be rewritten.
* sprite_generation_pipeline's `harness/teamcolor.go` indexes `Images` and
  duplicates the frame arithmetic; it moves to
  `animation.FrameAt(elapsed).Image`, keeping its zero-frames guard since
  `FrameAt` panics on an empty animation.
* nrg's runtime code is unaffected (its LPC sprites use `AddImage`, so offsets
  are zero), but `rts/rts_lpc_library_test.go` reads `Animations[...].Images`
  and moves to `Frames`.

## Testing

* `autoCropAtlas` unit tests: frames of one animation get different crop boxes
  and offsets; the anchor is the sheet anchor; an empty frame becomes a 1x1
  frame at offset (0, 0); an entirely empty animation loads; atlas pixels match
  the source.
* Draw-geometry tests: a cropped frame's draw matrix puts its anchor pixel on
  the same world position as the uniform load, for a plain and a mirrored
  animation, and at a display scale other than 1.
* `VisibleBounds` and `VisibleTopAboveZero` give the same answers for an
  auto-cropped sprite as for its uniform load.
* `render/pixeltest`'s A/B render compares uniform and auto-cropped loads of a
  synthetic sheet whose frames within one animation differ in size.
* A one-off measurement loads `out/sheets/sergeant.png` through the new
  `autoCropAtlas` and reports the atlas size, expected near the 33.1 MiB
  `measure_vram.py` predicts, give or take packing. It is a check, not a
  committed test, since the sheet lives in another repository.
