# Sprite showcase design

## Purpose

Give any game built on `vantage` a way to put every sprite it has loaded on
screen at once, animating, labelled with its name and the name of each of its
animations, so that art can be inspected without building a level around it.

The feature already exists in `nrg` as its `ShowcaseScene`
(`nrg/rts/rts_showcase.go`), triggered by that game's own `--scene showcase`
flag. It is not game content: the layout, the labelling and the camera handling
are engine concerns, and only the catalog of sprites it walks is game-specific.
This design moves the whole thing into `vantage` and, in doing so, moves sprite
naming and lookup up to the engine as well.

## Vocabulary

A *sprite* is `render.Sprite`: a set of named animations plus a scale, a zero
position and a sprite type. An *animation* is a frame sequence with a duration,
keyed by `render.AnimationType`.

A *sprite library* is a collection mapping display names to sprites. `nrg`
has one today at game level (`nrg/sprites/sprites.go`), keyed by a local
`SpriteID` int enum with a `stringer`-generated `String()`. This design
introduces the engine-level equivalent, keyed by plain strings.

A *showcase* is the read-only debug scene that draws every sprite in a library.

## Architecture

Three pieces, in three packages that already exist:

* `render` gains `SpriteLibrary` and a package-level default, `render.Sprites`.
  This is where sprite naming and lookup now live.
* `scene` gains `SpriteShowcaseScene`, which reads a library and draws it.
* `app` gains a `--sprite_showcase` flag and self-registers the scene when it
  is set.

`render` does not import `scene`, so `scene` importing `render` introduces no
cycle. `scene` already imports `ui`.

### Why sprite management moves to the engine

`vantage` already owns the sprite loader (`render.LoadSprite`), the animation
type enum, the drawing (`Sprite.DrawAnimation`), the text writer and the
camera. The catalog is the only remaining piece at game level, and it is
boilerplate each game would otherwise reinvent.

The type safety given up is small. In `nrg`, outside the showcase itself,
there are three non-test lookup sites against the enum-keyed map:
`rts/rts_scenario.go:123` and `rts/rts_terrain.go:32,38`. The enum is doing
naming work, not type-safety work.

Engine-level package state is an established pattern here:
`render.UsePlaceholderSpriteImages` (`render/render_sprite.go:14`) and
`asset.DefaultProportionalFont` / `asset.DefaultMonospaceFont`
(`asset/asset_font.go:21,25`).

### `render.SpriteLibrary`

New file `render/render_spritelibrary.go`.

```go
// SpriteLibrary maps display names to sprites. Games register their sprites
// into the package-level Sprites library at init time; engine debug tools such
// as the sprite showcase scene read it back.
type SpriteLibrary struct {
    sprites map[string]*Sprite
}

func NewSpriteLibrary() *SpriteLibrary
func (l *SpriteLibrary) Add(name string, s *Sprite) *Sprite
func (l *SpriteLibrary) Get(name string) (*Sprite, bool)
func (l *SpriteLibrary) Names() []string
func (l *SpriteLibrary) Len() int

// Sprites is the default library that engine tooling reads.
var Sprites = NewSpriteLibrary()
```

`Add` returns the sprite it was given so that registration composes with the
existing chainable setters:

```go
render.Sprites.Add("Character", render.MustLoadSprite(
    data.ImagePlayer, 6, 10, indexes, nil,
)).SetZeroPosition(geometry.NewVector2(24, 40)).SetType(render.SpriteTypeActor)
```

`Add` panics on an empty name, on a nil sprite, and on a name already present.
All three are load-time programming errors, and registration happens from
`init()`, so a panic surfaces the mistake immediately instead of yielding a
catalog that is silently missing or silently overwriting an entry. This follows
the style guide's preference for `panic()` on unrecoverable errors.

`Names()` returns names sorted lexicographically, so callers that iterate get a
stable order without sorting themselves. `Get` returns the comma-ok form rather
than panicking, because a lookup miss is a plausible runtime condition rather
than a load-time error.

The library is not safe for concurrent use. Registration happens at `init()`
time on one goroutine, and reads happen during draw on the same goroutine.

`LoadSprite` and `MustLoadSprite` are left exactly as they are. Registration
stays a separate, explicit call. Folding a name argument into the constructor
would make every sprite it builds land in the library as a side effect,
including the ones built by `render/render_sprite_test.go:19` and any ad-hoc or
procedural sprite a game constructs per instance, all of which would then
appear in the showcase.

### `scene.SpriteShowcaseScene`

New file `scene/scene_spriteshowcase.go`. A port of `nrg/rts/rts_showcase.go`
with the game-specific catalog replaced by a library.

```go
// SpriteShowcaseSceneName is the SceneName used by the engine's
// SpriteShowcaseScene.
const SpriteShowcaseSceneName SceneName = "sprite_showcase"

type SpriteShowcaseScene struct {
    BaseScene

    // library is the sprite library to display. Defaults to render.Sprites.
    library *render.SpriteLibrary

    durationSinceInit time.Duration
    camera            *render.Camera
    cameraController  *render.CameraController
    screenWidth       int
    screenHeight      int
}

func NewSpriteShowcaseScene() *SpriteShowcaseScene
func NewSpriteShowcaseSceneFor(library *render.SpriteLibrary) *SpriteShowcaseScene
```

`NewSpriteShowcaseScene` uses `render.Sprites`. `NewSpriteShowcaseSceneFor`
takes an explicit library, which is what tests use so that they do not have to
mutate global state to exercise the layout.

Scene lifecycle, all of which carries over from `nrg` unchanged in substance:

* `Init` resets the elapsed-time accumulator, builds a `render.Camera` sized to
  the screen, wraps it in a `render.CameraController`, and calls
  `SetZeroAsTopLeft` so the grid starts at the top-left corner.
* `Update` accumulates elapsed duration, which drives the animation phase, and
  forwards input to the camera controller when the scene has focus. The
  controller provides W/A/S/D panning and Q/E zoom.
* `Draw` returns immediately when the scene is not visible, and otherwise draws
  the grid.
* `LayerIndex` returns 0. This is a full-screen scene, not an overlay.

Layout, carried over from `nrg`:

* Sprites are split into those with more than one animation and those with
  exactly one, by `Sprite.AllAnimations()`.
* Multi-animation sprites come first, one row per sprite, one column per
  animation, animations ordered by name.
* Single-animation sprites follow, packed into a grid that wraps at ten columns
  so labels do not overlap.
* Each cell draws the animation via `Sprite.DrawAnimation` at the accumulated
  duration, then the sprite name and the animation name beneath it, centered,
  on a semi-transparent dark background, using `render.TextDefault` with
  `AlignCenter` and `WithBackground`. The background is `color.RGBA{0, 0, 0,
  180}`, carried over from `nrg`.
* Sprites are ordered by name, which `SpriteLibrary.Names()` already
  guarantees.

Two differences from the `nrg` original:

* Sprite labels are the library key verbatim. `nrg` strips a `"Sprite"` prefix
  from its enum names; with author-chosen string keys there is nothing to
  strip.
* Animation labels keep the `"Animation"` prefix trim, because those enum names
  are `vantage`'s own and do carry the prefix.

The spacing constants (column pitch, row pitch, label offsets, wrap column
count) become named package-level constants rather than the inline literals
they are in `nrg`.

The doc comment on `SceneName` at `scene/scene.go:11` currently states that the
engine reserves only `DialogSceneName`. It is updated to name both reserved
scene names.

### The `--sprite_showcase` flag

`app.DebugSettings` gains a `SpriteShowcase bool`, registered in
`Settings.RegisterFlags` (`app/settings.go:90`) alongside the existing `--debug`
and `--enable_debug_http_server` flags, and settable from `settings.toml` like
its neighbours.

When the flag is set, `App` registers the scene itself and shows it exclusively:
it calls `AddScene` with a `SpriteShowcaseScene`, then `ShowOnly` and
`SetExclusiveFocus` with `SpriteShowcaseSceneName`. A game therefore needs no
code change at all to get the showcase, only sprites registered into
`render.Sprites`.

Placement is constrained on both sides. It has to run after the game has
registered its own scenes, so not in `app.New`, and before
`a.manager.Init(a.screenWidth, a.screenHeight)` at `app/app.go:74`, since that
call is what initializes every registered scene. It therefore goes in
`App.Run`, between the window sizing and the manager `Init`.

If the flag is set and `render.Sprites` is empty, `App` logs a warning through
`util.Logger`, the pattern already used for the screenshot notice at
`app/app.go:86`, naming `render.Sprites.Add` so that a blank screen is
explained rather than looking like a rendering failure.

## Non-goals and documented traps

* No `Name` field on `render.Sprite`. The library holds the mapping, and
  duplicating the name onto the sprite would create two sources of truth for no
  current consumer. If debug overlays later need the name from a bare
  `*Sprite`, that is a separate change.
* No automatic registration from `LoadSprite`. Covered above.
* No removal or replacement of entries. `Add` is the only mutator. Reloading
  sprites at runtime is not a use case today.
* No interactivity beyond the camera. No filtering, no per-sprite selection, no
  pausing animation. The scene is a read-only inspection surface.
* Migrating `nrg` off its own `sprites.Sprites` map and deleting
  `rts/rts_showcase.go` is follow-up work in that repository, not part of this
  change. Both can coexist: `nrg` can register into `render.Sprites` while
  keeping its enum-keyed map, since `Add` returns the sprite.

## Testing

`SpriteLibrary` tests need no display and live in
`render/render_spritelibrary_test.go`:

* `Add` then `Get` round-trips a sprite, and `Get` on an unknown name returns
  false.
* `Add` returns the same pointer it was given, so chaining works.
* `Names` returns names sorted, verified with entries added out of order.
* `Len` reflects the number of entries.
* `Add` panics on an empty name, on a nil sprite, and on a duplicate name.
* A fresh `NewSpriteLibrary` is empty and independent of `render.Sprites`.

`SpriteShowcaseScene` tests live in `scene/scene_spriteshowcase_test.go` and run
under `task test:headless`, since drawing needs a display. Each builds a library
with synthetic sprites via `NewSpriteShowcaseSceneFor`, calls `Init`, `Update`
and `Draw` against an offscreen `ebiten.Image`, and asserts it does not panic:

* An empty library draws nothing and does not panic.
* A library of only single-animation sprites draws.
* A library of only multi-animation sprites draws.
* A mixed library draws.
* More than ten single-animation sprites, exercising the row wrap.
* `Draw` while not visible is a no-op.
* `SceneName` returns `SpriteShowcaseSceneName` and `LayerIndex` returns 0.

`app` tests cover the wiring without a display where possible: that the flag
registers under `RegisterFlags` and parses, and that `DebugSettings` round-trips
it from settings. Whether the scene ends up registered and exclusively visible
is asserted through `App.Manager()`.

## Documentation and release

* `docs/debugging.md` gains a "Sprite showcase" section, required by
  `CLAUDE.md` for any new debugging or development tool. It covers the
  `--sprite_showcase` flag and its settings-file equivalent, the W/A/S/D and
  Q/E camera keys, how a game registers sprites with `render.Sprites.Add`, and
  the empty-library warning.
* `render/doc.go` and `scene/doc.go` gain a sentence each for the new library
  and scene.
* `ARCHITECTURE.md` is updated where it describes the `render` and `scene`
  packages.
* `docs/performance_optimization.md` is reviewed per `CLAUDE.md`. The showcase
  redraws every sprite every frame with no culling, which is acceptable for a
  debug tool but worth recording if the entry does not already exist.
