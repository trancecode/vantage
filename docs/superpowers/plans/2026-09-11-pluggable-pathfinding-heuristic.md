# Pluggable pathfinding heuristic implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let each game choose `FindPath`'s estimate through a `pathfinding.Heuristic` set on `motion.System`, replace `MaxSpeedProvider` with a `ScaledOctile` strategy, and ship a `CoarseCost` strategy that learns fast and slow ground a cell at a time.

**Architecture:** `FindPath` gains a `Heuristic` parameter; nil keeps today's octile distance bit for bit. `CoarseCost` caches per-cell crossing rates built from real terrain costs, runs a resumable coarse search from the goal per search, and blends settled cell-centre values into tile estimates. Committed benchmarks measure every strategy over the same map families.

**Tech Stack:** Go (version from `go.mod`), standard library `container/heap`, testify for tests, `go test -bench` with `b.ReportMetric`.

**Spec:** [docs/superpowers/specs/2026-09-11-pluggable-pathfinding-heuristic-design.md](../specs/2026-09-11-pluggable-pathfinding-heuristic-design.md)

## Global Constraints

* Environment: `export GOMODCACHE=/tmp/go-mod-cache` before every Go command. On a fresh checkout run `task install:tools` once.
* Before every commit: `task lint` and `task test:headless` both pass. `go test ./pathfinding/ ./motion/ ./tilemap/` is fine for iterating (graphics-free packages).
* A nil heuristic returns identical routes and identical benchmark expansion counts to today: 257 expansions for `BenchmarkFindPathByPathLength/cardinal/distance=256`, 100,000 for `BenchmarkFindPathBudgetExhausted`.
* Follow `docs/styleguide.md`: panic messages are `<context>: <reason>`; every exported type, function, constant and struct field has a doc comment starting with its name; complete sentences in comments.
* In docs and comments: no em dashes; sentence case headings; `*` for bullet lists.
* Code identifiers spell it `center`, not `centre`.
* Exact constants: `DefaultCoarseCellSize = 32`, `DefaultCoarseCellBudget = 2048`, `coarseTieBreak = 1.01`.
* Nothing whose result depends on map iteration order. The coarse search's order comes only from its heap and the fixed `directions` slice.
* `CoarseCost` is not safe for concurrent use; say so in its doc comment.
* Commits go straight to `main` locally; the controller pushes after the final review. Author `Claude Code <herve.quiroz+claude@gmail.com>`, no `Co-Authored-By:` line, last line `Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27`.
* Do not touch any repository other than vantage.

---

### Task 1: `Heuristic` interface, `ScaledOctile`, and the `FindPath` parameter

Replaces `MaxSpeedProvider` with a strategy and threads a heuristic through `FindPath` and `motion.System`. After this task every existing caller passes nil and behaves exactly as before.

**Files:**
- Create: `pathfinding/heuristic.go`
- Modify: `pathfinding/astar.go` (remove `MaxSpeedProvider`, `heuristic`, `terrainMaxSpeed`; add the parameter)
- Modify: `pathfinding/astar_test.go` (migrate calls; replace the two `MaxSpeedProvider` tests and `speedBoundedTerrain`)
- Modify: `pathfinding/astar_bench_test.go`, `pathfinding/astar_roads_bench_test.go`
- Modify: `tilemap/tilemap_terrain_test.go`
- Modify: `motion/motion_system.go`, `motion/motion_path.go`, `motion/motion_path_test.go`
- Modify: `pathfinding/doc.go`, `docs/pathfinding_performance.md`, `docs/performance_optimization.md`

**Interfaces:**
- Produces:
  * `type Heuristic interface { ForSearch(start, goal Coord) Estimate }`
  * `type Estimate func(tile Coord) float64`
  * `type ScaledOctile struct { MaxSpeed float64 }` with `func (s ScaledOctile) ForSearch(start, goal Coord) Estimate`
  * unexported `func octile(from, to Coord) float64`
  * `func FindPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) []Coord`
  * unexported `func findPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) (path []Coord, expanded int)`
  * `motion.System.Heuristic pathfinding.Heuristic`

- [ ] **Step 1: Write the failing tests**

In `pathfinding/astar_test.go`, delete `speedBoundedTerrain`, its `MaxSpeedMultiplier` method, `TestFindPathMaxSpeedTakesRoadDetour` and `TestFindPathRequiresPositiveMaxSpeed`. Keep `pathCost`. Append:

```go
// recordingHeuristic records the start and goal of every search it served,
// estimating with octile distance.
type recordingHeuristic struct {
	searches [][2]Coord
}

func (h *recordingHeuristic) ForSearch(start, goal Coord) Estimate {
	h.searches = append(h.searches, [2]Coord{start, goal})
	return func(tile Coord) float64 { return octile(tile, goal) }
}

// TestFindPathAsksHeuristicOncePerSearch tests that FindPath asks the
// configured heuristic for one estimate per search it runs, with that search's
// start and goal, and not at all for a goal it rejects without searching.
func TestFindPathAsksHeuristicOncePerSearch(t *testing.T) {
	terrain := newMockTerrain(10, 10)
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}
	terrain.setWalkable(9, 9, false)
	heuristic := &recordingHeuristic{}

	path := FindPath(terrain, Coord{0, 0}, Coord{5, 5}, nil, testMaxExpansions, heuristic)
	require.NotNil(t, path)
	assert.Equal(t, [][2]Coord{{{0, 0}, {5, 5}}}, heuristic.searches)
	assert.Equal(t, FindPath(terrain, Coord{0, 0}, Coord{5, 5}, nil, testMaxExpansions, nil), path, "An octile heuristic should route like nil")

	assert.Nil(t, FindPath(terrain, Coord{0, 0}, Coord{9, 9}, nil, testMaxExpansions, heuristic))
	assert.Len(t, heuristic.searches, 1, "A rejected goal should not reach the heuristic")
}

// TestScaledOctileTakesRoadDetour tests that ScaledOctile finds the optimal
// route when that route detours onto a road off the straight line, and that
// plain octile distance misses the detour.
func TestScaledOctileTakesRoadDetour(t *testing.T) {
	terrain := newMockTerrain(40, 12)
	for y := range 12 {
		for x := range 40 {
			terrain.setWalkable(x, y, true)
		}
	}
	for x := range 40 {
		terrain.setSpeed(x, 1, 2.0)
	}
	start := Coord{0, 8}
	goal := Coord{39, 8}

	direct := FindPath(terrain, start, goal, nil, testMaxExpansions, nil)
	scaled := FindPath(terrain, start, goal, nil, testMaxExpansions, ScaledOctile{MaxSpeed: 2.0})
	// Dividing by an enormous speed leaves no estimate to speak of, which
	// turns the search into Dijkstra's: exhaustive and certainly optimal.
	exhaustive := FindPath(terrain, start, goal, nil, testMaxExpansions, ScaledOctile{MaxSpeed: 1e12})
	require.NotNil(t, direct)
	require.NotNil(t, scaled)
	require.NotNil(t, exhaustive)

	assert.InDelta(t, 39.0, pathCost(terrain, direct), 1e-9, "Octile distance should walk the straight line")
	assert.InDelta(t, pathCost(terrain, exhaustive), pathCost(terrain, scaled), 1e-9, "ScaledOctile should find the optimal route")
	assert.Less(t, pathCost(terrain, scaled), pathCost(terrain, direct), "The optimal route should use the road")
}

// TestScaledOctileRequiresPositiveMaxSpeed tests that a fastest speed that is
// not positive is a programming error rather than a speed to divide by.
func TestScaledOctileRequiresPositiveMaxSpeed(t *testing.T) {
	terrain := newMockTerrain(3, 3)
	for y := range 3 {
		for x := range 3 {
			terrain.setWalkable(x, y, true)
		}
	}

	for _, maxSpeed := range []float64{0, -1, math.NaN()} {
		heuristic := ScaledOctile{MaxSpeed: maxSpeed}
		assert.Panics(t, func() { FindPath(terrain, Coord{0, 0}, Coord{2, 2}, nil, testMaxExpansions, heuristic) }, "max speed %v", maxSpeed)
	}
}
```

Migrate every existing call in `pathfinding/astar_test.go` to the new parameter:

```bash
sed -i \
  -e 's/testMaxExpansions)/testMaxExpansions, nil)/g' \
  -e 's/nil, budget)/nil, budget, nil)/g' \
  -e 's/nil, 1000)/nil, 1000, nil)/g' \
  -e 's/nil, pathTiles)/nil, pathTiles, nil)/g' \
  -e 's/nil, pathTiles-1)/nil, pathTiles-1, nil)/g' \
  pathfinding/astar_test.go
```

The new tests already pass a heuristic argument, so none of these expressions touch them. Check that no call was missed: `grep -nE "(FindPath|findPath)\(" pathfinding/astar_test.go | grep -vE ", (nil|heuristic|ScaledOctile\{[^}]*\})\)"` should print nothing.

In `motion/motion_path_test.go`, add `"github.com/trancecode/vantage/pathfinding"` to the imports (between `geometry` and `tilemap`) and append:

```go
// countingHeuristic counts the searches it served, estimating with octile
// distance.
type countingHeuristic struct {
	searches int
}

func (h *countingHeuristic) ForSearch(start, goal pathfinding.Coord) pathfinding.Estimate {
	h.searches++
	return pathfinding.ScaledOctile{MaxSpeed: 1}.ForSearch(start, goal)
}

func TestFindTilePath_UsesSystemHeuristic(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	s.MaxPathExpansions = testMaxPathExpansions
	heuristic := &countingHeuristic{}
	s.Heuristic = heuristic

	path := s.FindTilePath(tilemap.TileCoord{X: 0, Y: 0}, tilemap.TileCoord{X: 3, Y: 0})

	if len(path) == 0 {
		t.Fatal("expected a path")
	}
	if heuristic.searches != 1 {
		t.Errorf("expected the System's heuristic to serve 1 search, got %d", heuristic.searches)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ ./motion/`
Expected: build failure, `undefined: Estimate`, `undefined: ScaledOctile`, `too many arguments in call to FindPath`, `s.Heuristic undefined`.

- [ ] **Step 3: Implement**

Create `pathfinding/heuristic.go`:

```go
package pathfinding

import (
	"fmt"
	"math"
)

// Heuristic supplies the estimate FindPath orders its search by: for each
// tile, a guess of the cost of reaching the goal from it. A guess that never
// exceeds the true cost returns optimal routes; a closer guess expands fewer
// nodes. FindPath never reopens a node it has closed, so a guess that jumps
// between neighbouring tiles costs route quality, never correctness: the
// search still reads walkability, step costs and occupancy from the terrain.
type Heuristic interface {
	// ForSearch returns the estimate for one search from start to goal.
	// FindPath calls it once per search that runs, after the goal rejections
	// that need no search. The returned Estimate may keep per-search state and
	// is used by that search only.
	ForSearch(start, goal Coord) Estimate
}

// Estimate returns the estimated cost of reaching one search's goal from tile.
type Estimate func(tile Coord) float64

// octile returns the octile distance between two coords, where a diagonal move
// costs sqrt(2) and a cardinal move 1. It is FindPath's estimate when no
// Heuristic is configured, and exact on open ground at speed 1.0.
func octile(from, to Coord) float64 {
	dx := math.Abs(float64(to.X - from.X))
	dy := math.Abs(float64(to.Y - from.Y))
	return cardinalCost*math.Max(dx, dy) + (diagonalCost-cardinalCost)*math.Min(dx, dy)
}

// ScaledOctile is octile distance divided by the fastest speed multiplier any
// tile reports. It never overestimates, so FindPath returns optimal routes even
// onto terrain faster than 1.0, such as roads; the price is a weaker estimate
// everywhere, about 0.8 x length² expansions on open grass at a declared speed
// of 2.0. See docs/pathfinding_performance.md.
type ScaledOctile struct {
	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. It must be positive. Declaring less than the fastest tile brings
	// back the overestimate, so routes onto those tiles can be missed silently;
	// declaring more keeps routes optimal and only costs expansions.
	MaxSpeed float64
}

// ForSearch returns octile distance to goal divided by MaxSpeed. It panics when
// MaxSpeed is not positive.
func (s ScaledOctile) ForSearch(start, goal Coord) Estimate {
	if !(s.MaxSpeed > 0) {
		panic(fmt.Sprintf("finding path from %v to %v: ScaledOctile.MaxSpeed must be positive, got %v", start, goal, s.MaxSpeed))
	}
	maxSpeed := s.MaxSpeed
	return func(tile Coord) float64 { return octile(tile, goal) / maxSpeed }
}
```

In `pathfinding/astar.go`:

* Delete the `MaxSpeedProvider` type and its doc comment, the `heuristic` function, and `terrainMaxSpeed`.
* Replace the `FindPath` doc comment's first paragraph and signature so it reads:

```go
// FindPath finds a path between two coordinates using A* pathfinding algorithm.
// It uses the terrain provider to query terrain properties and an optional
// occupancy checker to avoid occupied coordinates. It returns nil when no path
// exists, when start and goal are the same, and when the goal is out of bounds,
// unwalkable, occupied or has no enterable tile next to it — the last three
// being answered without searching, since a search would have to flood the map
// to reach them.
//
// heuristic supplies the estimate the search orders its work by; nil is octile
// distance, exact on open ground at speed 1.0. See Heuristic for what a
// strategy may and may not change.
//
```

keeping the existing `maxExpansions` paragraph after it, and change the two signatures and the `FindPath` body:

```go
func FindPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) []Coord {
	path, _ := findPath(terrain, start, goal, isOccupied, maxExpansions, heuristic)
	return path
}
```

```go
func findPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) (path []Coord, expanded int) {
```

* Remove the line `maxSpeed := terrainMaxSpeed(terrain, start, goal)`.
* Directly after the `isGoalApproachable` rejection block and before `// Initialize A* data structures`, insert:

```go
	estimate := func(tile Coord) float64 { return octile(tile, goal) }
	if heuristic != nil {
		estimate = heuristic.ForSearch(start, goal)
	}
```

* Replace `h:     heuristic(start, goal, maxSpeed),` with `h:     estimate(start),` and `h:     heuristic(neighbor, goal, maxSpeed),` with `h:     estimate(neighbor),`.

In `pathfinding/astar_bench_test.go`, `runFindPath`: change `findPath(terrain, start, goal, isOccupied, benchMaxExpansions)` to `findPath(terrain, start, goal, isOccupied, benchMaxExpansions, nil)` and `FindPath(terrain, start, goal, isOccupied, benchMaxExpansions)` to `FindPath(terrain, start, goal, isOccupied, benchMaxExpansions, nil)`.

In `pathfinding/astar_roads_bench_test.go` (replaced wholesale in Task 5; keep it compiling now):

* Change `runRoadJourney`'s signature to `func runRoadJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, heuristic Heuristic, path []Coord, expanded int, optimalCost float64)` and its loop body to `FindPath(terrain, start, goal, nil, benchMaxExpansions, heuristic)`.
* In `BenchmarkFindPathRoads`, replace `scaled := speedBoundedTerrain{TerrainProvider: terrain, maxSpeed: roadSpeed}` with `scaled := ScaledOctile{MaxSpeed: roadSpeed}`, the optimal search with `findPath(terrain, start, goal, nil, uncappedExpansions, scaled)`, the octile sub-benchmark body with:

```go
					path, expanded := findPath(terrain, start, goal, nil, uncappedExpansions, nil)
					runRoadJourney(b, terrain, start, goal, nil, path, expanded, optimalCost)
```

and the scaled sub-benchmark body with `runRoadJourney(b, terrain, start, goal, scaled, optimalPath, optimalExpanded, optimalCost)`.
* In its doc comment, replace "fastest speed (MaxSpeedProvider)" with "fastest speed (ScaledOctile)" and "heuristic=scaled is the same terrain declaring roadSpeed" with "heuristic=scaled is ScaledOctile at roadSpeed".

In `tilemap/tilemap_terrain_test.go`, change `pathfinding.FindPath(terrain, pathfinding.Coord{X: 0, Y: 0}, pathfinding.Coord{X: 2, Y: 0}, nil, 100)` to end `nil, 100, nil)`.

In `motion/motion_system.go`, directly after the `MaxPathExpansions int` field, add:

```go

	// Heuristic supplies the estimate every A* search the System runs orders
	// its work by. Nil is octile distance, exact on open ground at speed 1.0;
	// see pathfinding.Heuristic for the strategies and what they cost.
	Heuristic pathfinding.Heuristic
```

In `motion/motion_path.go`, change the `FindPath` call to `pathfinding.FindPath(s.Terrain, startCoord, goalCoord, isOccupied, s.MaxPathExpansions, s.Heuristic)`.

In `pathfinding/doc.go`, replace the sentences "A terrain with faster tiles, such as roads, can implement MaxSpeedProvider so that routes onto them are found, at the price of a search that expands a region rather than a corridor." with "Terrain faster or slower than 1.0 makes octile distance a poor estimate, and FindPath takes a Heuristic so a game can choose a better one: ScaledOctile for optimal routes at the price of a search that expands a region rather than a corridor." (rewrap the paragraph to 80 columns).

In `docs/pathfinding_performance.md`, section "Terrain faster than 1.0: scaling the heuristic", replace "A terrain that implements `MaxSpeedProvider` has the heuristic divided by its fastest speed, which never overestimates and so returns optimal routes, but weakens the estimate everywhere. Nothing implements it by default, so every existing search is unchanged." with "The `ScaledOctile` heuristic divides octile distance by the fastest speed, which never overestimates and so returns optimal routes, but weakens the estimate everywhere. A search with no heuristic configured is unchanged." and replace "with the terrain still declaring 2.0 because roads exist elsewhere" with "with `ScaledOctile` still declaring 2.0 because roads exist elsewhere".

In `docs/performance_optimization.md`, section "Weak heuristic on terrain faster than 1.0", replace "A terrain that implements `MaxSpeedProvider` has octile distance divided by its fastest speed" with "`ScaledOctile` divides octile distance by the fastest speed".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ ./motion/ ./tilemap/ && grep -rn "MaxSpeedProvider\|speedBoundedTerrain\|terrainMaxSpeed" --include=*.go --include=*.md . | grep -v docs/superpowers`
Expected: `ok` for all three packages; the grep prints nothing.

Run: `go test ./pathfinding/ -run '^$' -bench 'BenchmarkFindPath(ByPathLength|BudgetExhausted)' -benchtime 1x | grep -o 'Benchmark[^ ]*\|[0-9.]* expansions/op' | paste - -`
Expected: `cardinal/distance=256` reports `257.0 expansions/op` and `BudgetExhausted` `100000 expansions/op`.

- [ ] **Step 5: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add -A pathfinding motion tilemap docs/pathfinding_performance.md docs/performance_optimization.md
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Make FindPath's estimate a pluggable Heuristic

FindPath and motion.System take a pathfinding.Heuristic; nil is octile
distance, with identical routes and benchmark expansion counts.
ScaledOctile replaces MaxSpeedProvider: the fastest speed is set on the
strategy where the game configures its search, not declared by the
terrain.

Direct FindPath callers add a nil argument. nrg and lockstep only reach
FindPath through motion.System and need no edit.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 2: Cell crossing rates

Builds one cell's crossing rates from real terrain costs. No public API yet.

**Files:**
- Create: `pathfinding/coarse_cell.go`
- Test: `pathfinding/coarse_cell_test.go`

**Interfaces:**
- Consumes: `directions`, `cardinalCost`, `diagonalCost`, `isCardinalDirection` from `astar.go`.
- Produces:
  * `type cellRates struct { westEast, northSouth float64 }` with `func (r cellRates) uncrossable() bool`
  * `func buildCellRates(terrain TerrainProvider, cell Coord, size int) cellRates`
  * test helpers `type speedFuncTerrain struct { speed func(x, y int) float64 }` (edgeless, walkable exactly when speed > 0) and `type countingTerrain struct { TerrainProvider; speedQueries int }`

- [ ] **Step 1: Write the failing tests**

Create `pathfinding/coarse_cell_test.go`:

```go
package pathfinding

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// speedFuncTerrain is an edgeless map whose speed at each tile comes from a
// function, a tile being walkable exactly when its speed is positive.
type speedFuncTerrain struct {
	speed func(x, y int) float64
}

func (t speedFuncTerrain) IsInBounds(x, y int) bool { return true }

func (t speedFuncTerrain) IsWalkable(x, y int) bool { return t.speed(x, y) > 0 }

func (t speedFuncTerrain) GetTerrainSpeedMultiplier(x, y int) float64 { return t.speed(x, y) }

// countingTerrain counts the speed queries it forwards to the terrain it wraps.
type countingTerrain struct {
	TerrainProvider
	speedQueries int
}

func (t *countingTerrain) GetTerrainSpeedMultiplier(x, y int) float64 {
	t.speedQueries++
	return t.TerrainProvider.GetTerrainSpeedMultiplier(x, y)
}

// uniformTerrain is edgeless ground at one speed.
func uniformTerrain(speed float64) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 { return speed }}
}

// TestBuildCellRatesUniformGround tests that uniform ground crosses at the
// inverse of its speed both ways, wherever the cell lies.
func TestBuildCellRatesUniformGround(t *testing.T) {
	for _, ground := range []struct {
		speed, rate float64
	}{{1.0, 1.0}, {0.5, 2.0}} {
		rates := buildCellRates(uniformTerrain(ground.speed), Coord{3, -2}, 32)
		assert.InDelta(t, ground.rate, rates.westEast, 1e-9, "speed %v", ground.speed)
		assert.InDelta(t, ground.rate, rates.northSouth, 1e-9, "speed %v", ground.speed)
	}
}

// TestBuildCellRatesRoadAlongCell tests that a road running the length of a
// cell makes it cheap to cross along the road, while crossing it the other
// way stays close to the grass rate.
func TestBuildCellRatesRoadAlongCell(t *testing.T) {
	terrain := speedFuncTerrain{speed: func(x, y int) float64 {
		if y >= 10 && y <= 12 {
			return 2.0
		}
		return 1.0
	}}

	rates := buildCellRates(terrain, Coord{0, 0}, 32)

	assert.InDelta(t, 0.5, rates.westEast, 1e-9)
	assert.Greater(t, rates.northSouth, 0.9)
	assert.Less(t, rates.northSouth, 1.0)
}

// TestBuildCellRatesPoolAcrossCell tests that a wall of impassable tiles
// across a cell makes it uncrossable that way but not the other.
func TestBuildCellRatesPoolAcrossCell(t *testing.T) {
	terrain := speedFuncTerrain{speed: func(x, y int) float64 {
		if x == 16 {
			return 0
		}
		return 1.0
	}}

	rates := buildCellRates(terrain, Coord{0, 0}, 32)

	assert.True(t, math.IsInf(rates.westEast, 1))
	assert.InDelta(t, 1.0, rates.northSouth, 1e-9)
	assert.False(t, rates.uncrossable())
	assert.True(t, buildCellRates(uniformTerrain(0), Coord{0, 0}, 32).uncrossable())
}

// TestBuildCellRatesReadsEachTileOnce tests that building a cell reads each of
// its tiles' speed exactly once.
func TestBuildCellRatesReadsEachTileOnce(t *testing.T) {
	terrain := &countingTerrain{TerrainProvider: uniformTerrain(1)}

	buildCellRates(terrain, Coord{0, 0}, 32)

	assert.Equal(t, 32*32, terrain.speedQueries)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run TestBuildCellRates`
Expected: build failure, `undefined: buildCellRates`.

- [ ] **Step 3: Implement**

Create `pathfinding/coarse_cell.go`:

```go
package pathfinding

import (
	"container/heap"
	"math"
)

// cellRates are a cell's crossing rates: the cheapest real cost per tile of
// crossing it west to east and north to south, infinite when the cell cannot
// be crossed that way.
type cellRates struct {
	westEast   float64
	northSouth float64
}

// uncrossable reports whether the cell can be crossed neither way.
func (r cellRates) uncrossable() bool {
	return math.IsInf(r.westEast, 1) && math.IsInf(r.northSouth, 1)
}

// buildCellRates reads the size x size tiles of cell once and returns its
// crossing rates. Each rate comes from a multi-source Dijkstra search
// restricted to the cell, with FindPath's step cost and corner-cutting rule,
// from every walkable tile of the first column (or row) to the first tile of
// the last column (or row) it closes, divided by size - 1.
func buildCellRates(terrain TerrainProvider, cell Coord, size int) cellRates {
	walkable := make([]bool, size*size)
	speed := make([]float64, size*size)
	for j := range size {
		for i := range size {
			x, y := cell.X*size+i, cell.Y*size+j
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				walkable[j*size+i] = true
				speed[j*size+i] = terrain.GetTerrainSpeedMultiplier(x, y)
			}
		}
	}
	span := float64(size - 1)
	return cellRates{
		westEast:   crossingCost(walkable, speed, size, true) / span,
		northSouth: crossingCost(walkable, speed, size, false) / span,
	}
}

// crossingCost returns the cheapest cost of reaching the cell's last column
// from any walkable tile of its first column when westEast is true, or its
// last row from its first row otherwise, or +Inf when nothing gets across.
func crossingCost(walkable []bool, speed []float64, size int, westEast bool) float64 {
	cost := make([]float64, size*size)
	for i := range cost {
		cost[i] = math.Inf(1)
	}
	open := &tileQueue{}
	for s := range size {
		index := s * size // first column, row s
		if !westEast {
			index = s // first row, column s
		}
		if walkable[index] {
			cost[index] = 0
			heap.Push(open, tileEntry{index: index})
		}
	}

	for open.Len() > 0 {
		entry := heap.Pop(open).(tileEntry)
		if entry.cost > cost[entry.index] {
			continue // superseded by a cheaper entry
		}
		x, y := entry.index%size, entry.index/size
		if (westEast && x == size-1) || (!westEast && y == size-1) {
			return entry.cost
		}
		for _, dir := range directions {
			nx, ny := x+dir.X, y+dir.Y
			if nx < 0 || ny < 0 || nx >= size || ny >= size {
				continue
			}
			next := ny*size + nx
			if !walkable[next] {
				continue
			}
			step := cardinalCost
			if !isCardinalDirection(dir.X, dir.Y) {
				if !walkable[y*size+nx] && !walkable[ny*size+x] {
					continue
				}
				step = diagonalCost
			}
			multiplier := (speed[entry.index] + speed[next]) / 2
			if multiplier <= 0 {
				continue
			}
			nextCost := entry.cost + step/multiplier
			if nextCost < cost[next] {
				cost[next] = nextCost
				heap.Push(open, tileEntry{index: next, cost: nextCost})
			}
		}
	}
	return math.Inf(1)
}

// tileEntry is a tile index waiting in a crossing search, at its cost when
// queued.
type tileEntry struct {
	index int
	cost  float64
}

// tileQueue is the crossing search's priority queue, cheapest first.
type tileQueue []tileEntry

func (q tileQueue) Len() int           { return len(q) }
func (q tileQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q tileQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (q *tileQueue) Push(x any) { *q = append(*q, x.(tileEntry)) }

func (q *tileQueue) Pop() any {
	old := *q
	entry := old[len(old)-1]
	*q = old[:len(old)-1]
	return entry
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run TestBuildCellRates -v`
Expected: 4 tests PASS.

- [ ] **Step 5: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add pathfinding/coarse_cell.go pathfinding/coarse_cell_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Measure a cell's crossing rates from real terrain costs

A cell's west-east and north-south rates are the cheapest real cost per
tile of crossing it, from a Dijkstra search restricted to the cell with
FindPath's own step cost and corner rule. The cheapest crossing rather
than a median is what keeps a narrow road visible inside a cell.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 3: `CoarseCost` field and the resumable coarse search

Adds the public type, its config and cell cache, and the per-search coarse search that settles cell-center values. The estimate that turns values into a `Heuristic` comes in Task 4.

**Files:**
- Create: `pathfinding/coarse_cost.go`
- Test: `pathfinding/coarse_cost_test.go`

**Interfaces:**
- Consumes: `cellRates`, `buildCellRates`, `speedFuncTerrain`, `countingTerrain`, `uniformTerrain` (Task 2); `octile` (Task 1).
- Produces:
  * `const DefaultCoarseCellSize = 32`, `const DefaultCoarseCellBudget = 2048`
  * `type CoarseCostConfig struct { CellSize int; MaxSpeed float64; CellBudget int }`
  * `type CoarseCost struct` (unexported fields `terrain TerrainProvider`, `config CoarseCostConfig`, `cells map[Coord]cellRates`)
  * `func NewCoarseCost(terrain TerrainProvider, config CoarseCostConfig) *CoarseCost`
  * unexported: `func (c *CoarseCost) cellOf(tile Coord) Coord`, `func (c *CoarseCost) center(cell Coord) (x, y float64)`, `func (c *CoarseCost) rates(cell Coord) cellRates`, `func (c *CoarseCost) newSearch(start, goal Coord) *coarseSearch`, `type coarseSearch struct { ...; settled int }`, `func (s *coarseSearch) value(cell Coord) (float64, bool)`, `func (s *coarseSearch) surrounding(tile Coord) [4]Coord`, `func localCost(fromX, fromY, toX, toY float64, rates cellRates) float64`, `func floorDiv(a, b int) int`

- [ ] **Step 1: Write the failing tests**

Create `pathfinding/coarse_cost_test.go`:

```go
package pathfinding

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCoarseConfig is the default coarse configuration at a declared fastest
// speed.
func testCoarseConfig(maxSpeed float64) CoarseCostConfig {
	return CoarseCostConfig{CellSize: DefaultCoarseCellSize, MaxSpeed: maxSpeed, CellBudget: DefaultCoarseCellBudget}
}

// TestNewCoarseCostRejectsInvalidConfig tests that every config field must be
// set to a usable value, since a zero would silently mean a broken field.
func TestNewCoarseCostRejectsInvalidConfig(t *testing.T) {
	valid := testCoarseConfig(2)
	for _, invalid := range []struct {
		name   string
		mutate func(*CoarseCostConfig)
	}{
		{"cell size 0", func(c *CoarseCostConfig) { c.CellSize = 0 }},
		{"cell size 1", func(c *CoarseCostConfig) { c.CellSize = 1 }},
		{"max speed 0", func(c *CoarseCostConfig) { c.MaxSpeed = 0 }},
		{"negative max speed", func(c *CoarseCostConfig) { c.MaxSpeed = -1 }},
		{"NaN max speed", func(c *CoarseCostConfig) { c.MaxSpeed = math.NaN() }},
		{"cell budget 0", func(c *CoarseCostConfig) { c.CellBudget = 0 }},
	} {
		config := valid
		invalid.mutate(&config)
		assert.Panics(t, func() { NewCoarseCost(uniformTerrain(1), config) }, invalid.name)
	}
	assert.Panics(t, func() { NewCoarseCost(nil, valid) }, "nil terrain")
	assert.NotPanics(t, func() { NewCoarseCost(uniformTerrain(1), valid) })
}

// TestCoarseCostBuildsEachCellOnce tests that the field reads a cell's tiles
// the first time the cell is needed and never again.
func TestCoarseCostBuildsEachCellOnce(t *testing.T) {
	terrain := &countingTerrain{TerrainProvider: uniformTerrain(1)}
	field := NewCoarseCost(terrain, testCoarseConfig(1))
	cellTiles := DefaultCoarseCellSize * DefaultCoarseCellSize

	field.rates(Coord{0, 0})
	assert.Equal(t, cellTiles, terrain.speedQueries)
	field.rates(Coord{0, 0})
	assert.Equal(t, cellTiles, terrain.speedQueries, "A built cell should not read tiles again")
	field.rates(Coord{-1, 0})
	assert.Equal(t, 2*cellTiles, terrain.speedQueries)
}

// TestCoarseSearchValuesOnUniformGrass tests that on uniform grass settled
// cell values grow by exactly one cell width per cardinal step away from the
// goal and by sqrt(2) widths per diagonal step.
func TestCoarseSearchValuesOnUniformGrass(t *testing.T) {
	field := NewCoarseCost(uniformTerrain(1), testCoarseConfig(1))
	search := field.newSearch(Coord{15 + 30*DefaultCoarseCellSize, 15}, Coord{15, 15})
	size := float64(DefaultCoarseCellSize)

	valueAt := func(cell Coord) float64 {
		value, ok := search.value(cell)
		require.True(t, ok, "cell %v should settle", cell)
		return value
	}

	assert.InDelta(t, size, valueAt(Coord{5, 0})-valueAt(Coord{4, 0}), 1e-9)
	assert.InDelta(t, math.Sqrt2*size, valueAt(Coord{5, 5})-valueAt(Coord{4, 4}), 1e-9)
}

// TestCoarseSearchSeesRoadItHasNotReached tests that a cell value accounts for
// a road off the straight line, because the search's focus never
// overestimates.
func TestCoarseSearchSeesRoadItHasNotReached(t *testing.T) {
	size := DefaultCoarseCellSize
	terrain := speedFuncTerrain{speed: func(x, y int) float64 {
		if y >= 2*size+9 && y <= 2*size+11 {
			return 2.0
		}
		return 1.0
	}}
	field := NewCoarseCost(terrain, testCoarseConfig(2))
	search := field.newSearch(Coord{15 + 30*size, 15}, Coord{15, 15})

	value, ok := search.value(Coord{30, 0})

	require.True(t, ok)
	grassOnly := float64(30 * size)
	assert.Less(t, value, 0.8*grassOnly, "The value should route over the road two cells away")
}

// TestCoarseSearchUncrossableCellAnswersInfinity tests that a cell nothing can
// cross answers infinity without settling anything.
func TestCoarseSearchUncrossableCellAnswersInfinity(t *testing.T) {
	size := DefaultCoarseCellSize
	terrain := speedFuncTerrain{speed: func(x, y int) float64 {
		if x >= 3*size && x < 4*size && y >= 0 && y < size {
			return 0
		}
		return 1.0
	}}
	field := NewCoarseCost(terrain, testCoarseConfig(1))
	search := field.newSearch(Coord{15 + 10*size, 15}, Coord{15, 15})

	value, ok := search.value(Coord{3, 0})

	assert.True(t, ok)
	assert.True(t, math.IsInf(value, 1))
	assert.Equal(t, 0, search.settled)
}

// TestCoarseSearchStopsAtCellBudget tests that a search settles no more than
// its cell budget and then reports values it could not settle.
func TestCoarseSearchStopsAtCellBudget(t *testing.T) {
	config := testCoarseConfig(1)
	config.CellBudget = 3
	field := NewCoarseCost(uniformTerrain(1), config)
	search := field.newSearch(Coord{15 + 20*DefaultCoarseCellSize, 15}, Coord{15, 15})

	_, ok := search.value(Coord{20, 0})

	assert.False(t, ok)
	assert.Equal(t, 3, search.settled)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run 'TestNewCoarseCost|TestCoarseCost|TestCoarseSearch'`
Expected: build failure, `undefined: CoarseCostConfig`.

- [ ] **Step 3: Implement**

Create `pathfinding/coarse_cost.go`:

```go
package pathfinding

import (
	"container/heap"
	"fmt"
	"math"
)

// DefaultCoarseCellSize is the measured default side of a CoarseCost cell in
// tiles. Smaller cells settled four times as many cells for the same ground,
// and larger ones read more ground for the same route quality; see the design
// spec under docs/superpowers/specs.
const DefaultCoarseCellSize = 32

// DefaultCoarseCellBudget is the measured default number of cells one search
// may settle. It keeps every measured 1,000-tile journey on coarse values
// while capping how much ground a cold search reads.
const DefaultCoarseCellBudget = 2048

// CoarseCostConfig configures a CoarseCost. Every field must be set.
type CoarseCostConfig struct {
	// CellSize is the side of a cell in tiles, at least 2.
	CellSize int

	// MaxSpeed is the highest speed multiplier the terrain reports for any
	// tile. The coarse search divides its focus by it, which keeps cell values
	// from missing faster ground the search has not looked at yet. It must be
	// positive.
	MaxSpeed float64

	// CellBudget bounds how many cells one search may settle before its
	// estimates fall back to octile distance. It is what makes the coarse
	// search return on a terrain with no edge. It must be positive.
	CellBudget int
}

// CoarseCost is a Heuristic that learns where fast and slow ground lies, a
// cell at a time. Each cell of CellSize tiles records its cheapest real cost
// per tile of crossing west to east and north to south, built the first time
// a search needs it and kept for the life of the CoarseCost. Each search runs
// a coarse search from the goal over cell centers, resumed whenever the tile
// search asks for an estimate, and blends the values of the centers around a
// tile into its estimate. Routes are near-optimal rather than guaranteed
// optimal.
//
// A CoarseCost is bound to the terrain it was built with and must be given to
// FindPath together with that same terrain. It assumes the terrain does not
// change once a cell is built; a changed tile only makes estimates worse, since
// the tile search still reads the terrain itself. It is not safe for
// concurrent use: searches share its cell cache.
type CoarseCost struct {
	terrain TerrainProvider
	config  CoarseCostConfig
	cells   map[Coord]cellRates // built cells, keyed by cell coordinate
}

// NewCoarseCost returns a coarse cost field over terrain. It panics when
// terrain is nil or a config field is not set to a usable value.
func NewCoarseCost(terrain TerrainProvider, config CoarseCostConfig) *CoarseCost {
	if terrain == nil {
		panic("creating coarse cost field: terrain is nil")
	}
	if config.CellSize < 2 {
		panic(fmt.Sprintf("creating coarse cost field: CellSize must be at least 2, got %d", config.CellSize))
	}
	if !(config.MaxSpeed > 0) {
		panic(fmt.Sprintf("creating coarse cost field: MaxSpeed must be positive, got %v", config.MaxSpeed))
	}
	if config.CellBudget <= 0 {
		panic(fmt.Sprintf("creating coarse cost field: CellBudget must be positive, got %d", config.CellBudget))
	}
	return &CoarseCost{terrain: terrain, config: config, cells: make(map[Coord]cellRates)}
}

// cellOf returns the cell containing tile.
func (c *CoarseCost) cellOf(tile Coord) Coord {
	return Coord{X: floorDiv(tile.X, c.config.CellSize), Y: floorDiv(tile.Y, c.config.CellSize)}
}

// center returns the tile-space position of a cell's center.
func (c *CoarseCost) center(cell Coord) (x, y float64) {
	size := c.config.CellSize
	half := float64(size-1) / 2
	return float64(cell.X*size) + half, float64(cell.Y*size) + half
}

// rates returns a cell's crossing rates, building the cell on first use. It is
// the one place the field reads cells, so a later per-cell cost source, one a
// game answers without reading tiles, replaces buildCellRates here and nowhere
// else.
func (c *CoarseCost) rates(cell Coord) cellRates {
	if rates, ok := c.cells[cell]; ok {
		return rates
	}
	rates := buildCellRates(c.terrain, cell, c.config.CellSize)
	c.cells[cell] = rates
	return rates
}

// floorDiv divides a by a positive b, rounding toward negative infinity.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// localCost is the cost from one point to another nearby, using one cell's
// crossing rates: octile distance with each axis weighted by its rate. An
// infinite rate takes the other axis's rate, and both are 1.0 when neither is
// finite, so a local cost is always finite.
func localCost(fromX, fromY, toX, toY float64, rates cellRates) float64 {
	westEast, northSouth := rates.westEast, rates.northSouth
	switch {
	case math.IsInf(westEast, 1) && math.IsInf(northSouth, 1):
		westEast, northSouth = 1, 1
	case math.IsInf(westEast, 1):
		westEast = northSouth
	case math.IsInf(northSouth, 1):
		northSouth = westEast
	}
	dx, dy := math.Abs(toX-fromX), math.Abs(toY-fromY)
	return dx*westEast + dy*northSouth - math.Min(dx, dy)*(westEast+northSouth)*(1-math.Sqrt2/2)
}

// coarseSearch is one search's coarse search over cell centers, from the goal
// toward the start, paused between requests for values.
type coarseSearch struct {
	field     *CoarseCost
	goal      Coord
	goalCell  Coord
	startCell Coord
	cost      map[Coord]float64 // best known cost from a center to the goal
	closed    map[Coord]bool    // centers whose cost is settled
	open      cellQueue
	settled   int // centers settled so far, bounded by CellBudget
}

// newSearch starts a coarse search for one tile search from start to goal,
// seeded with the four centers surrounding the goal at their local cost from
// it.
func (c *CoarseCost) newSearch(start, goal Coord) *coarseSearch {
	s := &coarseSearch{
		field:     c,
		goal:      goal,
		goalCell:  c.cellOf(goal),
		startCell: c.cellOf(start),
		cost:      make(map[Coord]float64),
		closed:    make(map[Coord]bool),
	}
	rates := c.rates(s.goalCell)
	for _, cell := range s.surrounding(goal) {
		x, y := c.center(cell)
		s.offer(cell, localCost(float64(goal.X), float64(goal.Y), x, y, rates))
	}
	return s
}

// surrounding returns the four cells whose centers surround tile, in the order
// west-north, east-north, west-south, east-south.
func (s *coarseSearch) surrounding(tile Coord) [4]Coord {
	size := float64(s.field.config.CellSize)
	half := (size - 1) / 2
	x := int(math.Floor((float64(tile.X) - half) / size))
	y := int(math.Floor((float64(tile.Y) - half) / size))
	return [4]Coord{{X: x, Y: y}, {X: x + 1, Y: y}, {X: x, Y: y + 1}, {X: x + 1, Y: y + 1}}
}

// focus estimates the coarse cost from a cell to the start cell. Dividing by
// the fastest speed keeps it from ever overestimating, so a settled center's
// value is the cheapest coarse route to the goal.
func (s *coarseSearch) focus(cell Coord) float64 {
	return octile(cell, s.startCell) * float64(s.field.config.CellSize) / s.field.config.MaxSpeed
}

// offer records cost as a cell's cost to the goal when it improves on the best
// known one, and queues the cell.
func (s *coarseSearch) offer(cell Coord, cost float64) {
	if best, ok := s.cost[cell]; ok && cost >= best {
		return
	}
	s.cost[cell] = cost
	heap.Push(&s.open, cellEntry{cell: cell, cost: cost, priority: cost + s.focus(cell)})
}

// value returns the cost from a cell's center to the goal, resuming the coarse
// search until that center is settled. A cell that cannot be crossed either
// way answers infinity without searching. It reports false when the center
// cannot be settled: the open set emptied, or CellBudget centers have been
// settled in this search.
func (s *coarseSearch) value(cell Coord) (float64, bool) {
	if s.field.rates(cell).uncrossable() {
		return math.Inf(1), true
	}
	for !s.closed[cell] {
		if s.open.Len() == 0 || s.settled >= s.field.config.CellBudget {
			return 0, false
		}
		entry := heap.Pop(&s.open).(cellEntry)
		if s.closed[entry.cell] || entry.cost > s.cost[entry.cell] {
			continue // settled already, or superseded by a cheaper entry
		}
		s.closed[entry.cell] = true
		s.settled++
		s.relax(entry.cell, entry.cost)
	}
	return s.cost[cell], true
}

// relax offers each neighbor of a settled cell the route through it. An east
// or west edge costs a cell width times the two cells' mean west-east rate,
// north and south likewise, and a diagonal edge sqrt(2) widths times the mean
// of the two cells' mean rates. Relaxing builds the neighbor.
func (s *coarseSearch) relax(cell Coord, cost float64) {
	size := float64(s.field.config.CellSize)
	here := s.field.rates(cell)
	for _, dir := range directions {
		next := Coord{X: cell.X + dir.X, Y: cell.Y + dir.Y}
		if s.closed[next] {
			continue
		}
		there := s.field.rates(next)
		var edge float64
		switch {
		case !isCardinalDirection(dir.X, dir.Y):
			edge = diagonalCost * size * (meanRate(here) + meanRate(there)) / 2
		case dir.X != 0:
			edge = size * (here.westEast + there.westEast) / 2
		default:
			edge = size * (here.northSouth + there.northSouth) / 2
		}
		if math.IsInf(edge, 1) {
			continue
		}
		s.offer(next, cost+edge)
	}
}

// meanRate is the mean of a cell's two crossing rates.
func meanRate(rates cellRates) float64 {
	return (rates.westEast + rates.northSouth) / 2
}

// cellEntry is a cell center waiting in a coarse search, at its cost to the
// goal when queued and its priority, that cost plus the focus.
type cellEntry struct {
	cell     Coord
	cost     float64
	priority float64
}

// cellQueue is the coarse search's priority queue, lowest priority first.
type cellQueue []cellEntry

func (q cellQueue) Len() int           { return len(q) }
func (q cellQueue) Less(i, j int) bool { return q[i].priority < q[j].priority }
func (q cellQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (q *cellQueue) Push(x any) { *q = append(*q, x.(cellEntry)) }

func (q *cellQueue) Pop() any {
	old := *q
	entry := old[len(old)-1]
	*q = old[:len(old)-1]
	return entry
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run 'TestNewCoarseCost|TestCoarseCost|TestCoarseSearch' -v`
Expected: 6 tests PASS.

If `TestCoarseSearchValuesOnUniformGrass` fails by a tiny amount, do not loosen the delta beyond `1e-6`: a larger gap means the edges or the order are wrong.

- [ ] **Step 5: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add pathfinding/coarse_cost.go pathfinding/coarse_cost_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Add the CoarseCost field and its resumable coarse search

CoarseCost caches crossing rates per cell and runs, per search, an A*
over cell centers from the goal toward the start, focused by octile
distance over MaxSpeed so settled values never miss faster ground. The
search pauses between requests and stops at CellBudget settled cells.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 4: `CoarseCost` as a `Heuristic`

Turns settled cell values into tile estimates and makes `CoarseCost` usable by `FindPath`.

**Files:**
- Modify: `pathfinding/coarse_cost.go`
- Test: `pathfinding/coarse_cost_test.go`

**Interfaces:**
- Consumes: everything from Task 3; `octile`, `Estimate`, `findPath`, `FindPath`, `ScaledOctile` (Task 1); `pathCost`, `unboundedTerrain` (existing in `astar_test.go`).
- Produces: `func (c *CoarseCost) ForSearch(start, goal Coord) Estimate`, `const coarseTieBreak = 1.01`, unexported `func (s *coarseSearch) estimate(tile Coord) float64`.

- [ ] **Step 1: Write the failing tests**

Append to `pathfinding/coarse_cost_test.go`:

```go
// roadTestTerrain is grass with a 3-tile road at speed 2.0 centered on row
// roadY. When blocked is set and true, the road is impassable instead.
func roadTestTerrain(roadY int, blocked *bool) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if y >= roadY-1 && y <= roadY+1 {
			if blocked != nil && *blocked {
				return 0
			}
			return 2.0
		}
		return 1.0
	}}
}

// poolTestTerrain is half-speed forest with one circular pool of radius 30
// centered on (100, 0).
func poolTestTerrain() speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		dx, dy := x-100, y
		if dx*dx+dy*dy <= 30*30 {
			return 0
		}
		return 0.5
	}}
}

// TestCoarseCostRouteNearOptimalOnRoad tests that the coarse field finds the
// road detour octile distance misses, within 2% of the optimal cost.
func TestCoarseCostRouteNearOptimalOnRoad(t *testing.T) {
	terrain := roadTestTerrain(20, nil)
	start, goal := Coord{0, 0}, Coord{200, 0}
	const budget = 1_000_000

	optimal := FindPath(terrain, start, goal, nil, budget, ScaledOctile{MaxSpeed: 2})
	coarse := FindPath(terrain, start, goal, nil, budget, NewCoarseCost(terrain, testCoarseConfig(2)))
	direct := FindPath(terrain, start, goal, nil, budget, nil)
	require.NotNil(t, optimal)
	require.NotNil(t, coarse)
	require.NotNil(t, direct)

	optimalCost := pathCost(terrain, optimal)
	assert.LessOrEqual(t, pathCost(terrain, coarse), optimalCost*1.02)
	assert.GreaterOrEqual(t, pathCost(terrain, direct), optimalCost*1.3, "The map should make the road worth taking")
}

// TestCoarseCostStaysACorridorThroughForest tests that on half-speed forest,
// where octile distance falls short and floods, the coarse field keeps the
// search close to its route, around a pool on the straight line, and still
// returns a near-optimal route.
func TestCoarseCostStaysACorridorThroughForest(t *testing.T) {
	terrain := poolTestTerrain()
	start, goal := Coord{0, 0}, Coord{200, 0}
	const budget = 1_000_000

	optimal, _ := findPath(terrain, start, goal, nil, budget, ScaledOctile{MaxSpeed: 0.5})
	_, octileExpanded := findPath(terrain, start, goal, nil, budget, nil)
	coarse, coarseExpanded := findPath(terrain, start, goal, nil, budget, NewCoarseCost(terrain, testCoarseConfig(2)))
	require.NotNil(t, optimal)
	require.NotNil(t, coarse)

	assert.Less(t, coarseExpanded, 20*len(coarse), "The search should stay a corridor")
	assert.Less(t, coarseExpanded*10, octileExpanded, "The field should expand far less than octile distance")
	assert.LessOrEqual(t, pathCost(terrain, coarse), pathCost(terrain, optimal)*1.02)
}

// TestCoarseCostFallsBackPastCellBudget tests that a search whose cell budget
// runs out still reaches the goal, through the octile fallback.
func TestCoarseCostFallsBackPastCellBudget(t *testing.T) {
	terrain := poolTestTerrain()
	config := testCoarseConfig(2)
	config.CellBudget = 1

	path := FindPath(terrain, Coord{0, 0}, Coord{200, 0}, nil, 1_000_000, NewCoarseCost(terrain, config))

	require.NotNil(t, path)
	assert.Equal(t, Coord{200, 0}, path[len(path)-1])
	for _, tile := range path {
		assert.True(t, terrain.IsWalkable(tile.X, tile.Y), "Path must avoid the pool: %v", tile)
	}
}

// TestCoarseCostSealedGoalReturnsNoPath tests that on an edgeless map a goal
// sealed in a pocket returns no path under the tile budget: the coarse search
// is bounded too.
func TestCoarseCostSealedGoalReturnsNoPath(t *testing.T) {
	goal := Coord{0, 0}
	terrain := unboundedTerrain{pocketCenter: goal, ringRadius: 2}

	path := FindPath(terrain, Coord{10, 10}, goal, nil, 1000, NewCoarseCost(terrain, testCoarseConfig(1)))

	assert.Nil(t, path)
}

// TestCoarseCostStaleCellsStillReturnWalkablePaths tests that terrain changing
// after cells were built only degrades estimates: the route is still read from
// the terrain as it is at search time.
func TestCoarseCostStaleCellsStillReturnWalkablePaths(t *testing.T) {
	blocked := false
	terrain := roadTestTerrain(20, &blocked)
	field := NewCoarseCost(terrain, testCoarseConfig(2))
	start, goal := Coord{0, 0}, Coord{200, 0}
	require.NotNil(t, FindPath(terrain, start, goal, nil, 1_000_000, field))

	blocked = true
	path := FindPath(terrain, start, goal, nil, 1_000_000, field)

	require.NotNil(t, path)
	assert.Equal(t, goal, path[len(path)-1])
	for _, tile := range path {
		assert.True(t, terrain.IsWalkable(tile.X, tile.Y), "Path must avoid the now-blocked road: %v", tile)
	}
}

// TestCoarseCostIsDeterministic tests that searches over the same inputs return
// the same path, from a fresh field or a warm one.
func TestCoarseCostIsDeterministic(t *testing.T) {
	terrain := poolTestTerrain()
	start, goal := Coord{0, 0}, Coord{150, 90}
	field := NewCoarseCost(terrain, testCoarseConfig(2))

	want := FindPath(terrain, start, goal, nil, 1_000_000, field)
	require.NotNil(t, want)

	assert.Equal(t, want, FindPath(terrain, start, goal, nil, 1_000_000, field), "warm field")
	assert.Equal(t, want, FindPath(terrain, start, goal, nil, 1_000_000, NewCoarseCost(terrain, testCoarseConfig(2))), "fresh field")
}

// TestCoarseCostOnFiniteMap tests that the field works over a finite map, where
// cells straddle the edge, and still takes the road detour.
func TestCoarseCostOnFiniteMap(t *testing.T) {
	terrain := newMockTerrain(40, 12)
	for y := range 12 {
		for x := range 40 {
			terrain.setWalkable(x, y, true)
		}
	}
	for x := range 40 {
		terrain.setSpeed(x, 1, 2.0)
	}
	config := testCoarseConfig(2)
	config.CellSize = 8
	start, goal := Coord{0, 8}, Coord{39, 8}

	path := FindPath(terrain, start, goal, nil, testMaxExpansions, NewCoarseCost(terrain, config))

	require.NotNil(t, path)
	assert.Less(t, pathCost(terrain, path), 39.0, "The route should use the road")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run 'TestCoarseCost'`
Expected: build failure, `*CoarseCost does not implement Heuristic (missing method ForSearch)`.

- [ ] **Step 3: Implement**

Append to `pathfinding/coarse_cost.go`:

```go
// coarseTieBreak scales every coarse estimate. On uniform ground an oblique
// journey has a wide band of equally cheap routes, and an estimate that is
// nearly exact leaves their priorities tied, so A* expands the band; the slight
// scale breaks the ties toward the goal. Measured on a 1,000-tile oblique
// forest journey: 147,885 expansions unscaled, 1,038 scaled, with route quality
// unchanged.
const coarseTieBreak = 1.01

// ForSearch returns the coarse estimate for one search from start to goal.
func (c *CoarseCost) ForSearch(start, goal Coord) Estimate {
	return c.newSearch(start, goal).estimate
}

// estimate blends the values of the four centers surrounding tile by the
// tile's position between them, leaving out centers that are infinite or that
// the cell budget left unsettled and renormalizing the rest. When every
// remaining weight is zero it takes the cheapest route through one of them.
// Within one cell of the goal's cell it is at most the local cost straight to
// the goal, and with no center to go by it is octile distance. Every estimate
// is scaled by coarseTieBreak.
func (s *coarseSearch) estimate(tile Coord) float64 {
	here := s.field.cellOf(tile)
	rates := s.field.rates(here)
	x, y := float64(tile.X), float64(tile.Y)

	size := float64(s.field.config.CellSize)
	half := (size - 1) / 2
	fx := (x - half) / size
	fx -= math.Floor(fx)
	fy := (y - half) / size
	fy -= math.Floor(fy)
	weights := [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}

	sum, weight := 0.0, 0.0
	cheapest := math.Inf(1)
	for i, cell := range s.surrounding(tile) {
		value, ok := s.value(cell)
		if !ok || math.IsInf(value, 1) {
			continue
		}
		sum += weights[i] * value
		weight += weights[i]
		centerX, centerY := s.field.center(cell)
		cheapest = math.Min(cheapest, localCost(x, y, centerX, centerY, rates)+value)
	}

	estimate := cheapest
	if weight > 0 {
		estimate = sum / weight
	}
	if max(here.X-s.goalCell.X, s.goalCell.X-here.X) <= 1 && max(here.Y-s.goalCell.Y, s.goalCell.Y-here.Y) <= 1 {
		estimate = math.Min(estimate, localCost(x, y, float64(s.goal.X), float64(s.goal.Y), rates))
	}
	if math.IsInf(estimate, 1) {
		estimate = octile(tile, s.goal)
	}
	return estimate * coarseTieBreak
}
```

Also add `var _ Heuristic = (*CoarseCost)(nil)` directly below the `CoarseCost` struct declaration, and `var _ Heuristic = ScaledOctile{}` below `ScaledOctile` in `heuristic.go`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -v -run 'TestCoarseCost|TestCoarseSearch|TestNewCoarseCost|TestBuildCellRates'`
Expected: all PASS.

If `TestCoarseCostRouteNearOptimalOnRoad` or `TestCoarseCostStaysACorridorThroughForest` fails, report the measured cost ratio or expansion counts rather than loosening the assertion: the thresholds come from the spec's measured results.

- [ ] **Step 5: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add pathfinding/coarse_cost.go pathfinding/coarse_cost_test.go pathfinding/heuristic.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Estimate tile costs from the coarse cost field

CoarseCost implements Heuristic: a tile's estimate blends the settled
values of the four cell centers around it, is bounded near the goal by
the local cost to the goal, falls back to octile distance past the cell
budget, and is scaled by 1.01 to break ties on uniform ground.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 5: Benchmarks for every strategy

Replaces `astar_roads_bench_test.go` with committed benchmarks over the same map families for octile, `ScaledOctile` and `CoarseCost`, plus the cost of building and keeping a cell.

**Files:**
- Delete: `pathfinding/astar_roads_bench_test.go`
- Create: `pathfinding/astar_heuristics_bench_test.go`
- Create: `pathfinding/coarse_cost_bench_test.go`

**Interfaces:**
- Consumes: `speedFuncTerrain` (Task 2); `CoarseCost`, `CoarseCostConfig`, `DefaultCoarseCellSize`, `DefaultCoarseCellBudget`, `floorDiv`, `buildCellRates` (Tasks 2 to 4); `ScaledOctile`, `findPath`, `FindPath` (Task 1); `pathCost` (`astar_test.go`); `benchMaxExpansions` (`astar_bench_test.go`).
- Produces: `BenchmarkFindPathHeuristics`, `BenchmarkCoarseCostCellBuild`, `BenchmarkCoarseCostRetainedPerCell`; map helpers `grassBenchTerrain`, `offsetRoadBenchTerrain`, `gridBenchTerrain`, `reachesBenchTerrain`.

- [ ] **Step 1: Remove the old benchmark and write the new ones**

Run: `git rm pathfinding/astar_roads_bench_test.go`

Create `pathfinding/astar_heuristics_bench_test.go`:

```go
package pathfinding

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// Speed multipliers of the terrain kinds on the heuristic benchmark maps.
const (
	roadSpeed   = 2.0
	grassSpeed  = 1.0
	forestSpeed = 0.5
)

// declaredMaxSpeed is the fastest speed a game with roads declares to
// ScaledOctile and CoarseCost, whatever ground a journey happens to cross.
const declaredMaxSpeed = roadSpeed

// roadWidth is how many tiles across a road is.
const roadWidth = 3

// gridSpacing is the distance between neighbouring parallel roads on the grid
// map, so a journey starting in the middle of a cell is gridSpacing/2 tiles
// from the nearest road.
const gridSpacing = 200

// forestBlockSize is the side of the square blocks the grid map's forest comes
// in, and forestPercent the share of blocks that are forest.
const (
	forestBlockSize = 16
	forestPercent   = 30
)

// Pools on the Reaches map sit on a poolSpacing lattice; poolPercent of lattice
// cells hold one, with a radius from 40 to 149 tiles.
const (
	poolSpacing = 400
	poolPercent = 60
)

// benchOrigin places every journey far from zero, so the widest search stays on
// positive coordinates. It is a multiple of gridSpacing, which puts the grid
// map's roads through it.
const benchOrigin = 10_000

// uncappedExpansions is a budget no journey comes near, so a search run with it
// returns the route its heuristic leads to and the expansions needed to get
// there.
const uncappedExpansions = 50_000_000

// benchChunkSize is the chunk size extra-chunks/op counts in: the granularity at
// which a lazily generated world materializes its tiles (nrg's is 64).
const benchChunkSize = 64

// benchJourneyLengths are the straight-line journey lengths, in tiles.
var benchJourneyLengths = []int{250, 500, 1000, 2000}

// onRoad reports whether coordinate c lies on a road centered on center.
func onRoad(c, center int) bool {
	return max(c-center, center-c) <= roadWidth/2
}

// floorMod returns a modulo m in [0, m), whatever the sign of a.
func floorMod(a, m int) int {
	return (a%m + m) % m
}

// blockHash mixes a block coordinate and a salt into a fixed pseudo-random
// value, so the maps are the same on every run.
func blockHash(x, y int, salt uint64) uint64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ salt*0x165667B19E3779F9
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// isForest reports whether (x, y) falls in a forest block.
func isForest(x, y int) bool {
	return blockHash(floorDiv(x, forestBlockSize), floorDiv(y, forestBlockSize), 0)%100 < forestPercent
}

// inPool reports whether (x, y) lies in one of the Reaches map's pools.
func inPool(x, y int) bool {
	px, py := floorDiv(x, poolSpacing), floorDiv(y, poolSpacing)
	for j := py - 1; j <= py+1; j++ {
		for i := px - 1; i <= px+1; i++ {
			h := blockHash(i, j, 7)
			if h%100 >= poolPercent {
				continue
			}
			centerX := i*poolSpacing + 50 + int((h>>8)%300)
			centerY := j*poolSpacing + 50 + int((h>>20)%300)
			radius := 40 + int((h>>32)%110)
			dx, dy := x-centerX, y-centerY
			if dx*dx+dy*dy <= radius*radius {
				return true
			}
		}
	}
	return false
}

// grassBenchTerrain is grass everywhere: the cost of a heuristic where no road
// helps.
func grassBenchTerrain(int) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 { return grassSpeed }}
}

// offsetRoadBenchTerrain is grass with one road parallel to the journey, a tenth
// of the journey's length off to one side. Reaching it means heading away from
// the goal.
func offsetRoadBenchTerrain(length int) speedFuncTerrain {
	roadY := benchOrigin + length/10
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if onRoad(y, roadY) {
			return roadSpeed
		}
		return grassSpeed
	}}
}

// gridBenchTerrain is a road grid gridSpacing tiles apart, with blocks of forest
// between the roads.
func gridBenchTerrain(int) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if onRoad(floorMod(x+gridSpacing/2, gridSpacing), gridSpacing/2) ||
			onRoad(floorMod(y+gridSpacing/2, gridSpacing), gridSpacing/2) {
			return roadSpeed
		}
		if isForest(x, y) {
			return forestSpeed
		}
		return grassSpeed
	}}
}

// reachesBenchTerrain is half-speed forest with impassable pools a few hundred
// tiles wide, standing in for nrg's Blighted Reaches.
func reachesBenchTerrain(int) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if inPool(x, y) {
			return 0
		}
		return forestSpeed
	}}
}

// benchMap is one map family and journey direction.
type benchMap struct {
	name    string
	terrain func(length int) speedFuncTerrain
	origin  Coord
	// dx and dy are the journey's unit direction.
	dx, dy float64
	// fastestSpeed is the highest speed any tile of the map reports, which
	// makes ScaledOctile at that speed the optimal reference.
	fastestSpeed float64
}

var benchMaps = []benchMap{
	{name: "grass/cardinal", terrain: grassBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: grassSpeed},
	{name: "offset-road/cardinal", terrain: offsetRoadBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: roadSpeed},
	{name: "grid/cardinal", terrain: gridBenchTerrain, origin: Coord{X: benchOrigin + gridSpacing/2, Y: benchOrigin + gridSpacing/2}, dx: 1, fastestSpeed: roadSpeed},
	{name: "grid/oblique", terrain: gridBenchTerrain, origin: Coord{X: benchOrigin + gridSpacing/2, Y: benchOrigin + gridSpacing/2}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5), fastestSpeed: roadSpeed},
	{name: "reaches/cardinal", terrain: reachesBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: forestSpeed},
	{name: "reaches/oblique", terrain: reachesBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5), fastestSpeed: forestSpeed},
}

// benchJourney returns the start and goal of a journey of length tiles on a map:
// from the map's origin along its direction, each moved off a pool when one
// sits on it.
func benchJourney(terrain TerrainProvider, m benchMap, length int) (start, goal Coord) {
	start = m.origin
	for !terrain.IsWalkable(start.X, start.Y) {
		start.Y += 7
	}
	goal = Coord{
		X: start.X + int(math.Round(float64(length)*m.dx)),
		Y: start.Y + int(math.Round(float64(length)*m.dy)),
	}
	for !terrain.IsWalkable(goal.X, goal.Y) {
		goal.X++
	}
	return start, goal
}

// BenchmarkFindPathHeuristics measures every Heuristic strategy over the same
// map families and journey lengths: heuristic=octile is nil, heuristic=scaled
// is ScaledOctile at declaredMaxSpeed, and heuristic=coarse is CoarseCost with
// the default config at declaredMaxSpeed.
//
// Each case reports the expansions its search needs to reach the goal with no
// budget in the way; whether that fits benchMaxExpansions; and how far the
// route lies above the optimum, ScaledOctile at the map's fastest speed with no
// budget. ns/op times a call under benchMaxExpansions, on a warm field for
// heuristic=coarse. heuristic=coarse also reports cold-ms/op, one call under the
// budget on a fresh field; cells-built/op, the cells that call built; and
// extra-chunks/op, the benchChunkSize chunks those cells read that the tile
// search did not, which is what a lazily generated world pays to materialize.
//
// The longest Reaches journeys take seconds per search. Run it on its own, with
// -benchtime 1x when only the counts are wanted, or narrow it with a pattern
// such as 'BenchmarkFindPathHeuristics/reaches/cardinal/length=1000'.
func BenchmarkFindPathHeuristics(b *testing.B) {
	for _, m := range benchMaps {
		for _, length := range benchJourneyLengths {
			b.Run(fmt.Sprintf("%s/length=%d", m.name, length), func(b *testing.B) {
				terrain := m.terrain(length)
				start, goal := benchJourney(terrain, m, length)
				optimalPath, _ := findPath(terrain, start, goal, nil, uncappedExpansions, ScaledOctile{MaxSpeed: m.fastestSpeed})
				if optimalPath == nil {
					b.Fatalf("path from %v to %v: no optimal route", start, goal)
				}
				optimalCost := pathCost(terrain, optimalPath)

				b.Run("heuristic=octile", func(b *testing.B) {
					runHeuristicJourney(b, terrain, start, goal, nil, optimalCost)
				})
				b.Run("heuristic=scaled", func(b *testing.B) {
					runHeuristicJourney(b, terrain, start, goal, ScaledOctile{MaxSpeed: declaredMaxSpeed}, optimalCost)
				})
				b.Run("heuristic=coarse", func(b *testing.B) {
					runCoarseJourney(b, terrain, start, goal, optimalCost)
				})
			})
		}
	}
}

// runHeuristicJourney reports a stateless strategy's journey metrics and times
// FindPath over the journey under benchMaxExpansions.
func runHeuristicJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, heuristic Heuristic, optimalCost float64) {
	b.Helper()

	path, expanded := findPath(terrain, start, goal, nil, uncappedExpansions, heuristic)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		FindPath(terrain, start, goal, nil, benchMaxExpansions, heuristic)
	}
	reportJourney(b, terrain, start, goal, path, expanded, optimalCost)
}

// coarseBenchConfig is the default coarse configuration at declaredMaxSpeed.
func coarseBenchConfig() CoarseCostConfig {
	return CoarseCostConfig{CellSize: DefaultCoarseCellSize, MaxSpeed: declaredMaxSpeed, CellBudget: DefaultCoarseCellBudget}
}

// runCoarseJourney reports CoarseCost's journey metrics, cold and warm, and
// times FindPath over the journey on a warm field under benchMaxExpansions.
func runCoarseJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, optimalCost float64) {
	b.Helper()

	coldField := NewCoarseCost(terrain, coarseBenchConfig())
	coldStart := time.Now()
	FindPath(terrain, start, goal, nil, benchMaxExpansions, coldField)
	cold := time.Since(coldStart)
	cellsBuilt := len(coldField.cells)
	extraChunks := coarseExtraChunks(terrain, start, goal)

	field := NewCoarseCost(terrain, coarseBenchConfig())
	path, expanded := findPath(terrain, start, goal, nil, uncappedExpansions, field)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		FindPath(terrain, start, goal, nil, benchMaxExpansions, field)
	}
	reportJourney(b, terrain, start, goal, path, expanded, optimalCost)
	b.ReportMetric(float64(cold.Microseconds())/1000, "cold-ms/op")
	b.ReportMetric(float64(cellsBuilt), "cells-built/op")
	b.ReportMetric(float64(extraChunks), "extra-chunks/op")
}

// reportJourney reports a journey's expansions, whether they fit
// benchMaxExpansions, and its route's cost above optimal, failing the benchmark
// when the route is missing or cheaper than the optimum.
func reportJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, path []Coord, expanded int, optimalCost float64) {
	b.Helper()

	if path == nil {
		b.Fatalf("path from %v to %v: no route", start, goal)
	}
	aboveOptimal := (pathCost(terrain, path) - optimalCost) / optimalCost * 100
	if aboveOptimal < -1e-9 {
		b.Fatalf("path from %v to %v: route is %.6f%% below the optimum", start, goal, -aboveOptimal)
	}
	fits := 0.0
	if expanded <= benchMaxExpansions {
		fits = 1
	}
	b.ReportMetric(float64(expanded), "expansions/op")
	b.ReportMetric(fits, "found-within-budget/op")
	b.ReportMetric(aboveOptimal, "pct-above-optimal/op")
}

// chunkRecorder is a terrain that notes which benchChunkSize chunks were
// queried through it.
type chunkRecorder struct {
	TerrainProvider
	chunks map[Coord]bool
}

func newChunkRecorder(terrain TerrainProvider) *chunkRecorder {
	return &chunkRecorder{TerrainProvider: terrain, chunks: make(map[Coord]bool)}
}

func (r *chunkRecorder) note(x, y int) {
	r.chunks[Coord{X: floorDiv(x, benchChunkSize), Y: floorDiv(y, benchChunkSize)}] = true
}

func (r *chunkRecorder) IsWalkable(x, y int) bool {
	r.note(x, y)
	return r.TerrainProvider.IsWalkable(x, y)
}

func (r *chunkRecorder) GetTerrainSpeedMultiplier(x, y int) float64 {
	r.note(x, y)
	return r.TerrainProvider.GetTerrainSpeedMultiplier(x, y)
}

// coarseExtraChunks returns how many chunks a fresh coarse field read, for one
// call under benchMaxExpansions, that the tile search itself did not.
func coarseExtraChunks(terrain TerrainProvider, start, goal Coord) int {
	tiles := newChunkRecorder(terrain)
	cells := newChunkRecorder(terrain)
	FindPath(tiles, start, goal, nil, benchMaxExpansions, NewCoarseCost(cells, coarseBenchConfig()))
	extra := 0
	for chunk := range cells.chunks {
		if !tiles.chunks[chunk] {
			extra++
		}
	}
	return extra
}
```

Create `pathfinding/coarse_cost_bench_test.go`:

```go
package pathfinding

import (
	"runtime"
	"testing"
)

// BenchmarkCoarseCostCellBuild measures building one cell of
// DefaultCoarseCellSize over each kind of ground, from the tile reads to both
// crossing rates.
func BenchmarkCoarseCostCellBuild(b *testing.B) {
	for _, ground := range []struct {
		name    string
		terrain speedFuncTerrain
	}{
		{name: "grass", terrain: grassBenchTerrain(0)},
		{name: "grid", terrain: gridBenchTerrain(0)},
		{name: "reaches", terrain: reachesBenchTerrain(0)},
	} {
		b.Run("ground="+ground.name, func(b *testing.B) {
			b.ReportAllocs()
			cell := 0
			for b.Loop() {
				buildCellRates(ground.terrain, Coord{X: cell, Y: benchOrigin / DefaultCoarseCellSize}, DefaultCoarseCellSize)
				cell++
			}
		})
	}
}

// BenchmarkCoarseCostRetainedPerCell measures the heap memory a CoarseCost keeps
// per built cell, over 4,096 cells.
func BenchmarkCoarseCostRetainedPerCell(b *testing.B) {
	const cellCount = 4096
	terrain := grassBenchTerrain(0)
	var retained float64
	for b.Loop() {
		field := NewCoarseCost(terrain, coarseBenchConfig())
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for i := range cellCount {
			field.rates(Coord{X: i, Y: 0})
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		retained = float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / cellCount
		runtime.KeepAlive(field)
	}
	b.ReportMetric(retained, "retained-bytes/cell")
}
```

- [ ] **Step 2: Run a narrow case to verify the benchmarks work**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run '^$' -bench 'BenchmarkFindPathHeuristics/(offset-road|reaches)/cardinal/length=250|BenchmarkCoarseCost' -benchtime 1x`
Expected: PASS, with `heuristic=octile` on `offset-road/cardinal/length=250` near `48.86 pct-above-optimal/op` and `251.0 expansions/op` (the figures recorded in `docs/pathfinding_performance.md` from 5c863c1), `heuristic=coarse` reporting `cold-ms/op`, `cells-built/op` and `extra-chunks/op`, and `retained-bytes/cell` a positive number.

- [ ] **Step 3: Run the full benchmark once and keep the output**

Run: `export GOMODCACHE=/tmp/go-mod-cache && go test ./pathfinding/ -run '^$' -bench 'BenchmarkFindPathHeuristics|BenchmarkCoarseCost' -benchtime 1x -timeout 60m | tee /tmp/claude-1000/-home-exedev-src-vantage/afee52d4-e187-4435-931f-bf48825cf2d1/scratchpad/heuristics_bench.txt`
Expected: PASS. Every case reports; none fails with "below the optimum". Hand the output file's path to Task 6.

- [ ] **Step 4: Lint, full tests, commit**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add pathfinding/astar_heuristics_bench_test.go pathfinding/coarse_cost_bench_test.go
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Benchmark every pathfinding heuristic over the same map families

BenchmarkFindPathHeuristics runs octile, ScaledOctile and CoarseCost
over grass, offset road, the road grid with forest and a Reaches-like
map of half-speed forest with pools, for journeys of 250 to 2,000 tiles.
It reports expansions, fit within 100,000, cost above optimal, time per
call, and for CoarseCost cold time, cells built and the extra 64-tile
chunks its cells read. BenchmarkCoarseCostCellBuild and
BenchmarkCoarseCostRetainedPerCell measure one cell's build cost and
memory. It replaces BenchmarkFindPathRoads, whose maps it keeps.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

### Task 6: Documentation and measured results

Records the committed benchmark results and describes the strategies where readers meet them.

**Files:**
- Modify: `pathfinding/doc.go`
- Modify: `docs/pathfinding_performance.md` (rewrite section "Terrain faster than 1.0: scaling the heuristic")
- Modify: `docs/performance_optimization.md` (rewrite section "Weak heuristic on terrain faster than 1.0 (pathfinding/astar.go)")
- Modify: `ARCHITECTURE.md` (package map row for `pathfinding`)

**Interfaces:**
- Consumes: the benchmark output file from Task 5 Step 3. If it is missing, re-run that step.

- [ ] **Step 1: Package doc**

In `pathfinding/doc.go`, replace the paragraph beginning "On open terrain no faster than 1.0" with:

```go
// On open terrain at speed 1.0, a search that succeeds costs one node expansion
// per tile of the path it returns, independently of the size of the map. Terrain
// faster or slower than that makes octile distance a poor estimate, so FindPath
// takes a Heuristic and a game chooses its strategy:
//
//   - nil, octile distance: exact on open grass, but routes miss roads faster
//     than 1.0 and searches flood over slow ground such as forest.
//   - ScaledOctile: optimal routes, at about 0.8 x length² expansions on open
//     grass when the declared fastest speed is 2.0.
//   - CoarseCost: near-optimal routes and corridor-shaped searches over both
//     fast and slow ground, learned from the terrain a cell at a time; the
//     first search over new ground pays for reading it.
```

(keep the following paragraph about failed searches unchanged).

- [ ] **Step 2: Performance doc**

In `docs/pathfinding_performance.md`, replace the whole section "Terrain faster than 1.0: scaling the heuristic" (from its heading up to, not including, "## What is left") with a section headed `## Heuristic strategies` containing, in this order:

1. One paragraph on the problem: step cost is distance over speed, octile distance assumes speed 1.0, roads make it overestimate and forest makes it fall short.
2. One paragraph per strategy (nil, `ScaledOctile`, `CoarseCost`) naming what it trades.
3. The map families as a bullet list: grass, offset road (one road a tenth of the journey off to one side), grid (roads 200 tiles apart, forest at 0.5 in 16-tile blocks on about 30% of the ground; cardinal and 2:1 oblique journeys), Reaches (half-speed forest with pools 80 to 300 tiles wide on a 400-tile lattice; cardinal and oblique).
4. How to re-run, as a code block:

```bash
export GOMODCACHE=/tmp/go-mod-cache
go test ./pathfinding/ -run '^$' -bench BenchmarkFindPathHeuristics -benchtime 1x -timeout 60m
go test ./pathfinding/ -run '^$' -bench BenchmarkCoarseCost
```

with one sentence on narrowing to one map, e.g. `-bench 'BenchmarkFindPathHeuristics/reaches/cardinal'`.

5. A results table built from the Task 5 output, one row per map and length, with columns: `Map | Length | Octile expansions | Octile fits 100k | Octile above optimal | Scaled expansions | Scaled fits 100k | Coarse expansions | Coarse fits 100k | Coarse above optimal | Coarse cold | Coarse warm | Cells built | Extra chunks`. `Scaled above optimal` is always 0.0% and is said once in prose instead of a column. "Coarse warm" is `ns/op` of `heuristic=coarse`, "Coarse cold" is `cold-ms/op`. Use the numbers exactly as the output reports them, rounded for reading (thousands separators, one decimal for percentages, milliseconds for times).
6. One line with the cell build cost per ground kind and retained bytes per cell, from `BenchmarkCoarseCostCellBuild` and `BenchmarkCoarseCostRetainedPerCell`.
7. A "What the table shows" bullet list, written from the actual numbers, covering: how `CoarseCost` routes compare with octile near roads; Reaches expansions against octile; which journeys exhaust `DefaultCoarseCellBudget` and fall back to octile, and what that costs; and the cold look-ahead cost in extra chunks with the sentence "On a world that generates terrain lazily, every extra chunk is ground materialized only to look ahead; nrg measured 1.7 ms per 64-tile chunk." Link the spec for the formulation and the rejected variants: `[design spec](superpowers/specs/2026-09-11-pluggable-pathfinding-heuristic-design.md)`.

- [ ] **Step 3: Optimization opportunities**

In `docs/performance_optimization.md`, replace the section "Weak heuristic on terrain faster than 1.0 (pathfinding/astar.go)" with a section headed `## Coarse cost field (pathfinding/coarse_cost.go, pathfinding/coarse_cell.go)` covering, one paragraph each, citing figures from the Task 5 output:

* Cold look-ahead: a fresh field reads the ground a detour could use, measured as extra chunks; a game-supplied per-cell cost source (answering a cell's rates from knowledge the game already has, such as a world graph of roads, without reading tiles) would remove that cost for lazily generated worlds. It is additive to `CoarseCost` and left undone until a game has such a source.
* Cell build constant factor: each build allocates its walkable, speed and cost slices and runs `container/heap` with interface boxing; a reused scratch buffer and a typed heap would cut the build cost, which matters only for cold searches.
* Per-search coarse search state: the cost and closed maps are allocated per search; many agents converging on one destination each rebuild the same coarse search, which a small per-goal cache of settled values would share.
* `ScaledOctile` remains expensive by design; the note about landmark heuristics from the old section stays as one sentence.

- [ ] **Step 4: Architecture package map**

In `ARCHITECTURE.md`, change the `pathfinding` row's purpose to `A* search with terrain awareness and pluggable heuristics (octile, ScaledOctile, CoarseCost)`.

- [ ] **Step 5: Verify and commit**

Run: `git diff -- docs pathfinding/doc.go ARCHITECTURE.md | grep '^+' | grep '—'`
Expected: prints nothing; no line this task adds contains an em dash (pre-existing lines may).

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless
git add pathfinding/doc.go docs/pathfinding_performance.md docs/performance_optimization.md ARCHITECTURE.md
git commit --author="Claude Code <herve.quiroz+claude@gmail.com>" -m "$(cat <<'EOF'
Document the heuristic strategies and their measured costs

docs/pathfinding_performance.md records BenchmarkFindPathHeuristics for
octile, ScaledOctile and CoarseCost over every map family, with how to
re-run it. The package doc says what each strategy trades, and
performance_optimization.md lists what the coarse field leaves undone.

Claude-Session: https://claude.ai/code/session_01Vm78pNiy2q1P95JvbdZN27
EOF
)"
```

---

## After the tasks (controller)

* Final whole-branch review over the six commits (subagent-driven development's closing review).
* `/deep-review --range 0e4ced8..HEAD` before pushing; the `styleguide-compliance` lens is added automatically. Fix wave, or rationale in the commit message for any style guide finding not complied with.
* Pull with rebase and push `main`.
* Tag `v0.1.21` as an annotated tag: `git tag -a v0.1.21 -m "v0.1.21: pluggable pathfinding heuristics and a coarse cost field"` and `git push origin v0.1.21`.
* Send exe.dev:nrg the tag, the API shape, the results table, and the consumer migration note (direct `FindPath` callers add a nil argument; none exist in nrg or lockstep).
