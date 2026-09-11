# Pathfinding performance

This documents what `pathfinding.FindPath` costs, why long-distance routing was
unusable at scale, and what was done about it. The numbers come from
`pathfinding/astar_bench_test.go`, which is committed so the conclusions can be
re-checked. Run it with:

```bash
export GOMODCACHE=/tmp/go-mod-cache
go test ./pathfinding/ -run '^$' -bench BenchmarkFindPath
```

Figures below are from an AMD EPYC container, 4 cores. Absolute values will
differ elsewhere; the ratios are the point.

## What was measured

The starting symptom: 500 agents each routing toward one shared destination
across a 304x304 open map blew a 40-minute budget for 30 simulated beats, while
20,000 agents doing purely local movement on the same map finished in 12.5
seconds. Short two-tile paths cost about 2 µs regardless of map size, so the
cost had to be specific to long paths, to the convergence on one destination, or
to both.

The benchmark measures five things: cost against journey length, cost against
map size, cost when the destination is occupied, cost when the destination is
sealed off by terrain, and cost when the search runs out of budget on a map
with no edge. Each case also reports **node expansions** — how many nodes the
search popped from the open set — which is what separates a search that walked
a corridor to the goal from one that flooded the map. A count equal to the
budget passed to `FindPath` (100,000 in the benchmarks) with no path returned
means the budget stopped the search, not an emptied open set.

## Conclusion: it is not path length, it is unreachable goals

**Journey length is not the problem.** On open terrain the search expands
exactly one node per tile of the resulting path: 257 expansions for a 256-tile
journey, 33 for a 32-tile one. That is the ideal. It happens because octile
distance is an *exact* heuristic on an obstacle-free uniform grid, so A* never
strays off the optimal corridor. The heuristic tie-breaking usually prescribed
for uniform grids would buy nothing here — there is no explored region to
narrow. Cost is linear in distance at roughly 1.2 µs per tile, and flat in map
size, exactly as the two-tile measurements suggested.

**An unreachable goal is the problem.** A search for a goal it cannot enter has
no way to conclude that except by expanding every reachable tile on the map. On
the 304x304 map that is 92,415 expansions, ~130 ms, and ~20 MB of allocation per
call — **370 times the cost of the longest journey that does succeed there**.

Those two facts together explain the original blowout. When 500 agents converge
on one destination, the first one to arrive occupies it, and from then on every
other agent's request is a request for an unreachable goal. 499 agents x 30
beats x 137 ms is about 34 minutes, which is the budget that was blown. The
local-movement workload never triggered it because those agents always had a
free destination.

The convergence workload also runs through `motion.MoveEntityTowardsArea`, which
calls `FindPathBetween` once per candidate tile in a widening ring. Every
candidate already taken by another agent was costing a full map flood, so the
amplification was worse than one flood per agent per beat.

## What was fixed

`FindPath` now rejects a goal it can prove unenterable before searching, next to
the existing out-of-bounds and unwalkable checks:

* the goal tile itself is occupied, or
* every one of the goal's eight neighbours is out of bounds, unwalkable or
  occupied, so there is no tile to step to it from.

The second check deliberately ignores the diagonal corner-cutting rule and
exempts the search's own start tile (occupancy normally includes the moving
entity's own reservation, and the search never applies the occupancy check to
where it starts). It is therefore conservative: a goal it accepts may still turn
out to be sealed off, in which case the search runs as before.

The search loop also lost its separate `closedSet` map — the closed flag lives
on the node in `nodeMap`, so the neighbour loop does one hash lookup instead of
two.

| Case | Before | After |
| --- | --- | --- |
| Occupied destination, 304x304 | 137 ms, 92,415 expansions, 93,488 allocs | 9 ns, 0 expansions, 0 allocs |
| Destination sealed by terrain | 130 ms, 92,407 expansions, 93,480 allocs | 54 ns, 0 expansions, 0 allocs |
| 256-tile journey, open terrain | 373 µs, 257 expansions, 823 allocs | 314 µs, 257 expansions, 810 allocs |
| 128-tile journey, open terrain | 187 µs, 129 expansions, 433 allocs | 156 µs, 129 expansions, 422 allocs |

Successful searches are about 16% faster from the map merge. The convergence
case, which is what made long-distance routing unusable, is no longer a search
at all.

## Search budget: maps with no edge

A procedurally generated world with no edge (`IsInBounds` true everywhere) has
no natural end to a flood: a goal sealed inside a pocket of impassable terrain
leaves the open set growing forever, and the caller hangs. `FindPath` therefore
takes an expansion budget, `maxExpansions`, and returns no path once the search
has expanded that many nodes. The budget must be positive; a zero budget would
silently mean an unbounded search, which is the hang the parameter exists to
rule out, so `FindPath` panics on it instead.

On a finite map the budget never changes a result as long as it is at or above
the map's tile count, because a flood expands each tile at most once: the
304x304 map's worst case is 92,415 expansions. `motion.System` carries the
budget as `MaxPathExpansions`; 100,000 covers every map measured so far.

On an edgeless map the budget is the latency cap on a failed request:

| Case | Cost |
| --- | --- |
| Budget of 100,000 exhausted on an edgeless map | 77 ms, 100,000 expansions, 13.5 MB, 101,992 allocs |

That is about 0.8 µs and 135 bytes per expansion, so a game picks its budget by
how long a failed request may stall it and how far a sealed pocket can be from
the searcher: a budget of N expansions explores roughly the N tiles closest to
the straight line toward the goal. The successful-search figures above are
unchanged by the budget.

## Terrain faster than 1.0: scaling the heuristic

A step costs its distance divided by the terrain speed, and the heuristic is
plain octile distance, which assumes no step costs less than its distance. Road
at speed 2.0 breaks that assumption: near a road the heuristic overestimates,
so a search can finish on the direct route over grass without ever expanding
the tiles that lead onto a quicker road detour. The `ScaledOctile` heuristic
divides octile distance by the fastest speed, which never overestimates and so
returns optimal routes, but weakens the estimate everywhere. A search with no
heuristic configured is unchanged.

`BenchmarkFindPathRoads` in `pathfinding/astar_roads_bench_test.go` measures
both sides of that trade on edgeless maps, so that a wide search is never cut
short by a map edge:

* **grass**: grass everywhere, with `ScaledOctile` still declaring 2.0 because
  roads exist elsewhere. This is the cost of scaling where no road helps.
* **offset road**: grass with one 3-tile-wide road at 2.0 parallel to the
  journey, a tenth of the journey's length off to one side.
* **grid**: roads 200 tiles apart in both directions, with forest at 0.5 in
  16-tile blocks covering about 30% of the ground between them. Journeys start
  in the middle of a cell, 100 tiles from the nearest road, and run either
  along the grid (cardinal) or at a 2:1 slope across it (oblique).

```bash
go test ./pathfinding/ -run '^$' -bench BenchmarkFindPathRoads -benchtime 1x
```

Expansions are counted with no budget in the way, so the count is what a search
needs to reach the goal. The budget check follows the goal check, so a search
fits the 100,000 budget the other benchmarks use exactly when that count is at
most 100,000. The percentage is how far the unscaled route lies above the scaled
one, which is the optimum; for the one unscaled search that needs more than the
budget, it is the route that search reaches without a budget.

| Map | Length | Unscaled expansions | Fits 100k | Above optimal | Scaled expansions | Fits 100k |
| --- | ---: | ---: | :---: | ---: | ---: | :---: |
| grass | 250 | 251 | yes | 0.0% | 50,428 | yes |
| grass | 500 | 501 | yes | 0.0% | 201,707 | no |
| grass | 1,000 | 1,001 | yes | 0.0% | 806,802 | no |
| grass | 2,000 | 2,001 | yes | 0.0% | 3,227,199 | no |
| offset road | 250 | 251 | yes | 48.9% | 13,841 | yes |
| offset road | 500 | 501 | yes | 47.7% | 56,367 | yes |
| offset road | 1,000 | 1,001 | yes | 47.0% | 226,824 | no |
| offset road | 2,000 | 2,001 | yes | 46.7% | 910,657 | no |
| grid, cardinal | 250 | 4,024 | yes | 0.2% | 62,178 | yes |
| grid, cardinal | 500 | 7,360 | yes | 31.7% | 96,233 | yes |
| grid, cardinal | 1,000 | 35,433 | yes | 49.9% | 339,306 | no |
| grid, cardinal | 2,000 | 141,042 | no | 72.9% | 679,882 | no |
| grid, oblique | 250 | 3,012 | yes | 19.8% | 26,406 | yes |
| grid, oblique | 500 | 23,471 | yes | 10.2% | 191,393 | no |
| grid, oblique | 1,000 | 13,024 | yes | 38.5% | 392,916 | no |
| grid, oblique | 2,000 | 71,926 | yes | 33.4% | 1,859,478 | no |

What the table shows:

* **Scaling turns a corridor into a region.** On grass the scaled search expands
  about 0.8 x length² tiles instead of one per tile of the route, so a budget of
  100,000 covers roughly 350 tiles of open ground instead of every journey on
  the map. With roads nearby the optimum is cheaper and the region smaller, but
  of the 16 journeys only the 250-tile ones and two of the 500-tile ones fit.
* **Leaving it unscaled costs a lot wherever a road is worth taking.** A parallel
  road a tenth of the journey away makes the unscaled route 47% to 49% dearer at
  every length, and on the grid 10% to 73% for journeys of 500 tiles and more.
  Where no road is worth the detour, as on the 250-tile cardinal grid journey
  with the nearest road 100 tiles off, the difference is 0.2%.
* **Forest already widens unscaled searches.** Slow terrain makes octile distance
  an underestimate, so the unscaled search on the grid spreads around forest
  blocks: the 2,000-tile cardinal journey needs 141,042 expansions and does not
  fit the budget even without scaling.
* **A search the budget stops is expensive.** Under a budget of 100,000 each
  scaled journey that does not fit costs about 98 to 126 ms per call here, in
  one-shot timings, and returns no path.

## What is left

A goal walled off at a distance — not by its immediate neighbours, but by a
barrier somewhere between it and the searcher — still costs a flood, now capped
at the budget rather than at the map. Ruling it out up front needs connectivity
components maintained across terrain changes, which is a design decision rather
than a local fix. See
[performance_optimization.md](performance_optimization.md) for that and for the
remaining per-search costs (the per-node allocations and the two-map-free but
still map-based node store), none of which is currently the binding constraint.
