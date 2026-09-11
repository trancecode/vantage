# Benchmark suite implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Benchmarks parameterised by design questions for every engine aspect whose cost bounds a game, a `task bench` target, and `docs/performance_limits.md` stating each limit against a frame or a beat.

**Architecture:** Each benchmark lives in the package it measures, in a `*_bench_test.go` file, with deterministic hashed fixtures. Draw throughput runs as one game-loop benchmark in a test-only package, `render/drawbench`, because Ebitengine permits one `RunGame` per process. `task bench` runs everything under `xvfb-run -a`. The limits document is written from one recorded run.

**Tech Stack:** Go (version from `go.mod`), `testing` benchmarks with `b.Loop` and `b.ReportMetric`, Ebitengine v2, Taskfile v3, `xvfb-run`.

**Spec:** [docs/superpowers/specs/2026-09-11-benchmark-suite-design.md](../specs/2026-09-11-benchmark-suite-design.md)

## Global Constraints

* Environment: `export GOMODCACHE=/tmp/go-mod-cache` before every Go command. Never run anything that opens a window: every benchmark or test run that can touch Ebitengine goes through `xvfb-run -a` (or `task bench` / `task test:headless`).
* Before every commit: `task lint` and `task test:headless` pass.
* No exported API or behaviour changes; production code is not modified. No release tag.
* Benchmark files are named `<existing file>_bench_test.go` in the package they measure, matching `sim_eventqueue_bench_test.go` and `astar_bench_test.go`.
* Fixtures are deterministic: positions and values come from a fixed hash, never `math/rand` without a seed or map iteration.
* Every benchmark calls `b.ReportAllocs()` and uses `b.Loop()`. A benchmark whose fixture is wrong fails with `b.Fatalf`.
* Style (`docs/styleguide.md`): doc comments start with the symbol name and are complete sentences; no em dashes in any line added; sentence case headings; `*` bullets; identifiers spell `center`.
* Budgets: frame 16.7 ms (`time.Second / 60`), beat 1 s. Crossing points: 1 ms, whole budget, and for per-tick and per-beat aspects budget ÷ 10 (1.67 ms per tick, 100 ms per beat). Draw aspects have no 10x crossing point.
* Commits go straight to `main` locally; the controller pushes after the final review. Author `Claude Code <herve.quiroz+claude@gmail.com>`, no `Co-Authored-By:` line, last line `Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27`.
* Do not touch any repository other than vantage.

---

### Task 1: `task bench` and the tilemap benchmarks

**Files:**
- Modify: `Taskfile.yml` (add the `bench` task after `test:nodisplay`)
- Create: `tilemap/tilemap_grid_bench_test.go`
- Create: `tilemap/tilemap_bench_test.go`

**Interfaces:**
- Consumes: `tilemap.NewSpatialGrid`, `SpatialGrid.AddEntity`, `GetRange`, `UpdateEntityPosition`; `tilemap.NewTileOccupancyManager`, `SetOccupant`, `ClearOccupant`, `TileCoord`; `geometry.NewVector2`, `geometry.NewRectangleFromPoints`, `Vector2.X()`, `Vector2.Y()`; `ecs.NewWorld`, `World.NewEntity`.
- Produces: `BenchmarkSpatialGridGetRange`, `BenchmarkSpatialGridUpdateEntityPosition`, `BenchmarkTileOccupancyChurn`; the `task bench` target with vars `PKG`, `BENCH`, `BENCHTIME`.

- [ ] **Step 1: Add the task target**

In `Taskfile.yml`, directly after the `test:nodisplay` task and before `lint:`, add:

```yaml
  bench:
    desc: Run the benchmark suite under a virtual display
    summary: |
      Runs every benchmark, or the ones PKG and BENCH select, under xvfb-run so
      the draw throughput benchmark in render/drawbench has a display.
      BENCHTIME sets -benchtime, for example 1x for a smoke run. See
      docs/performance_limits.md for what each benchmark measures and the
      recorded results.

        task bench
        task bench PKG=./tilemap/
        task bench PKG=./tilemap/ BENCH=GetRange
        task bench BENCHTIME=1x
    vars:
      PKG: '{{.PKG | default "./..."}}'
      BENCH: '{{.BENCH | default "."}}'
      BENCHTIME: '{{.BENCHTIME | default "1s"}}'
    cmds:
      - xvfb-run -a go test -run '^$' -bench '{{.BENCH}}' -benchtime '{{.BENCHTIME}}' -timeout 60m {{.PKG}}
```

- [ ] **Step 2: Write the range query and index benchmarks**

Create `tilemap/tilemap_grid_bench_test.go`:

```go
package tilemap

import (
	"fmt"
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// benchWorldSize is the side, in tiles, of the square world the range query
// benchmark populates. It leaves room around the largest query half-width.
const benchWorldSize = 256

// benchHash mixes an index and a salt into a fixed pseudo-random value, so
// benchmark fixtures are identical on every run.
func benchHash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// populatedGrid returns a grid of the given cell size holding density entities
// per tile at hashed positions over a side by side world, with the entities and
// their positions in the order they were added.
func populatedGrid(cellSize float64, side int, density float64) (*SpatialGrid, []ecs.EntityId, []geometry.Vector2) {
	world := ecs.NewWorld()
	grid := NewSpatialGrid(cellSize)
	count := int(float64(side*side) * density)
	ids := make([]ecs.EntityId, count)
	positions := make([]geometry.Vector2, count)
	scale := uint64(side) * 1000
	for i := range count {
		ids[i] = world.NewEntity()
		positions[i] = geometry.NewVector2(
			float64(benchHash(i, 1)%scale)/1000,
			float64(benchHash(i, 2)%scale)/1000,
		)
		grid.AddEntity(ids[i], positions[i])
	}
	return grid, ids, positions
}

// BenchmarkSpatialGridGetRange measures one range query against the query's
// half-width, the entity density and the grid's cell size. The query is a square
// of half-width radius tiles centered in a populated world; entities-found/op
// is how many entities it returns.
func BenchmarkSpatialGridGetRange(b *testing.B) {
	for _, cellSize := range []float64{1, 4, 16} {
		for _, density := range []float64{0.01, 0.1, 0.5} {
			grid, _, _ := populatedGrid(cellSize, benchWorldSize, density)
			for _, radius := range []float64{4, 8, 16, 32, 64} {
				b.Run(fmt.Sprintf("cell=%g/density=%g/radius=%g", cellSize, density, radius), func(b *testing.B) {
					center := float64(benchWorldSize) / 2
					rect := geometry.NewRectangleFromPoints(center-radius, center-radius, center+radius, center+radius)
					found := len(grid.GetRange(rect))
					if expected := density * 4 * radius * radius; expected >= 4 && found == 0 {
						b.Fatalf("query of half-width %g at density %g found no entities, expected about %g", radius, density, expected)
					}

					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						grid.GetRange(rect)
					}
					b.ReportMetric(float64(found), "entities-found/op")
				})
			}
		}
	}
}

// BenchmarkSpatialGridUpdateEntityPosition measures moving one entity one tile
// across a cell boundary, in a grid of cell size 1 populated at density 0.1,
// against the population. Each op moves the next entity in turn; odd passes
// over the population move each entity back, so the population stays in place.
func BenchmarkSpatialGridUpdateEntityPosition(b *testing.B) {
	for _, population := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("population=%d", population), func(b *testing.B) {
			side := int(math.Sqrt(float64(population) / 0.1))
			grid, ids, positions := populatedGrid(1, side, 0.1)
			if len(ids) == 0 {
				b.Fatalf("population %d: fixture holds no entities", population)
			}

			b.ReportAllocs()
			b.ResetTimer()
			i := 0
			for b.Loop() {
				n := i % len(ids)
				step := 1.0
				if (i/len(ids))%2 == 1 {
					step = -1
				}
				from := positions[n]
				to := geometry.NewVector2(from.X()+step, from.Y())
				grid.UpdateEntityPosition(ids[n], from, to)
				positions[n] = to
				i++
			}
		})
	}
}
```

- [ ] **Step 3: Write the reservation churn benchmark**

Create `tilemap/tilemap_bench_test.go`:

```go
package tilemap

import (
	"fmt"
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
)

// BenchmarkTileOccupancyChurn measures one reservation moving to the next tile,
// ClearOccupant on its tile and SetOccupant on its neighbour, against how many
// tiles are occupied. Reservations sit on every other column, so a move never
// lands on another reservation; each op moves the next one in turn, and odd
// passes move each back, so the occupied set stays the same size.
func BenchmarkTileOccupancyChurn(b *testing.B) {
	for _, occupied := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("occupied=%d", occupied), func(b *testing.B) {
			world := ecs.NewWorld()
			occupancy := NewTileOccupancyManager()
			side := int(math.Ceil(math.Sqrt(float64(occupied))))
			ids := make([]ecs.EntityId, occupied)
			tiles := make([]TileCoord, occupied)
			for i := range occupied {
				ids[i] = world.NewEntity()
				tiles[i] = TileCoord{X: 2 * (i % side), Y: i / side}
				occupancy.SetOccupant(tiles[i], ids[i])
			}

			b.ReportAllocs()
			b.ResetTimer()
			i := 0
			for b.Loop() {
				n := i % occupied
				next := TileCoord{X: tiles[n].X + 1, Y: tiles[n].Y}
				if (i/occupied)%2 == 1 {
					next.X = tiles[n].X - 1
				}
				occupancy.ClearOccupant(tiles[n])
				occupancy.SetOccupant(next, ids[n])
				tiles[n] = next
				i++
			}
		})
	}
}
```

- [ ] **Step 4: Run the benchmarks once through the new target**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task bench PKG=./tilemap/ BENCHTIME=1x`
Expected: PASS; 45 `BenchmarkSpatialGridGetRange/cell=.../density=.../radius=...` lines each reporting `entities-found/op`, 3 `BenchmarkSpatialGridUpdateEntityPosition/population=...` lines, 3 `BenchmarkTileOccupancyChurn/occupied=...` lines, no `FAIL`.

Run: `task bench PKG=./tilemap/ BENCH=OccupancyChurn BENCHTIME=1x`
Expected: only the 3 `BenchmarkTileOccupancyChurn` lines run.

- [ ] **Step 5: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add Taskfile.yml tilemap/tilemap_grid_bench_test.go tilemap/tilemap_bench_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Add task bench and benchmark range queries and occupancy churn

task bench runs the benchmark suite under a virtual display, filtered by
PKG, BENCH and BENCHTIME. The tilemap benchmarks measure a range query
against its half-width, entity density and grid cell size, moving an
entity across the spatial index against population, and reservation
churn against occupied tiles.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 2: motion benchmarks

**Files:**
- Create: `motion/motion_system_bench_test.go`
- Create: `motion/motion_towards_bench_test.go`

**Interfaces:**
- Consumes: `newTestSystem() (*System, *ecs.World)` (motion/motion_system_test.go, a System with `Spatials`, `Movements` and a cell-size-1 `Grid`); `testTerrain{width, height, blocked}` (motion/motion_path_test.go); `System.MoveEntity`, `Tick`, `MoveEntityTowards`, `MoveEntityTowardsArea`, `MoveOptions{Speed, Ease}`, `MoveStart.Started()`, `MoveStart.Outcome`; `easing.CurveLinear`, `easing.CurveInOut`; `tilemap.NewTileOccupancyManager`, `tilemap.TileToWorldPosition`, `tilemap.WorldPositionToTile`, `tilemap.TileCoord`.
- Produces: `BenchmarkSystemTick`, `BenchmarkMoveEntityTowards`, `BenchmarkMoveEntityTowardsArea`, test helpers `benchFrame`, `benchHash`, `newDecisionSystem`.

- [ ] **Step 1: Write the movement tick benchmark**

Create `motion/motion_system_bench_test.go`:

```go
package motion

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
)

// benchFrame is one update at Ebitengine's default 60 updates per second, the
// elapsed time a game's frame hands to Tick.
const benchFrame = time.Second / 60

// benchHash mixes an index and a salt into a fixed pseudo-random value, so
// benchmark fixtures are identical on every run.
func benchHash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// BenchmarkSystemTick measures one Tick advancing every moving entity by one
// frame, with the spatial grid kept in sync, against the number of moving
// entities, for constant-speed and eased moves. Entities are spread at density
// 0.1 and head for destinations 100,000 tiles away, so none arrives during the
// benchmark.
func BenchmarkSystemTick(b *testing.B) {
	for _, ease := range []struct {
		name  string
		curve easing.Curve
	}{
		{name: "linear", curve: easing.CurveLinear},
		{name: "eased", curve: easing.CurveInOut},
	} {
		for _, count := range []int{1_000, 10_000, 100_000} {
			b.Run(fmt.Sprintf("ease=%s/entities=%d", ease.name, count), func(b *testing.B) {
				s, w := newTestSystem()
				side := uint64(math.Sqrt(float64(count) / 0.1))
				for i := range count {
					id := w.NewEntity()
					position := geometry.NewVector2(float64(benchHash(i, 1)%side), float64(benchHash(i, 2)%side))
					s.Spatials.Add(id, Spatial{Position: position})
					s.Grid.AddEntity(id, position)
					destination := geometry.NewVector2(position.X()+100_000, position.Y())
					if move := s.MoveEntity(id, destination, MoveOptions{Speed: 1, Ease: ease.curve}); !move.Started() {
						b.Fatalf("entity %d: move did not start: %v", i, move.Outcome)
					}
				}

				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					s.Tick(benchFrame)
				}
			})
		}
	}
}
```

- [ ] **Step 2: Write the movement decision benchmarks**

Create `motion/motion_towards_bench_test.go`:

```go
package motion

import (
	"fmt"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// decisionTerrainSize is the side, in tiles, of the open terrain the decision
// benchmarks run on.
const decisionTerrainSize = 256

// newDecisionSystem returns a System over open terrain decisionTerrainSize tiles
// wide, with occupancy and a one-tile step, and one entity standing on its
// reservation at start.
func newDecisionSystem(start geometry.Vector2) (*System, ecs.EntityId) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	s.Terrain = &testTerrain{width: decisionTerrainSize, height: decisionTerrainSize}
	s.MaxPathExpansions = 100_000
	s.MaxMoveActionDistance = 1.5
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: start})
	s.Grid.AddEntity(id, start)
	s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
	return s, id
}

// BenchmarkMoveEntityTowards measures one movement decision, planning a path to
// a destination journey tiles east and issuing the next step, on open terrain
// with occupancy and no heuristic configured. The entity is never ticked, so
// every op plans the same journey from the same tile.
func BenchmarkMoveEntityTowards(b *testing.B) {
	for _, journey := range []int{8, 32, 128} {
		b.Run(fmt.Sprintf("journey=%d", journey), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			destination := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4 + journey, Y: 64})
			s, id := newDecisionSystem(start)
			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				if move := s.MoveEntityTowards(id, destination, opts); !move.Started() {
					b.Fatalf("journey of %d tiles, decision %d: started no move: %v", journey, attempt, move.Outcome)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.MoveEntityTowards(id, destination, opts)
			}
		})
	}
}

// BenchmarkMoveEntityTowardsArea measures one decision toward a circular area of
// the given radius whose center lies 32 tiles east: finding the nearest reachable
// tile in the area, planning to it and issuing the next step. The entity is never
// ticked, so every op makes the same decision.
func BenchmarkMoveEntityTowardsArea(b *testing.B) {
	for _, radius := range []float64{1, 4, 8} {
		b.Run(fmt.Sprintf("radius=%g", radius), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 36, Y: 64})
			s, id := newDecisionSystem(start)
			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				if move := s.MoveEntityTowardsArea(id, center, radius, opts); !move.Started() {
					b.Fatalf("area of radius %g, decision %d: started no move: %v", radius, attempt, move.Outcome)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.MoveEntityTowardsArea(id, center, radius, opts)
			}
		})
	}
}
```

- [ ] **Step 3: Run the benchmarks once**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task bench PKG=./motion/ BENCHTIME=1x`
Expected: PASS; 6 `BenchmarkSystemTick/ease=.../entities=...` lines, 3 `BenchmarkMoveEntityTowards/journey=...` and 3 `BenchmarkMoveEntityTowardsArea/radius=...` lines, no `FAIL`.

If a decision benchmark fails its two-decision check, report the outcome rather than changing the fixture: the benchmark relies on a re-issued decision starting a move each time.

- [ ] **Step 4: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add motion/motion_system_bench_test.go motion/motion_towards_bench_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Benchmark the movement tick and movement decisions

BenchmarkSystemTick measures one frame's Tick against the number of
moving entities, for constant-speed and eased moves, with the spatial
grid in sync. BenchmarkMoveEntityTowards and
BenchmarkMoveEntityTowardsArea measure one decision against journey
length and area radius on open terrain with occupancy.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 3: event dispatch and draw ordering benchmarks

**Files:**
- Create: `sim/sim_driver_bench_test.go`
- Create: `render/render_drawlist_bench_test.go`

**Interfaces:**
- Consumes: `sim.NewDriver(handler EventHandler) *Driver`, `Driver.Queue() *EventQueue`, `Driver.RegisterTickSystem`, `Driver.RunUntil(target util.Time)`, `EventQueue.Add(Event)`, `Event{Time, Entity, Key}`, `util.Time`, `util.Time.Add(time.Duration)`, `benchEntities(n int) []ecs.EntityId` (sim/sim_eventqueue_bench_test.go); `render.DrawList[T]` with `Add(layer int, y float64, payload T)`, `Clear()`, `Each(visit func(payload T))`.
- Produces: `BenchmarkDriverRunUntil`, `BenchmarkDrawListOrdering`.

- [ ] **Step 1: Write the event dispatch benchmark**

Create `sim/sim_driver_bench_test.go`:

```go
package sim

import (
	"fmt"
	"testing"
	"time"

	"github.com/trancecode/vantage/util"
)

// rescheduleHandler schedules every event it handles again one beat later, so
// the queue holds the same events every beat.
type rescheduleHandler struct {
	queue *EventQueue
}

func (h *rescheduleHandler) HandleEvent(now util.Time, e Event) {
	e.Time = e.Time.Add(time.Second)
	h.queue.Add(e)
}

// noopTickSystem is a registered tick system that does nothing, so each stop
// pays the dispatch a game's own tick systems add to.
type noopTickSystem struct{}

func (noopTickSystem) Tick(time.Duration) {}

// BenchmarkDriverRunUntil measures one beat of event dispatch: one RunUntil
// advancing the clock by a second, dispatching every event due in it through a
// handler that schedules each again one beat later, with one registered no-op
// tick system. Events sit at distinct times spread across the beat, so each is
// its own stop where tick systems run. ns-per-event/op is the beat's cost
// divided by the events it dispatched.
func BenchmarkDriverRunUntil(b *testing.B) {
	pool := benchEntities(1024)
	for _, events := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("events=%d", events), func(b *testing.B) {
			handler := &rescheduleHandler{}
			driver := NewDriver(handler)
			handler.queue = driver.Queue()
			driver.RegisterTickSystem(noopTickSystem{})
			for i := range events {
				offset := time.Duration(i)*time.Second/time.Duration(events) + time.Nanosecond
				driver.Queue().Add(Event{Time: util.Time(0).Add(offset), Entity: pool[i%len(pool)], Key: uint64(i)})
			}

			beat := util.Time(0)
			driver.RunUntil(beat.Add(time.Second))
			beat = beat.Add(time.Second)
			if got := driver.Queue().Len(); got != events {
				b.Fatalf("after one beat the queue holds %d events, want %d", got, events)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				beat = beat.Add(time.Second)
				driver.RunUntil(beat)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(events), "ns-per-event/op")
		})
	}
}
```

- [ ] **Step 2: Write the draw ordering benchmark**

Read `render/render_drawlist.go` first. If it declares a constructor, use it in place of `var list DrawList[int]` below; otherwise the zero value is the list.

Create `render/render_drawlist_bench_test.go`:

```go
package render

import (
	"fmt"
	"testing"
)

// drawListBenchHash mixes an index into a fixed pseudo-random value, so the
// benchmark's layers and Y values are identical on every run.
func drawListBenchHash(i int) uint64 {
	h := uint64(i) * 0x9E3779B97F4A7C15
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// BenchmarkDrawListOrdering measures ordering one frame's drawables: Clear, then
// one Add per drawable with a hashed layer (0 to 3) and Y value, then one Each
// over the payloads in painter's order, against the number of drawables.
func BenchmarkDrawListOrdering(b *testing.B) {
	for _, count := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("drawables=%d", count), func(b *testing.B) {
			layers := make([]int, count)
			ys := make([]float64, count)
			for i := range count {
				h := drawListBenchHash(i)
				layers[i] = int(h % 4)
				ys[i] = float64((h>>8)%100_000) / 100
			}
			var list DrawList[int]
			visited := 0

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				list.Clear()
				for i := range count {
					list.Add(layers[i], ys[i], i)
				}
				visited = 0
				list.Each(func(int) { visited++ })
			}
			if visited != count {
				b.Fatalf("Each visited %d of %d drawables", visited, count)
			}
		})
	}
}
```

- [ ] **Step 3: Run the benchmarks once**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task bench PKG=./sim/ BENCH=DriverRunUntil BENCHTIME=1x && task bench PKG=./render/ BENCH=DrawListOrdering BENCHTIME=1x`
Expected: PASS; 3 `BenchmarkDriverRunUntil/events=...` lines each reporting `ns-per-event/op`, 3 `BenchmarkDrawListOrdering/drawables=...` lines, no `FAIL`.

- [ ] **Step 4: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add sim/sim_driver_bench_test.go render/render_drawlist_bench_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Benchmark event dispatch per beat and draw ordering per frame

BenchmarkDriverRunUntil measures one beat of RunUntil dispatching and
rescheduling a queue of events at distinct times, with a tick system
registered, and reports the cost per event. BenchmarkDrawListOrdering
measures one frame's Clear, Adds and painter's-order Each against the
number of drawables.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 4: draw throughput benchmark

**Files:**
- Create: `render/drawbench/drawbench_test.go`
- Modify: `scripts/check-headless-packages.sh` (add `./render/drawbench` to `GRAPHICS_PACKAGES`)

**Interfaces:**
- Consumes: `render.NewSprite() *Sprite`, `Sprite.AddImage(AnimationType, *ebiten.Image)`, `Sprite.Draw(screen *ebiten.Image, c *Camera, p geometry.Vector2, a AnimationType)`, `render.AnimationDefault`, `render.NewScreenCamera(w, h int) *Camera`, `render.NewTextWriter() *TextWriter`, `TextWriter.Text(msg string) *TextWriter`, `TextWriter.Draw(screen *ebiten.Image, camera *Camera, position geometry.Vector2)`; `ebiten.RunGame`, `ebiten.Termination`, `ebiten.NewImage`, `ebiten.SetWindowSize`, `ebiten.SetVsyncEnabled`, `ebiten.SetTPS`, `ebiten.SyncWithFPS`.
- Produces: `BenchmarkDrawThroughput` reporting `ms-per-frame-<case>/op` for the cases `sprites-100`, `sprites-1k`, `sprites-10k`, `sprites-50k`, `labels-10`, `labels-100`, `labels-1k`.

- [ ] **Step 1: Write the benchmark package**

Create `render/drawbench/drawbench_test.go`:

```go
// Package drawbench measures how many sprites and text labels vantage draws per
// frame. It is a package of its own, containing only test files, because
// Ebitengine permits one ebiten.RunGame per process and draws only execute
// inside the game loop: the benchmark runs one game that steps through every
// count. It needs a display; run it through task bench, which wraps it in
// xvfb-run.
//
// Under a virtual display the GPU work runs on Mesa's software rasterizer, so
// absolute frame times describe that machine rather than a game's hardware.
// What transfers is the scaling across counts and the CPU-side cost; a game
// re-measures on its own machine with the same benchmark.
package drawbench

import (
	"fmt"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
)

const (
	// canvasWidth and canvasHeight are the window size the benchmark draws into.
	canvasWidth  = 1280
	canvasHeight = 720

	// spritePixels is the side of the synthetic sprite, one tile at the default
	// tile size.
	spritePixels = 16

	// warmupFrames are drawn at each count before timing starts, so the frame
	// times exclude uploading textures and building glyph caches.
	warmupFrames = 10

	// measuredFrames are timed at each count.
	measuredFrames = 60
)

// drawCase is one count of one kind of drawable.
type drawCase struct {
	// name is the metric suffix, e.g. "sprites-1k".
	name string
	// draw draws this case's drawables for one frame.
	draw func(screen *ebiten.Image)
}

// hash mixes an index and a salt into a fixed pseudo-random value, so drawables
// land at the same positions on every run.
func hash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// positions returns count hashed positions inside the canvas.
func positions(count int, salt uint64) []geometry.Vector2 {
	out := make([]geometry.Vector2, count)
	for i := range count {
		out[i] = geometry.NewVector2(float64(hash(i, salt)%canvasWidth), float64(hash(i, salt+1)%canvasHeight))
	}
	return out
}

// countLabel renders a count as the metric suffix spells it: 100, 1k, 50k.
func countLabel(count int) string {
	if count >= 1000 {
		return fmt.Sprintf("%dk", count/1000)
	}
	return fmt.Sprintf("%d", count)
}

// buildCases returns every case in the order the benchmark runs them. It creates
// images, so it runs inside the game loop.
func buildCases() []drawCase {
	img := ebiten.NewImage(spritePixels, spritePixels)
	img.Fill(color.White)
	sprite := render.NewSprite()
	sprite.AddImage(render.AnimationDefault, img)
	camera := render.NewScreenCamera(canvasWidth, canvasHeight)
	label := render.NewTextWriter().Text("label")

	var cases []drawCase
	for _, count := range []int{100, 1_000, 10_000, 50_000} {
		at := positions(count, 1)
		cases = append(cases, drawCase{
			name: "sprites-" + countLabel(count),
			draw: func(screen *ebiten.Image) {
				for _, p := range at {
					sprite.Draw(screen, camera, p, render.AnimationDefault)
				}
			},
		})
	}
	for _, count := range []int{10, 100, 1_000} {
		at := positions(count, 3)
		cases = append(cases, drawCase{
			name: "labels-" + countLabel(count),
			draw: func(screen *ebiten.Image) {
				for _, p := range at {
					label.Draw(screen, camera, p)
				}
			},
		})
	}
	return cases
}

// throughputGame draws each case for warmupFrames and then measuredFrames
// frames, and records the mean wall time between consecutive frames while
// measuring. With vsync off and ticks synchronized with frames, that interval
// is the whole frame: the draw calls, the GPU flush and the loop's own work.
type throughputGame struct {
	cases     []drawCase
	current   int
	frame     int
	lastFrame time.Time
	elapsed   time.Duration
	results   []time.Duration
}

// Update builds the cases on the first frame and ends the game once every case
// has been measured.
func (g *throughputGame) Update() error {
	if g.cases == nil {
		g.cases = buildCases()
	}
	if g.current >= len(g.cases) {
		return ebiten.Termination
	}
	return nil
}

// Draw draws the current case and times the frame.
func (g *throughputGame) Draw(screen *ebiten.Image) {
	if g.current >= len(g.cases) {
		return
	}
	now := time.Now()
	if g.frame > warmupFrames {
		g.elapsed += now.Sub(g.lastFrame)
	}
	g.lastFrame = now
	g.cases[g.current].draw(screen)
	g.frame++
	if g.frame > warmupFrames+measuredFrames {
		g.results = append(g.results, g.elapsed/measuredFrames)
		g.current++
		g.frame = 0
		g.elapsed = 0
	}
}

// Layout keeps the canvas at its fixed size.
func (g *throughputGame) Layout(int, int) (int, int) {
	return canvasWidth, canvasHeight
}

// gameRun holds the one game loop's outcome, shared by every call to the
// benchmark in this process.
var (
	gameRun     sync.Once
	gameResults []time.Duration
	gameNames   []string
	gameErr     error
)

// BenchmarkDrawThroughput measures one frame's wall time drawing a synthetic
// 16-pixel sprite 100, 1,000, 10,000 and 50,000 times, and a short text label
// 10, 100 and 1,000 times, at hashed positions on a 1280x720 canvas, with vsync
// off. It reports ms-per-frame-<case>/op for each case. The game loop runs once
// per process, the first time the benchmark is called; later calls report the
// same results, so b.N and -benchtime do not change what it measures.
func BenchmarkDrawThroughput(b *testing.B) {
	gameRun.Do(func() {
		ebiten.SetWindowSize(canvasWidth, canvasHeight)
		ebiten.SetVsyncEnabled(false)
		ebiten.SetTPS(ebiten.SyncWithFPS)
		game := &throughputGame{}
		gameErr = ebiten.RunGame(game)
		gameResults = game.results
		for _, c := range game.cases {
			gameNames = append(gameNames, c.name)
		}
	})
	if gameErr != nil {
		b.Fatalf("running the draw throughput game: %v", gameErr)
	}
	if len(gameResults) != len(gameNames) || len(gameNames) == 0 {
		b.Fatalf("draw throughput game measured %d of %d cases", len(gameResults), len(gameNames))
	}

	for b.Loop() {
	}
	for i, name := range gameNames {
		b.ReportMetric(float64(gameResults[i].Microseconds())/1000, "ms-per-frame-"+name+"/op")
	}
}
```

- [ ] **Step 2: Register the package as a graphics package**

In `scripts/check-headless-packages.sh`, add `    ./render/drawbench` to the `GRAPHICS_PACKAGES` array directly below `    ./render`.

- [ ] **Step 3: Run the benchmark once**

Run: `export GOMODCACHE=/tmp/go-mod-cache && task bench PKG=./render/drawbench/ BENCHTIME=1x`
Expected: PASS; one `BenchmarkDrawThroughput` line reporting seven `ms-per-frame-.../op` metrics, no `FAIL`. A window must not appear on any real display: the run goes through `xvfb-run -a`.

Run: `task test:nodisplay`
Expected: PASS, with the new package recognised as a graphics package.

- [ ] **Step 4: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add render/drawbench/drawbench_test.go scripts/check-headless-packages.sh
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Benchmark sprites and labels drawn per frame

render/drawbench runs one Ebitengine game that draws a synthetic sprite
at 100 to 50,000 copies and a text label at 10 to 1,000 copies, and
reports the mean frame time per count. It is a test-only package
because Ebitengine permits one RunGame per process; under xvfb the GPU
work runs on Mesa's software rasterizer, so the scaling transfers and
absolute times describe the machine.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 5: run the suite and write the limits document

**Files:**
- Create: `docs/performance_limits.md`
- Modify: `docs/debugging.md` (add a `## Benchmarks` section after `## Profiler`)
- Modify: `docs/performance_optimization.md` (entries for what the run reveals)
- Modify: `README.md` only if it lists docs; check with `grep -n "performance" README.md` and add a link beside the existing performance docs if they are listed

**Interfaces:**
- Consumes: every benchmark from Tasks 1 to 4, the existing `BenchmarkEventQueueSteadyState` and `BenchmarkEventQueueAdd`, and the pathfinding table in `docs/pathfinding_performance.md`.

- [ ] **Step 1: Run the whole suite and keep the output**

Your Bash tool has a 10-minute limit per call, so run one package per command, appending to one file. Record the machine first.

```bash
export GOMODCACHE=/tmp/go-mod-cache
OUT=/tmp/claude-1000/-home-exedev-src-vantage/afee52d4-e187-4435-931f-bf48825cf2d1/scratchpad/limits_bench.txt
{ git rev-parse --short HEAD; lscpu | grep -E "Model name|^CPU\(s\)"; go version; } > $OUT
task bench PKG=./tilemap/ BENCH='SpatialGrid|TileOccupancy' | tee -a $OUT
task bench PKG=./motion/ | tee -a $OUT
task bench PKG=./sim/ | tee -a $OUT
task bench PKG=./render/ BENCH=DrawListOrdering | tee -a $OUT
task bench PKG=./render/drawbench/ | tee -a $OUT
```

If a package still exceeds the limit, split it further by `BENCH` pattern. Expected: every benchmark line present once, no `FAIL`.

- [ ] **Step 2: Write `docs/performance_limits.md`**

Write it from the numbers in the output file only. Sections in this order:

1. `# Performance limits`, then a paragraph on purpose: what a design discussion can read here, measured on demand, and that figures are one machine's.
2. `## Machine and how to re-run`: CPU, core count, Go version and commit from the output file's first lines; `task bench` and its `PKG`, `BENCH`, `BENCHTIME` variables with two examples.
3. `## Budgets`: the frame (16.7 ms) and beat (1 s) budgets, the 1 ms and whole-budget crossing points, and time acceleration: per-tick and per-beat work fits 10x only within a tenth of its budget, so those aspects show a third crossing point (1.67 ms per tick, 100 ms per beat); draw aspects do not, because a game draws once per frame at any clock rate. Crossing points are read off measured rows, never extrapolated.
4. `## Summary`: one table, one row per aspect, columns `Aspect | Dimension | Fits 1 ms | Fits budget | Fits at 10x | Details`, where the three fit columns give the largest measured parameter that fits (write `none measured` when even the smallest row does not fit, `all measured` when the largest does; `n/a` in the 10x column for draw aspects). Rows: range queries (at density 0.1, cell size 1 and at the best cell size), index maintenance, reservation churn, movement tick (linear), movement decisions (journey length), event dispatch (events per beat), draw ordering, sprites per frame, labels per frame, pathfinding (one row per strategy, quoted from docs/pathfinding_performance.md: the longest journey that fits 100,000 expansions on the grass and Reaches maps, with the link).
5. One section per aspect, headed by the aspect (for example `## Range queries`): the question it answers, the benchmark name and the command that re-runs it alone (`task bench PKG=./tilemap/ BENCH=GetRange`), the cost curve as a table with a cost-per-unit column, and the crossing points stated in prose. Range queries: one table per cell size, rows by radius and density, plus a sentence on what a larger cell buys, from the numbers. Event dispatch: also quote `BenchmarkEventQueueSteadyState` from the same run and link the "Alloc-free event queue heap" section of docs/performance_optimization.md. Draw throughput: state the software-rasterizer caveat prominently.
6. `## Pathfinding`: the headline envelope per strategy (nil, `ScaledOctile`, `CoarseCost`) quoted from docs/pathfinding_performance.md's table, a link to it, and no copied table.
7. `## Not benchmarked`: ui widgets, scene.Manager, geometry, easing and config, and auto-crop, each with its reason from the spec.

Arithmetic for crossing points: a row fits a budget when its measured time per op (ns/op, or ms-per-frame for draws) is at most the budget. For per-op aspects where a designer runs many ops per frame (range queries, index maintenance, reservation churn, decisions), also give "ops that fit" per budget: budget divided by ns/op, rounded down, and say it is derived from the measured per-op cost.

- [ ] **Step 3: Document the target where development tools are documented**

In `docs/debugging.md`, add after the `## Profiler` section:

```markdown
## Benchmarks

`task bench` runs the benchmark suite under a virtual display, so the draw
throughput benchmark has a display without opening a window:

```bash
task bench                                  # the whole suite
task bench PKG=./tilemap/                   # one package
task bench PKG=./tilemap/ BENCH=GetRange    # one benchmark
task bench BENCHTIME=1x                     # counts and a smoke run
```

`PKG` defaults to `./...`, `BENCH` to `.` and `BENCHTIME` to `1s`. What each
benchmark measures, and the recorded limits, are in
[performance_limits.md](performance_limits.md).
```

- [ ] **Step 4: Record what the run reveals**

For each result that points at a worthwhile optimization (for example a per-op allocation count that grows with the parameter, or a cost curve steeper than its algorithm suggests), add a section to `docs/performance_optimization.md` citing the benchmark and its figures, following the existing entries' form. Update the existing "SpatialGrid.GetRange result sorting" and "Alloc-free event queue heap" sections with this run's figures where their quoted numbers are superseded. Do not change production code.

- [ ] **Step 5: Verify and commit**

Run: `git diff -- docs README.md | grep '^+' | grep '—'`
Expected: prints nothing.

Check every figure in docs/performance_limits.md against the output file.

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add docs/performance_limits.md docs/debugging.md docs/performance_optimization.md README.md
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Record vantage's measured limits against frame and beat budgets

docs/performance_limits.md gives, for every aspect the benchmark suite
measures, the cost curve and the largest measured parameter that fits a
1 ms share, the whole frame or beat budget, and for per-tick and
per-beat work a tenth of it at 10x time acceleration. Pathfinding and
the event queue are quoted with links to their records. debugging.md
documents task bench.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

## After the tasks (controller)

* Final whole-branch review over the range from the spec commit to HEAD.
* `/deep-review --range 33e72d0..HEAD` alongside it; one fix wave for both.
* Pull with rebase and push `main`. No tag.
* Send exe.dev:nrg the summary: what is covered, how to run it, and the headline limits.
* Then take nrg's CoarseCost shore-underestimation finding as its own piece of work.
