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

The benchmark measures four things: cost against journey length, cost against
map size, cost when the destination is occupied, and cost when the destination
is sealed off by terrain. Each case also reports **node expansions** — how many
nodes the search popped from the open set — which is what separates a search
that walked a corridor to the goal from one that flooded the map.

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

## What is left

A goal walled off at a distance — not by its immediate neighbours, but by a
barrier somewhere between it and the searcher — still costs a full flood. Ruling
that out needs connectivity components maintained across terrain changes, which
is a design decision rather than a local fix. See
[performance_optimization.md](performance_optimization.md) for that and for the
remaining per-search costs (the per-node allocations and the two-map-free but
still map-based node store), none of which is currently the binding constraint.
