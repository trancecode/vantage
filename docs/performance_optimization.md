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
steady-state pop+insert at ~156 ns (100 queued) rising to ~280 ns (100k queued),
each with 2 allocs/op. A generics-native heap — hand-written sift-up/sift-down
over the `[]T` backing slice instead of `container/heap` — would remove both
allocations and the per-operation interface dispatch, at the cost of ~30 extra
lines. Left as-is because the scheduler is not alloc-bound at realistic event
rates (100k events/sec is ~3 MB/s of tiny, short-lived garbage); revisit if the
event queue shows up in allocation profiles under load.

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
304x304 map. Ruling it out cheaply needs connectivity components (a per-tile
region id, recomputed or repaired when terrain changes), which is a design
decision about terrain-change notification rather than a local optimization.
Left undone until a workload actually routes toward walled-off goals; see
[pathfinding_performance.md](pathfinding_performance.md).

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
