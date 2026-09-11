# Benchmark suite design

## Purpose

Give vantage a benchmark suite that answers the questions a game design
discussion starts from: how many moving bodies a frame can hold, how many
perception checks, how much scheduled work per beat, how many sprites per frame.
Each answer is a measured cost curve and the point where it stops fitting a
stated budget, recorded with the machine and commit, and re-measurable with one
command.

The request originates from `nrg`. Its road-speed decision had to wait on a
one-off measurement (vantage 5c863c1); the numbers should be on tap instead.
vantage is infrastructure, and its limits should be measurable on demand.

This adds benchmarks, a Taskfile target and documentation. It changes no
exported API and no behaviour, so it needs no release tag.

## Budgets

* **Frame budget: 16.7 ms.** One update at Ebitengine's default 60 updates per
  second. Work a game runs every frame is judged against it: the movement tick
  (nrg calls `motion.System.Tick` from its scene's `Update`), range queries,
  drawing.
* **Beat budget: 1 s.** nrg's simulation beat. Work a game schedules per beat,
  such as event dispatch and movement decisions, is judged against it as well.

A frame is shared with everything else a game runs, so every limit reports two
crossing points: the largest measured parameter whose cost fits **1 ms**, a
per-system share of a frame, and the largest that fits the **whole budget**.
Crossing points are read off measured rows, never extrapolated; each table also
gives the cost per unit (per entity, per query, per sprite) so a designer can
estimate between rows.

**Time acceleration.** nrg lets the player run the clock at 1x, 2x, 5x and 10x,
and acceleration runs more fixed-size ticks per wall second, never larger ticks
(nrg's terrain generation design, "Time and acceleration"). Per-tick and
per-beat work therefore fits 10x only if it fits a tenth of its budget. Those
limits (range queries, index maintenance, reservation churn, the movement tick,
movement decisions, event dispatch) report a third crossing point, **at 10x**:
the largest measured parameter that fits the budget divided by 10 (1.67 ms for
per-tick work, 100 ms per beat). Draw ordering and draw throughput do not: a game
draws once per frame whatever the clock rate, so acceleration never multiplies
them.

## Current state

At de072e5 benchmarks exist in `pathfinding` (per heuristic strategy, recorded
in [pathfinding_performance.md](../../pathfinding_performance.md)), `sim`
(`BenchmarkEventQueueAdd`, `BenchmarkEventQueueSteadyState`, figures quoted in
[performance_optimization.md](../../performance_optimization.md)) and `render`
(`BenchmarkAutoCropAtlas`, a load-time cost). Nothing measures `motion`,
`tilemap`, the `sim` driver, draw ordering or draw throughput, and there is no
`task` target for benchmarks.

## What is measured

Every benchmark is parameterised by the dimension a designer asks about, uses
fixtures built from fixed hashes so runs are identical, and reports
`b.ReportAllocs`. Where a benchmark's operation is not a single call, its doc
comment says what one op is.

### Range queries (`tilemap.SpatialGrid.GetRange`)

**Question:** how many perception checks per frame at radius R, and what does
the grid's cell size buy? nrg's world simulator design calls range queries the
largest single cost in the engine, growing with the square of the radius.

`GetRange` visits every cell in the query rectangle, collects the entities in
each, and sorts the result by `EntityId`, so its cost has a part that grows with
the rectangle's cell count and a part that grows with the entities found.

* One op: one `GetRange` over a square of half-width R centred in a populated
  world.
* Parameters: R in 4, 8, 16, 32, 64 tiles; density in 0.01, 0.1, 0.5 entities
  per tile; cell size in 1, 4, 16.
* Fixture: a square world 256 tiles wide, populated at the given density at
  hashed positions, indexed at the given cell size.
* Metrics: ns/op, allocations, and `entities-found/op`.

### Index maintenance (`tilemap.SpatialGrid.UpdateEntityPosition`)

**Question:** what does moving N bodies cost the spatial index?

* One op: one entity moves one tile across a cell boundary (cell size 1).
* Parameters: population 1k, 10k, 100k at density 0.1.
* Metrics: ns/op, allocations.

### Reservation churn (`tilemap.TileOccupancyManager`)

**Question:** how much occupancy churn fits a frame?

* One op: one reservation moves: `ClearOccupant` on its tile, `SetOccupant` on the
  next one.
* Parameters: 1k, 10k, 100k occupied tiles.
* Metrics: ns/op, allocations.

### Movement tick (`motion.System.Tick`)

**Question:** how many moving bodies per frame?

* One op: one `Tick` advancing every moving entity by 16.7 ms, with the spatial
  grid kept in sync.
* Parameters: 1k, 10k, 100k moving entities; constant-speed and eased moves.
* Fixture: entities spread over a world at density 0.1, each moving toward a
  destination far enough that none arrives during the benchmark.
* Metrics: ns/op, allocations.

### Movement decisions (`motion.System.MoveEntityTowards`, `MoveEntityTowardsArea`)

**Question:** how many movement decisions per frame, or per beat?

* One op: one decision from a fixed position: plan the path and issue the next
  step, on open finite terrain with occupancy, with nil heuristic.
* Parameters: `MoveEntityTowards` for journeys of 8, 32 and 128 tiles;
  `MoveEntityTowardsArea` for area radii of 1, 4 and 8 tiles, with the area's
  centre 32 tiles away.
* Metrics: ns/op, allocations.
* Longer journeys and other heuristics are pathfinding's to measure, and the
  limits document links there.

### Event dispatch (`sim.Driver.RunUntil`)

**Question:** how many scheduled actions per beat?

* One op: one `RunUntil` advancing the clock by one beat (1 s), dispatching every
  event due in it through a handler that re-schedules each event one beat later,
  with one registered no-op tick system.
* Parameters: 1k, 10k, 100k events per beat, at distinct times spread across the
  beat, so each is its own stop where tick systems run.
* Metrics: ns/op, allocations, and `ns-per-event/op`.
* The existing `EventQueue` benchmarks stay, and their figures are folded into the
  limits document.

### Draw ordering (`render.DrawList`)

**Question:** what does ordering N drawables cost per frame? This is the
display-free part of drawing.

* One op: `Clear`, N `Add` calls with hashed layers and Y values, and one `Each`
  over the payloads.
* Parameters: 1k, 10k, 100k payloads.
* Metrics: ns/op, allocations.

### Draw throughput (`render.Sprite.Draw`, `render.TextWriter.Draw`)

**Question:** how many sprites and labels per frame?

Drawing only executes inside Ebitengine's game loop, and Ebitengine permits one
`ebiten.RunGame` per process. The benchmark therefore lives in a package of its
own that contains only test files, `render/drawbench`, as `render/pixeltest`
does, and runs under a virtual display (`xvfb-run -a`).

* `BenchmarkDrawThroughput` runs one `RunGame` with vsync off. Inside the loop it
  steps through each count, draws that many copies of a synthetic 16-tile sprite
  (or text labels) at hashed positions every frame for a fixed number of frames,
  and records the mean frame time. It reports one metric per count
  (`ms-per-frame-sprites-1k/op` and so on). It ignores `b.N` beyond the first
  call, runs its game loop once per process, and says so in its doc comment.
* Parameters: sprites 100, 1k, 10k, 50k; labels 10, 100, 1k.
* Caveat, stated in the doc comment and the limits document: under a virtual
  display the GPU work runs on Mesa's software rasterizer, so absolute frame times
  describe that machine, not a game's hardware. What transfers is the scaling and
  the CPU-side cost; a game re-measures on its own machine with the same
  benchmark.

## Skipped

* **ui widgets.** A screen holds a handful; no dimension scales.
* **scene.Manager.** It dispatches to scenes and costs what the scenes cost.
* **geometry, easing, config.** No design question depends on their cost.
* **pathfinding.** Already measured per strategy; linked, not re-measured.
* **Auto-crop.** A load-time cost, already benchmarked, outside frame and beat
  budgets.

## Task target

```bash
task bench                                  # the whole suite
task bench PKG=./tilemap/                   # one package
task bench PKG=./tilemap/ BENCH=GetRange    # one benchmark
task bench BENCHTIME=1x                     # counts and a smoke run
```

`task bench` runs `xvfb-run -a go test -run '^$' -bench "$BENCH" -benchtime
"$BENCHTIME" $PKG`, with `PKG` defaulting to `./...`, `BENCH` to `.` and
`BENCHTIME` to Go's default. The virtual display covers `render/drawbench`;
display-free packages run under it unaffected, as they already do in
`task test:headless`.

## Limits document

`docs/performance_limits.md`, in the style of
[pathfinding_performance.md](../../pathfinding_performance.md):

* The machine (CPU, core count, Go version), the commit, and the command that
  produced the figures.
* The two budgets and how crossing points are read.
* A summary at the top: one row per aspect with its headline limit and crossing
  points, so a reader sees every aspect in one place.
* One section per aspect above: the question, the cost curve as a table, the
  crossing points (two, or three for per-tick and per-beat work), the cost per
  unit, and the command that re-runs that aspect alone.
* Pathfinding gets a short section and a summary row: the headline envelope per
  strategy (nil octile, `ScaledOctile`, `CoarseCost`) quoted from
  pathfinding_performance.md, with the link. The event queue's existing figures
  are quoted the same way, linking their record. Neither is re-measured or copied
  in full.
* A skipped section with the reasons above.

Anything a benchmark reveals as worth optimising gets an entry in
`docs/performance_optimization.md`; nothing is fixed in this work.

## Rulings

1. Two crossing points per limit, 1 ms and the whole budget, read off measured rows
   without extrapolation.
2. Frame budget 16.7 ms at Ebitengine's default rate; beat budget 1 s from nrg.
3. Draw throughput runs as one game-loop benchmark reporting per-count metrics,
   because Ebitengine permits one `RunGame` per process.
4. `task bench` runs every package under a virtual display rather than splitting
   display-dependent packages out, matching `task test:headless`.
5. No release tag: benchmarks and docs change no exported API or behaviour.
6. Per-tick and per-beat limits report a third crossing point at 10x (budget
   divided by 10), because nrg's time acceleration runs more ticks, not larger
   ones. Draw limits do not, because drawing happens once per frame at any clock
   rate. A column per limit rather than a derivation rule, because a designer
   reading a row should not have to divide.
7. The limits document opens with a summary of every aspect, pathfinding included
   as quoted headline envelopes with a link.

## Testing

The benchmark files compile as part of `go test ./...`, so CI catches a benchmark
that no longer builds; CI never runs them. Each benchmark fails loudly (`b.Fatalf`)
when its fixture is wrong, such as a query finding no entities at a density above
zero or a decision that starts no move. The suite is run once with the default
benchtime to record the limits document, and `task bench BENCHTIME=1x` is the
smoke check a later change runs before touching these files.
