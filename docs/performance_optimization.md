# Performance optimization opportunities

This document tracks potential performance optimizations that were deliberately
not applied, to keep code clear and faithful to its origin, per the project's
performance guidance (optimize for clarity unless there is a measured need).

## Screenshot pixel conversion (app/app_screenshot.go)

`SaveScreenshot` converts the frame to an `*image.RGBA` with a per-pixel
`rgbaImg.Set(...)` loop. Ebiten's `Image.ReadPixels` already returns bytes in
RGBA order matching `image.RGBA.Pix`, so the loop could be replaced with a
single `copy(rgbaImg.Pix, pixels)`, which is both simpler and substantially
faster for large frames (the per-pixel path does ~2M bounds-checked calls at
1080p). Left as-is for now because capture is not on the hot path; revisit if
high-frequency frame-sequence capture becomes a bottleneck.

## Frame capture pixel conversion (visualtest/capture/capture.go)

`imageFromScreen` (used by `SavePNG`) converts the frame to an `*image.RGBA`
with the same per-pixel `Set(...)` loop as `app.SaveScreenshot`, and carries
the same optimization opportunity: `Image.ReadPixels` returns RGBA-ordered
bytes that could be `copy`ed straight into `image.RGBA.Pix`. Left as-is to
mirror the existing screenshot code; revisit alongside the app-side entry above
if high-frequency capture becomes a bottleneck.

## Image comparison pixel scan (visualtest/diff.go)

`CompareImages` scans row by row, calling `image.Image.At` and converting each
pixel through `color.RGBAModel` for both images. When both images are already
`*image.RGBA` (the common case for captured frames), a direct `bytes.Equal` on
the `Pix` slices — falling back to the per-pixel path only to locate the first
differing coordinate — would be substantially faster than ~W*H bounds-checked,
interface-dispatched `At` calls plus color conversions. Left as a clear
per-pixel scan because a visual-regression diff runs offline, not on any hot
path; revisit if diffing large golden sets becomes slow.

## Alloc-free event queue heap (sim/sim_eventqueue.go)

`EventQueue` (like `util.PriorityQueue`) is built on `container/heap`, whose
interface is `any`-based. Each `Add` boxes the element into an interface and
each `Next` boxes the popped value on the way out, so a pop-and-reschedule cycle
costs 2 allocations. Benchmarks (`sim_eventqueue_bench_test.go`) measure a
steady-state pop+insert at ~167 ns (100 queued) rising to ~341 ns (100k queued),
each with 2 allocs/op and 48 B/op. `BenchmarkDriverRunUntil` shows the same
2 allocations and 48 B per event dispatched through `sim.Driver`, at 249.7 ns
per event for 1,000 events per beat and 302.4 ns for 100,000 (see
[performance_limits.md](performance_limits.md#event-dispatch)). A
generics-native heap (hand-written sift-up/sift-down over the `[]T` backing
slice instead of `container/heap`) would remove both allocations and the
per-operation interface dispatch, at the cost of ~30 extra lines. Left as-is
because the scheduler is not alloc-bound at realistic event rates (100,000
events per beat is 4.8 MB per beat of tiny, short-lived garbage); revisit if
the event queue shows up in allocation profiles under load.

## Event queue Reschedule/Cancel index (sim/sim_eventqueue.go)

`EventQueue.Reschedule` and `Cancel` locate their target with an O(n) scan
(`indexOf`) before an O(log n) `heap.Fix`/`heap.Remove`. This keeps `Add`/`Pop`
(the hot path) index-free. A live `map[(entity,key)]int` position index would
make Reschedule/Cancel O(log n), but every `Add`/`Pop`/`Swap` would then have to
maintain it, taxing the common path for a rare operation. Left as a scan because
reschedules (stagger) and cancels (interrupt/death) are occasional and the queue
holds roughly one event per active entity; revisit only if reschedule/cancel
shows up as hot in a profile.

## Per-search allocation in A* (pathfinding/astar.go)

`FindPath` allocates a `map[Coord]*pathNode`, a heap, and one `pathNode` per
touched tile on every call. Benchmarks (`pathfinding/astar_bench_test.go`)
measure a 256-tile journey at ~314 µs with 810 allocations and 129 kB, roughly
1.2 µs per expanded tile, which is dominated by hashing and node allocation
rather than by the search itself. Storing nodes by value in a
`map[Coord]pathNode` (with `parent` as a `Coord`), or backing the node store
with a dense per-map slice reused across calls, would remove almost all of it.
Left as-is because the measured blocker was never the cost of a successful
search but the cost of an unsuccessful one, which is now rejected up front; see
[pathfinding_performance.md](pathfinding_performance.md). Revisit if long
journeys are re-planned often enough for the constant factor to matter, which
would also make path caching with explicit invalidation worth having.

## Distant unreachable goals still flood the map (pathfinding/astar.go)

`FindPath` rejects a goal whose own tile or whose every neighbour is unenterable
without searching, which covers agents converging on one destination. A goal cut
off by a barrier further away is still established as unreachable the only way
A* can: by expanding every reachable tile, ~92k expansions and ~130 ms on a
304x304 map, or by spending the caller's expansion budget, 100,000 expansions
and ~77 ms at the budget `motion.System` documents. The budget keeps the cost
finite on an edgeless map but does not make it small. Ruling such goals out
cheaply needs connectivity components (a per-tile region id, recomputed or
repaired when terrain changes), which is a design decision about terrain-change
notification rather than a local optimization. Left undone until a workload
actually routes toward walled-off goals often enough to matter; see
[pathfinding_performance.md](pathfinding_performance.md).

## Coarse cost field (pathfinding/coarse_cost.go, pathfinding/coarse_cell.go)

A fresh `CoarseCost` field reads terrain a tile search does not otherwise need,
to look ahead for a faster or slower detour. On the measured journeys
(`BenchmarkFindPathHeuristics`, see
[pathfinding_performance.md](pathfinding_performance.md)) that cost 29 to 651
extra 64-tile chunks beyond what the tile search itself touches, and a cold
call runs from about 40 ms on a 250-tile offset-road journey to about 1.9
seconds on a 2,000-tile Reaches journey. A game-supplied per-cell cost source,
answering a cell's crossing rates from knowledge the game already has (such as
a world graph of roads) without reading tiles, would remove that cost for a
lazily generated world. It is additive to `CoarseCost` and left undone until a
game has such a source.

Building one cell allocates its walkable, speed and cost slices and runs
`container/heap` with `any`-based interface boxing. `BenchmarkCoarseCostCellBuild`
measures about 459 µs per cell on grass ground, 322 µs on the grid, and 461 µs
on Reaches ground. A reused scratch buffer and a typed heap would cut the build
cost, which matters only for cold searches, since a warm `CoarseCost` never
rebuilds a cell it has already built.

A search with `CoarseCost` pays a fixed overhead even on a short hop: it builds
its goal cell and the cells of the tiles it estimates from, and allocates its
per-search maps; onto new ground, a short hop builds a few cells.

One known worst case is a cell that can be crossed one way but that no coarse
edge reaches, for example a cell with one infinite rate whose neighbours block
the other axis. The uncrossable-cell shortcut in `coarseSearch.value` does not
catch it, so one value request for such a cell runs the coarse search until it
has settled `CellBudget` cells. A guard that treats a cell whose eight coarse
edges are all infinite like an uncrossable one would bound that, at the price
of building its eight neighbour cells on every such request. It is left undone
because pools shaped like round blobs rarely produce such cells.

Each search allocates its own coarse-search cost and closed maps, even though
many agents converging on one destination run the same coarse search from
scratch. A small per-goal cache of settled cell values would let those searches
share it instead of each rebuilding it.

`ScaledOctile` remains expensive by design: about 0.8 x length² expansions on
open grass at a declared speed of 2.0, so 3.2 million expansions for a
2,000-tile journey where octile distance expands 2,001
(`BenchmarkFindPathHeuristics`). A landmark heuristic (precomputed shortest-path
costs from a few chosen tiles, compared through the triangle inequality) would
recover most of that while staying optimal, but needs precomputed data
invalidated when terrain changes, which is a design decision rather than a
local optimization.

## Path-following search costs (motion/motion_towards.go)

`findAreaTarget` discards candidate tiles that no path can end on by tile
lookup and searches the rest nearest first, so a movement decision normally
costs a single A* run. What remains is the sealed-off candidate: a tile that is
in bounds, walkable and unreserved, and whose immediate surroundings are open
enough that `FindPath` cannot reject it up front, yet is cut off from the entity
by terrain further away. Such a tile costs a search that explores the entity's
whole region before failing, and a ring can hold several of them. That is the
caller-side face of the section above, and it is left alone for the same reason:
ruling it out cheaply needs connectivity information the pathfinder does not
keep, and sealed pockets inside a target area are rare. Separately,
`moveAlongPath`'s fallback (taken whenever no waypoint on the direct path is
reachable) scans an O(maxTileDistance^2) grid of tiles around the entity,
calling `CanReach` per cell. It is ported verbatim from the game sources
(nrg/lockstep) and is only worth optimizing if profiling shows it hot.

## SpatialGrid.GetRange result sorting (tilemap/tilemap_grid.go)

`GetRange` sorts its result by EntityId before returning, because cells hold
entities in sets and leaking map iteration order to callers breaks simulation
determinism. The sort is O(n log n) per query on top of the collection cost.
If range queries show up hot in a profile, keep each cell as an EntityId-sorted
slice instead of a set (insert/remove become O(cell size), queries become an
ordered merge with no final sort), which also shrinks per-cell memory.

The result slice also starts empty and grows by appending, so allocations per
query grow with the entities found. `BenchmarkSpatialGridGetRange` measures, at
cell size 1, 3 allocations for a query returning 3 entities (density 0.01,
radius 4, 1,773 ns) rising to 18 for one returning 8,385 (density 0.5,
radius 64, 2,225,697 ns and 259,321 B/op). Summing the visited cells' sizes
before collecting would allow one allocation per query, and a caller-supplied
buffer none. See
[performance_limits.md](performance_limits.md#range-queries) for the full
curve.

## SpatialGrid cell sets on the movement tick (tilemap/tilemap_grid.go)

`SpatialGrid.AddEntity` allocates a new set for a cell that has none, and
`RemoveEntity` deletes the entity from its cell's set but keeps the emptied set
in the grid's map. As bodies travel onto ground nobody occupied, every crossing
into a new cell allocates a set, and the map keeps every cell ever entered.
Reusing emptied sets would remove those allocations, and deleting a cell once
its set is empty would keep the map to occupied cells. `BenchmarkSystemTick`
does not measure this case: its bodies shuttle over cells they have crossed
before (see [performance_limits.md](performance_limits.md#movement-tick)).
Left as-is until a world has bodies roaming over new ground for long periods.

## Movement component re-add on arrival (motion/motion_move.go)

`System.Tick` removes an arrived entity's `Movement`, and a game that sends the
body on from `OnArrival` adds it straight back through `MoveEntity`, whose
`Movements.GetOrAdd` reaches `ecs.Accessor.insertMissing` in
`github.com/trancecode/ecs`. That function returns a pointer to its `value`
parameter on its dead-entity and deferred paths, so the compiler moves `value`
to the heap on every call (`go build -a -gcflags='all=-m' ./motion/` reports
"moved to heap: ecs.value" for it), and every re-add allocates once.

`BenchmarkSystemTick` shows allocations tracking arrivals: 7, 48 and 423
allocs/op for 1,000, 10,000 and 100,000 moving entities, against about 4, 42
and 417 bodies arriving per frame (see
[performance_limits.md](performance_limits.md#movement-tick)). An allocation
profile at commit c6725c1, produced by:

```bash
export GOMODCACHE=/tmp/go-mod-cache
xvfb-run -a go test -run '^$' -bench 'SystemTick/ease=eased/entities=100000$' -benchtime 300x -memprofile tick.mem -memprofilerate 1 -o motion.test ./motion/
go tool pprof -sample_index=alloc_objects -focus 'System\)\.Tick' -top motion.test tick.mem
```

attributes 498,211 of the 1,108,090 objects allocated under `Tick` to
`insertMissing`, reached through `MoveEntity` from the benchmark's `OnArrival`,
and the other 598,368 to `SpatialGrid.AddEntity` through
`UpdateEntityPosition`. The profile covers the 900 warm-up ticks as well as the
300 timed ones, so it does not split the timed ticks' 425 allocs/op between
the two. Taking the pointer only on the paths that return it would let the
immediate re-add allocate nothing. Left as-is because the change belongs in
`github.com/trancecode/ecs`, not here; it is worth making there if allocations
on arriving bodies show up in a game's profile.

## Tile ratio and screen multiplier computed per draw (render/render_sprite.go, render/render_camera.go)

`Sprite.TileRatio` (`TileSize / SourceTileSize`) and `Camera.screenMultiplier`
(`screenHeight / (defaultVerticalTileCount * TileSize)`) are both divisions
recomputed on every call, `TileRatio` inside `buildDrawOp` on every sprite draw
and `screenMultiplier` inside `EffectiveZoom` on every camera use, rather than
being cached on the `Sprite`/`Camera` or constant-folded at build time. This is
deliberate, not an oversight: `TileSize` is a `var`, read where used rather
than captured, specifically so that a game changing it after sprites and
cameras are constructed (which is the normal case, since sprites register from
`init()` before settings are applied) takes effect everywhere. Caching either
value would reintroduce the frozen-scale bug this change exists to remove.
Left as a per-call division because it is cheap relative to the surrounding
draw call; revisit only if profiling shows either division hot, and only with
an invalidation scheme that still reacts to a `TileSize` change after
construction.

## DrawList ordering sort (render/render_drawlist.go)

`DrawList.Each` orders its entries with `sort.SliceStable`, whose swaps go
through a reflection-based swapper. `sort.SliceStable`'s documentation states
no complexity; it runs the same insertion-sort-and-merge algorithm as
`sort.Stable`, which documents O(n log n) calls to Less and O(n log² n) calls
to Swap. `BenchmarkDrawListOrdering` measures one frame's ordering at
336.3 ns per drawable for 1,000 drawables (336,255 ns), 641.2 ns for 10,000
(6,412,156 ns) and 1,019.3 ns for 100,000 (101,930,232 ns, about six frames).
The cost per drawable grows 3.03 times over a hundredfold count, about 9%
above the 2.78 times an O(n log² n) sort's would grow ((log 100,000 /
log 1,000)², derived), a gap within the run-to-run spread recorded in
[performance_limits.md](performance_limits.md#machine-and-how-to-re-run).
The `sort` documentation notes that "in many situations, the newer
slices.SortStableFunc function is more ergonomic and runs faster".
`slices.SortStableFunc` runs the same algorithm without the reflection-based
swapper, and carrying an insertion sequence number in each entry as the final
tie-break would let `slices.SortFunc`, an O(n log n) unstable sort, keep
insertion order for equal keys. Left as-is because 10,000 drawables order in
6,412,156 ns,
inside a frame; revisit if a scene orders tens of thousands of drawables per
frame. See [performance_limits.md](performance_limits.md#draw-ordering).

## Auto-crop startup scan cost (render/render_spriteautocrop.go)

`autoCropAtlas`, used by `LoadSpriteAutoCropped`, scans a sprite sheet's full
alpha channel at load time to find a tight crop box per animation, then
repacks the referenced frames into a smaller atlas. The design assumed this
startup cost was acceptable by analogy with a full-image CPU pass a consuming
game already makes per sheet; that was an inference, not a measurement.

Benchmarks (`render/render_spriteautocrop_test.go`, `BenchmarkAutoCropAtlas`)
measure the scan and repack on a sheet shaped like the real ones (a 38x6 grid
of 192px cells, 8,404,992 pixels, with roughly 4% of each cell opaque). All
figures below were measured on a 4-core machine; a different core count or a
noisier host can shift them.

The headline figure is **32.97 ms/op**, the average of four runs at
`-benchtime 5s` (178-186 reps each, `xvfb-run -a go test ./render/ -run '^$'
-bench AutoCropAtlas -benchtime 5s`). Reproduce it with that command, run a
few times and averaged; that many reps is what makes the number steady.
**Do not use `-benchtime 3x`** (three reps) to reproduce this: it reads
systematically high, because a couple of cold-start iterations dominate a
three-sample average. A literal `-benchtime 3x` run measured 44.26 ms/op, and
a slightly larger but still short `-benchtime 2s` run (60 reps) measured
36.38 ms/op; both are noise from too few samples, not the honest number, but
both are real, reproducible readings a maintainer running the obvious short
command will see.

The published sheets are 7296x10624 (a 38x64 grid), 77,512,704 pixels each,
about 9.22x the benchmark's pixel count (77,512,704 / 8,404,992). Scaling
linearly with pixel count and totaling six sheets:

* Headline (32.97 ms/op): ~304.1 ms/sheet, **~1.82 s total**. Under the
  roughly two-second threshold where a visible startup delay would make
  precomputed crop boxes the better default.
* Short-run readings a maintainer might reproduce by accident: 36.38 ms/op
  extrapolates to ~335.5 ms/sheet, **~2.01 s total**, over the threshold.
  44.26 ms/op (the literal `-benchtime 3x`) extrapolates to ~408.2 ms/sheet,
  **~2.45 s total**, well over.

So the "under threshold" verdict is a close call, not a comfortable one: the
headline figure clears it by under 10%, and some legitimately-measured short
runs cross it. This is worth re-measuring, with the multi-rep methodology
above, if more sheets are added or if startup time becomes a complaint. The
escape hatch needs no further engine work: precompute the crop boxes offline
and call `LoadSpriteAnimations` directly with the resulting `AnimationSpec`
map, skipping `LoadSpriteAutoCropped`'s scan entirely.

## Sprite showcase per-frame redraw cost

`scene.SpriteShowcaseScene.Draw` rebuilds the whole cell list every frame
through `cellsToDraw` and `showcaseLayout`, which walks every sprite in the
library and measures each one's frames in `showcaseFitScale`. Every resulting
cell is then drawn, with no culling against the camera viewport, so on a large
library most of that work lands off screen. The cell list depends only on the
library, which does not change after load, so caching it and invalidating on
registration would remove the per-frame walk. This is acceptable for a debug
scene reached by an explicit flag, and it is deliberately not optimized, but a
library of thousands of sprites would need viewport culling on top of the
caching.
