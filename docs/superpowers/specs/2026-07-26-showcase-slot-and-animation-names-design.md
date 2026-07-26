# Showcase slot size and animation names design

## Purpose

Make the sprite showcase usable for judging art, not merely for confirming a
sprite loads. Two things stop it today, both surfaced by
`sprite_generation_pipeline`, whose experiment 3 harness is the first consumer
with character-sized sprites and game-defined animation types.

* Every cell gives its art a slot of exactly one tile. Their frames are around
  96 by 112 pixels and their characters are taller than one tile, so the
  showcase shrinks them to a tile and they cannot be judged. Their own design
  doc concluded the scene is "not the surface for judging art quality" partly
  for this reason.
* Their eight facing directions are `AnimationGameBase + i`, which the engine's
  stringer renders as `AnimationType(64)` through `AnimationType(71)`. Every
  label in their run reads like that. Their doc calls it "ugly but
  deterministic".

## Vocabulary

The *slot* is the square each grid cell gives its art, measured in tiles. Art
larger than the slot is scaled down to fit; art at or below it is left alone.

A *display name* is what the showcase labels an animation with. The engine has
names for its own animation types through a generated `String()` method; a game
defining its own types has none until it registers them.

## Slot size

### A setting in tiles, not pixels

`scene` gains a package-level variable:

```go
// ShowcaseSlotTiles is how many tiles square a slot the sprite showcase gives
// each cell's art. Engine configuration, set through [scene]
// showcase_slot_tiles. Art larger than the slot is scaled down to fit it.
var ShowcaseSlotTiles float64 = 1
```

Tiles rather than pixels, because the rest of the grid is already in tiles and
because a slot expressed in tiles keeps its meaning when the tile size changes.
A game whose characters are two tiles tall wants a slot of two or three tiles
whatever it sets `[render] tile_size` to.

`app.SceneSettings` gains `ShowcaseSlotTiles float64` with a
`[scene] showcase_slot_tiles` default of 1, a `--scene_showcase_slot_tiles`
flag, and an assignment in `Settings.Apply`. `Settings.Validate` rejects a
non-positive value the same way it rejects a non-positive tile size.

It is read where used, never captured, for the reason the tile size is: nothing
may freeze configuration that settings apply after `init()`.

### Grid geometry derived from the slot

The grid constants are currently fixed at values that assume a one-tile slot.
They become functions of the slot size `S`:

| Quantity | Now | Becomes | At S=1 |
| --- | --- | --- | --- |
| Column pitch | 2.0 | `S + 1` | 2.0 |
| Row pitch | 4.0 | `S + 3` | 4.0 |
| Label centre X | 0.5 | `S / 2` | 0.5 |
| Sprite label Y | 1.2 | `S + 0.2` | 1.2 |
| Animation label Y | 1.8 | `S + 0.8` | 1.8 |

Every one reproduces today's value exactly at a slot of one tile, which is the
compatibility guarantee and is pinned by the existing layout tests.

The additive form is deliberate. A multiplicative rule would scale the gap and
the label space with the slot, so a large slot would push cells absurdly far
apart, which is the mistake an earlier iteration of this scene already made with
art-derived spacing. The gap is one tile and the label space is two tiles
regardless of how big the art is, because labels are drawn at a fixed pixel size
and do not grow.

`showcaseOriginX`, `showcaseOriginY` and `showcaseColumnsPerRow` are unchanged.

## Animation names

### A registry in render

`render` gains a name registry beside the existing sprite library:

```go
// RegisterAnimationName gives an animation type a display name, for types the
// engine's own String() does not know: a game defining types at or above
// AnimationGameBase has no generated name for them. Panics on an empty name or
// a type already registered, both load-time mistakes.
func RegisterAnimationName(a AnimationType, name string)

// AnimationName returns an animation type's display name: the registered one if
// there is one, otherwise its String() with the engine's "Animation" prefix
// trimmed, so AnimationIdleDown reads as IdleDown.
func AnimationName(a AnimationType) string
```

The fallback moves the `strings.CutPrefix(..., "Animation")` trim out of the
showcase and into the engine, so display naming lives in one place and any
future consumer gets the same answer.

Registration is allowed for engine-owned types too. A game that wants
`AnimationIdleDown` to read as "Idle down" is doing something legitimate, and
refusing it would buy nothing.

Panicking on a duplicate matches `SpriteLibrary.Add`: registration happens at
`init()` time, so a clash is a programming error and failing loudly beats a
catalog that silently depends on which package initialized first.

### Ordering changes to match

The showcase currently sorts a sprite's animations by `String()`. Once names
exist that becomes actively wrong: a game's eight facings named S, SE, E, NE, N,
NW, W, SW would sort to E, N, NE, NW, S, SE, SW, W, scrambling the direction
order the names exist to convey.

Animations are therefore sorted by their `AnimationType` value instead, which
respects the order a game declared them in. For the engine's own types this is
also an improvement: value order gives Default, the four Move variants, the four
Idle variants, then the four Attack variants, grouped by category, where
alphabetical order interleaves them as Attack, Default, Idle, Move.

This changes existing behavior and the tests that pin it, which is intended and
called out here so it is not mistaken for a regression.

## Non-goals

* No per-sprite slot size. The grid is a contact sheet; cells differing in size
  would defeat the comparison it exists for.
* No automatic slot sizing from the library's contents. An earlier iteration
  derived spacing from the largest art and one outsized sprite spread the whole
  grid. The slot is configuration.
* No name registry for sprite types or scene names. Only animation types have
  the problem, because only they are an open enumeration a game extends.
* No localization. Display names are whatever the game registers.
* No change to how the fit scale works, beyond taking the slot from
  configuration rather than a constant.

## Testing

`render`:

* A registered name is returned by `AnimationName`.
* An unregistered engine type falls back to its trimmed `String()`, so
  `AnimationIdleDown` gives `IdleDown`.
* An unregistered game type falls back to the untrimmed `String()`, giving
  `AnimationType(64)`, since there is no prefix to trim.
* Registering an empty name, or a type twice, panics.
* Registration works for an engine-owned type.

Tests must not leak registrations into each other, since the registry is
package-level. They register types well above `AnimationGameBase` and clean up.

`scene`:

* At a slot of 1 tile, every derived quantity equals the value the constants
  had, pinned as literals. This is the compatibility guarantee.
* At a slot of 3 tiles, cells are 4 tiles apart horizontally and 6 vertically,
  and the labels sit below a 3 tile sprite.
* A sprite two tiles tall fits a 3 tile slot at scale 1, and is shrunk at a 1
  tile slot.
* The wrap still happens at the same column count whatever the slot size.
* Animations order by `AnimationType` value, not by name, checked with types
  whose value order and name order differ.
* Labels use registered names when present.

`app`:

* `showcase_slot_tiles` defaults to 1, loads from TOML and from the flag.
* A non-positive value fails `Validate`.
* `Apply` sets `scene.ShowcaseSlotTiles`.

## Documentation

* `docs/debugging.md`'s sprite showcase section gains the slot setting beside
  the tile size one, and explains registering animation names with a worked
  example of a game's eight facings.
* `render/doc.go` mentions the animation name registry.
* `scene/doc.go` mentions the configurable slot.
