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

## Heuristic strategies

A step's cost is its distance divided by the terrain's speed there, and octile
distance is exact only when it assumes every step costs its distance, that is,
speed 1.0. A road at 2.0 breaks that assumption from below: the heuristic
overestimates near a road, so a search can finish on the direct route over
grass without ever expanding the tiles that lead onto a quicker road detour.
Forest at 0.5 breaks it from above: the heuristic underestimates, so a search
floods outward before it can rule out beating the direct route.

* **nil**, plain octile distance, is exact on open grass, at one node
  expansion per tile of a successful route. It is the cheapest strategy to run
  and the worst at picking routes: it misses roads faster than 1.0 and floods
  over ground slower than 1.0.
* **`ScaledOctile`** divides octile distance by the fastest speed the terrain
  declares, `MaxSpeed`, which never overestimates and so returns optimal
  routes onto roads. The price is a heuristic weakened everywhere: on open
  grass at a declared speed of 2.0 the search expands about 0.8 x length²
  tiles instead of one per tile of the route.
* **`CoarseCost`** learns the terrain a cell at a time: a coarse search over a
  grid of cells, each holding its cheapest cost to cross west to east and
  north to south, blended bilinearly into a per-tile estimate. It keeps
  searches corridor-shaped and routes within a few percent of optimal on both
  fast and slow ground, at the cost of the first search over new ground
  building the cells it touches.

The map families:

* **grass**: grass everywhere.
* **offset road**: grass with one road, 3 tiles wide at speed 2.0, parallel to
  the journey and a tenth of the journey's length off to one side.
* **grid**: roads 200 tiles apart in both directions, with forest at 0.5 in
  16-tile blocks on about 30% of the ground between them; cardinal journeys
  run along the grid, oblique journeys at a 2:1 slope across it.
* **Reaches**: half-speed forest with pools 80 to 300 tiles wide on a
  400-tile lattice; cardinal and oblique journeys as on the grid.
* **shore**: half-speed forest south of a straight shoreline, with open water
  north of it. Journeys end on the shore, ten rows into a cell whose top four
  rows are water, as a journey to a pool's edge in nrg's Blighted Reaches does:
  cardinal journeys from due south, oblique ones at a 2:1 slope.

```bash
export GOMODCACHE=/tmp/go-mod-cache
go test ./pathfinding/ -run '^$' -bench BenchmarkFindPathHeuristics -benchtime 1x -timeout 60m
go test ./pathfinding/ -run '^$' -bench BenchmarkCoarseCost
```

The Reaches and shore journeys take seconds per case, and their uncapped
2,000-tile searches ran a 16 GB machine out of memory when every case shared
one process. Narrow to one map and length per process with a pattern such as
`-bench 'BenchmarkFindPathHeuristics/reaches/cardinal/length=2000$'`.

Expansions are counted with no budget in the way, so the count is what a
search needs to reach the goal; "fits 100k" says whether that count is at most
the 100,000-expansion budget the other benchmarks use. "Above optimal" is how
far a route lies above `ScaledOctile`'s cost with no budget, the optimum; for a
search that does not fit the budget, it is the route that search reaches
without one. `ScaledOctile` itself is always 0.0% above optimal and is left
out of the table. Times are one-shot figures from a single warm call under the
budget (`-benchtime 1x`), in milliseconds; "Coarse cold" is a fresh
`CoarseCost` field's first call, "Coarse warm" a field that has already served
the same journey. On a row whose search does not fit the 100,000 budget, the
time columns time a search the budget stopped, not one that reached the goal.

| Map | Length | Octile expansions | Octile fits 100k | Octile above optimal | Scaled expansions | Scaled fits 100k | Coarse expansions | Coarse fits 100k | Coarse above optimal | Coarse cold | Coarse warm | Cells built | Cells settled | Extra chunks |
| --- | ---: | ---: | :---: | ---: | ---: | :---: | ---: | :---: | ---: | ---: | ---: | ---: | ---: | ---: |
| grass | 250 | 251 | yes | 0.0% | 50,428 | yes | 251 | yes | 0.0% | 93 ms | 0.7 ms | 153 | 103 | 43 |
| grass | 500 | 501 | yes | 0.0% | 201,707 | no | 501 | yes | 0.0% | 180 ms | 1.1 ms | 381 | 295 | 97 |
| grass | 1,000 | 1,001 | yes | 0.0% | 806,802 | no | 1,001 | yes | 0.0% | 517 ms | 6.9 ms | 1,141 | 985 | 290 |
| grass | 2,000 | 2,001 | yes | 0.0% | 3,227,199 | no | 3,432 | yes | 0.0% | 1,160 ms | 7.6 ms | 2,319 | 2,048 | 581 |
| offset road | 250 | 251 | yes | 48.9% | 13,841 | yes | 1,326 | yes | 0.5% | 53 ms | 1.5 ms | 108 | 64 | 29 |
| offset road | 500 | 501 | yes | 47.7% | 56,367 | yes | 2,193 | yes | 0.5% | 97 ms | 2.6 ms | 229 | 159 | 56 |
| offset road | 1,000 | 1,001 | yes | 47.0% | 226,824 | no | 19,932 | yes | 0.3% | 281 ms | 21 ms | 461 | 349 | 107 |
| offset road | 2,000 | 2,001 | yes | 46.7% | 910,657 | no | 53,170 | yes | 0.2% | 879 ms | 83 ms | 1,317 | 1,113 | 310 |
| grid, cardinal | 250 | 4,024 | yes | 0.2% | 62,178 | yes | 1,750 | yes | 1.1% | 73 ms | 2.0 ms | 136 | 91 | 33 |
| grid, cardinal | 500 | 7,360 | yes | 31.7% | 96,233 | yes | 14,811 | yes | 0.3% | 205 ms | 26 ms | 348 | 249 | 83 |
| grid, cardinal | 1,000 | 35,433 | yes | 49.9% | 339,306 | no | 29,974 | yes | 0.8% | 469 ms | 46 ms | 574 | 435 | 135 |
| grid, cardinal | 2,000 | 141,042 | no | 72.9% | 679,882 | no | 81,496 | yes | 0.1% | 808 ms | 176 ms | 1,087 | 849 | 224 |
| grid, oblique | 250 | 3,012 | yes | 19.8% | 26,406 | yes | 11,330 | yes | 0.4% | 163 ms | 21 ms | 183 | 119 | 42 |
| grid, oblique | 500 | 23,471 | yes | 10.2% | 191,393 | no | 21,578 | yes | 0.3% | 303 ms | 48 ms | 441 | 347 | 106 |
| grid, oblique | 1,000 | 13,024 | yes | 38.5% | 392,916 | no | 42,586 | yes | 0.2% | 727 ms | 99 ms | 1,011 | 856 | 239 |
| grid, oblique | 2,000 | 71,926 | yes | 33.4% | 1,859,478 | no | 134,609 | no | 0.3% | 1,446 ms | 207 ms | 2,330 | 2,048 | 477 |
| reaches, cardinal | 250 | 50,202 | yes | 0.0% | 88,573 | yes | 251 | yes | 0.0% | 108 ms | 1.1 ms | 199 | 137 | 52 |
| reaches, cardinal | 500 | 177,131 | no | 0.0% | 312,787 | no | 501 | yes | 0.0% | 403 ms | 1.9 ms | 532 | 412 | 137 |
| reaches, cardinal | 1,000 | 674,637 | no | 0.0% | 1,286,449 | no | 1,001 | yes | 0.0% | 1,250 ms | 7.6 ms | 1,754 | 1,480 | 457 |
| reaches, cardinal | 2,000 | 2,755,106 | no | 0.0% | 5,287,660 | no | 2,038,254 | no | 0.0% | 1,989 ms | 203 ms | 2,578 | 2,048 | 651 |
| reaches, oblique | 250 | 58,499 | yes | 0.0% | 100,853 | no | 225 | yes | 0.0% | 147 ms | 1.5 ms | 223 | 149 | 59 |
| reaches, oblique | 500 | 218,762 | no | 0.0% | 382,702 | no | 877 | yes | 0.0% | 481 ms | 2.6 ms | 619 | 479 | 159 |
| reaches, oblique | 1,000 | 856,155 | no | 0.0% | 1,544,108 | no | 1,324 | yes | 0.0% | 1,658 ms | 12 ms | 2,120 | 1,811 | 540 |
| reaches, oblique | 2,000 | 3,656,883 | no | 0.0% | 6,263,063 | no | 2,681,649 | no | 0.0% | 1,764 ms | 191 ms | 2,578 | 2,048 | 649 |
| shore, cardinal | 250 | 50,428 | yes | 0.0% | 94,687 | yes | 251 | yes | 0.0% | 56 ms | 0.4 ms | 136 | 86 | 36 |
| shore, cardinal | 500 | 201,707 | no | 0.0% | 378,769 | no | 501 | yes | 0.0% | 172 ms | 1.2 ms | 385 | 299 | 101 |
| shore, cardinal | 1,000 | 806,802 | no | 0.0% | 1,515,109 | no | 1,001 | yes | 0.0% | 684 ms | 2.8 ms | 1,254 | 1,092 | 322 |
| shore, cardinal | 2,000 | 3,227,199 | no | 0.0% | 6,060,431 | no | 2,122,822 | no | 0.0% | 1,714 ms | 95 ms | 2,413 | 2,048 | 606 |
| shore, oblique | 250 | 57,355 | yes | 0.0% | 94,035 | yes | 225 | yes | 0.0% | 74 ms | 1.3 ms | 155 | 101 | 44 |
| shore, oblique | 500 | 225,292 | no | 0.0% | 370,388 | no | 448 | yes | 0.0% | 253 ms | 2.6 ms | 403 | 313 | 105 |
| shore, oblique | 1,000 | 893,084 | no | 0.0% | 1,470,303 | no | 895 | yes | 0.0% | 806 ms | 3.3 ms | 1,200 | 1,040 | 305 |
| shore, oblique | 2,000 | 3,561,088 | no | 0.0% | 5,867,230 | no | 2,311,831 | no | 0.0% | 1,612 ms | 107 ms | 2,416 | 2,048 | 608 |

The `CoarseCost` columns come from a run at the commit that added the shore
map, after coarse edges moved to movement rates (d68b2a8); the octile and
scaled columns of the other maps come from the original run, since those
searches did not change. Each case ran in its own process, one map and length
at a time, because the uncapped 2,000-tile searches ran the machine out of
memory in a single process.

`BenchmarkCoarseCostCellBuild` measures the cost of building one cell: about
459 µs on grass ground, 322 µs on the grid, and 461 µs on Reaches ground.
`BenchmarkCoarseCostRetainedPerCell` measures what a built cell keeps: about
82 retained bytes per cell.

What the table shows:

* **`CoarseCost` stays close to optimal wherever octile misses a road.**
  Octile comes out 46.7% to 48.9% above optimal on the offset-road journeys
  and up to 72.9% on the grid; `CoarseCost` stays within 0.1% to 1.1% at every
  length on both maps.
* **On the Reaches map, `CoarseCost` matches the optimal expansion count on
  the cardinal journeys while it fits the budget.** The 250-, 500- and
  1,000-tile cardinal journeys cost `CoarseCost` exactly as many expansions as
  a search on open grass of the same length (251, 501, 1,001); the oblique
  journeys come close but not exact, at 225, 877 and 1,324. Octile needs
  50,202 to 856,155 expansions across those six journeys and already misses
  the 100,000 budget past 250 tiles on both directions. The 500- and
  1,000-tile oblique journeys cost 591 and 1,038 expansions before coarse edges
  moved to movement rates, with the same optimal routes; that fits the
  substitution erring low where a pool blocks a cell one way, though the
  benchmark does not isolate it.
* **On the shore map, movement rates are what keep `CoarseCost` a corridor.**
  Its 250-, 500- and 1,000-tile journeys cost 251, 501 and 1,001 expansions
  cardinal and 225, 448 and 895 oblique. With raw crossing rates, one run of
  the same benchmark over d68b2a8's parent `coarse_cost.go` costs 48,457,
  198,037 and 795,504 expansions cardinal and 48,043, 205,478 and 847,977
  oblique: every cell of the goal's row has an infinite north-south rate, the
  coarse search settles its whole 2,048-cell budget along the shore, and every
  estimate away from that row falls back to octile distance.
* **Six of the 2,000-tile journeys reach `DefaultCoarseCellBudget`.** At
  2,000 tiles, `grass`, `grid, oblique`, `reaches, cardinal`,
  `reaches, oblique`, `shore, cardinal` and `shore, oblique` each settle the
  full 2,048 cells the budget allows, and past that point their estimates fall
  back to octile distance. What that costs differs sharply by map: `grass`
  keeps its route optimal at 3,432 expansions against octile's 2,001, and still
  fits the 100,000 budget; `grid, oblique` needs 134,609 expansions and misses
  the budget; the Reaches and shore journeys fall all the way into an octile
  flood, at 2,038,254 to 2,681,649 expansions, also missing the budget. Cells
  built counts every neighbour cell a search relaxes as well as every one it
  settles, so it runs higher than cells settled, the count the budget actually
  bounds: `reaches, oblique` at 1,000 tiles builds 2,120 cells while settling
  only 1,811, well under budget, so a high built count on its own does not mean
  the budget was reached.
* **Cold calls pay for the ground they read to look ahead.** A fresh
  `CoarseCost` field reads terrain the tile search never enters, from 29 extra
  64-tile chunks on `offset road` at 250 tiles to 651 on `reaches, cardinal`
  at 2,000 tiles; on rows past the cell budget that count is the
  ground read before the search gave up, not the ground a completed coarse
  search would have needed. On a world that generates terrain lazily, every
  extra chunk is ground materialized only to look ahead; nrg measured 1.7 ms
  per 64-tile chunk. See the [design spec](superpowers/specs/2026-09-11-pluggable-pathfinding-heuristic-design.md)
  for the formulation and the variants measured and rejected.

## What is left

A goal walled off at a distance — not by its immediate neighbours, but by a
barrier somewhere between it and the searcher — still costs a flood, now capped
at the budget rather than at the map. Ruling it out up front needs connectivity
components maintained across terrain changes, which is a design decision rather
than a local fix. See
[performance_optimization.md](performance_optimization.md) for that and for the
remaining per-search costs (the per-node allocations and the two-map-free but
still map-based node store), none of which is currently the binding constraint.
