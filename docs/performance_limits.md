# Performance limits

This records what vantage's hot paths cost against the budgets a game design
discussion starts from: how many perception checks a frame holds, how many
moving bodies, how much scheduled work per beat, how many sprites. Each aspect
gives its measured cost curve and the largest measured parameter that still
fits a budget, so a design question can be answered from a table rather than
from a one-off measurement. The figures are one machine's, and any aspect can
be re-measured on demand with one command; absolute figures describe that
machine, and the ratios between rows are what carries over to another.

## Machine and how to re-run

The figures below come from one run of the suite:

* CPU: AMD EPYC 9554P 64-Core Processor, 4 CPUs.
* Go: go1.27.0 linux/amd64.
* Commit: 41d260e.

They were produced by these commands, run one after another:

```bash
export GOMODCACHE=/tmp/go-mod-cache
task bench PKG=./tilemap/ BENCH='SpatialGrid|TileOccupancy'
task bench PKG=./motion/
task bench PKG=./sim/
task bench PKG=./render/ BENCH=DrawListOrdering
task bench PKG=./render/drawbench/
```

`task bench` runs `go test -run '^$' -bench` under a virtual display. `PKG`
selects the packages (default `./...`), `BENCH` the benchmark pattern (default
`.`) and `BENCHTIME` the `-benchtime` (default `1s`). For example:

```bash
task bench PKG=./sim/ BENCH=DriverRunUntil   # event dispatch alone
task bench BENCHTIME=1x                      # the whole suite as a smoke run
```

## Budgets

* **Frame: 16.7 ms.** One update at Ebitengine's default 60 updates per second.
  Range queries, index maintenance, reservation churn, the movement tick and
  drawing are judged against it.
* **Beat: 1 s.** A simulation beat. Event dispatch is judged against it.
  Movement decisions are judged against both, since a game makes them per
  frame or per beat.

A frame is shared with everything else a game runs, so every aspect reports
two crossing points: the largest measured parameter whose cost fits **1 ms**,
a per-system share of a frame, and the largest that fits the **whole budget**.

**Time acceleration.** A game that runs its clock at 10x runs ten fixed-size
ticks per wall-clock frame, not larger ticks. Per-tick and per-beat work
therefore fits 10x only within a tenth of its budget, so range queries, index
maintenance, reservation churn, the movement tick, movement decisions and event
dispatch report a third crossing point, **at 10x**: 1.67 ms per tick, 100 ms
per beat. Draw ordering and draw throughput do not, because a game draws once
per frame at any clock rate.

Crossing points are read off measured rows, never extrapolated: a row fits a
budget when its measured time per op is at most the budget. Each table also
gives a cost per unit so a reader can estimate between rows. Where a game runs
many ops per frame (range queries, index maintenance, reservation churn,
decisions), tables also give the ops that fit each budget, derived from the
measured per-op cost: the budget divided by ns/op, rounded down.

## Summary

| Aspect | Dimension | Fits 1 ms | Fits budget | Fits at 10x | Details |
| --- | --- | --- | --- | --- | --- |
| Range queries, cell size 1 | Query half-width 4 to 64 tiles, density 0.1, one query | all measured | all measured | all measured | At radius 64: 1 query per ms, 20 per frame, 2 per tick at 10x. [Range queries](#range-queries) |
| Range queries, cell size 4 (best at density 0.1) | Query half-width 4 to 64 tiles, density 0.1, one query | all measured | all measured | all measured | At radius 64: 3 queries per ms, 65 per frame, 6 per tick at 10x. [Range queries](#range-queries) |
| Index maintenance | Population 1,000 to 100,000, one move | all measured | all measured | all measured | At 100,000: 2,595 moves per ms, 43,342 per frame, 4,334 per tick at 10x. [Index maintenance](#index-maintenance) |
| Reservation churn | Occupied tiles 1,000 to 100,000, one reservation move | all measured | all measured | all measured | At 100,000: 8,688 per ms, 145,091 per frame, 14,509 per tick at 10x. [Reservation churn](#reservation-churn) |
| Movement tick (linear) | Moving entities 1,000 to 100,000, one tick | 10,000 | all measured | 10,000 | 7.219 ms at 100,000. Eased moves: 10,000 is the largest that fits 1 ms, the frame and 10x. [Movement tick](#movement-tick) |
| Movement decisions | Journey 8 to 128 tiles, one decision | all measured | all measured (frame and beat) | all measured (tick and beat) | At 128 tiles: 4 per ms, 73 per frame, 7 per tick at 10x, 4,385 per beat, 438 per beat at 10x. [Movement decisions](#movement-decisions) |
| Event dispatch | Events per beat 1,000 to 100,000 | 1,000 | all measured | all measured | 30.244 ms at 100,000, 302.4 ns per event. [Event dispatch](#event-dispatch) |
| Draw ordering | Drawables 1,000 to 100,000 | 1,000 | 10,000 | n/a | 101.930 ms at 100,000. [Draw ordering](#draw-ordering) |
| Sprites per frame | Sprites 100 to 50,000 | none measured | 10,000 | n/a | Software rasterizer. [Draw throughput](#draw-throughput) |
| Labels per frame | Labels 10 to 1,000 | none measured | all measured | n/a | Software rasterizer. [Draw throughput](#draw-throughput) |
| Pathfinding, nil heuristic | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 2,000 (all measured), Reaches 250 cardinal and oblique. [Pathfinding](#pathfinding) |
| Pathfinding, `ScaledOctile` | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 250, Reaches 250 cardinal, none measured oblique. [Pathfinding](#pathfinding) |
| Pathfinding, `CoarseCost` | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 2,000 (all measured), Reaches 1,000 cardinal and oblique. [Pathfinding](#pathfinding) |

The pathfinding rows are quoted from
[pathfinding_performance.md](pathfinding_performance.md#heuristic-strategies)
and read against the 100,000-expansion search budget rather than a time budget,
since that record counts expansions per strategy.

## Range queries

**Question:** how many perception checks per frame at radius R, and what does
the grid's cell size buy?

`BenchmarkSpatialGridGetRange` times one `tilemap.SpatialGrid.GetRange` over a
square of half-width R centered in a world 256 tiles wide, populated at the
given density at hashed positions and indexed at the given cell size.
Entities found is how many entities the query returned. The cost per unit is
ns per tile of query area, ns/op divided by (2R)². Re-run it alone with:

```bash
task bench PKG=./tilemap/ BENCH=GetRange
```

Cell size 1:

| Density | Radius | Entities found | ns/op | Allocs/op | ns per tile of area | Queries per 1 ms | Queries per frame | Queries per tick at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0.01 | 4 | 3 | 1,773 | 3 | 27.7 | 564 | 9,419 | 941 |
| 0.01 | 8 | 6 | 5,881 | 4 | 23.0 | 170 | 2,839 | 283 |
| 0.01 | 16 | 17 | 22,469 | 6 | 21.9 | 44 | 743 | 74 |
| 0.01 | 32 | 60 | 83,006 | 7 | 20.3 | 12 | 201 | 20 |
| 0.01 | 64 | 178 | 337,256 | 9 | 20.6 | 2 | 49 | 4 |
| 0.1 | 4 | 12 | 3,440 | 5 | 53.8 | 290 | 4,854 | 485 |
| 0.1 | 8 | 38 | 12,500 | 7 | 48.8 | 80 | 1,336 | 133 |
| 0.1 | 16 | 117 | 41,921 | 8 | 40.9 | 23 | 398 | 39 |
| 0.1 | 32 | 441 | 182,200 | 10 | 44.5 | 5 | 91 | 9 |
| 0.1 | 64 | 1,695 | 812,268 | 13 | 49.6 | 1 | 20 | 2 |
| 0.5 | 4 | 50 | 6,094 | 7 | 95.2 | 164 | 2,740 | 274 |
| 0.5 | 8 | 181 | 27,834 | 9 | 108.7 | 35 | 599 | 59 |
| 0.5 | 16 | 564 | 103,032 | 11 | 100.6 | 9 | 162 | 16 |
| 0.5 | 32 | 2,158 | 498,914 | 14 | 121.8 | 2 | 33 | 3 |
| 0.5 | 64 | 8,385 | 2,225,697 | 18 | 135.8 | 0 | 7 | 0 |

Cell size 4:

| Density | Radius | Entities found | ns/op | Allocs/op | ns per tile of area | Queries per 1 ms | Queries per frame | Queries per tick at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0.01 | 4 | 4 | 504.5 | 3 | 7.9 | 1,982 | 33,102 | 3,310 |
| 0.01 | 8 | 7 | 1,088 | 4 | 4.2 | 919 | 15,349 | 1,534 |
| 0.01 | 16 | 20 | 3,429 | 6 | 3.3 | 291 | 4,870 | 487 |
| 0.01 | 32 | 62 | 10,409 | 7 | 2.5 | 96 | 1,604 | 160 |
| 0.01 | 64 | 180 | 42,303 | 9 | 2.6 | 23 | 394 | 39 |
| 0.1 | 4 | 19 | 1,566 | 6 | 24.5 | 638 | 10,664 | 1,066 |
| 0.1 | 8 | 49 | 4,529 | 7 | 17.7 | 220 | 3,687 | 368 |
| 0.1 | 16 | 143 | 17,854 | 9 | 17.4 | 56 | 935 | 93 |
| 0.1 | 32 | 475 | 67,113 | 10 | 16.4 | 14 | 248 | 24 |
| 0.1 | 64 | 1,774 | 253,547 | 13 | 15.5 | 3 | 65 | 6 |
| 0.5 | 4 | 90 | 7,827 | 8 | 122.3 | 127 | 2,133 | 213 |
| 0.5 | 8 | 231 | 22,672 | 9 | 88.6 | 44 | 736 | 73 |
| 0.5 | 16 | 677 | 80,739 | 11 | 78.8 | 12 | 206 | 20 |
| 0.5 | 32 | 2,342 | 341,872 | 14 | 83.5 | 2 | 48 | 4 |
| 0.5 | 64 | 8,770 | 1,191,487 | 18 | 72.7 | 0 | 14 | 1 |

Cell size 16:

| Density | Radius | Entities found | ns/op | Allocs/op | ns per tile of area | Queries per 1 ms | Queries per frame | Queries per tick at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0.01 | 4 | 15 | 816.2 | 5 | 12.8 | 1,225 | 20,460 | 2,046 |
| 0.01 | 8 | 15 | 716.7 | 5 | 2.8 | 1,395 | 23,301 | 2,330 |
| 0.01 | 16 | 40 | 2,745 | 7 | 2.7 | 364 | 6,083 | 608 |
| 0.01 | 32 | 79 | 6,386 | 8 | 1.6 | 156 | 2,615 | 261 |
| 0.01 | 64 | 212 | 22,109 | 9 | 1.3 | 45 | 755 | 75 |
| 0.1 | 4 | 111 | 8,445 | 8 | 132.0 | 118 | 1,977 | 197 |
| 0.1 | 8 | 111 | 10,339 | 8 | 40.4 | 96 | 1,615 | 161 |
| 0.1 | 16 | 239 | 21,466 | 9 | 21.0 | 46 | 777 | 77 |
| 0.1 | 32 | 665 | 66,920 | 11 | 16.3 | 14 | 249 | 24 |
| 0.1 | 64 | 2,091 | 265,588 | 14 | 16.2 | 3 | 62 | 6 |
| 0.5 | 4 | 536 | 57,294 | 11 | 895.2 | 17 | 291 | 29 |
| 0.5 | 8 | 536 | 59,935 | 11 | 234.1 | 16 | 278 | 27 |
| 0.5 | 16 | 1,201 | 145,816 | 12 | 142.4 | 6 | 114 | 11 |
| 0.5 | 32 | 3,239 | 483,620 | 15 | 118.1 | 2 | 34 | 3 |
| 0.5 | 64 | 10,366 | 1,850,213 | 19 | 112.9 | 0 | 9 | 0 |

One query fits 1 ms, the frame and a tick at 10x at every measured row except
density 0.5 at radius 64. There, at cell size 1 (2.226 ms), the query fits only
the frame, so radius 32 is the largest that fits 1 ms and a tick at 10x; at
cell size 4 (1.191 ms) it fits the frame and a tick at 10x but not 1 ms; at
cell size 16 (1.850 ms) it fits only the frame. A game runs many queries per
frame, so the last three columns are the useful limit: how many queries of that
row fit each budget.

At cell size 1, each doubling of the radius multiplies the cost by 3.3 to 4.8
(derived from successive rows), close to the fourfold growth of the query area.

What a larger cell buys: `GetRange` returns every entity in every cell the
query rectangle touches, so a larger cell visits fewer cells but also returns
entities from outside the rectangle, which a caller wanting the exact area
filters out. At density 0.1 and radius 4, cell sizes 1, 4 and 16 return 12, 19
and 111 entities. At density 0.1, cell size 4 costs 2.2 to 3.2 times less than
cell size 1 at every radius, and is the cheapest of the three at every radius
but 32, where cell size 16 is 0.3% cheaper (derived). Cell size 16 pays off on
sparse ground with a large radius: at density 0.01 it costs 8.2 to 15.3 times
less than cell size 1 from radius 8 up. On dense ground with a small radius it
costs the most: at density 0.5 and radius 4, 57,294 ns against 6,094 ns at cell
size 1.

## Index maintenance

**Question:** what does moving N bodies cost the spatial index?

`BenchmarkSpatialGridUpdateEntityPosition` times one
`tilemap.SpatialGrid.UpdateEntityPosition`: one entity moves one tile across a
cell boundary, in a grid of cell size 1 populated at density 0.1. Re-run it
alone with:

```bash
task bench PKG=./tilemap/ BENCH=UpdateEntityPosition
```

| Population | ns per move | B/op | Allocs/op | Moves per 1 ms | Moves per frame | Moves per tick at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 144.0 | 0 | 0 | 6,944 | 115,972 | 11,597 |
| 10,000 | 161.9 | 0 | 0 | 6,176 | 103,150 | 10,315 |
| 100,000 | 385.3 | 8 | 0 | 2,595 | 43,342 | 4,334 |

One move fits 1 ms, the frame and a tick at 10x at every measured population.
The per-move cost grows 1.12 times from 1,000 to 10,000 and 2.38 times from
10,000 to 100,000 (derived).

## Reservation churn

**Question:** how much occupancy churn fits a frame?

`BenchmarkTileOccupancyChurn` times one reservation moving to the next tile on
a `tilemap.TileOccupancyManager`: `ClearOccupant` on its tile, `SetOccupant` on
the next one. Re-run it alone with:

```bash
task bench PKG=./tilemap/ BENCH=TileOccupancy
```

| Occupied tiles | ns per reservation move | B/op | Allocs/op | Moves per 1 ms | Moves per frame | Moves per tick at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 69.40 | 0 | 0 | 14,409 | 240,634 | 24,063 |
| 10,000 | 86.53 | 0 | 0 | 11,556 | 192,996 | 19,299 |
| 100,000 | 115.1 | 0 | 0 | 8,688 | 145,091 | 14,509 |

One reservation move fits 1 ms, the frame and a tick at 10x at every measured
size. The per-move cost grows 1.25 times from 1,000 to 10,000 and 1.33 times
from 10,000 to 100,000 (derived), with no allocation.

## Movement tick

**Question:** how many moving bodies per frame?

`BenchmarkSystemTick` times one `motion.System.Tick` advancing every moving
entity by one frame (1/60 s), with the spatial grid kept in sync. Entities are
spread at density 0.1 and head for destinations far enough away that none
arrives during the benchmark, with constant-speed (linear) and eased moves.
Re-run it alone with:

```bash
task bench PKG=./motion/ BENCH=SystemTick
```

| Ease | Entities | ns/op | ms/op | ns per entity | B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| linear | 1,000 | 48,074 | 0.048 | 48.1 | 508 | 3 |
| linear | 10,000 | 611,700 | 0.612 | 61.2 | 9,628 | 74 |
| linear | 100,000 | 7,219,110 | 7.219 | 72.2 | 317,797 | 1,995 |
| eased | 1,000 | 59,046 | 0.059 | 59.0 | 61 | 0 |
| eased | 10,000 | 880,973 | 0.881 | 88.1 | 1,820 | 12 |
| eased | 100,000 | 22,095,325 | 22.095 | 221.0 | 464,373 | 3,391 |

Linear moves: 10,000 entities is the largest measured count that fits 1 ms
(0.612 ms) and a tick at 10x; all measured counts fit the frame, 100,000 at
7.219 ms. Eased moves: 10,000 entities is the largest measured count that fits
1 ms (0.881 ms), the frame and a tick at 10x; 100,000 at 22.095 ms does not fit
a frame.

The cost per entity rises with the population, from 48.1 to 72.2 ns for linear
moves and from 59.0 to 221.0 ns for eased moves (derived). Allocations per tick
grow with the population too; see "SpatialGrid cell sets on the movement tick"
in [performance_optimization.md](performance_optimization.md#spatialgrid-cell-sets-on-the-movement-tick-tilemaptilemap_gridgo).

## Movement decisions

**Question:** how many movement decisions per frame, or per beat?

`BenchmarkMoveEntityTowards` times one `motion.System.MoveEntityTowards`
decision from a fixed tile: plan a path to a destination the given number of
tiles east and issue the next step, on open finite terrain with occupancy and
no heuristic configured. `BenchmarkMoveEntityTowardsArea` times one
`MoveEntityTowardsArea` decision toward an area of the given radius whose center
lies 32 tiles east. After each decision the timed loop releases the step tile
the decision reserved and reserves the start tile again, so that restore is
included in each op. Re-run both alone with:

```bash
task bench PKG=./motion/ BENCH=MoveEntityTowards
```

| Journey (tiles) | ns per decision | B/op | Allocs/op | ns per tile | Per 1 ms | Per frame | Per tick at 10x | Per beat | Per beat at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 8 | 15,468 | 6,440 | 51 | 1,933.5 | 64 | 1,079 | 107 | 64,649 | 6,464 |
| 32 | 44,891 | 18,600 | 129 | 1,402.8 | 22 | 372 | 37 | 22,276 | 2,227 |
| 128 | 228,009 | 69,096 | 424 | 1,781.3 | 4 | 73 | 7 | 4,385 | 438 |

| Area radius (tiles) | ns per decision | B/op | Allocs/op | Per 1 ms | Per frame | Per tick at 10x | Per beat | Per beat at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 55,664 | 18,616 | 130 | 17 | 300 | 30 | 17,964 | 1,796 |
| 4 | 50,562 | 18,616 | 130 | 19 | 330 | 33 | 19,777 | 1,977 |
| 8 | 42,393 | 18,616 | 130 | 23 | 393 | 39 | 23,588 | 2,358 |

Every measured decision fits 1 ms on its own, and so fits every budget: the
largest measured journey, 128 tiles at 0.228 ms, and every measured area radius
fit 1 ms, the frame, a tick at 10x, the beat and a beat at 10x. The columns on
the right give how many decisions of each row fit. Quadrupling the journey
multiplies the cost by 2.90 from 8 to 32 tiles and by 5.08 from 32 to 128 tiles
(derived). Longer journeys and other heuristics are measured by pathfinding;
see [Pathfinding](#pathfinding).

## Event dispatch

**Question:** how many scheduled actions per beat?

`BenchmarkDriverRunUntil` times one `sim.Driver.RunUntil` advancing the clock
by one beat, dispatching every event due in it through a handler that
schedules each event again one beat later, with one registered no-op tick
system. Events sit at distinct times spread across the beat, so each is its
own stop where tick systems run. Re-run it alone with:

```bash
task bench PKG=./sim/ BENCH=DriverRunUntil
```

| Events per beat | ns/op | ms/op | ns per event | B/op | Allocs/op | Allocs per event |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 249,664 | 0.250 | 249.7 | 48,001 | 2,000 | 2.00 |
| 10,000 | 2,817,695 | 2.818 | 281.8 | 480,002 | 20,000 | 2.00 |
| 100,000 | 30,244,489 | 30.244 | 302.4 | 4,800,022 | 200,000 | 2.00 |

1,000 events is the largest measured count that fits 1 ms (0.250 ms; 10,000
take 2.818 ms). All measured counts fit the beat and a beat at 10x: 100,000
events take 30.244 ms against the 100 ms at 10x.

The event queue's own benchmarks, from the same run:

| Benchmark | ns/op | B/op | Allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkEventQueueAdd` | 105.8 | | |
| `BenchmarkEventQueueSteadyState`, 100 queued | 167.4 | 48 | 2 |
| `BenchmarkEventQueueSteadyState`, 1,000 queued | 260.9 | 48 | 2 |
| `BenchmarkEventQueueSteadyState`, 10,000 queued | 326.4 | 48 | 2 |
| `BenchmarkEventQueueSteadyState`, 100,000 queued | 341.1 | 48 | 2 |

A steady-state op is one pop of the earliest event and one insert of a future
one, which is what dispatch does per event: the driver's 2 allocations per
event match the queue's 2 per op. Those allocations are `container/heap`'s
interface boxing; see
[Alloc-free event queue heap](performance_optimization.md#alloc-free-event-queue-heap-simsim_eventqueuego).

## Draw ordering

**Question:** what does ordering N drawables cost per frame? This is the
display-free part of drawing.

`BenchmarkDrawListOrdering` times one frame of `render.DrawList`: `Clear`, one
`Add` per drawable with a hashed layer and Y value, and one `Each` over the
payloads in painter's order. Re-run it alone with:

```bash
task bench PKG=./render/ BENCH=DrawListOrdering
```

| Drawables | ns/op | ms/op | ns per drawable | B/op | Allocs/op |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 336,255 | 0.336 | 336.3 | 121 | 3 |
| 10,000 | 6,412,156 | 6.412 | 641.2 | 6,262 | 3 |
| 100,000 | 101,930,232 | 101.930 | 1,019.3 | 1,407,888 | 5 |

1,000 drawables is the largest measured count that fits 1 ms (0.336 ms), and
10,000 the largest that fits the frame (6.412 ms); 100,000 take 101.930 ms,
about six frames.

The cost per drawable triples from 1,000 to 100,000 drawables (derived), a
curve steeper than a plain O(n log n) sort; see "DrawList ordering sort" in
[performance_optimization.md](performance_optimization.md#drawlist-ordering-sort-renderrender_drawlistgo).
A `DrawList` keeps its capacity across `Clear`, so its bytes per op are the
list's first growth spread over the iterations the run made, which at 100,000
drawables was only 10; they are not a per-frame cost.

## Draw throughput

**Question:** how many sprites and labels per frame?

> **Caveat: software rasterizer.** `task bench` runs this benchmark under a
> virtual display, where the GPU work runs on Mesa's software rasterizer. The
> absolute frame times describe that machine, not a game's hardware. What
> transfers is the scaling across counts and the CPU-side cost; a game
> re-measures on its own machine with the same benchmark.

`BenchmarkDrawThroughput` runs one Ebitengine game loop with vsync off. For each
count it draws a synthetic 16-pixel sprite, or a short text label, that many
times at hashed positions on a 1280x720 canvas, for 10 warm-up frames and then
60 timed frames, and reports the mean wall time per frame: the draw calls, the
GPU flush and the loop's own work. The cost per unit is the µs each added
drawable costs over the previous row. Re-run it alone with:

```bash
task bench PKG=./render/drawbench/
```

| Drawables | ms per frame | µs per added drawable (derived) |
| --- | ---: | ---: |
| Sprites, 100 | 6.832 | |
| Sprites, 1,000 | 7.033 | within run-to-run noise |
| Sprites, 10,000 | 9.593 | 0.284 |
| Sprites, 50,000 | 26.42 | 0.421 |
| Labels, 10 | 6.953 | |
| Labels, 100 | 7.098 | 1.611 |
| Labels, 1,000 | 9.319 | 2.468 |

No measured row fits 1 ms: even 100 sprites (6.832 ms) and 10 labels (6.953 ms)
exceed it, because the frame time includes a fixed per-frame cost. 10,000
sprites is the largest measured count that fits the frame (9.593 ms); 50,000
take 26.42 ms. All measured label counts fit the frame, 1,000 at 9.319 ms.

At 100 and 1,000 sprites the fixed per-frame cost dominates, and across
repeated runs the two frame times can swap order within about 15%, so do not
read 100 against 1,000 as a ranking. From 1,000 sprites up, and at every label
count, the frame time scales consistently.

## Pathfinding

Pathfinding is not re-measured here. Its per-strategy record is the table in
[pathfinding_performance.md](pathfinding_performance.md#heuristic-strategies),
which gives, for grass, offset road, grid and Reaches journeys of 250 to 2,000
tiles, how many node expansions each strategy needs and whether that fits the
100,000-expansion search budget. The headline envelope, quoted from that
table, is the longest measured journey that fits the budget:

* **nil** (plain octile distance): 2,000 tiles on grass, every length
  measured; 250 tiles on Reaches, cardinal and oblique.
* **`ScaledOctile`**: 250 tiles on grass; 250 tiles on Reaches cardinal, and
  no measured length on Reaches oblique, where 250 tiles already needs 100,853
  expansions.
* **`CoarseCost`**: 2,000 tiles on grass, every length measured; 1,000 tiles on
  Reaches, cardinal and oblique.

That record times `CoarseCost` too: at those longest journeys a warm call takes
7.9 ms on grass and 8.3 ms and 7.0 ms on Reaches cardinal and oblique, while a
cold call over new ground takes 1,092 ms, 1,165 ms and 1,121 ms.

## Not benchmarked

* **ui widgets.** A screen holds a handful; no dimension scales.
* **scene.Manager.** It dispatches to scenes and costs what the scenes cost.
* **geometry, easing and config.** No design question depends on their cost.
* **Auto-crop.** A load-time cost, already benchmarked by
  `BenchmarkAutoCropAtlas` and recorded in
  [performance_optimization.md](performance_optimization.md#auto-crop-startup-scan-cost-renderrender_spriteautocropgo),
  outside the frame and beat budgets.
