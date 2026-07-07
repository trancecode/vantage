# Architecture

Vantage is a reusable 2D game engine built on [Ebitengine](https://ebitengine.org/)
and the entity-component-system module
[`github.com/trancecode/ecs`](https://github.com/trancecode/ecs). Games consume
it as a Go module.

Read this first for orientation, then the `doc.go` of the package you care
about, then the design specs under `docs/superpowers/specs/` for rationale on
the bigger subsystems (the `sim` scheduler and savegame serialization).

## Engine charter

The engine holds *mechanism*, never a single game's *rules*. Scheduling,
movement, spatial indexing, rendering, and the app loop live here; combat
scoring, stats, equipment, AI directives, catalogs, and other game content live
in the consuming game. When deciding where code belongs, ask "would every 2D
game on this engine want it?" If not, it belongs in the game. This boundary is
the most important invariant to preserve when extending the engine.

## Package map

| Package | Purpose | Depends on (vantage / ecs) |
|---|---|---|
| `util` | Shared infrastructure: `Time`, `Profiler`, `ScreenLogger`, `PriorityQueue`, logging, debug HTTP | — |
| `geometry` | 2D geometric types and operations (`Vector2`, shapes) | `util` |
| `config` | Layered configuration loader (`Loader`, `Duration`) | — |
| `asset` | Engine-bundled assets (default fonts), embedded | — |
| `pathfinding` | A* search with terrain awareness | — |
| `tilemap` | Tile coordinates, `SpatialGrid` (range queries), `TileOccupancyManager` | `geometry`, `ecs` |
| `sim` | Deterministic event scheduling: `Driver`, `EventQueue`, `Event`, `TickSystem`, `EventHandler` | `util`, `ecs` |
| `motion` | Movement components (`Spatial`, `Movement`) and `System` (a tick system) | `geometry`, `pathfinding`, `tilemap`, `ecs` |
| `render` | Graphics layer: camera, sprites, text | `asset`, `geometry` |
| `ui` | Interactive user-interface components | `asset` |
| `scene` | `Scene` interface and the `Manager` that drives scenes | `render`, `ui` |
| `app` | Top-level `App` (implements `ebiten.Game`), window, screenshots | `config`, `render`, `scene`, `util` |

## Dependency layering

The graph is acyclic and splits into two stacks over shared foundations.

* Foundations (no vantage dependencies): `util`, `config`, `asset`,
  `pathfinding`.
* Simulation stack (the deterministic game substrate): `ecs` + `sim` + `motion`
  + `tilemap` + `pathfinding` + `geometry` + `util`. This is what a game's world
  and logic are built on; it has no dependency on rendering.
* Presentation stack (Ebitengine graphics and the app loop): `render` + `ui` +
  `scene` + `app` + `asset`. It draws whatever the game hands it.

Only `sim`, `motion`, and `tilemap` depend on `ecs`. The simulation and
presentation stacks meet only in the game and in `app`, never inside the
simulation packages, so headless simulation and testing stay possible.

## Key abstractions

* **ecs (external).** Entities are opaque `ecs.EntityId`s; components are plain
  structs held in per-type sparse-set stores, accessed through
  `ecs.Accessor[C]` handles (`Get`, `Add`, `All`, ...). The game owns the
  component types; the engine packages define the components they operate on
  (for example `motion.Spatial`).
* **`sim.Driver` and the tick-versus-event split.** The `Driver` owns the game
  clock (`Now()`) and advances it event by event to a target. Two channels of
  time:
  * `TickSystem.Tick(elapsed)` runs continuously over each interval between
    stops (for example `motion.System` moving entities). It is for things that
    accrue *over* an interval.
  * A single `EventHandler.HandleEvent(now, e)` drains the one `EventQueue` of
    `Event{Time, Entity, Key}` at each stop, for things that happen *at* an
    instant. `Key` is a game-defined discriminator; ordering is a pure function
    of the queued set (`Time`, then `Key`, then `Entity`), so runs are
    deterministic regardless of insertion order.

    See `docs/superpowers/specs/2026-07-05-sim-event-queue-v2-design.md`.
* **Spatial indexing (`tilemap`).** `SpatialGrid` answers range queries;
  `TileOccupancyManager` tracks which entity occupies which tile. Movement and
  AI use these instead of scanning all entities.
* **Debug and profiling (`util`).** `Profiler` accumulates named wall-time
  timings (the `Driver` records its systems and drain into one when attached);
  `ScreenLogger` and the debug HTTP server surface diagnostics. These never
  affect the simulation. See `docs/debugging.md` for usage and configuration.

## Building a game on vantage

A game defines its own `World` that composes the engine pieces:

```go
type World struct {
    ecs    *ecs.World       // entities and components
    Driver *sim.Driver      // owns the clock and event queue
    motion *motion.System   // movement tick system
    // game-specific: component Accessors, catalogs, rules, RNG, ...
}

func NewWorld() *World {
    w := &World{ecs: ecs.NewWorld(), motion: &motion.System{ /* ... */ }}
    w.Driver = sim.NewDriver(&handler{world: w}) // handler dispatches events by Key
    w.Driver.RegisterTickSystem(w.motion)        // continuous systems, in phase order
    return w
}

// The game schedules discrete events on entities:
//   w.Driver.Queue().Add(sim.Event{Time: t, Entity: id, Key: keyAction})
// and advances time:
//   w.Driver.RunUntil(target)
// The event handler resolves each event from the entity's components.
```

For presentation, the game embeds `app.App` (which implements `ebiten.Game`) and
drives scenes through `scene.Manager`, calling `World.Driver.RunUntil` from its
update step. The simulation stack runs headless in tests without any of this.

Worked reference: the `herve-quiroz/lockstep` game is the canonical consumer of
the simulation stack (its `core` package wires exactly the pattern above).

## Extending the engine

* Keep the charter: add mechanism, not one game's rules. If a change encodes a
  specific game's content, it belongs in the game.
* Follow `docs/styleguide.md` (errors, naming, file layout, documentation) and
  the file-naming convention (`<pkg>_<topic>.go`, tests `<pkg>_<topic>_test.go`,
  package overview in `doc.go`).
* Prefer contributing missing ECS capabilities upstream to `trancecode/ecs`
  rather than re-introducing boilerplate in a consuming package.
* Run the required checks before pushing: `task lint`, `task test:headless`
  (the `render`, `ui`, `scene`, and `app` packages need a display, provided by
  xvfb in that target), and `go vet ./...`. Set `GOMODCACHE=/tmp/go-mod-cache`.
* Larger subsystems get a design spec under `docs/superpowers/specs/` before
  implementation; keep it as the durable rationale.
