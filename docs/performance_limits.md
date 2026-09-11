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

The figures below come from two recorded runs on one machine:

* CPU: AMD EPYC 9554P 64-Core Processor, 4 CPUs.
* Go: go1.27.0 linux/amd64.

Range queries, index maintenance, reservation churn, event dispatch and draw
ordering come from the original run, at commit 41d260e, produced by these
commands run one after another:

```bash
export GOMODCACHE=/tmp/go-mod-cache
task bench PKG=./tilemap/ BENCH='SpatialGrid|TileOccupancy'
task bench PKG=./sim/
task bench PKG=./render/ BENCH=DrawListOrdering
```

The movement tick, movement decisions and draw throughput come from a second
run, at commit c6725c1, after those benchmarks' fixtures were corrected,
produced by these commands run one after another:

```bash
export GOMODCACHE=/tmp/go-mod-cache
task bench PKG=./motion/
task bench PKG=./render/drawbench/
```

**Run-to-run spread.** Repeated runs of one benchmark on this machine differ.
Five consecutive runs of a few representative rows, at commit 088d1ae (the
decision benchmarks are unchanged from there to c6725c1), were produced by:

```bash
export GOMODCACHE=/tmp/go-mod-cache
xvfb-run -a go test -run '^$' -bench 'SpatialGridGetRange/cell=(4|16)/density=0.1/radius=64$' -count 5 ./tilemap/
xvfb-run -a go test -run '^$' -bench 'MoveEntityTowardsArea' -count 5 ./motion/
```

| Benchmark | Fastest ns/op | Slowest ns/op | Spread |
| --- | ---: | ---: | ---: |
| `SpatialGridGetRange`, cell size 4, density 0.1, radius 64 | 248,298 | 335,679 | 35% |
| `SpatialGridGetRange`, cell size 16, density 0.1, radius 64 | 236,922 | 254,794 | 7.5% |
| `MoveEntityTowardsArea`, radius 1 | 42,507 | 51,321 | 21% |
| `MoveEntityTowardsArea`, radius 4 | 52,187 | 57,990 | 11% |
| `MoveEntityTowardsArea`, radius 8 | 51,499 | 63,291 | 23% |

The spread is the slowest run minus the fastest, divided by the fastest. This
document draws no comparison between two figures that differ by less than
35%, the largest spread observed, and says instead that they are within the
run-to-run spread. Crossing points are still read off the recorded row, so a
row within the spread of a budget can land on the other side of it in another
run.

`task bench` runs `go test -run '^$' -bench` under a virtual display. `PKG`
selects the packages (default `./...`), `BENCH` the benchmark pattern (default
`.`) and `BENCHTIME` the `-benchtime` (default `1s`). For example:

```bash
task bench PKG=./sim/ BENCH=DriverRunUntil   # event dispatch alone
task bench BENCHTIME=1x                      # the whole suite as a smoke run
```

## Budgets

* **Frame: 16.7 ms.** The engine's budget: one update at Ebitengine's default
  60 updates per second. Range queries, index maintenance, reservation churn,
  the movement tick and drawing are judged against it.
* **Beat: 1 s.** nrg's simulation beat. nrg is the game this suite was built
  for; the engine itself has no beat. Event dispatch is judged against it.
  Movement decisions are judged against both, since a game makes them per
  frame or per beat.

Per-beat work still runs inside frames, so a per-beat crossing point holds
only when a game spreads that work across the beat's frames; a game that does
all of it at the beat boundary has one frame, and the frame budget bounds it.

A frame is shared with everything else a game runs, so every aspect reports
two crossing points: the largest measured parameter whose cost fits **1 ms**,
a per-system share of a frame, and the largest that fits the **whole budget**.

**Time acceleration.** nrg lets the player run its clock at 1x, 2x, 5x and
10x; the engine has no acceleration of its own. A game that runs its clock at
10x runs ten fixed-size ticks per wall-clock frame, not larger ticks. Per-tick
and per-beat work therefore fits 10x only within a tenth of its budget, so
range queries, index maintenance, reservation churn, the movement tick,
movement decisions and event dispatch report a third crossing point, **at
10x**: 1.67 ms per tick, 100 ms per beat. Draw ordering and draw throughput do
not, because a game draws once per frame at any clock rate.

Another game substitutes its own beat length and clock rates, and re-derives
the crossing points from the per-unit costs the tables give.

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
| Range queries, cell size 4 (best or tied at density 0.1) | Query half-width 4 to 64 tiles, density 0.1, one query | all measured | all measured | all measured | At radius 64: 3 queries per ms, 65 per frame, 6 per tick at 10x. [Range queries](#range-queries) |
| Index maintenance | Population 1,000 to 100,000, one move | all measured | all measured | all measured | At 100,000: 2,595 moves per ms, 43,342 per frame, 4,334 per tick at 10x. [Index maintenance](#index-maintenance) |
| Reservation churn | Occupied tiles 1,000 to 100,000, one reservation move | all measured | all measured | all measured | At 100,000: 8,688 per ms, 145,091 per frame, 14,509 per tick at 10x. [Reservation churn](#reservation-churn) |
| Movement tick (linear) | Moving entities 1,000 to 100,000, one tick | 10,000 | all measured | 10,000 | 16.477 ms at 100,000, within the run-to-run spread of the frame. Eased moves: the same crossing points, 13.734 ms at 100,000, also within the run-to-run spread of the frame. [Movement tick](#movement-tick) |
| Movement decisions | Journey 8 to 128 tiles, one decision | all measured | all measured (frame and beat) | all measured (tick and beat) | At 128 tiles: 5 per ms, 84 per frame, 8 per tick at 10x, 5,052 per beat, 505 per beat at 10x. [Movement decisions](#movement-decisions) |
| Event dispatch | Events per beat 1,000 to 100,000 | 1,000 | all measured | all measured | 30.244 ms at 100,000, 302.4 ns per event. [Event dispatch](#event-dispatch) |
| Draw ordering | Drawables 1,000 to 100,000 | 1,000 | 10,000 | n/a | 101.930 ms at 100,000. [Draw ordering](#draw-ordering) |
| Sprites per frame | Sprites 100 to 50,000 | none measured | 1,000 | n/a | 18.62 ms at 10,000. Software rasterizer. [Draw throughput](#draw-throughput) |
| Labels per frame | Labels 10 to 1,000 | none measured | all measured | n/a | Software rasterizer. [Draw throughput](#draw-throughput) |
| Pathfinding, nil heuristic | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 2,000 (all measured), Reaches 250 cardinal and oblique. [Pathfinding](#pathfinding) |
| Pathfinding, `ScaledOctile` | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 250, Reaches 250 cardinal, none measured oblique. [Pathfinding](#pathfinding) |
| Pathfinding, `CoarseCost` | Journey 250 to 2,000 tiles within 100,000 expansions | n/a | n/a | n/a | Longest journey that fits: grass 2,000 (all measured), Reaches 1,000 cardinal and oblique. [Pathfinding](#pathfinding) |

For range queries, index maintenance, reservation churn and movement decisions,
"all measured" means one op fits the budget at every measured row; a game runs
many ops, and Details and each section's table give how many fit each budget.

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
cell size 1 at every radius, and is the cheapest of the three at radii 4 and
8; from radius 16 up, cell sizes 4 and 16 differ by less than the run-to-run
spread (see [Machine and how to re-run](#machine-and-how-to-re-run)), so
neither ranks ahead. Cell size 16 pays off on
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
The per-move cost grows 2.38 times from 10,000 to 100,000 (derived); from
1,000 to 10,000 it changes by less than the run-to-run spread.

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
size. The per-move cost grows 1.66 times from 1,000 to 100,000 (derived), with
no allocation; each tenfold step on its own changes it by less than the
run-to-run spread.

## Movement tick

**Question:** how many moving bodies per frame?

`BenchmarkSystemTick` times one `motion.System.Tick` advancing every moving
entity by one frame (1/60 s), with the spatial grid kept in sync, for
constant-speed (linear) and eased moves. Entities spawn at tile centres spread
at density 0.1 and shuttle at speed 1 between their spawn tile centre and the
tile centre 4 tiles east. Each arrival turns a body around through
`System.OnArrival`, and that re-issued `MoveEntity` is part of the measured
tick, as a game's arrivals are. Each body arrives once every 4 s, and starts
are staggered so that arrivals spread evenly over the frames: about one body in
240 arrives on each frame. The benchmark ticks 900 frames before timing, so the
timed ticks cross only cells bodies have crossed before. Re-run it alone with:

```bash
task bench PKG=./motion/ BENCH=SystemTick
```

| Ease | Entities | ns/op | ms/op | ns per entity | B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| linear | 1,000 | 56,478 | 0.056 | 56.5 | 728 | 7 |
| linear | 10,000 | 714,449 | 0.714 | 71.4 | 8,595 | 48 |
| linear | 100,000 | 16,477,377 | 16.477 | 164.8 | 75,561 | 423 |
| eased | 1,000 | 44,303 | 0.044 | 44.3 | 737 | 7 |
| eased | 10,000 | 655,399 | 0.655 | 65.5 | 8,475 | 48 |
| eased | 100,000 | 13,734,023 | 13.734 | 137.3 | 75,596 | 423 |

Linear moves: 10,000 entities is the largest measured count that fits 1 ms
(0.714 ms) and a tick at 10x; all measured counts fit the frame, but 100,000
at 16.477 ms fits it by less than the run-to-run spread, so treat 100,000 as
the frame's edge. Eased moves: 10,000 entities is the largest measured count
that fits 1 ms (0.655 ms) and a tick at 10x; all measured counts fit the
frame, but 100,000 at 13.734 ms also fits it by less than the run-to-run
spread, so treat 100,000 as the frame's edge here too. At each count, linear
and eased moves differ by less than the run-to-run spread.

The cost per entity grows 2.92 times from 1,000 to 100,000 entities for linear
moves and 3.10 times for eased moves (derived). Allocations per tick track the
arrivals: 7, 48 and 423 allocs/op against about 4, 42 and 417 bodies arriving
per frame (the population divided by 240, derived); see "Movement component
re-add on arrival" in
[performance_optimization.md](performance_optimization.md#movement-component-re-add-on-arrival-motionmotion_movego).

## Movement decisions

**Question:** how many movement decisions per frame, or per beat?

`BenchmarkMoveEntityTowards` times one `motion.System.MoveEntityTowards`
decision from a fixed tile: plan a path to a destination the given number of
tiles east and issue the next step, on open finite terrain with occupancy and
no heuristic configured. `BenchmarkMoveEntityTowardsArea` times one
`MoveEntityTowardsArea` decision toward an area of the given radius whose center
lies 32 tiles east. Another entity reserves every tile of the area inside its
outermost ring, as a crowd standing at a gather point would, so the decision
scans the reserved inner rings by tile lookup and plans to the nearest free
tile on the outer ring, which lies radius tiles nearer than the center. After
each decision the timed loop releases the step tile the decision reserved and
reserves the start tile again, so that restore is included in each op. Re-run
both alone with:

```bash
task bench PKG=./motion/ BENCH=MoveEntityTowards
```

| Journey (tiles) | ns per decision | B/op | Allocs/op | ns per tile | Per 1 ms | Per frame | Per tick at 10x | Per beat | Per beat at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 8 | 12,599 | 6,440 | 51 | 1,574.9 | 79 | 1,325 | 132 | 79,371 | 7,937 |
| 32 | 49,073 | 18,600 | 129 | 1,533.5 | 20 | 340 | 34 | 20,377 | 2,037 |
| 128 | 197,941 | 69,096 | 424 | 1,546.4 | 5 | 84 | 8 | 5,052 | 505 |

| Area radius (tiles) | ns per decision | B/op | Allocs/op | Per 1 ms | Per frame | Per tick at 10x | Per beat | Per beat at 10x |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 41,133 | 17,480 | 130 | 24 | 406 | 40 | 24,311 | 2,431 |
| 4 | 42,114 | 18,488 | 138 | 23 | 396 | 39 | 23,745 | 2,374 |
| 8 | 68,267 | 27,416 | 156 | 14 | 244 | 24 | 14,648 | 1,464 |

Every measured decision fits 1 ms on its own, and so fits every budget: the
largest measured journey, 128 tiles at 0.198 ms, and every measured area radius
fit 1 ms, the frame, a tick at 10x, the beat and a beat at 10x. The columns on
the right give how many decisions of each row fit. A game that makes all its
decisions at a beat boundary makes them within one frame, so the per-frame
columns bound it, not the per-beat ones.

Quadrupling the journey multiplies the cost by 3.89 from 8 to 32 tiles and by
4.03 from 32 to 128 tiles (derived), in proportion to the journey. Between area
radii, bytes and allocations per decision grow with the radius, from 17,480 B
and 130 allocations at radius 1 to 27,416 B and 156 at radius 8. The times are
less settled: the recorded run puts radius 8 at 68,267 ns against 41,133 ns at
radius 1, but the five repeated runs in
[Machine and how to re-run](#machine-and-how-to-re-run) took 51,499 to 63,291 ns
at radius 8 and 42,507 to 51,321 ns at radius 1, so the times at radii 1, 4 and
8 are within the run-to-run spread of each other. Longer journeys and other
heuristics are measured by pathfinding; see [Pathfinding](#pathfinding).

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

These crossing points assume the events are spread across the beat, as the
benchmark spreads them. Events that fall due at one instant are dispatched
within one frame, so the frame budget bounds them, not the beat budget:
100,000 events at 30.244 ms would not fit a 16.7 ms frame, while 10,000 at
2.818 ms would.

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

The cost per drawable grows 3.03 times from 1,000 to 100,000 drawables, about
9% above the 2.78 times an O(n log² n) sort's would grow ((log 100,000 /
log 1,000)², derived). That gap is within the run-to-run spread, so the
measured growth matches an O(n log² n) sort; see "DrawList ordering sort" in
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
times at hashed positions covering a 1280x720 canvas, for 10 warm-up frames and
then 60 timed frames, and reports the mean wall time per frame: the draw calls,
the GPU flush and the loop's own work. Positions are in tiles, under a camera
with world zero at the canvas's top-left corner, and before drawing the
benchmark checks that every drawable's top-left pixel lies on the canvas. The
cost per unit is the µs each added drawable costs over the previous row.
Re-run it alone with:

```bash
task bench PKG=./render/drawbench/
```

| Drawables | ms per frame | µs per added drawable (derived) |
| --- | ---: | ---: |
| Sprites, 100 | 6.338 | |
| Sprites, 1,000 | 6.882 | within run-to-run noise |
| Sprites, 10,000 | 18.62 | 1.304 |
| Sprites, 50,000 | 73.47 | 1.371 |
| Labels, 10 | 5.674 | |
| Labels, 100 | 6.382 | within run-to-run noise |
| Labels, 1,000 | 10.81 | 4.920 |

No measured row fits 1 ms: even 100 sprites (6.338 ms) and 10 labels (5.674 ms)
exceed it, because the frame time includes a fixed per-frame cost. 1,000
sprites is the largest measured count that fits the frame (6.882 ms); 10,000
take 18.62 ms. All measured label counts fit the frame, 1,000 at 10.81 ms.

`RunGame` runs once per process, so `-count` does not apply to this benchmark.
For repeat evidence, `task bench PKG=./render/drawbench/` ran twice more at
c6725c1, right after the recorded run:

| Drawables | Recorded ms per frame | Repeat 1 | Repeat 2 |
| --- | ---: | ---: | ---: |
| Sprites, 100 | 6.338 | 7.344 | 6.408 |
| Sprites, 1,000 | 6.882 | 10.54 | 8.787 |
| Sprites, 10,000 | 18.62 | 22.47 | 22.65 |
| Sprites, 50,000 | 73.47 | 74.11 | 76.20 |
| Labels, 10 | 5.674 | 7.488 | 7.276 |
| Labels, 100 | 6.382 | 6.553 | 6.097 |
| Labels, 1,000 | 10.81 | 12.65 | 9.955 |

The crossing points hold in all three runs: 1,000 sprites fit the frame and
10,000 do not, and 1,000 labels fit it. At 100 and 1,000 sprites, and at 10
and 100 labels, the fixed per-frame cost dominates: 1,000 sprites took 6.882 to
10.54 ms across the runs, and 100 labels drew faster than 10 labels in two of
them, so do not read the low counts against each other. From 10,000 to 50,000
sprites, and from 100 to 1,000 labels, the frame time grows in every run.

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
