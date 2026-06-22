# Vantage engine: design

## Overview

Vantage is a reusable 2D game-engine module extracted from the NRG codebase. It
bundles the engine-level functionality that NRG already developed (scene
management, 2D rendering, camera, sprites and animation, UI, spatial and
movement systems, configuration, logging, and performance monitoring) into a
standalone module that multiple games can import.

NRG becomes the engine's first consumer. The second is a planned top-down,
tile-based, turn-based SRD5e tactical RPG (the "heatsink" cyberpunk game). The
engine is built on [Ebitengine](https://ebitengine.org/) for rendering and
[`github.com/trancecode/ecs`](https://github.com/trancecode/ecs) for the
entity-component-system layer.

## Goals

* Extract the already-generic, game-agnostic packages from NRG into a shared
  module that games import.
* Let a game be written by implementing scenes and registering them, without
  ever touching Ebiten's `Game` interface or the window/run plumbing.
* Provide a layered configuration service that the engine owns and games extend.
* Invert the asset dependency so games supply their own sprites and fonts; the
  engine ships sensible defaults.
* Keep the engine free of any single game's content or rules.

## Non-goals

* No new gameplay features. This is an extraction and generalization effort.
* No console scene and no scripting engine in this work. They are future,
  engine-native additions with their own design cycles (see Out of scope).
* No change to NRG's gameplay behavior. After migration, NRG runs as before.

## Module and repository

* Module path: `github.com/trancecode/vantage`, sibling to `trancecode/ecs`.
* Public repository, for convenient importing from game repositories.
* Depends on `trancecode/ecs`. The ECS layer is a first-class engine dependency;
  games built on Vantage use ECS.

## Licensing

* Engine code: MIT, matching `trancecode/ecs`. A `LICENSE` file is added at the
  repository root.
* Bundled fonts: SIL Open Font License (OFL). Google Sans Flex and Google Sans
  Code are both OFL-licensed and may be embedded and redistributed.
* OFL requires the license text to travel with the font files. Each embedded
  font lives alongside its license as `data/font/<name>/OFL.txt`.
* Optional `THIRD_PARTY.md` listing dependency attributions (Apache-2.0 for
  Ebitengine, OFL for fonts). Good hygiene, not strictly required.

## Engine dependencies

All current NRG dependencies used by the extracted packages are permissive and
MIT-compatible:

* `github.com/hajimehoshi/ebiten/v2` (Apache-2.0)
* `github.com/BurntSushi/toml` (MIT)
* `github.com/rs/zerolog` (MIT)
* `github.com/spf13/pflag` (BSD-3-Clause)
* `github.com/trancecode/ecs` (MIT)
* `github.com/stretchr/testify` (MIT, test-only)

## Package layout

### Engine packages (extracted from NRG, made game-agnostic)

| Package | Contents |
|---------|----------|
| `geometry` | `Vector2`, `Rectangle` |
| `util` | Logging (zerolog), `Time`, `PriorityQueue`, debug HTTP server, `Watchdog` (performance monitoring), number helpers |
| `render` | `Camera` (pure transform), `Sprite`, `Animation`, `AnimationType`, sprite-sheet loading, `TextWriter`, `CameraController` |
| `ui` | `Button`, `Dialog` |
| `scene` | `SceneName`, `Scene`, `BaseScene`, `Manager`, `App` |
| `config` | Layered configuration service |
| `pathfinding` | A* with `TerrainProvider` interface |
| `motion` | `PositionComponent`, `MovingComponent`, movement physics (depends on ECS) |
| `tilemap` | `TileCoord`, coordinate conversion, `TileOccupancyManager`, `SpatialGrid` (depends on ECS) |

### Stays in NRG (game-specific)

* `rts` — RTS simulation, world, AI behaviors, factions, drawing.
* `game` — reduced to a thin constructor that builds a Vantage `App`, registers
  NRG's scenes, and supplies NRG's configuration.
* `data` — NRG's own assets and its sprite catalog.

## Scene management

### Scene identity

`type SceneName string`. The engine defines the type; each game defines its own
constants of that type (for example `SceneName("rts")`, `SceneName("menu")`).
This replaces NRG's hard-coded `SceneName int` enum (`SceneRTS`,
`SceneShowcase`, `SceneDialog`), which is game content.

### Scene interface

The `Scene` interface keeps its current shape (`Init`, `Update`, `Draw`,
`LayerIndex`, visibility, focus) with `SceneName()` returning the typed string.
`BaseScene` provides default implementations.

### Manager

The scene `Manager` owns the registry and lifecycle currently scattered across
NRG's `game` package:

* Register scenes keyed by `SceneName`.
* Visibility control, including `ShowOnly`.
* Focus control, including `SetExclusiveFocus`.
* Layered update and draw: iterate scenes in `LayerIndex` order; focused scenes
  read input directly via Ebiten (input remains a convention, not an
  abstraction).

### App

The `App` is a thin type that implements Ebiten's `Game` interface so games do
not have to. It embeds a `Manager` and owns the genuinely generic loop plumbing
currently in NRG's `game` package:

* `Layout`, the Ebiten update/draw cycle.
* Window setup (title, size, fullscreen) and the `ebiten.RunGame` call, exposed
  through an `App.Run()` method.
* The debug-frame `Watchdog`.
* Screenshot capture (path pattern, delay, frequency).
* The `exitAfter` hook used by automated testing and profiling.

The `App` is kept strictly free of game concepts. It does not know about NRG's
configuration struct or scenarios. A game constructs the `App`, registers its
scenes, and calls `Run()`. The `Manager` remains accessible for games that want
finer control.

## Configuration service

The engine owns configuration as infrastructure. NRG's existing pattern (decode
an embedded default, then decode a local file into the same struct, with
BurntSushi performing field-level partial merge) generalizes into a layered
service.

### Layers, lowest to highest precedence

1. Engine `settings.toml`, embedded in the Vantage module.
2. Game-registered default file(s), passed as bytes or path. A game can override
   engine defaults here when an engine setting must take a specific value for
   the game to work.
3. Local `settings.toml` on disk, overriding any default.
4. `--config_override key=value` flags, highest precedence.

### Registration and routing

* The engine registers its own settings struct, organized into TOML-tagged
  sections (for example `[window]`, `[camera]`, `[debug]`).
* A game registers its own settings struct(s) with their own sections (for
  example `[game]`, `[ai]`).
* Each TOML layer is decoded into every registered target. BurntSushi silently
  ignores sections a given struct does not define, so `[window]` lands on the
  engine struct and `[game]` on the game struct. `MetaData.Undecoded()` is
  checked across the union of targets to surface typo'd keys rather than
  silently dropping them.
* `--config_override` uses a single flag, owned by the engine. Each
  `section.key=value` is routed by reflection over TOML tags to whichever
  registered struct owns that section (the approach proven in griddelve's
  `ApplyOverrides`).

This keeps one `--config_override` surface for the player while the engine and
each game own their settings independently.

## Asset injection

The sprite and font mechanics in `render` are generic; only NRG's catalog and
default-font reference are content. The dependency is inverted so assets flow in
from the game.

### Sprites

* The sprite catalog (NRG's `render_sprite_data.go`: `SpriteCharacter`, the
  `SpritePlains*` entries, loaded from `data.Image*`) leaves the engine entirely
  and moves into NRG's `data` package.
* The engine keeps `LoadSprite(img, width, height, indexes, durations)`,
  `Sprite`, `Animation`, and related mechanics. Games build their own catalogs.
* `AnimationType` (the directional Move/Idle/Attack enum) stays in the engine as
  a shared default, since helpers and `ui` lean on it. Games may add their own
  animation types.

### Fonts

* `TextWriter` and `ui` take a font (`*text.GoTextFaceSource`) at construction
  instead of referencing NRG's `data.FontDefault`.
* The engine embeds two default fonts so text works out of the box:
  * Google Sans Flex for proportional text.
  * Google Sans Code for monospace text.
* Each font ships with its `OFL.txt` under `data/font/<name>/`.
* Games may supply their own fonts.

## Camera and camera controller

The camera splits into a pure transform and a pluggable controller.

* `Camera` is pure transform math: position, zoom, `WorldToScreen`,
  `ScreenToWorld`, draw-option helpers. It contains no Ebiten input handling.
* `CameraController` is an optional, pluggable input handler that drives a
  `Camera`. The default controller provides the pan/zoom scheme (keyboard pan,
  zoom, mouse drag, wheel zoom) out of the box, with bindings and pan/zoom
  speeds read from the `[camera]` config section. It is deliberately not named
  after any genre.
* Games opt in. The RTS and the tactical RPG both use the default controller to
  look around the map; the tactical RPG additionally drives the camera directly
  (for example snap-to-selected-unit).

## Engine flags versus game flags

* The engine owns flags for engine settings (window size and fullscreen,
  screenshot capture, run-for duration, log level, debug, debug HTTP server).
  Every game inherits these, which is correct since they are universal.
* Package-level flags currently declared in `render`
  (`use_placeholder_sprite_images`) and `util` (`DebugMode`) move into the
  engine's flag set or config, removing scattered global flag registration.
* Games declare flags only for their own logic (for example NRG's `scenario`,
  `list_scenarios`). Engine settings are not re-exposed as game flags; they are
  reachable through the config system.

## Migration and extraction sequencing

1. Create the `trancecode/vantage` module with `LICENSE` (MIT) and the package
   skeleton.
2. Move the engine packages across, preserving structure: `geometry`, `util`,
   `render`, `ui`, `scene`, `pathfinding`, `motion`, `tilemap`.
3. Invert the asset dependencies: remove the sprite catalog from `render`, embed
   the two OFL fonts, and change `TextWriter`/`ui` to take an injected font.
4. Generalize scene management: `SceneName` as a typed string, lift the
   `Manager` and `App` out of NRG's `game` package into `scene`.
5. Build the configuration service in `config` with layered loading,
   registration, and `section.key` override routing.
6. Move engine flags and package-level flags into the engine flag set / config.
7. Repoint NRG: add the Vantage dependency, delete NRG's local copies of the
   moved packages, move NRG's sprite catalog into NRG's `data`, and reduce
   NRG's `game` package to a thin constructor over the Vantage `App`.
8. Verify NRG builds and runs unchanged (lint, headless tests, a visual check).

## Out of scope

These are future, engine-native features. They are not extracted now (no code
exists to extract) and are not designed here. Each gets its own design cycle.

* Console scene. NRG's backlog describes a scene that renders recent log lines
  and accepts commands. Because the `scene` system and `util` logging both move
  into the engine, the console becomes a small engine-native scene later.
* Go-like scripting engine. NRG's requirements describe a scripting language
  usable from the console. This is a separate, larger project.

The architecture stays ready to host both: the console is a scene over engine
logs, and the scripting engine plugs into the console.

## Details to resolve during implementation

* Which concrete scenes move. The generic dialog scene is a candidate for the
  engine; NRG's sprite showcase scene is game content and stays in NRG.
* The exact engine config schema (`[window]`, `[camera]`, `[debug]`, and which
  keys each holds).
* Font sourcing: vendoring the two OFL font files and their `OFL.txt`.
* The `App.Run()` and configuration entry-point signatures.

## Risks

* API churn. As a shared module, breaking changes ripple to every game. With two
  consumers now, the API is shaped by concrete needs; speculative generality is
  avoided.
* Extraction surface. Moving nine packages at once is mechanical but broad. The
  clean existing layering (no engine package imports `rts` or `game`) keeps the
  risk low. NRG's build and tests are the acceptance check.
