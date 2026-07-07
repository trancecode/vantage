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
