# Sprite tile size design

## Purpose

Let a game choose its tile size, and let each sprite declare the tile size its
art was drawn for, so the engine scales art to the game rather than the game
being pinned to whatever resolution its art happens to have.

Two things force this. `render.TileSize` is a compile-time constant of 16, so
every game built on the engine renders a tile as 16 pixels whether or not that
suits its art. And `Sprite.Scale` is a bare multiplier with no stated reference
point: `0.5` says nothing about why, and silently becomes wrong if either the
art or the tile size changes.

The immediate consumer is `sprite_generation_pipeline`, whose Blender renders
arrive at whatever resolution its camera produced and are character-sized rather
than tile-sized. The general case is any game that is not pixel art, where
characters are routinely authored at finer detail than terrain: terrain at 32
pixels per tile, characters at 64, both occupying the same world footprint, with
the character carrying twice the source detail so it survives zooming in.

## Vocabulary

The *game tile size* is how many pixels one world tile occupies before camera
zoom. It is engine configuration, one value for a whole game.

A sprite's *source tile size* is the tile size its art was drawn against. It is
authorial metadata, not derivable from the image: a 64 pixel image drawn for 128
pixel tiles and a 64 pixel image drawn for 32 pixel tiles are different sprites
that happen to share a file size. The engine must never infer one from the
other.

A sprite's *footprint* is how many tiles it covers, `imageSize / sourceTileSize`.
It is derived, never declared. A 128 pixel character drawn for 64 pixel tiles
covers two tiles.

The *tile ratio* is `gameTileSize / sourceTileSize`, the scale the engine applies
so art drawn for one tile size lands correctly at another.

## Architecture

Three changes, all in `render` except the settings surface in `app`.

### A configurable game tile size

`render.TileSize` changes from an untyped constant to a package variable:

```go
// TileSize is how many pixels one world tile occupies before camera zoom. Set
// by engine configuration through the [render] tile_size setting.
var TileSize float64 = 16
```

Every use of it inside the engine is already float64 arithmetic, and so is every
use in the two consuming games. All eight consumer call sites were checked:
`lockstep/shell/shell_draw.go:272,284`, `lockstep/shell/shell_scene.go:404,405,410,411`
and `nrg/rts/rts_scene.go:137,138`. Each multiplies or divides a float64, so
giving `TileSize` a concrete float64 type compiles unchanged.

One place in the engine does have to change. `scene/scene_spriteshowcase.go:46`
declares `showcaseSlotPixels = render.TileSize` inside a `const` block, which
stops compiling the moment `TileSize` is a variable. It becomes a variable too,
or better, the showcase reads `render.TileSize` where it needs it, since a slot
frozen at load time would ignore a configured tile size for the same reason
described under the initialization-order trap below. No consumer has this
problem; only vantage's own showcase does.

### Why a variable rather than an accessor

An exported variable can be assigned by a game, which an accessor would prevent.
It is still the right choice.

`Settings.Apply` has to set the value, so an accessor implies an exported
`SetTileSize` beside it, and a game can call a late setter exactly as easily as
it can assign a late variable. The encapsulation does not prevent mistimed
changes, it only adds a step. Validation is the one thing it would genuinely
buy, and that already comes from `Settings.validate()` at startup.

Against it, the engine already treats global configuration as exported mutable
state, in `render.UsePlaceholderSpriteImages`, `render.SpriteFilter`,
`render.Sprites` and `util.DebugMode`, the last of which F12 deliberately
toggles while running. An accessor here would be the exception. It would also
turn an upgrade that needs no consumer edits into one that rewrites all eight
call sites as `render.TileSize()`.

The hazard worth fixing is not mutability but capture, covered next.

`app.RenderSettings` gains `TileSize float64` with a `[render] tile_size`
default of 16, a `--tile_size` flag, and `Settings.Apply` assigning it to
`render.TileSize` alongside the existing globals.

A tile size of zero or less is a configuration error, not something to clamp.
`Settings` gains an unexported `validate() error` that `App.Run` calls before
`Apply`, returning an error naming the bad value. This follows the scene-name
validation already in `Run` rather than inventing a wrapper type: a plain number
does not need the `UnmarshalText` treatment `FilterSetting` gets, because there
is no name to parse, only a range to check.

### A source tile size on the sprite

`render.Sprite` gains one field:

```go
// SourceTileSize is the tile size this sprite's art was drawn for. Zero means
// the art was drawn for whatever the game's tile size is, so no correction
// applies. It is authorial metadata and is never inferred from the image.
SourceTileSize float64
```

Set through a chainable setter, `SetSourceTileSize`, matching `SetScale`,
`SetZeroPosition` and `SetType`:

```go
render.Sprites.Add("Knight", render.MustLoadSprite(sheet, 20, 8, indexes, durations)).
    SetSourceTileSize(64).
    SetZeroPosition(geometry.NewVector2(48, 104))
```

Not a `LoadSprite` parameter. The loader has the image but not the intent, and
the source tile size is authorial in exactly the way `ZeroPosition` and `Type`
are. Adding it to the signature would also break every one of nrg's registration
sites for a value most of them do not need. A setter keeps the change additive.

Zero as the default is what makes this invisible to existing sprites: every
sprite in nrg keeps drawing exactly as it does now, with no edits.

### Folding the ratio into the draw

`tileRatio()` returns `TileSize / SourceTileSize`, or `1.0` when
`SourceTileSize` is zero or negative.

`buildDrawOp` currently reads:

```go
op.GeoM.Scale(s.Scale*displayScale, s.Scale*displayScale)
op.GeoM.Translate(-s.ZeroPosition.X()*displayScale, -s.ZeroPosition.Y()*displayScale)
```

The tile ratio behaves exactly like `displayScale`: both are uniform scales the
engine applies about the anchor, on top of what the sprite declares about
itself. So it multiplies in at both sites:

```go
scale := s.Scale * s.tileRatio() * displayScale
op.GeoM.Scale(scale, scale)
anchor := s.tileRatio() * displayScale
op.GeoM.Translate(-s.ZeroPosition.X()*anchor, -s.ZeroPosition.Y()*anchor)
```

This deliberately leaves `ZeroPosition`'s meaning alone. It is documented as
being in drawn pixels, with the frame scaled by `Scale` before the offset is
applied (see the comment on `VisibleTopAboveZero`), which is why the translate
multiplies by `displayScale` and not by `Scale`. The tile ratio is engine
applied, like `displayScale`, so the anchor scales with it. Redefining
`ZeroPosition` in source pixels would be cleaner in the abstract and is
explicitly not part of this change.

### Visible-extent helpers

`VisibleTopAboveZero` and `VisibleBounds` report drawn pixels and currently
account for `Scale` only. They feed `DrawNameplate` and `DrawFloatingBar`, which
anchor a label a constant screen gap above a sprite's visible top. Once the tile
ratio changes how big a sprite draws, those must account for it too, or a
nameplate on a rescaled sprite sits at the wrong height.

Both cache their result. The cache is computed lazily on first use, which happens
during drawing and therefore after settings are applied, so a configured tile
size is already in effect. Setting `SourceTileSize` after a sprite has been drawn
would leave a stale cache; the setter therefore clears both caches, which is
cheap and removes a trap that would otherwise only appear in a game that
reconfigured sprites at runtime.

## The initialization-order trap

Games register sprites from `init()`, but settings are applied in `App.Run`.
Anything that read `TileSize` while building a sprite would capture the
compiled-in default and silently ignore the configured value.

The design avoids this by storing `SourceTileSize` as data and computing the
ratio at draw time. No code path may capture `TileSize` at construction. This is
the single most important constraint in this document, because getting it wrong
produces art that is correct in a game using the default tile size and subtly
wrong in every other, with no error anywhere.

### The camera already captures it

`NewCamera` (`render/render_camera.go:29`) computes
`screenMultiplier = screenHeight / (defaultVerticalTileCount * TileSize)` once,
at construction, and stores it. Every zoom calculation then runs through that
stored number.

This is the trap already present in the code, and it is the reason mutability is
not the thing to worry about. A camera built before the tile size is configured
keeps scaling by the old value, silently, and no accessor or setter prevents it,
because the freezing happens on read rather than on write.

`Camera` therefore stops storing `screenMultiplier` and computes it where it is
used, in `EffectiveZoom`. It is a division per call on a value already read every
frame, so the cost does not signify, and it removes the only place in the engine
where the tile size is frozen. With that done, a game assigning `render.TileSize`
at any point behaves correctly rather than half-correctly.

Scene cameras are built in `Scene.Init`, which now runs from `App.Layout` and so
after `Settings.Apply`, meaning settings-driven configuration is already safe
today. The fix is for everything else: a game constructing its own camera, and
any later change to the tile size.

## Non-goals

* No asset modules. Declaring a source tile size is what would eventually let a
  sprite pack be imported into a game with any tile size, which matters for
  modding. That is a consequence worth having, not a feature being built here,
  and nothing in this change should be shaped around it.
* No removal of `Scale`. It stays as an independent artistic multiplier, for a
  boss deliberately drawn at twice its art size, which is a different question
  from what the art was drawn against. The two compose:
  `Scale × tileRatio × displayScale`. It is currently used exactly once across
  both games, at `nrg/sprites/sprites.go:83`, where it is set to `1.0`.
* No redefinition of `ZeroPosition` as source pixels, per the section above.
* No per-frame or per-animation source tile size. One value per sprite, matching
  the engine's existing one-anchor-per-sprite model.
* No runtime tile size changes. It is read at draw time, so a change would take
  effect, but nothing supports or tests reconfiguring a running game.
* No change to how the sprite showcase sizes its slots, beyond the mechanical
  one its `const` forces. That is a separate piece of work which this change
  makes tractable, since a sprite's footprint in tiles becomes derivable rather
  than guessed.
* No accessor for the tile size, and no attempt to stop a game assigning it. See
  the reasoning above; the capture fix is what makes assignment safe.

## Testing

`render`:

* `tileRatio` returns 1.0 when `SourceTileSize` is zero, negative, or equal to
  the game tile size.
* Art drawn for 64 pixel tiles in a 32 pixel game halves; drawn for 16 in a 32
  pixel game doubles.
* `Scale`, the tile ratio and `displayScale` compose multiplicatively, asserted
  on the resulting `GeoM` rather than on pixels, since pixel readback needs a
  running game loop.
* The anchor scales with the tile ratio: a sprite with a non-zero
  `ZeroPosition` drawn at a ratio other than 1 puts its anchored point on the
  same world position as at ratio 1.
* A sprite with no `SourceTileSize` produces a `GeoM` identical to today's, which
  is the compatibility guarantee for every existing sprite.
* `SetSourceTileSize` clears the visible-extent caches.
* `VisibleTopAboveZero` accounts for the ratio.

* `Camera.EffectiveZoom` tracks a tile size changed after the camera was built,
  which is the regression guard for the capture described above. Building a
  camera, changing `render.TileSize`, and reading `EffectiveZoom` must reflect
  the new value.
* A camera built at a given screen height reports the same effective zoom as
  before this change when the tile size is left at its default, which is the
  compatibility guarantee for existing games.

`app`:

* `tile_size` defaults to 16 and loads from TOML and from `--tile_size`.
* Zero and negative values are a startup error naming the value.
* `Apply` sets `render.TileSize`.

A regression guard worth having: a test asserting `render.TileSize` is 16 by
default, so a change to the shipped default is a deliberate act rather than a
silent one for every consuming game.

## Documentation and release

* `docs/debugging.md` needs nothing; this is not a debug tool.
* `render/doc.go` gains a sentence on the tile size being configuration and on
  sprites declaring what they were drawn for.
* `ARCHITECTURE.md`'s `render` row mentions the sprite library already and should
  mention tile scaling.
* The release note must call out that `render.TileSize` stops being a constant.
  Consumers compile unchanged, which was verified, but a game that had come to
  rely on it being a compile-time constant, for instance in an array size, would
  not.
