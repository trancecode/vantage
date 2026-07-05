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

## Path-following search costs (motion/motion_towards.go)

`MoveEntityTowardsArea` searches concentric square rings around the area center
and calls `FindPathBetween` (a full A* run) for every candidate tile in every
ring, so a single bounded move step can trigger dozens of full pathfinding
searches. A cheaper reachability probe, memoization of results across
candidates, or a single multi-goal search from the entity would cut this
substantially. Separately, `MoveEntityTowards`'s fallback (taken whenever no
waypoint on the direct path is reachable) scans an O(maxTileDistance^2) grid of
tiles around the entity, calling `CanReach` per cell. Both are ported verbatim
from the game sources (nrg/lockstep) and are only worth optimizing if profiling
shows them hot.
