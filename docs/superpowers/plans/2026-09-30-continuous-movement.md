# Continuous movement building blocks implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the pieces a game needs to let bodies stand and walk anywhere in straight legs (circle reservations, a straight walk test, a route search between arbitrary points, route tightening and fixed-duration legs) without changing anything tile-based.

**Architecture:** `motion.System.Occupancy` becomes an interface that the existing tile ledger and a new per-entity circle table both satisfy. The geometry test lives in `geometry`, the corner-safe A* in `pathfinding`, and the walk test, route and tightening in `motion`, which already holds both terrain and positions.

**Tech Stack:** Go, `github.com/trancecode/ecs`, `testing` (plus `testify` where a package's tests already use it), Taskfile.

**Spec:** [docs/superpowers/specs/2026-09-30-continuous-movement-design.md](../specs/2026-09-30-continuous-movement-design.md). Read it before your task; this plan argues from it.

## Global Constraints

* Additive only: no existing exported behaviour changes. Every existing test in the repository must keep passing without edits to what it asserts. The only test edits allowed are mechanical ones where a test called a `TileOccupancyManager` method through `s.Occupancy` (Task 4).
* Positions are `float64` in tile units; tile `(x, y)` covers `[x, x+1] × [y, y+1]` and its centre is `(x+0.5, y+0.5)` (`tilemap.TileToWorldPosition`).
* Touching counts as touching for the walk test (distance equal to the radius is blocked). For circle reservations, touching does not conflict (conflict is strictly less than the sum of radii).
* Deterministic: no map iteration order may leak into a result. `SpatialGrid.GetRange` returns candidates sorted by `EntityId`.
* Style: follow [docs/styleguide.md](../../styleguide.md). Panic messages are `<action being attempted>: <reason>`, for example `"testing walk from (1, 2) to (3, 4): radius must not be negative, got -1"`. Document every exported identifier starting with its name. Struct field comments go above the field. Files are named `<pkg>_<topic>.go`, tests `<pkg>_<topic>_test.go`.
* Environment: `export GOMODCACHE=/tmp/go-mod-cache` before any Go command. Run package tests with `go test ./<pkg>/` (the packages touched here, `geometry`, `pathfinding`, `tilemap` and `motion`, need no display). Before each commit run `task lint`; the final task also runs `task test:headless`.
* Commits go straight to `main`. Author is already configured as `Claude Code <herve.quiroz+claude@gmail.com>`. End each commit message with:

  ```
  Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01JRMCVuKS29RMrESngA7MTU
  ```

  Do not push; the controller pushes.

## Review Focus

1. A body standing exactly on a tile boundary, or a leg ending exactly at a radius from a blocked tile. Expected: touching is blocked, and floating-point rounding in the column scan never skips a tile the exact capsule test would catch (Task 3's brute-force comparison with boundary-aligned inputs).
2. A walk from or to a point outside the map. Expected: out-of-bounds tiles are blocked like unwalkable ones (Task 3's out-of-bounds test).
3. A body of radius 0.5 or more asking for a route through a one-tile corridor. Expected: `TightenRoute` never returns a leg through a wall; it stops short or reports false (Task 8's large-body test).
4. `CancelMove` on an entity whose cancelled destination tile has since been taken by another entity, or whose current tile is held by another. Expected: the other entity's reservation is never cleared or overwritten (Task 4's tile-ledger `Stop` tests).
5. A long route with a small `maxLength`. Expected: tightening costs a handful of walk tests, not one per route point (Task 8's counting-terrain test).

## Baseline for the consumer check (measured 2026-09-30)

Lockstep (`~/src/lockstep`, commit `b88c659`) passes its whole suite at its pinned vantage v0.1.19. Against vantage `main` before this work (`6a5784b`), it panics with `MaxPathExpansions not configured`, a field made mandatory after v0.1.19. With one scratch line adding `MaxPathExpansions: 100_000,` to its `motion.System` literal in `core/core_world.go`, `core` and `arena` pass, and `shell` fails only `TestSheetLibraryRebasesAnchorPerAnimation`, a sprite crop box regression unrelated to movement. Task 9 must reproduce exactly that result after this work.

## File map

| File | Task | Responsibility |
|---|---|---|
| `geometry/geometry_capsule.go` (+ test) | 1 | Capsule against rectangle |
| `pathfinding/astar.go`, `pathfinding/astar_body_test.go` | 2 | `FindBodyPath` sharing the A* core |
| `motion/motion_walk.go` (+ test) | 3 | `WalkIsClear` |
| `motion/motion_occupancy.go` (+ test), `tilemap/tilemap.go`, `tilemap/tilemap_occupancy_test.go`, `motion/motion_move.go`, `motion/motion_path.go`, `motion/motion_towards.go`, `motion/motion_system.go` | 4 | `Occupancy` interface, tile ledger methods, `CancelMove` |
| `tilemap/tilemap_circles.go` (+ test) | 5 | `CircleReservations` |
| `motion/motion.go`, `motion/motion_move.go` | 6 | `MoveOptions.Duration`, `Movement.Timed` |
| `motion/motion_route.go` (+ test) | 7 | `System.FindRoute` |
| `motion/motion_tighten.go` (+ test) | 8 | `TightenRoute` |
| `ARCHITECTURE.md`, `docs/performance_optimization.md` | 9 | Docs and consumer check |

Package overviews (`doc.go`) are updated in the task that adds each piece.

---

### Task 1: Capsule against a rectangle

**Files:**
* Create: `geometry/geometry_capsule.go`
* Create: `geometry/geometry_capsule_test.go`
* Modify: `geometry/doc.go`

**Interfaces:**
* Consumes: `geometry.Vector2` (`X()`, `Y()`, `NewVector2`), `geometry.Rectangle{Min, Max}`.
* Produces: `func CapsuleTouchesRectangle(a, b Vector2, radius float64, r Rectangle) bool`.

- [ ] **Step 1: Write the failing test**

`geometry/geometry_capsule_test.go`:

```go
package geometry

import "testing"

func TestCapsuleTouchesRectangle(t *testing.T) {
	// The unit tile at (1, 1). Values are chosen to be exact in binary floating
	// point so the boundary cases do not depend on rounding.
	tile := NewRectangleFromPoints(1.0, 1.0, 2.0, 2.0)

	cases := []struct {
		name   string
		a, b   Vector2
		radius float64
		want   bool
	}{
		{"crosses the middle, both ends far outside", NewVector2(-3.0, 1.5), NewVector2(6.0, 1.5), 0.25, true},
		{"ends inside", NewVector2(-3.0, 1.5), NewVector2(1.5, 1.5), 0.25, true},
		{"parallel to an edge at exactly the radius", NewVector2(-3.0, 0.75), NewVector2(6.0, 0.75), 0.25, true},
		{"parallel to an edge beyond the radius", NewVector2(-3.0, 0.5), NewVector2(6.0, 0.5), 0.25, false},
		// The line x + y = 1.6 passes the corner (1, 1) at 0.4/sqrt(2) = 0.283.
		{"passes a corner just outside the radius", NewVector2(2.6, -1.0), NewVector2(-1.0, 2.6), 0.25, false},
		// The line x + y = 1.7 passes it at 0.3/sqrt(2) = 0.212.
		{"passes a corner just inside the radius", NewVector2(2.7, -1.0), NewVector2(-1.0, 2.7), 0.25, true},
		{"zero length, clear", NewVector2(0.0, 0.0), NewVector2(0.0, 0.0), 0.25, false},
		{"zero length, touching an edge", NewVector2(0.75, 1.5), NewVector2(0.75, 1.5), 0.25, true},
		{"zero radius along an edge", NewVector2(1.0, 0.5), NewVector2(1.0, 2.5), 0, true},
		{"zero radius crossing, ends outside", NewVector2(0.0, 1.5), NewVector2(3.0, 1.75), 0, true},
		{"zero radius missing", NewVector2(0.0, 0.5), NewVector2(3.0, 0.75), 0, false},
		{"segment inside, radius zero", NewVector2(1.25, 1.25), NewVector2(1.75, 1.5), 0, true},
	}
	for _, c := range cases {
		if got := CapsuleTouchesRectangle(c.a, c.b, c.radius, tile); got != c.want {
			t.Errorf("%s: CapsuleTouchesRectangle(%v, %v, %v) = %v, want %v", c.name, c.a, c.b, c.radius, got, c.want)
		}
		// The test is symmetric in the segment's direction.
		if got := CapsuleTouchesRectangle(c.b, c.a, c.radius, tile); got != c.want {
			t.Errorf("%s reversed: got %v, want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./geometry/ -run TestCapsuleTouchesRectangle`
Expected: FAIL, `undefined: CapsuleTouchesRectangle`.

- [ ] **Step 3: Implement**

`geometry/geometry_capsule.go`:

```go
package geometry

import "math"

// CapsuleTouchesRectangle reports whether the capsule of radius around the
// segment from a to b, the shape a circle sweeps when moved along it, touches
// the closed rectangle r: whether some point of r lies within radius of the
// segment. Touching counts, so a capsule whose edge meets r exactly touches
// it. A zero-length segment is a circle.
//
// The test is exact. Two disjoint convex shapes are closest at a vertex of one
// of them, so it checks both ends of the segment against r, the corners of r
// against the segment, and whether the segment crosses an edge of r.
func CapsuleTouchesRectangle(a, b Vector2, radius float64, r Rectangle) bool {
	r2 := radius * radius
	if pointRectangleDistanceSquared(a, r) <= r2 || pointRectangleDistanceSquared(b, r) <= r2 {
		return true
	}

	corners := [4]Vector2{r.Min, NewVector2(r.Max.X(), r.Min.Y()), r.Max, NewVector2(r.Min.X(), r.Max.Y())}
	for _, corner := range corners {
		if pointSegmentDistanceSquared(corner, a, b) <= r2 {
			return true
		}
	}
	for i := range corners {
		if segmentsIntersect(a, b, corners[i], corners[(i+1)%len(corners)]) {
			return true
		}
	}
	return false
}

// pointRectangleDistanceSquared returns the squared distance from p to the
// closest point of r, zero when p is inside.
func pointRectangleDistanceSquared(p Vector2, r Rectangle) float64 {
	dx := math.Max(math.Max(r.Min.X()-p.X(), 0), p.X()-r.Max.X())
	dy := math.Max(math.Max(r.Min.Y()-p.Y(), 0), p.Y()-r.Max.Y())
	return dx*dx + dy*dy
}

// pointSegmentDistanceSquared returns the squared distance from p to the
// closest point of the segment from a to b.
func pointSegmentDistanceSquared(p, a, b Vector2) float64 {
	abx, aby := b.X()-a.X(), b.Y()-a.Y()
	apx, apy := p.X()-a.X(), p.Y()-a.Y()
	length2 := abx*abx + aby*aby
	dot := apx*abx + apy*aby
	if length2 == 0 || dot <= 0 {
		return apx*apx + apy*apy
	}
	if dot >= length2 {
		bpx, bpy := p.X()-b.X(), p.Y()-b.Y()
		return bpx*bpx + bpy*bpy
	}
	cross := apx*aby - apy*abx
	return cross * cross / length2
}

// orientation reports which side of the line through a and b the point c is
// on: 1 for counter-clockwise, -1 for clockwise, 0 for on the line.
func orientation(a, b, c Vector2) int {
	v := (b.X()-a.X())*(c.Y()-a.Y()) - (b.Y()-a.Y())*(c.X()-a.X())
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// withinBounds reports whether p lies in the bounding box of the segment from
// a to b, which for a p on the segment's line means on the segment.
func withinBounds(a, p, b Vector2) bool {
	return p.X() >= math.Min(a.X(), b.X()) && p.X() <= math.Max(a.X(), b.X()) &&
		p.Y() >= math.Min(a.Y(), b.Y()) && p.Y() <= math.Max(a.Y(), b.Y())
}

// segmentsIntersect reports whether the segments ab and cd share a point,
// touching and collinear overlap included.
func segmentsIntersect(a, b, c, d Vector2) bool {
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	if o1*o2 < 0 && o3*o4 < 0 {
		return true
	}
	return (o1 == 0 && withinBounds(a, c, b)) ||
		(o2 == 0 && withinBounds(a, d, b)) ||
		(o3 == 0 && withinBounds(c, a, d)) ||
		(o4 == 0 && withinBounds(c, b, d))
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./geometry/`
Expected: PASS.

- [ ] **Step 5: Document**

In `geometry/doc.go`, add a sentence to the package overview naming `CapsuleTouchesRectangle` as the exact test of whether a circle swept along a segment touches a rectangle, used by movement to test straight walks against blocked tiles. Keep the file's existing style.

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add geometry/
git commit -m "Add an exact capsule against rectangle test" -m "<trailer lines from Global Constraints>"
```

---

### Task 2: A* for round bodies

**Files:**
* Modify: `pathfinding/astar.go` (`canMoveDiagonally`, `FindPath`, new `FindBodyPath`, new unexported `findPath`)
* Create: `pathfinding/astar_body_test.go`
* Modify: `pathfinding/doc.go`

**Interfaces:**
* Consumes: existing `TerrainProvider`, `Coord`, `Heuristic`, `FindPath`.
* Produces: `func FindBodyPath(terrain TerrainProvider, start, goal Coord, maxExpansions int, heuristic Heuristic) (path []Coord, expanded int)`. Like `FindPath`, the path includes both `start` and `goal`.

- [ ] **Step 1: Write the failing test**

`pathfinding/astar_body_test.go` (the package's tests use `newMockTerrain`, `setWalkable` and `testMaxExpansions` from `astar_test.go`; `newMockTerrain` starts every tile unwalkable, hence `openMockTerrain`):

```go
package pathfinding

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func openMockTerrain(width, height int) *mockTerrain {
	terrain := newMockTerrain(width, height)
	for y := range height {
		for x := range width {
			terrain.setWalkable(x, y, true)
		}
	}
	return terrain
}

func TestFindBodyPathDoesNotCutABlockedCorner(t *testing.T) {
	terrain := openMockTerrain(10, 10)
	// Stepping from (4, 5) to (5, 4) passes the corner (5, 5), whose tile is
	// blocked; (4, 4), the other tile beside the step, stays open.
	terrain.setWalkable(5, 5, false)

	pointPath, _ := FindPath(terrain, Coord{4, 5}, Coord{5, 4}, nil, testMaxExpansions, nil)
	require.Equal(t, []Coord{{4, 5}, {5, 4}}, pointPath, "FindPath still cuts the corner")

	bodyPath, _ := FindBodyPath(terrain, Coord{4, 5}, Coord{5, 4}, testMaxExpansions, nil)
	require.Equal(t, []Coord{{4, 5}, {4, 4}, {5, 4}}, bodyPath)
}

func TestFindBodyPathTakesAnOpenDiagonal(t *testing.T) {
	terrain := openMockTerrain(10, 10)

	path, _ := FindBodyPath(terrain, Coord{1, 1}, Coord{3, 3}, testMaxExpansions, nil)

	require.Equal(t, []Coord{{1, 1}, {2, 2}, {3, 3}}, path)
}

func TestFindBodyPathIgnoresNothingButTerrain(t *testing.T) {
	terrain := openMockTerrain(10, 10)
	terrain.setWalkable(7, 7, false)

	path, expanded := FindBodyPath(terrain, Coord{0, 0}, Coord{7, 7}, testMaxExpansions, nil)
	require.Nil(t, path, "unwalkable goal")
	require.Zero(t, expanded)

	path, _ = FindBodyPath(terrain, Coord{0, 0}, Coord{0, 0}, testMaxExpansions, nil)
	require.Nil(t, path, "start equals goal")
}

func TestFindBodyPathNeverCutsCornersOnRandomMaps(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := range 200 {
		terrain := openMockTerrain(12, 12)
		for range 30 {
			terrain.setWalkable(rng.IntN(12), rng.IntN(12), false)
		}
		start := Coord{rng.IntN(12), rng.IntN(12)}
		goal := Coord{rng.IntN(12), rng.IntN(12)}
		terrain.setWalkable(start.X, start.Y, true)

		path, _ := FindBodyPath(terrain, start, goal, testMaxExpansions, nil)
		for i := 1; i < len(path); i++ {
			prev, curr := path[i-1], path[i]
			dx, dy := curr.X-prev.X, curr.Y-prev.Y
			if dx == 0 || dy == 0 {
				continue
			}
			if !terrain.IsWalkable(prev.X+dx, prev.Y) || !terrain.IsWalkable(prev.X, prev.Y+dy) {
				t.Fatalf("trial %d: step %v to %v cuts a blocked corner", trial, prev, curr)
			}
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pathfinding/ -run FindBodyPath`
Expected: FAIL, `undefined: FindBodyPath`.

- [ ] **Step 3: Implement**

In `pathfinding/astar.go`:

1. Give `canMoveDiagonally` a `strictCorners bool` parameter. Its doc becomes: "canMoveDiagonally checks if diagonal movement is allowed from one coord to another. Diagonal movement is allowed if at least one adjacent cardinal path is traversable, or both of them when strictCorners is set." Its last line becomes:

```go
	if strictCorners {
		return walkable1 && walkable2
	}
	return walkable1 || walkable2
```

2. Rename the body of `FindPath` to an unexported function with one extra trailing parameter, and make `FindPath` call it. Keep `FindPath`'s doc comment exactly where it is:

```go
func FindPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic) (path []Coord, expanded int) {
	return findPath(terrain, start, goal, isOccupied, maxExpansions, heuristic, false)
}

// FindBodyPath finds a path for a round body, with the search FindPath runs
// and two differences. A diagonal step is allowed only when both tiles beside
// it are walkable: the segment between two diagonal tile centres runs through
// their shared corner, so a body of any radius cannot pass a blocked corner
// there. And reservations play no part, since bodies do not route around each
// other. The budget, the heuristic, the quick rejections and what expanded
// reports are as FindPath documents.
func FindBodyPath(terrain TerrainProvider, start, goal Coord, maxExpansions int, heuristic Heuristic) (path []Coord, expanded int) {
	return findPath(terrain, start, goal, nil, maxExpansions, heuristic, true)
}

// findPath is FindPath and FindBodyPath's shared search; strictCorners selects
// FindBodyPath's diagonal rule.
func findPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, maxExpansions int, heuristic Heuristic, strictCorners bool) (path []Coord, expanded int) {
	// ... the former body of FindPath, unchanged except for the diagonal check:
	//	if !isCardinalDirection(dir.X, dir.Y) && !canMoveDiagonally(terrain, current.coord, neighbor, strictCorners) {
}
```

3. Update every other caller of `canMoveDiagonally` in the package (search with `grep -n canMoveDiagonally pathfinding/*.go`) to pass `false`, so their behaviour is unchanged.

- [ ] **Step 4: Run the package tests**

Run: `go test ./pathfinding/`
Expected: PASS, including every existing `FindPath` test unchanged.

- [ ] **Step 5: Document**

In `pathfinding/doc.go`, add a paragraph after the heuristics list: "FindBodyPath is the same search for a round body: it never steps diagonally past a blocked tile, since a body cannot squeeze through the corner, and it ignores reservations, since bodies do not route around each other. Continuous movement in the motion package routes with it."

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add pathfinding/
git commit -m "Add a corner-safe A* search for round bodies" -m "<trailer lines>"
```

---

### Task 3: Straight walk test

**Files:**
* Create: `motion/motion_walk.go`
* Create: `motion/motion_walk_test.go`
* Modify: `motion/doc.go`

**Interfaces:**
* Consumes: `geometry.CapsuleTouchesRectangle` (Task 1), `pathfinding.TerrainProvider`, the test helper `testTerrain{width, height int; blocked map[tilemap.TileCoord]bool}` in `motion/motion_path_test.go`.
* Produces: `func WalkIsClear(terrain pathfinding.TerrainProvider, from, to geometry.Vector2, radius float64) bool`.

- [ ] **Step 1: Write the failing tests**

`motion/motion_walk_test.go`:

```go
package motion

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func v(x, y float64) geometry.Vector2 { return geometry.NewVector2(x, y) }

// oneBlockedTile is a 10x10 map whose only blocked tile is (5, 5), the box
// [5, 6] x [5, 6].
func oneBlockedTile() *testTerrain {
	return &testTerrain{width: 10, height: 10, blocked: map[tilemap.TileCoord]bool{{X: 5, Y: 5}: true}}
}

func TestWalkIsClear_BlockedCorners(t *testing.T) {
	terrain := oneBlockedTile()
	// Each pair of legs passes one corner of the blocked tile, first at
	// 0.4/sqrt(2) = 0.283 (clear of a 0.25 body), then at 0.3/sqrt(2) = 0.212
	// (touching). Both ends of every leg stand well clear of the tile.
	cases := []struct {
		name     string
		from, to geometry.Vector2
		want     bool
	}{
		{"corner (5, 5) clear", v(4.1, 5.5), v(5.5, 4.1), true},
		{"corner (5, 5) touching", v(4.15, 5.55), v(5.55, 4.15), false},
		{"corner (6, 5) clear", v(5.5, 4.1), v(6.9, 5.5), true},
		{"corner (6, 5) touching", v(5.45, 4.15), v(6.85, 5.55), false},
		{"corner (6, 6) clear", v(5.5, 6.9), v(6.9, 5.5), true},
		{"corner (6, 6) touching", v(5.45, 6.85), v(6.85, 5.45), false},
		{"corner (5, 6) clear", v(4.1, 5.5), v(5.5, 6.9), true},
		{"corner (5, 6) touching", v(4.15, 5.45), v(5.55, 6.85), false},
		// A diagonal step between two tile centres runs through the shared
		// corner of the tiles beside it: the corner cut FindBodyPath refuses.
		{"diagonal step past the corner", v(4.5, 5.5), v(5.5, 4.5), false},
		{"cardinal step beside the tile", v(4.5, 4.5), v(6.5, 4.5), true},
	}
	for _, c := range cases {
		if got := WalkIsClear(terrain, c.from, c.to, 0.25); got != c.want {
			t.Errorf("%s: WalkIsClear(%v, %v) = %v, want %v", c.name, c.from, c.to, got, c.want)
		}
		if got := WalkIsClear(terrain, c.to, c.from, 0.25); got != c.want {
			t.Errorf("%s reversed: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWalkIsClear_OneTileHole(t *testing.T) {
	blocked := map[tilemap.TileCoord]bool{}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				blocked[tilemap.TileCoord{X: 5 + dx, Y: 5 + dy}] = true
			}
		}
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	centre := v(5.5, 5.5)

	if !WalkIsClear(terrain, centre, centre, 0.25) {
		t.Error("a 0.25 body should fit at the centre of a one-tile hole")
	}
	if !WalkIsClear(terrain, centre, v(5.625, 5.5), 0.25) {
		t.Error("a walk ending with the body's edge at x = 5.875 should be clear")
	}
	if WalkIsClear(terrain, centre, v(5.75, 5.5), 0.25) {
		t.Error("a walk ending with the body's edge exactly on the wall should be blocked")
	}
	if WalkIsClear(terrain, centre, v(5.8, 5.5), 0.25) {
		t.Error("a walk carrying the body into the hole's wall should be blocked")
	}
}

func TestWalkIsClear_OutOfBoundsIsBlocked(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}

	if WalkIsClear(terrain, v(0.5, 0.5), v(0.25, 0.5), 0.25) {
		t.Error("a body whose edge reaches x = 0 exactly touches the out-of-bounds tile and should be blocked")
	}
	if !WalkIsClear(terrain, v(0.5, 0.5), v(0.375, 0.5), 0.25) {
		t.Error("a body whose edge stays at x = 0.125 should be clear")
	}
}

func TestWalkIsClear_BodyAlreadyTouchingCannotWalk(t *testing.T) {
	terrain := oneBlockedTile()

	// (4.9, 5.5) is 0.1 from the blocked tile's west edge.
	if WalkIsClear(terrain, v(4.9, 5.5), v(3.5, 5.5), 0.25) {
		t.Error("a body starting within its radius of a blocked tile should not be able to walk")
	}
}

func TestWalkIsClear_MatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for trial := range 3000 {
		terrain := &testTerrain{width: 16, height: 16, blocked: map[tilemap.TileCoord]bool{}}
		for range 40 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(16), Y: rng.IntN(16)}] = true
		}
		// Half the points sit on tile boundaries or quarter marks, to catch
		// rounding at the column and row limits.
		point := func() geometry.Vector2 {
			if trial%2 == 0 {
				return v(float64(rng.IntN(64))/4, float64(rng.IntN(64))/4)
			}
			return v(rng.Float64()*16, rng.Float64()*16)
		}
		from, to := point(), point()
		radius := float64(rng.IntN(5)) / 8 // 0 to 0.5 in eighths

		want := bruteForceWalkIsClear(terrain, from, to, radius)
		if got := WalkIsClear(terrain, from, to, radius); got != want {
			t.Fatalf("trial %d: WalkIsClear(%v, %v, %v) = %v, brute force says %v", trial, from, to, radius, got, want)
		}
	}
}

// bruteForceWalkIsClear tests every tile of the capsule's bounding box, grown
// by a tile on every side.
func bruteForceWalkIsClear(terrain *testTerrain, from, to geometry.Vector2, radius float64) bool {
	minX := int(math.Floor(math.Min(from.X(), to.X())-radius)) - 1
	maxX := int(math.Floor(math.Max(from.X(), to.X())+radius)) + 1
	minY := int(math.Floor(math.Min(from.Y(), to.Y())-radius)) - 1
	maxY := int(math.Floor(math.Max(from.Y(), to.Y())+radius)) + 1
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				continue
			}
			tile := geometry.NewRectangleFromPoints(x, y, x+1, y+1)
			if geometry.CapsuleTouchesRectangle(from, to, radius, tile) {
				return false
			}
		}
	}
	return true
}

func TestWalkIsClear_PanicsOnNegativeRadius(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for a negative radius")
		}
	}()
	WalkIsClear(&testTerrain{width: 10, height: 10}, v(1, 1), v(2, 2), -0.1)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./motion/ -run WalkIsClear`
Expected: FAIL, `undefined: WalkIsClear`.

- [ ] **Step 3: Implement**

`motion/motion_walk.go`:

```go
package motion

import (
	"fmt"
	"math"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
)

// walkScanMargin widens the tile rows and columns WalkIsClear considers, so
// that floating-point rounding in the slab arithmetic never skips a tile the
// capsule touches exactly on its boundary. Every tile it adds is still tested
// exactly, so the margin costs a few extra checks and changes no answer.
const walkScanMargin = 1e-9

// WalkIsClear reports whether a round body of radius can walk in a straight
// line from from to to over terrain: whether the capsule the body sweeps
// touches no tile that is out of bounds or not walkable. Touching counts as
// blocked. The capsule includes the body at both ends, so a body already
// touching a blocked tile cannot walk anywhere, and WalkIsClear(terrain, p, p,
// radius) tells whether a body fits at p.
//
// It visits the tile columns the capsule spans and, in each, only the rows the
// capsule can reach there, so its cost grows with the length of the walk
// rather than with the area of its bounding box. It panics on a negative
// radius.
func WalkIsClear(terrain pathfinding.TerrainProvider, from, to geometry.Vector2, radius float64) bool {
	if radius < 0 {
		panic(fmt.Sprintf("testing walk from %v to %v: radius must not be negative, got %v", from, to, radius))
	}

	reach := radius + walkScanMargin
	minX := int(math.Floor(math.Min(from.X(), to.X()) - reach))
	maxX := int(math.Floor(math.Max(from.X(), to.X()) + reach))
	for x := minX; x <= maxX; x++ {
		low, high := segmentYRange(from, to, float64(x)-reach, float64(x+1)+reach)
		for y := int(math.Floor(low - reach)); y <= int(math.Floor(high+reach)); y++ {
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				continue
			}
			tile := geometry.NewRectangleFromPoints(x, y, x+1, y+1)
			if geometry.CapsuleTouchesRectangle(from, to, radius, tile) {
				return false
			}
		}
	}
	return true
}

// segmentYRange returns the lowest and highest y of the segment from a to b
// over the part of it whose x lies in [x0, x1]. The capsule's points in the
// tile column [x0 + reach, x1 - reach] lie within the radius of that part.
// When the slab misses the segment by a rounding margin, the range collapses
// onto the nearest end, which is still a safe place to look.
func segmentYRange(a, b geometry.Vector2, x0, x1 float64) (low, high float64) {
	if a.X() == b.X() {
		return math.Min(a.Y(), b.Y()), math.Max(a.Y(), b.Y())
	}
	lo := math.Max(x0, math.Min(a.X(), b.X()))
	hi := math.Min(x1, math.Max(a.X(), b.X()))
	if lo > hi {
		lo = hi
	}
	yAt := func(x float64) float64 {
		t := (x - a.X()) / (b.X() - a.X())
		return a.Y() + t*(b.Y()-a.Y())
	}
	y0, y1 := yAt(lo), yAt(hi)
	return math.Min(y0, y1), math.Max(y0, y1)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./motion/ -run WalkIsClear`
Expected: PASS. If the brute-force test fails, print the case and compare which tile the scan skipped; do not loosen the brute force.

- [ ] **Step 5: Document**

In `motion/doc.go`, add a paragraph at the end (Tasks 4 to 8 extend it; write it so later tasks add sentences to it):

```go
// Continuous movement lets a body stand and walk anywhere rather than hop
// between tile centres. WalkIsClear tests whether a round body can walk
// straight from one point to another without touching a blocked tile.
```

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add motion/motion_walk.go motion/motion_walk_test.go motion/doc.go
git commit -m "Add a straight walk test for round bodies" -m "<trailer lines>"
```

---

### Task 4: Pluggable reservations and cancelling a move

**Files:**
* Create: `motion/motion_occupancy.go`
* Create: `motion/motion_occupancy_test.go`
* Modify: `tilemap/tilemap.go` (three methods on `TileOccupancyManager`)
* Create: `tilemap/tilemap_occupancy_test.go`
* Modify: `motion/motion_system.go` (the `Occupancy` field)
* Modify: `motion/motion_move.go` (`MoveEntity`, new `CancelMove`)
* Modify: `motion/motion_path.go` (`CanReachTile`, `CanReach`, `FindTilePath`)
* Modify: `motion/motion_towards.go` (`canEndPathOnTile`)
* Modify (mechanical): `motion/motion_system_test.go`, `motion/motion_path_test.go`, `motion/motion_move_test.go`, `motion/motion_towards_test.go`, `motion/motion_towards_bench_test.go`
* Modify: `motion/doc.go`, `tilemap/doc.go`

**Interfaces:**
* Consumes: `tilemap.TileOccupancyManager` (`GetOccupant`, `SetOccupant`, `ClearOccupant`), `tilemap.WorldPositionToTile`, `tilemap.TileToWorldPosition`.
* Produces:
  * `type Occupancy interface { Available(id ecs.EntityId, position geometry.Vector2) bool; Claim(id ecs.EntityId, from, destination geometry.Vector2) bool; Stop(id ecs.EntityId, destination, position geometry.Vector2) }` in `motion`.
  * `System.Occupancy Occupancy` (field type changes; name unchanged).
  * `func (tom *TileOccupancyManager) Available(id ecs.EntityId, position geometry.Vector2) bool`, `Claim(id ecs.EntityId, from, destination geometry.Vector2) bool`, `Stop(id ecs.EntityId, destination, position geometry.Vector2)`.
  * `func (s *System) CancelMove(id ecs.EntityId) bool`.
  * Test helper `func ledgerOf(s *System) *tilemap.TileOccupancyManager` in `motion/motion_system_test.go`.

- [ ] **Step 1: Write the failing tile ledger tests**

`tilemap/tilemap_occupancy_test.go`:

```go
package tilemap

import (
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

func twoEntities() (ecs.EntityId, ecs.EntityId) {
	w := ecs.NewWorld()
	return w.NewEntity(), w.NewEntity()
}

func TestTileOccupancyManager_Available(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	tom.SetOccupant(TileCoord{X: 1, Y: 1}, other)
	tom.SetOccupant(TileCoord{X: 2, Y: 2}, id)

	if !tom.Available(id, geometry.NewVector2(0.2, 0.9)) {
		t.Error("a free tile should be available")
	}
	if tom.Available(id, geometry.NewVector2(1.9, 1.1)) {
		t.Error("a tile another entity holds should not be available")
	}
	if !tom.Available(id, geometry.NewVector2(2.5, 2.5)) {
		t.Error("a tile the entity holds itself should be available")
	}
}

func TestTileOccupancyManager_Claim(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	from, dest := geometry.NewVector2(0.5, 0.5), geometry.NewVector2(2.5, 0.5)
	tom.SetOccupant(TileCoord{X: 0, Y: 0}, id)

	if !tom.Claim(id, from, dest) {
		t.Fatal("claiming a free tile should succeed")
	}
	if tom.IsOccupied(TileCoord{X: 0, Y: 0}) {
		t.Error("claiming should release the origin tile")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != id {
		t.Error("claiming should reserve the destination tile")
	}

	if tom.Claim(other, geometry.NewVector2(3.5, 0.5), dest) {
		t.Error("claiming a tile another entity holds should fail")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != id {
		t.Error("a refused claim should change nothing")
	}
}

func TestTileOccupancyManager_ClaimClearsTheOriginWhoeverHoldsIt(t *testing.T) {
	// MoveEntity has always released the tile the entity stands on without
	// checking who holds it; Claim keeps that.
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	tom.SetOccupant(TileCoord{X: 0, Y: 0}, other)

	tom.Claim(id, geometry.NewVector2(0.5, 0.5), geometry.NewVector2(1.5, 0.5))

	if tom.IsOccupied(TileCoord{X: 0, Y: 0}) {
		t.Error("expected the origin tile cleared")
	}
}

func TestTileOccupancyManager_Stop(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, _ := twoEntities()
	dest, position := geometry.NewVector2(2.5, 0.5), geometry.NewVector2(1.5, 0.5)
	tom.SetOccupant(TileCoord{X: 2, Y: 0}, id)

	tom.Stop(id, dest, position)

	if tom.IsOccupied(TileCoord{X: 2, Y: 0}) {
		t.Error("stopping should release the destination the entity held")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 1, Y: 0}); occupant != id {
		t.Error("stopping should claim the tile the entity stands on")
	}
}

func TestTileOccupancyManager_StopLeavesOtherEntitiesAlone(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	dest, position := geometry.NewVector2(2.5, 0.5), geometry.NewVector2(1.5, 0.5)
	tom.SetOccupant(TileCoord{X: 2, Y: 0}, other)
	tom.SetOccupant(TileCoord{X: 1, Y: 0}, other)

	tom.Stop(id, dest, position)

	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != other {
		t.Error("stopping should not release a destination another entity holds")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 1, Y: 0}); occupant != other {
		t.Error("stopping should not take a tile another entity holds")
	}
}
```

Check the top of `tilemap/tilemap_test.go`: it carries `//go:build !race`. The new file does not need it.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tilemap/ -run TileOccupancyManager_`
Expected: FAIL, `tom.Available undefined`.

- [ ] **Step 3: Implement the tile ledger methods**

Append to `tilemap/tilemap.go`:

```go
// Available reports whether id could claim the tile holding position: the tile
// is unreserved or already id's.
func (tom *TileOccupancyManager) Available(id ecs.EntityId, position geometry.Vector2) bool {
	occupant, occupied := tom.occupancy[WorldPositionToTile(position)]
	return !occupied || occupant == id
}

// Claim reserves the tile holding destination for id as it sets off from from,
// and reports false, changing nothing, when another entity holds that tile.
// The tile holding from is cleared whoever holds it, which is how a move has
// always released its origin, so redirecting an entity mid-move strands the
// old destination's reservation and can clear a tile the entity never held.
func (tom *TileOccupancyManager) Claim(id ecs.EntityId, from, destination geometry.Vector2) bool {
	if !tom.Available(id, destination) {
		return false
	}
	tom.ClearOccupant(WorldPositionToTile(from))
	tom.SetOccupant(WorldPositionToTile(destination), id)
	return true
}

// Stop records that id halted at position on its way to destination: the
// destination tile is released if id holds it, and the tile holding position
// becomes id's unless another entity holds it.
func (tom *TileOccupancyManager) Stop(id ecs.EntityId, destination, position geometry.Vector2) {
	destinationTile := WorldPositionToTile(destination)
	if occupant, occupied := tom.occupancy[destinationTile]; occupied && occupant == id {
		tom.ClearOccupant(destinationTile)
	}
	if tom.Available(id, position) {
		tom.SetOccupant(WorldPositionToTile(position), id)
	}
}
```

Run: `go test ./tilemap/`
Expected: PASS.

- [ ] **Step 4: Add the interface and switch the System over**

`motion/motion_occupancy.go`:

```go
package motion

import (
	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// Occupancy records where each body stands or is headed, and refuses a move
// whose end another body holds. System.Occupancy reserves through it.
// tilemap.TileOccupancyManager reserves whole tiles, one entity per tile;
// tilemap.CircleReservations reserves a circle per body, for bodies that stand
// anywhere.
type Occupancy interface {
	// Available reports whether id could claim position: nothing another
	// entity holds is there.
	Available(id ecs.EntityId, position geometry.Vector2) bool

	// Claim reserves destination for id as it sets off from from, releasing
	// what id held at from. It reports false, and changes nothing, when
	// another entity holds destination.
	Claim(id ecs.EntityId, from, destination geometry.Vector2) bool

	// Stop records that id halted at position before reaching destination: it
	// drops id's claim on destination and claims position.
	Stop(id ecs.EntityId, destination, position geometry.Vector2)
}

var _ Occupancy = (*tilemap.TileOccupancyManager)(nil)

// tileLedger is the per-tile lookup the tile route helpers use to route around
// reserved tiles. A tilemap.TileOccupancyManager provides it; an Occupancy
// without it leaves those helpers ignoring bodies.
type tileLedger interface {
	GetOccupant(tile tilemap.TileCoord) (ecs.EntityId, bool)
}

// ledger returns the System's Occupancy as a per-tile ledger, when it is
// one.
func (s *System) ledger() (tileLedger, bool) {
	if s.Occupancy == nil {
		return nil, false
	}
	ledger, ok := s.Occupancy.(tileLedger)
	return ledger, ok
}
```

In `motion/motion_system.go`, replace the `Occupancy` field and its comment:

```go
	// Occupancy, when non-nil, records reservations: MoveEntity refuses a
	// destination another entity holds and moves the entity's claim as it sets
	// off, and CancelMove settles the claim where the entity stopped. A
	// tilemap.TileOccupancyManager reserves whole tiles, and the tile route
	// helpers (FindTilePath and everything built on it) route around its
	// reserved tiles. Any other Occupancy, such as tilemap.CircleReservations,
	// is ignored by those helpers.
	Occupancy Occupancy
```

In `motion/motion_move.go`, replace `MoveEntity`'s occupancy handling. The block from `// Refuse destinations reserved by another entity.` through the at-destination `return` becomes:

```go
	if s.Occupancy != nil && !s.Occupancy.Claim(id, sc.Position, destination) {
		return MoveStart{Outcome: MoveOutcomeDestinationOccupied, Destination: destination}
	}

	if sc.Position == destination {
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: destination}
	}
```

and delete the later `if s.Occupancy != nil { s.Occupancy.SetOccupant(...) }` block before the final `return`. Update the `MoveEntity` doc comment's first paragraph to: "When the System has an Occupancy, the destination must be available to the entity, and Occupancy.Claim moves the entity's reservation to it as the move starts." Keep the rest of the comment.

Add `CancelMove` to `motion/motion_move.go` after `MoveEntity`:

```go
// CancelMove stops id's in-flight move where the body now stands, and reports
// whether there was one. It removes the Movement and, when the System has an
// Occupancy, moves the entity's claim from the cancelled destination to its
// current position through Occupancy.Stop. With no move in flight it changes
// nothing. Cancel before displacing a body (a push, a teleport): an eased or
// timed move would otherwise pull it back on its next tick. An entity with a
// Movement must have a Spatial; CancelMove panics otherwise.
func (s *System) CancelMove(id ecs.EntityId) bool {
	mc, ok := s.Movements.Get(id)
	if !ok {
		return false
	}
	destination := mc.Destination
	s.Movements.Remove(id)

	if s.Occupancy != nil {
		sc, ok := s.Spatials.Get(id)
		if !ok {
			panic(fmt.Sprintf("cancelling move of entity %v: no Spatial component", id))
		}
		s.Occupancy.Stop(id, destination, sc.Position)
	}
	return true
}
```

In `motion/motion_path.go`, replace `CanReachTile` and `CanReach`:

```go
// CanReachTile reports whether entityId can move onto tile: the tile must be
// in bounds and walkable (always true when Terrain is nil) and its centre
// available to the entity (always true when Occupancy is nil).
func (s *System) CanReachTile(entityId ecs.EntityId, tile tilemap.TileCoord) bool {
	return s.CanReach(entityId, tilemap.TileToWorldPosition(tile))
}

// CanReach reports whether entityId can move to destination: the tile holding
// it must be in bounds and walkable (always true when Terrain is nil) and
// destination available to the entity (always true when Occupancy is nil). It
// checks only the destination, not the way there; use FindPathBetween or
// WalkIsClear for that.
func (s *System) CanReach(entityId ecs.EntityId, destination geometry.Vector2) bool {
	tile := tilemap.WorldPositionToTile(destination)
	if s.Terrain != nil && (!s.Terrain.IsInBounds(tile.X, tile.Y) || !s.Terrain.IsWalkable(tile.X, tile.Y)) {
		return false
	}
	return s.Occupancy == nil || s.Occupancy.Available(entityId, destination)
}
```

In `FindTilePath`, replace the `isOccupied` closure with:

```go
	var isOccupied pathfinding.OccupancyChecker
	if ledger, ok := s.ledger(); ok {
		isOccupied = func(coord pathfinding.Coord) bool {
			_, occupied := ledger.GetOccupant(tilemap.TileCoord{X: coord.X, Y: coord.Y})
			return occupied
		}
	}
```

(A nil checker and one that always reports false are equivalent in `FindPath`, which tests `isOccupied != nil` before every call.) Update `FindTilePath`'s doc: "routing around tiles reserved in a tile ledger Occupancy".

In `motion/motion_towards.go`, `canEndPathOnTile`, replace the `if s.Occupancy != nil { ... }` block with:

```go
	if ledger, ok := s.ledger(); ok {
		// Any reservation blocks the goal, including this entity's own:
		// pathfinding routes around reserved tiles without exception.
		if _, occupied := ledger.GetOccupant(tile); occupied {
			return false
		}
	}
```

- [ ] **Step 5: Migrate the tests mechanically**

Add to `motion/motion_system_test.go`, after `newTestSystem`:

```go
// ledgerOf returns the tile ledger a test installed as the System's Occupancy.
func ledgerOf(s *System) *tilemap.TileOccupancyManager {
	return s.Occupancy.(*tilemap.TileOccupancyManager)
}
```

Then replace every `s.Occupancy.<TileMethod>(` call in `motion/*_test.go` (`SetOccupant`, `GetOccupant`, `ClearOccupant`, `IsOccupied`) with `ledgerOf(s).<TileMethod>(`. Find them with `grep -n "Occupancy\.\(SetOccupant\|GetOccupant\|ClearOccupant\|IsOccupied\)" motion/*_test.go`. Assignments such as `s.Occupancy = tilemap.NewTileOccupancyManager()` stay as they are. Change nothing a test asserts.

Run: `go test ./motion/`
Expected: PASS, every existing test.

- [ ] **Step 6: Write the new motion tests**

`motion/motion_occupancy_test.go`:

```go
package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestCancelMove_SettlesTheTileReservation(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	s.Grid.AddEntity(id, geometry.NewVector2(0.5, 0.5))
	ledgerOf(s).SetOccupant(tilemap.TileCoord{X: 0, Y: 0}, id)
	s.MoveEntity(id, geometry.NewVector2(2.5, 0.5), MoveOptions{Speed: 1.0})
	s.Tick(time.Second) // now at (1.5, 0.5), tile (1, 0)

	if !s.CancelMove(id) {
		t.Fatal("expected CancelMove to report an in-flight move")
	}

	if s.Movements.Has(id) {
		t.Error("expected the Movement removed")
	}
	if ledgerOf(s).IsOccupied(tilemap.TileCoord{X: 2, Y: 0}) {
		t.Error("expected the cancelled destination released")
	}
	if occupant, _ := ledgerOf(s).GetOccupant(tilemap.TileCoord{X: 1, Y: 0}); occupant != id {
		t.Error("expected the tile the entity stopped on reserved for it")
	}
}

func TestCancelMove_NoMoveInFlight(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})

	if s.CancelMove(id) {
		t.Error("expected CancelMove to report no move")
	}
	if ledgerOf(s).IsOccupied(tilemap.TileCoord{X: 0, Y: 0}) {
		t.Error("expected CancelMove without a move to change nothing")
	}
}

func TestCancelMove_WithoutOccupancy(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	s.MoveEntity(id, geometry.NewVector2(2.5, 0.5), MoveOptions{Speed: 1.0})

	if !s.CancelMove(id) || s.Movements.Has(id) {
		t.Error("expected the move cancelled")
	}
}
```

Run: `go test ./motion/`
Expected: PASS.

- [ ] **Step 7: Document**

* `motion/doc.go`: in the System paragraph, after the sentence about `MoveEntity`, add: "System.Occupancy reserves through the Occupancy interface: a tilemap.TileOccupancyManager for tile-based games, or a tilemap.CircleReservations for bodies that stand anywhere. CancelMove stops a move where the body stands and settles its reservation there."
* `tilemap/doc.go`: after the `TileOccupancyManager` sentence, add: "Its Available, Claim and Stop methods let a motion.System reserve through it."

- [ ] **Step 8: Lint and commit**

```bash
task lint
git add motion/ tilemap/
git commit -m "Reserve moves through a pluggable Occupancy interface" -m "System.Occupancy becomes an interface the tile ledger satisfies, so tile-based games keep their behaviour; the tile route helpers still route around a tile ledger's reserved tiles. Add CancelMove to stop a move and settle its reservation." -m "<trailer lines>"
```

---

### Task 5: Circle reservations

**Files:**
* Create: `tilemap/tilemap_circles.go`
* Create: `tilemap/tilemap_circles_test.go`
* Modify: `motion/motion_occupancy.go` (compile-time assertion), `motion/motion_occupancy_test.go`
* Modify: `tilemap/doc.go`

**Interfaces:**
* Consumes: `tilemap.SpatialGrid` (`NewSpatialGrid`, `AddEntity`, `RemoveEntity`, `UpdateEntityPosition`, `GetRange`), `motion.Occupancy` (Task 4), `System.CancelMove` (Task 4).
* Produces:
  * `type CircleReservations struct` with `func NewCircleReservations(cellSize float64) *CircleReservations`,
  * `Place(id ecs.EntityId, position geometry.Vector2, radius float64)`, `Remove(id ecs.EntityId)`, `Reservation(id ecs.EntityId) (centre geometry.Vector2, radius float64, ok bool)`,
  * `Available(id ecs.EntityId, position geometry.Vector2) bool`, `Claim(id ecs.EntityId, from, destination geometry.Vector2) bool`, `Stop(id ecs.EntityId, destination, position geometry.Vector2)`.

- [ ] **Step 1: Write the failing tests**

`tilemap/tilemap_circles_test.go`:

```go
package tilemap

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

func TestCircleReservations_ConflictIsStrictlyLessThanTheSumOfRadii(t *testing.T) {
	c := NewCircleReservations(1.0)
	small, large := twoEntities()
	c.Place(small, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(large, geometry.NewVector2(5.0, 5.0), 0.5)

	if c.Claim(large, geometry.NewVector2(5.0, 5.0), geometry.NewVector2(0.7, 0.0)) {
		t.Error("centres 0.7 apart with radii 0.25 and 0.5 overlap; the claim should fail")
	}
	if centre, _, _ := c.Reservation(large); centre != geometry.NewVector2(5.0, 5.0) {
		t.Error("a refused claim should leave the reservation where it was")
	}
	if !c.Claim(large, geometry.NewVector2(5.0, 5.0), geometry.NewVector2(0.75, 0.0)) {
		t.Error("circles that only touch do not conflict; the claim should succeed")
	}
}

func TestCircleReservations_FindsALargeBodyFromASmallOnesQuery(t *testing.T) {
	c := NewCircleReservations(1.0)
	large, small := twoEntities()
	c.Place(large, geometry.NewVector2(0.0, 0.0), 2.0)
	c.Place(small, geometry.NewVector2(10.0, 10.0), 0.25)

	if c.Available(small, geometry.NewVector2(2.1, 0.0)) {
		t.Error("2.1 from a radius 2 circle is inside it for a radius 0.25 body")
	}
	if !c.Available(small, geometry.NewVector2(2.25, 0.0)) {
		t.Error("exactly the sum of radii away should be available")
	}
}

func TestCircleReservations_AvailableIgnoresTheEntitysOwnCircle(t *testing.T) {
	c := NewCircleReservations(1.0)
	id, _ := twoEntities()
	c.Place(id, geometry.NewVector2(0.0, 0.0), 0.25)

	if !c.Available(id, geometry.NewVector2(0.1, 0.0)) {
		t.Error("an entity's own reservation should not block it")
	}
}

func TestCircleReservations_StopRecordsWhereTheBodyIsEvenOnAnotherCircle(t *testing.T) {
	c := NewCircleReservations(1.0)
	a, b := twoEntities()
	c.Place(a, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(b, geometry.NewVector2(3.0, 0.0), 0.25)

	c.Stop(b, geometry.NewVector2(3.0, 0.0), geometry.NewVector2(0.1, 0.0))

	if centre, _, _ := c.Reservation(b); centre != geometry.NewVector2(0.1, 0.0) {
		t.Errorf("expected b reserved where it stopped, got %v", centre)
	}
}

func TestCircleReservations_PlaceAndRemove(t *testing.T) {
	c := NewCircleReservations(1.0)
	a, b := twoEntities()
	c.Place(a, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(b, geometry.NewVector2(3.0, 0.0), 0.25)

	c.Place(a, geometry.NewVector2(5.0, 5.0), 0.25)
	if !c.Available(b, geometry.NewVector2(0.0, 0.0)) {
		t.Error("placing an entity again should move it")
	}

	c.Remove(a)
	if _, _, ok := c.Reservation(a); ok {
		t.Error("expected a removed")
	}
	if !c.Available(b, geometry.NewVector2(5.0, 5.0)) {
		t.Error("a removed entity should block nothing")
	}
}

func TestCircleReservations_Panics(t *testing.T) {
	cases := map[string]func(c *CircleReservations){
		"negative radius": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Place(id, geometry.NewVector2(0.0, 0.0), -0.1)
		},
		"available for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Available(id, geometry.NewVector2(0.0, 0.0))
		},
		"claim for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Claim(id, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(1.0, 0.0))
		},
		"stop for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Stop(id, geometry.NewVector2(1.0, 0.0), geometry.NewVector2(0.0, 0.0))
		},
	}
	for name, call := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			call(NewCircleReservations(1.0))
		}()
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./tilemap/ -run CircleReservations`
Expected: FAIL, `undefined: NewCircleReservations`.

- [ ] **Step 3: Implement**

`tilemap/tilemap_circles.go`:

```go
package tilemap

import (
	"fmt"
	"math"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// CircleReservations reserves a circle of floor per body, each body with its
// own radius, at the end of its current move or where it stands. It satisfies
// motion.Occupancy, so a motion.System reserves moves through it. Two
// reservations conflict when their centres are closer than the sum of their
// radii; circles that only touch do not. Only where a move ends is checked:
// bodies pass through each other's reservations on the way.
//
// Lookups search a SpatialGrid of the reservation centres, separate from any
// grid of current positions, over the entity's radius plus the largest radius
// the table has held. That largest radius never shrinks, which can widen a
// search after a large body leaves but never misses a conflict.
type CircleReservations struct {
	grid      *SpatialGrid
	circles   map[ecs.EntityId]reservedCircle
	maxRadius float64
}

// reservedCircle is one entity's reservation.
type reservedCircle struct {
	centre geometry.Vector2
	radius float64
}

// NewCircleReservations returns an empty table whose grid has cells of
// cellSize tiles.
func NewCircleReservations(cellSize float64) *CircleReservations {
	return &CircleReservations{
		grid:    NewSpatialGrid(cellSize),
		circles: make(map[ecs.EntityId]reservedCircle),
	}
}

// Place reserves a circle of radius at position for id unconditionally, moving
// it if id already holds one. It is for spawning, teleports and loading saves;
// moves claim through Claim. It panics on a negative radius.
func (c *CircleReservations) Place(id ecs.EntityId, position geometry.Vector2, radius float64) {
	if radius < 0 {
		panic(fmt.Sprintf("placing entity %v in circle reservations: radius must not be negative, got %v", id, radius))
	}
	if old, ok := c.circles[id]; ok {
		c.grid.RemoveEntity(id, old.centre)
	}
	c.circles[id] = reservedCircle{centre: position, radius: radius}
	c.grid.AddEntity(id, position)
	c.maxRadius = math.Max(c.maxRadius, radius)
}

// Remove drops id's reservation, if it holds one.
func (c *CircleReservations) Remove(id ecs.EntityId) {
	old, ok := c.circles[id]
	if !ok {
		return
	}
	c.grid.RemoveEntity(id, old.centre)
	delete(c.circles, id)
}

// Reservation returns the centre and radius of id's reservation, and false
// when id holds none.
func (c *CircleReservations) Reservation(id ecs.EntityId) (centre geometry.Vector2, radius float64, ok bool) {
	held, ok := c.circles[id]
	return held.centre, held.radius, ok
}

// Available reports whether id's circle at position would conflict with no
// other entity's reservation. id must have been placed; Available panics
// otherwise, since the table cannot know its radius.
func (c *CircleReservations) Available(id ecs.EntityId, position geometry.Vector2) bool {
	own := c.mustHold(id, "checking reservation availability")
	return !c.conflicts(id, position, own.radius)
}

// Claim moves id's reservation to destination, and reports false, changing
// nothing, when it would conflict with another entity's. The origin needs no
// release, since an entity holds one circle. id must have been placed; Claim
// panics otherwise.
func (c *CircleReservations) Claim(id ecs.EntityId, _, destination geometry.Vector2) bool {
	own := c.mustHold(id, "claiming reservation")
	if c.conflicts(id, destination, own.radius) {
		return false
	}
	c.move(id, destination)
	return true
}

// Stop moves id's reservation to position, where it halted, unconditionally: a
// body stopped mid-move may stand on another's reservation, since bodies pass
// through each other, and the table records where it is. id must have been
// placed; Stop panics otherwise.
func (c *CircleReservations) Stop(id ecs.EntityId, _, position geometry.Vector2) {
	c.mustHold(id, "stopping reservation")
	c.move(id, position)
}

// mustHold returns id's reservation, panicking with the action being attempted
// when id was never placed.
func (c *CircleReservations) mustHold(id ecs.EntityId, action string) reservedCircle {
	held, ok := c.circles[id]
	if !ok {
		panic(fmt.Sprintf("%s for entity %v: entity was never placed", action, id))
	}
	return held
}

// conflicts reports whether a circle of radius at position overlaps any
// reservation other than id's own.
func (c *CircleReservations) conflicts(id ecs.EntityId, position geometry.Vector2, radius float64) bool {
	reach := radius + c.maxRadius
	area := geometry.NewRectangleFromPoints(position.X()-reach, position.Y()-reach, position.X()+reach, position.Y()+reach)
	for _, other := range c.grid.GetRange(area) {
		if other == id {
			continue
		}
		held := c.circles[other]
		if position.DistanceTo(held.centre) < radius+held.radius {
			return true
		}
	}
	return false
}

// move relocates id's reservation to position.
func (c *CircleReservations) move(id ecs.EntityId, position geometry.Vector2) {
	held := c.circles[id]
	c.grid.UpdateEntityPosition(id, held.centre, position)
	held.centre = position
	c.circles[id] = held
}
```

Run: `go test ./tilemap/`
Expected: PASS.

- [ ] **Step 4: Wire it to motion and test through the System**

In `motion/motion_occupancy.go`, below the tile ledger assertion, add:

```go
var _ Occupancy = (*tilemap.CircleReservations)(nil)
```

Append to `motion/motion_occupancy_test.go`:

```go
func TestMoveEntity_WithCircleReservations(t *testing.T) {
	s, w := newTestSystem()
	circles := tilemap.NewCircleReservations(1.0)
	s.Occupancy = circles
	mover, blocker := w.NewEntity(), w.NewEntity()
	s.Spatials.Add(mover, Spatial{Position: geometry.NewVector2(0.0, 0.0)})
	circles.Place(mover, geometry.NewVector2(0.0, 0.0), 0.25)
	circles.Place(blocker, geometry.NewVector2(2.0, 0.0), 0.25)

	refused := s.MoveEntity(mover, geometry.NewVector2(1.6, 0.0), MoveOptions{Speed: 1.0})
	if refused.Outcome != MoveOutcomeDestinationOccupied {
		t.Fatalf("a leg ending 0.4 from another body's 0.25 circle should be refused, got %+v", refused)
	}

	// A leg through the blocker's circle to a clear end is fine: bodies pass
	// through each other.
	started := s.MoveEntity(mover, geometry.NewVector2(3.0, 0.0), MoveOptions{Speed: 1.0})
	if !started.Started() {
		t.Fatalf("a leg passing through another body should start, got %+v", started)
	}
	if centre, _, _ := circles.Reservation(mover); centre != geometry.NewVector2(3.0, 0.0) {
		t.Errorf("expected the mover's circle at its leg end, got %v", centre)
	}

	s.Tick(1500 * time.Millisecond) // at (1.5, 0), inside the blocker's circle
	s.CancelMove(mover)
	if centre, _, _ := circles.Reservation(mover); centre != geometry.NewVector2(1.5, 0.0) {
		t.Errorf("expected the mover's circle where it stopped, got %v", centre)
	}
}

func TestCanReach_WithCircleReservationsUsesThePoint(t *testing.T) {
	s, w := newTestSystem()
	circles := tilemap.NewCircleReservations(1.0)
	s.Occupancy = circles
	id, other := w.NewEntity(), w.NewEntity()
	circles.Place(id, geometry.NewVector2(8.0, 8.0), 0.25)
	circles.Place(other, geometry.NewVector2(1.5, 1.5), 0.25)

	if s.CanReach(id, geometry.NewVector2(1.6, 1.5)) {
		t.Error("a point inside another body's reservation should not be reachable")
	}
	if !s.CanReach(id, geometry.NewVector2(1.1, 1.1)) {
		t.Error("a point in the same tile but clear of the other circle should be reachable")
	}
}
```

Run: `go test ./motion/`
Expected: PASS.

- [ ] **Step 5: Document**

In `tilemap/doc.go`, after the `SpatialGrid` paragraph, add: "CircleReservations reserves a circle of floor per body, each with its own radius, at the end of its current move or where it stands, for games whose bodies stand anywhere rather than on tile centres. It satisfies motion.Occupancy."

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add tilemap/ motion/
git commit -m "Add per-entity circle reservations" -m "<trailer lines>"
```

---

### Task 6: Legs with an explicit duration

**Files:**
* Modify: `motion/motion.go` (`Movement.Timed`, `Movement.Speed` doc, `ProcessMove`)
* Modify: `motion/motion_move.go` (`MoveOptions.Duration`, `MoveEntity`)
* Create: `motion/motion_timed_test.go`
* Modify: `motion/doc.go`

**Interfaces:**
* Consumes: `MoveEntity`, `ProcessMove`, `System.Tick`.
* Produces: `MoveOptions.Duration time.Duration`, `Movement.Timed bool`.

- [ ] **Step 1: Write the failing tests**

`motion/motion_timed_test.go`:

```go
package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
)

func TestMoveEntity_WithDuration(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	start := s.MoveEntity(id, geometry.NewVector2(3.0, 4.0), MoveOptions{Duration: 2 * time.Second})

	if !start.Started() || start.Duration != 2*time.Second || start.Distance != 5.0 {
		t.Fatalf("expected a 2s move over 5 tiles, got %+v", start)
	}
	mc, _ := s.Movements.Get(id)
	if !mc.Timed || mc.Total != 2*time.Second || mc.Speed != 2.5 {
		t.Errorf("expected a timed Movement of 2s at 2.5 tiles/s, got %+v", mc)
	}
}

func TestTimedMove_EndsExactlyAtItsDuration(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	dest := geometry.NewVector2(1.7, 1.3)
	s.MoveEntity(id, dest, MoveOptions{Duration: 2 * time.Second})

	s.Tick(1999 * time.Millisecond)
	if !s.Movements.Has(id) {
		t.Fatal("expected the move still in flight a millisecond before its end")
	}
	s.Tick(time.Millisecond)
	if s.Movements.Has(id) {
		t.Fatal("expected the move complete at its duration")
	}
	if sc, _ := s.Spatials.Get(id); sc.Position != dest {
		t.Errorf("expected the body exactly at %v, got %v", dest, sc.Position)
	}
}

func TestTimedMove_IsIndependentOfTickSlicing(t *testing.T) {
	from, dest := geometry.NewVector2(0.5, 0.5), geometry.NewVector2(1.7, 1.3)
	newMover := func() (*System, ecsEntity) {
		s, w := newTestSystem()
		id := w.NewEntity()
		s.Spatials.Add(id, Spatial{Position: from})
		s.MoveEntity(id, dest, MoveOptions{Duration: 1300 * time.Millisecond})
		return s, ecsEntity{id: id, world: w}
	}
	whole, a := newMover()
	sliced, b := newMover()

	whole.Tick(700 * time.Millisecond)
	for range 7 {
		sliced.Tick(100 * time.Millisecond)
	}
	wholePos, _ := whole.Spatials.Get(a.id)
	slicedPos, _ := sliced.Spatials.Get(b.id)
	if wholePos.Position != slicedPos.Position {
		t.Fatalf("mid-leg positions differ: one tick %v, seven ticks %v", wholePos.Position, slicedPos.Position)
	}

	whole.Tick(600 * time.Millisecond)
	for range 6 {
		sliced.Tick(100 * time.Millisecond)
	}
	if whole.Movements.Has(a.id) || sliced.Movements.Has(b.id) {
		t.Error("expected both moves complete at 1.3s")
	}
}

func TestMoveEntity_PanicsWithNeitherSpeedNorDuration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic with neither a speed nor a duration")
		}
	}()
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})
	s.MoveEntity(id, geometry.NewVector2(1.0, 0.0), MoveOptions{})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./motion/ -run "Timed|WithDuration|NeitherSpeed"`
Expected: FAIL, `unknown field Duration`.

- [ ] **Step 3: Implement**

In `motion/motion.go`, add to `Movement` after `Total`:

```go
	// Timed makes the move run on the parametric path whatever its Ease: the
	// position is a pure function of Start, Destination and Elapsed over
	// Total, so it comes out the same however game time is sliced into
	// ticks. MoveEntity sets it for a move started with a fixed
	// MoveOptions.Duration. False, the zero value, routes a CurveLinear move
	// to the incremental constant-speed path, as before Timed existed.
	Timed bool
```

In `Movement.Speed`'s comment, change "but has no effect on an eased move, whose Total was fixed when the move started" to "but has no effect on an eased or timed move, whose Total was fixed when the move started".

In `ProcessMove`, change the routing condition to:

```go
	if mc.Ease == easing.CurveLinear && !mc.Timed {
```

and in its doc comment, change "It routes constant-speed moves to ProcessMovement and eased moves to the parametric formula" to "It routes constant-speed moves to ProcessMovement, and eased or timed moves to the parametric formula".

In `motion/motion_move.go`, add to `MoveOptions` after `Speed`:

```go
	// Duration, when positive, fixes how long the move takes, and Speed is then
	// not needed: the move runs at distance divided by Duration. Such a move is
	// timed (see Movement.Timed), so its position is independent of how game
	// time is sliced into ticks. Zero leaves the duration to Speed. The tile
	// route helpers (MoveEntityTowards, MoveEntityTowardsArea) still require
	// a positive Speed.
	Duration time.Duration
```

Change `Speed`'s comment from "It must be positive; the move entry points panic otherwise." to "It must be positive unless Duration is set; the move entry points panic otherwise."

In `MoveEntity`, replace the speed check with:

```go
	if opts.Speed <= 0 && opts.Duration <= 0 {
		panic(fmt.Sprintf("moving entity %v: needs a positive speed or duration, got speed %v and duration %v", id, opts.Speed, opts.Duration))
	}
```

and replace the duration computation and the `mc.Speed`/`mc.Total` assignments with:

```go
	distance := sc.Position.DistanceTo(destination)
	speed := opts.Speed
	total := time.Duration(distance / speed * float64(time.Second))
	if opts.Duration > 0 {
		speed = distance / opts.Duration.Seconds()
		total = opts.Duration
	}
```

```go
	mc.Speed = speed
	...
	mc.Total = total
	mc.Timed = opts.Duration > 0
```

(Keep the other re-anchoring assignments in place; `total` is computed only after the at-destination return, as today.) Update `MoveEntity`'s doc: "opts.Speed must be positive" becomes "opts must carry a positive Speed or Duration".

Check the existing test `TestMoveEntity_PanicsOnNonPositiveSpeed` still passes: it passes no Duration, so it still panics.

- [ ] **Step 4: Run the tests**

Run: `go test ./motion/`
Expected: PASS.

- [ ] **Step 5: Document**

In `motion/doc.go`, in the first paragraph about the two paths, change "ProcessMove routes between them by the movement's easing.Curve" to "ProcessMove routes between them by the movement's easing.Curve and its Timed flag: a move started with a fixed MoveOptions.Duration is timed and runs on the parametric path even at constant speed".

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add motion/
git commit -m "Let a move take an explicit duration" -m "A move started with MoveOptions.Duration is timed: it runs on the parametric path even at constant speed, so its position does not depend on tick slicing. Movement.Timed defaults to false, so saves decode as before." -m "<trailer lines>"
```

---

### Task 7: Route between arbitrary points

**Files:**
* Create: `motion/motion_route.go`
* Create: `motion/motion_route_test.go`
* Modify: `motion/doc.go`
* Modify: `docs/superpowers/specs/2026-09-30-continuous-movement-design.md` (FindRoute signature, see Step 5)

**Interfaces:**
* Consumes: `pathfinding.FindBodyPath` (Task 2), `WalkIsClear` (Task 3), `System.Terrain`, `System.MaxPathExpansions`, `System.Heuristic`, `System.RecordPhase`, test helpers `testTerrain`, `testMaxPathExpansions`, `newTestSystem`, `v` (Task 3).
* Produces: `func (s *System) FindRoute(from, to geometry.Vector2) ([]geometry.Vector2, bool)`. The spec wrote it returning only a slice, nil for no route; this plan returns an explicit `ok`, since an empty route (already there) and no route must not hang on nil versus empty.

- [ ] **Step 1: Write the failing tests**

`motion/motion_route_test.go`:

```go
package motion

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func newRouteSystem(terrain *testTerrain) *System {
	s, _ := newTestSystem()
	s.Terrain = terrain
	s.MaxPathExpansions = testMaxPathExpansions
	return s
}

func TestFindRoute_Shapes(t *testing.T) {
	s := newRouteSystem(&testTerrain{width: 10, height: 10})
	cases := []struct {
		name     string
		from, to geometry.Vector2
		want     []geometry.Vector2
	}{
		{"off-centre to off-centre", v(1.2, 1.3), v(4.7, 1.6),
			[]geometry.Vector2{v(1.5, 1.5), v(2.5, 1.5), v(3.5, 1.5), v(4.5, 1.5), v(4.7, 1.6)}},
		{"from a centre", v(1.5, 1.5), v(3.7, 1.6),
			[]geometry.Vector2{v(2.5, 1.5), v(3.5, 1.5), v(3.7, 1.6)}},
		{"to a centre", v(1.2, 1.3), v(3.5, 1.5),
			[]geometry.Vector2{v(1.5, 1.5), v(2.5, 1.5), v(3.5, 1.5)}},
		{"within one tile", v(1.2, 1.3), v(1.8, 1.1),
			[]geometry.Vector2{v(1.5, 1.5), v(1.8, 1.1)}},
		{"within one tile from its centre", v(1.5, 1.5), v(1.8, 1.1),
			[]geometry.Vector2{v(1.8, 1.1)}},
		{"same point", v(1.2, 1.3), v(1.2, 1.3), []geometry.Vector2{}},
	}
	for _, c := range cases {
		got, ok := s.FindRoute(c.from, c.to)
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("%s: FindRoute(%v, %v) = %v, %v; want %v, true", c.name, c.from, c.to, got, ok, c.want)
		}
	}
}

func TestFindRoute_NoRoute(t *testing.T) {
	blocked := map[tilemap.TileCoord]bool{{X: 7, Y: 7}: true}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				blocked[tilemap.TileCoord{X: 2 + dx, Y: 2 + dy}] = true
			}
		}
	}
	s := newRouteSystem(&testTerrain{width: 10, height: 10, blocked: blocked})

	if route, ok := s.FindRoute(v(0.5, 5.5), v(7.5, 7.5)); ok || route != nil {
		t.Errorf("an unwalkable goal should have no route, got %v, %v", route, ok)
	}
	if route, ok := s.FindRoute(v(0.5, 5.5), v(2.5, 2.5)); ok || route != nil {
		t.Errorf("a sealed-off goal should have no route, got %v, %v", route, ok)
	}
}

func TestFindRoute_DoesNotCutABlockedCorner(t *testing.T) {
	s := newRouteSystem(oneBlockedTile())

	got, ok := s.FindRoute(v(4.5, 5.5), v(5.5, 4.5))

	want := []geometry.Vector2{v(4.5, 4.5), v(5.5, 4.5)}
	if !ok || !slices.Equal(got, want) {
		t.Errorf("expected the route around the corner %v, got %v, %v", want, got, ok)
	}
}

func TestFindRoute_IncludesTheGoalTileCentre(t *testing.T) {
	// The goal sits near the corner of its tile that touches the blocked tile
	// (4, 4). A straight walk from the west neighbour's centre to it grazes
	// that tile: the route that ends there stalls a body one tile short. The
	// goal tile's centre in between avoids it.
	s := newRouteSystem(&testTerrain{width: 10, height: 10, blocked: map[tilemap.TileCoord]bool{{X: 4, Y: 4}: true}})
	west, goal := v(4.5, 5.5), v(5.21, 5.01)
	const radius = 0.2
	if !WalkIsClear(s.Terrain, goal, goal, radius) {
		t.Fatal("test setup: the goal should be a legal position")
	}
	if WalkIsClear(s.Terrain, west, goal, radius) {
		t.Fatal("test setup: the straight walk from the west centre should graze the blocked tile")
	}

	got, ok := s.FindRoute(west, goal)

	want := []geometry.Vector2{v(5.5, 5.5), goal}
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v, %v", want, got, ok)
	}
	if !WalkIsClear(s.Terrain, west, got[0], radius) || !WalkIsClear(s.Terrain, got[0], got[1], radius) {
		t.Error("every step of the route should be walkable")
	}
}

func TestFindRoute_EveryStepIsWalkable(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	found := 0
	for trial := range 2000 {
		terrain := &testTerrain{width: 12, height: 12, blocked: map[tilemap.TileCoord]bool{}}
		for range 28 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(12), Y: rng.IntN(12)}] = true
		}
		s := newRouteSystem(terrain)
		radius := 0.05 + rng.Float64()*0.44
		from, to := legalPoint(rng, terrain, radius), legalPoint(rng, terrain, radius)

		route, ok := s.FindRoute(from, to)
		if !ok {
			continue
		}
		found++
		previous := from
		for i, point := range route {
			if !WalkIsClear(terrain, previous, point, radius) {
				t.Fatalf("trial %d, radius %v: step %d from %v to %v is not walkable (route %v)", trial, radius, i, previous, point, route)
			}
			previous = point
		}
	}
	if found < 500 {
		t.Fatalf("only %d of 2000 trials found a route; the test is not exercising routes", found)
	}
}

// legalPoint draws a random point where a body of radius fits.
func legalPoint(rng *rand.Rand, terrain *testTerrain, radius float64) geometry.Vector2 {
	for {
		p := v(rng.Float64()*float64(terrain.width), rng.Float64()*float64(terrain.height))
		if WalkIsClear(terrain, p, p, radius) {
			return p
		}
	}
}

func TestFindRoute_RecordsPhase(t *testing.T) {
	s := newRouteSystem(&testTerrain{width: 10, height: 10})
	var phases []string
	s.RecordPhase = func(name string, _ time.Duration) { phases = append(phases, name) }

	s.FindRoute(v(1.2, 1.3), v(4.7, 1.6))

	if !slices.Equal(phases, []string{"pathfinding"}) {
		t.Errorf("expected one pathfinding phase, got %v", phases)
	}
}

func TestFindRoute_PanicsWithoutConfiguration(t *testing.T) {
	for name, s := range map[string]*System{
		"no terrain":          {MaxPathExpansions: testMaxPathExpansions},
		"no expansion budget": {Terrain: &testTerrain{width: 10, height: 10}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			s.FindRoute(v(1.2, 1.3), v(4.7, 1.6))
		}()
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./motion/ -run FindRoute`
Expected: FAIL, `s.FindRoute undefined`.

- [ ] **Step 3: Implement**

`motion/motion_route.go`:

```go
package motion

import (
	"fmt"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
	"github.com/trancecode/vantage/tilemap"
)

// FindRoute returns the points a body walks through from from to to, for
// bodies that stand anywhere rather than on tile centres. In order: the centre
// of from's tile unless from is exactly on it, the centres of the tiles between
// found by pathfinding.FindBodyPath, the centre of to's tile unless to is
// exactly on it, and to itself. The route never cuts a blocked corner and
// ignores reservations, since bodies pass through each other.
//
// Both end tiles' centres are what make the route walkable in straight legs:
// for a body of radius under 0.5 whose from and to are legal positions
// (WalkIsClear of a point to itself), every step between consecutive points
// passes WalkIsClear, so TightenRoute always finds a leg. A route running from
// the last centre between straight to to could graze a blocked corner of to's
// tile and stall a body one tile short.
//
// It reports false, with a nil route, when to's tile is out of bounds or not
// walkable, or when the search found nothing within MaxPathExpansions. When
// from equals to the route is empty. Terrain and MaxPathExpansions must be
// set; FindRoute panics otherwise. The time spent is recorded under the
// "pathfinding" phase.
func (s *System) FindRoute(from, to geometry.Vector2) ([]geometry.Vector2, bool) {
	if s.RecordPhase != nil {
		defer func(start time.Time) { s.RecordPhase("pathfinding", time.Since(start)) }(time.Now())
	}
	if s.Terrain == nil {
		panic(fmt.Sprintf("finding route from %v to %v: System.Terrain is nil", from, to))
	}
	if s.MaxPathExpansions <= 0 {
		panic(fmt.Sprintf("finding route from %v to %v: MaxPathExpansions not configured", from, to))
	}

	if from == to {
		return []geometry.Vector2{}, true
	}

	startTile := tilemap.WorldPositionToTile(from)
	goalTile := tilemap.WorldPositionToTile(to)
	if !s.Terrain.IsInBounds(goalTile.X, goalTile.Y) || !s.Terrain.IsWalkable(goalTile.X, goalTile.Y) {
		return nil, false
	}

	var between []pathfinding.Coord
	if startTile != goalTile {
		path, _ := pathfinding.FindBodyPath(s.Terrain,
			pathfinding.Coord{X: startTile.X, Y: startTile.Y},
			pathfinding.Coord{X: goalTile.X, Y: goalTile.Y},
			s.MaxPathExpansions, s.Heuristic)
		if path == nil {
			return nil, false
		}
		between = path[1 : len(path)-1]
	}

	route := make([]geometry.Vector2, 0, len(between)+3)
	add := func(point geometry.Vector2) {
		last := from
		if len(route) > 0 {
			last = route[len(route)-1]
		}
		if point != last {
			route = append(route, point)
		}
	}
	add(tilemap.TileToWorldPosition(startTile))
	for _, coord := range between {
		add(tilemap.TileToWorldPosition(tilemap.TileCoord{X: coord.X, Y: coord.Y}))
	}
	add(tilemap.TileToWorldPosition(goalTile))
	add(to)
	return route, true
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./motion/`
Expected: PASS. If `TestFindRoute_EveryStepIsWalkable` fails, the reported step is a counterexample to the spec's route guarantee: stop and report it to the controller with the map, radius and route, rather than loosening the test.

- [ ] **Step 5: Record the signature change in the spec**

In the spec, under "`System.FindRoute`", change the signature line to `System.FindRoute(from, to geometry.Vector2) ([]geometry.Vector2, bool)` and the sentence "It returns nil when no route exists" to "It reports false, with a nil route, when no route exists". Under "Rulings made during implementation", replace "None yet." with:

```markdown
1. `FindRoute` returns `([]geometry.Vector2, bool)` rather than a slice alone, so an empty route
   (already there) and no route are told apart by `ok` rather than by nil versus empty.
```

- [ ] **Step 6: Document**

Append to the continuous movement paragraph in `motion/doc.go`: "System.FindRoute plans a route between two arbitrary points through tile centres, never cutting a blocked corner, and keeps every step between its points walkable for a body of radius under 0.5."

- [ ] **Step 7: Lint and commit**

```bash
task lint
git add motion/ docs/superpowers/specs/2026-09-30-continuous-movement-design.md
git commit -m "Add a route search between arbitrary points" -m "<trailer lines>"
```

---

### Task 8: Route tightening

**Files:**
* Create: `motion/motion_tighten.go`
* Create: `motion/motion_tighten_test.go`
* Modify: `motion/doc.go`

**Interfaces:**
* Consumes: `WalkIsClear` (Task 3), `System.FindRoute` (Task 7), test helpers `newRouteSystem`, `legalPoint`, `oneBlockedTile`, `v`, `testTerrain`.
* Produces: `func TightenRoute(terrain pathfinding.TerrainProvider, from geometry.Vector2, route []geometry.Vector2, radius, maxLength float64) (geometry.Vector2, bool)`.

- [ ] **Step 1: Write the failing tests**

`motion/motion_tighten_test.go`:

```go
package motion

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestTightenRoute_CapsAtMaxLength(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5), v(3.5, 1.5), v(4.5, 1.5), v(5.5, 1.5), v(6.5, 1.5)}

	got, ok := TightenRoute(terrain, v(1.5, 1.5), route, 0.25, 1.5)

	// The first point beyond 1.5 is (3.5, 1.5), 2 away; the cap scales the leg
	// toward it by 0.75, which is exact in floating point.
	if !ok || got != v(3.0, 1.5) {
		t.Errorf("expected the leg capped at (3.0, 1.5), got %v, %v", got, ok)
	}
}

func TestTightenRoute_UncappedReturnsTheLastPoint(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5), v(3.5, 2.5), v(6.2, 2.9)}

	got, ok := TightenRoute(terrain, v(1.5, 1.5), route, 0.25, math.Inf(1))

	if !ok || got != v(6.2, 2.9) {
		t.Errorf("expected the whole route in one leg on open ground, got %v, %v", got, ok)
	}
}

func TestTightenRoute_StopsWhereTheWallBlocksTheView(t *testing.T) {
	// A wall at x = 3 from y = 0 to y = 5 forces the route up and around.
	blocked := map[tilemap.TileCoord]bool{}
	for y := range 6 {
		blocked[tilemap.TileCoord{X: 3, Y: y}] = true
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	s := newRouteSystem(terrain)
	from := v(1.5, 1.5)
	route, ok := s.FindRoute(from, v(5.5, 1.5))
	if !ok {
		t.Fatal("test setup: expected a route around the wall")
	}

	got, ok := TightenRoute(terrain, from, route, 0.25, math.Inf(1))

	if !ok || !WalkIsClear(terrain, from, got, 0.25) {
		t.Fatalf("expected a clear leg, got %v, %v", got, ok)
	}
	i := slicesIndex(route, got)
	if i < 0 {
		t.Fatalf("expected an uncapped leg to end on a route point, got %v", got)
	}
	if i+1 < len(route) && WalkIsClear(terrain, from, route[i+1], 0.25) {
		t.Errorf("expected the next route point %v to be out of view", route[i+1])
	}
}

func slicesIndex(route []geometry.Vector2, p geometry.Vector2) int {
	for i, q := range route {
		if q == p {
			return i
		}
	}
	return -1
}

func TestTightenRoute_ReportsFalseWhenNothingIsClear(t *testing.T) {
	terrain := oneBlockedTile()

	if _, ok := TightenRoute(terrain, v(4.5, 5.5), []geometry.Vector2{v(6.5, 5.5)}, 0.25, math.Inf(1)); ok {
		t.Error("a route whose first point lies behind a blocked tile has no leg")
	}
	if _, ok := TightenRoute(terrain, v(4.5, 5.5), nil, 0.25, math.Inf(1)); ok {
		t.Error("an empty route has no leg")
	}
	if _, ok := TightenRoute(terrain, v(4.5, 4.5), []geometry.Vector2{v(4.5, 4.5)}, 0.25, math.Inf(1)); ok {
		t.Error("a route leading only to where the body stands has no leg")
	}
}

// countingTerrain counts the tile lookups WalkIsClear makes.
type countingTerrain struct {
	*testTerrain
	lookups int
}

func (c *countingTerrain) IsInBounds(x, y int) bool {
	c.lookups++
	return c.testTerrain.IsInBounds(x, y)
}

func TestTightenRoute_ScanIsBoundedByMaxLength(t *testing.T) {
	terrain := &countingTerrain{testTerrain: &testTerrain{width: 2000, height: 3}}
	route := make([]geometry.Vector2, 0, 1000)
	for x := 1; x <= 1000; x++ {
		route = append(route, v(float64(x)+0.5, 1.5))
	}

	got, ok := TightenRoute(terrain, v(0.5, 1.5), route, 0.25, 1.0)

	if !ok || got != v(1.5, 1.5) {
		t.Fatalf("expected a one-tile leg, got %v, %v", got, ok)
	}
	if terrain.lookups > 100 {
		t.Errorf("expected a handful of tile lookups for a 1-tile leg, got %d", terrain.lookups)
	}
}

func TestTightenRoute_LargeBodyNeverWalksThroughAWall(t *testing.T) {
	// A one-tile corridor at y = 5 through a wall at x = 5. A 0.6 body does not
	// fit through it.
	blocked := map[tilemap.TileCoord]bool{}
	for y := range 10 {
		if y != 5 {
			blocked[tilemap.TileCoord{X: 5, Y: y}] = true
		}
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	s := newRouteSystem(terrain)
	from := v(2.5, 5.5)
	route, ok := s.FindRoute(from, v(8.5, 5.5))
	if !ok {
		t.Fatal("test setup: expected a route through the corridor")
	}

	got, ok := TightenRoute(terrain, from, route, 0.6, math.Inf(1))

	if !ok || !WalkIsClear(terrain, from, got, 0.6) {
		t.Fatalf("expected a walkable leg toward the corridor, got %v, %v", got, ok)
	}
	// On y = 5.5 a 0.6 body keeps 0.6 from the corners (5, 5) and (5, 6), so
	// its centre stays below x = 5 - sqrt(0.6² - 0.5²) = 4.668.
	if got.X() > 4.668 {
		t.Errorf("a 0.6 body cannot reach x = %v, inside the corridor mouth", got.X())
	}

	// From there the next point is inside the corridor: the body stops short.
	route, ok = s.FindRoute(got, v(8.5, 5.5))
	if !ok {
		t.Fatal("expected a route from the corridor mouth")
	}
	if next, ok := TightenRoute(terrain, got, route, 0.6, math.Inf(1)); ok {
		t.Errorf("expected no leg through the narrow corridor, got one to %v", next)
	}
}

func TestTightenRoute_WalksEveryRouteToItsEnd(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	walks := 0
	for trial := range 500 {
		terrain := &testTerrain{width: 12, height: 12, blocked: map[tilemap.TileCoord]bool{}}
		for range 28 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(12), Y: rng.IntN(12)}] = true
		}
		s := newRouteSystem(terrain)
		radius := 0.05 + rng.Float64()*0.44
		position, goal := legalPoint(rng, terrain, radius), legalPoint(rng, terrain, radius)
		if _, ok := s.FindRoute(position, goal); !ok {
			continue
		}
		walks++

		for leg := 0; position != goal; leg++ {
			if leg > 400 {
				t.Fatalf("trial %d, radius %v: no arrival at %v after 400 legs, stuck at %v", trial, radius, goal, position)
			}
			route, ok := s.FindRoute(position, goal)
			if !ok {
				t.Fatalf("trial %d: route lost from %v", trial, position)
			}
			next, ok := TightenRoute(terrain, position, route, radius, 1.0)
			if !ok {
				t.Fatalf("trial %d, radius %v: no leg from %v along %v", trial, radius, position, route)
			}
			if !WalkIsClear(terrain, position, next, radius) {
				t.Fatalf("trial %d: leg from %v to %v is not walkable", trial, position, next)
			}
			position = next
		}
	}
	if walks < 150 {
		t.Fatalf("only %d walks ran; the test is not exercising routes", walks)
	}
}

func TestTightenRoute_Panics(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5)}
	for name, call := range map[string]func(){
		"negative radius": func() { TightenRoute(terrain, v(1.5, 1.5), route, -0.1, 1.0) },
		"zero max length": func() { TightenRoute(terrain, v(1.5, 1.5), route, 0.25, 0) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			call()
		}()
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./motion/ -run TightenRoute`
Expected: FAIL, `undefined: TightenRoute`.

- [ ] **Step 3: Implement**

`motion/motion_tighten.go`:

```go
package motion

import (
	"fmt"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/pathfinding"
)

// TightenRoute returns where a body of radius standing at from should end its
// next leg along route, normally one from System.FindRoute: the farthest
// route point it can walk to in a straight line (WalkIsClear), shortened to
// maxLength along that line. Pass math.Inf(1) for no cap.
//
// It walks the route in order and stops at the first point it cannot walk to,
// and also right after the first point farther than maxLength from from, so a
// long route costs a handful of walk tests. It reports false when no point is
// walkable, or when the only one is from itself.
//
// For a radius under 0.5 and a route from FindRoute whose ends are legal
// positions, the first point is always walkable, so tightening always makes
// progress. A larger body can meet a gap narrower than itself on a route,
// since routes know nothing of clearance: tightening then stops short or
// reports false, but never returns a leg through a blocked tile. It panics on
// a negative radius or a maxLength that is not positive.
func TightenRoute(terrain pathfinding.TerrainProvider, from geometry.Vector2, route []geometry.Vector2, radius, maxLength float64) (geometry.Vector2, bool) {
	if radius < 0 {
		panic(fmt.Sprintf("tightening route from %v: radius must not be negative, got %v", from, radius))
	}
	if maxLength <= 0 {
		panic(fmt.Sprintf("tightening route from %v: maxLength must be positive, got %v", from, maxLength))
	}

	var best geometry.Vector2
	found := false
	for _, point := range route {
		if !WalkIsClear(terrain, from, point, radius) {
			break
		}
		best, found = point, true
		if from.DistanceTo(point) > maxLength {
			break
		}
	}
	if !found || best == from {
		return geometry.Vector2{}, false
	}

	if distance := from.DistanceTo(best); distance > maxLength {
		best = from.Lerp(best, maxLength/distance)
	}
	return best, true
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./motion/`
Expected: PASS. If `TestTightenRoute_WalksEveryRouteToItsEnd` reports a stuck walk, stop and report the trial's map, radius and positions to the controller; it is a counterexample to the progress guarantee and must not be papered over.

- [ ] **Step 5: Document**

Append to the continuous movement paragraph in `motion/doc.go`: "TightenRoute picks the next leg along such a route: the farthest point a body can walk to in a straight line, capped at a leg length. A game walks by starting each leg with MoveEntity, normally with a fixed MoveOptions.Duration, reserving through a tilemap.CircleReservations."

- [ ] **Step 6: Lint and commit**

```bash
task lint
git add motion/
git commit -m "Add route tightening for straight legs" -m "<trailer lines>"
```

---

### Task 9: Architecture docs and consumer check

**Files:**
* Modify: `ARCHITECTURE.md`
* Modify: `docs/performance_optimization.md`
* Scratch only (never committed): worktrees of `~/src/lockstep` and `~/src/nrg`

**Interfaces:**
* Consumes: everything above.
* Produces: documentation, and a written report of the consumer check for the controller.

- [ ] **Step 1: Update `ARCHITECTURE.md`**

* Package map: `geometry` purpose becomes "2D geometric types and operations (`Vector2`, shapes, capsule contact)"; `pathfinding` gains "and `FindBodyPath` for round bodies"; `tilemap` becomes "Tile coordinates, `SpatialGrid` (range queries), `TileOccupancyManager`, `CircleReservations`"; `motion` becomes "Movement components (`Spatial`, `Movement`), `System` (a tick system), the `Occupancy` interface, and continuous movement (`WalkIsClear`, `FindRoute`, `TightenRoute`)".
* Key abstractions, the spatial indexing bullet: add "`motion.System` reserves moves through the `motion.Occupancy` interface: `TileOccupancyManager` for tile-based games, or `CircleReservations`, one circle per body with its own radius, for bodies that stand anywhere. See `docs/superpowers/specs/2026-09-30-continuous-movement-design.md`."

- [ ] **Step 2: Update `docs/performance_optimization.md`**

Append three sections in the file's style:

```markdown
## Straight walk test tile scan (motion/motion_walk.go)

`WalkIsClear` visits every tile the capsule can touch column by column and
calls the terrain's `IsInBounds` and `IsWalkable` on each, even on open ground
where no tile is blocked. A game with a coarse blocked-tile index (for example,
a bitset per chunk that is all-walkable) could skip whole columns. Left as a
plain scan because a one-tile leg touches about a dozen tiles; revisit if walk
tests show up in a profile, most likely from long direct-to-goal checks.

## Circle reservation search margin (tilemap/tilemap_circles.go)

`CircleReservations` searches around a point by the entity's radius plus the
largest radius the table has ever held, and that maximum never shrinks. One very
large body therefore widens every later search, even after it is removed.
Tracking radii in a sorted multiset, or indexing large bodies separately, would
tighten the search. Left simple because every body has one radius today.

## Route allocation (motion/motion_route.go)

`FindRoute` allocates a fresh route slice per call, and `FindBodyPath`
allocates its node map per search. A game planning a leg per decision for many
bodies could pass a reusable buffer. Left as is because the search itself costs
far more than the allocation.
```

- [ ] **Step 3: Run the full checks**

```bash
export GOMODCACHE=/tmp/go-mod-cache
task lint
task test:headless
```

Expected: both pass.

- [ ] **Step 4: Consumer check against lockstep**

Run in the session's scratchpad directory (the controller gives you its path as `$SP`):

```bash
export GOMODCACHE=/tmp/go-mod-cache GOFLAGS=-mod=mod
cd ~/src/lockstep && git worktree add -q --detach "$SP/lockstep-check" HEAD
cd "$SP/lockstep-check"
go mod edit -replace github.com/trancecode/vantage=/home/exedev/src/vantage
# The baseline's one scratch line: MaxPathExpansions became mandatory after v0.1.19.
sed -i 's/^\t\tMaxMoveActionDistance: MaxDistancePerMoveAction,/&\n\t\tMaxPathExpansions:     100_000,/' core/core_world.go
xvfb-run -a go test ./... 2>&1 | grep -E "^(ok|FAIL|--- FAIL|panic:)"
```

Expected, matching the baseline: `arena`, `config`, `core`, `spritepack` and `spritesheet` pass; `shell` fails only `TestSheetLibraryRebasesAnchorPerAnimation`. Any other difference is a regression to report.

- [ ] **Step 5: Consumer check against nrg**

```bash
cd ~/src/nrg && git worktree add -q --detach "$SP/nrg-check" HEAD
cd "$SP/nrg-check"
xvfb-run -a go test ./... 2>&1 | grep -E "^(ok|FAIL|--- FAIL|panic:)" > "$SP/nrg-baseline.txt"
go mod edit -replace github.com/trancecode/vantage=/home/exedev/src/vantage
xvfb-run -a go test ./... 2>&1 | grep -E "^(ok|FAIL|--- FAIL|panic:)" > "$SP/nrg-after.txt"
diff <(sed 's/\t[0-9.]*s$//; s/(cached)//' "$SP/nrg-baseline.txt") <(sed 's/\t[0-9.]*s$//; s/(cached)//' "$SP/nrg-after.txt")
```

The baseline runs at nrg's pinned vantage (v0.1.27). Expected: no difference in which packages and tests fail. If the replace does not compile, report the error: a change after v0.1.27 that is not part of this plan may be the cause, and the controller decides.

- [ ] **Step 6: Clean up the scratch worktrees**

```bash
cd ~/src/lockstep && git worktree remove --force "$SP/lockstep-check"
cd ~/src/nrg && git worktree remove --force "$SP/nrg-check"
```

Confirm `git -C ~/src/lockstep status --short` and `git -C ~/src/nrg status --short` show nothing this task created.

- [ ] **Step 7: Commit the docs**

```bash
git add ARCHITECTURE.md docs/performance_optimization.md
git commit -m "Document the continuous movement building blocks" -m "<trailer lines>"
```

Report the lockstep and nrg results verbatim to the controller.

---

## After the plan (controller)

* Run `/deep-review --controller <dir> --fix --range da39710..HEAD`, carry the summary line and any style guide rationales into the spec's rulings, then the whole-branch review's fix wave.
* Push, tag `v0.1.29` and push the tag.
* Send the nrg session the version, the API, and the divergences listed in the spec's "Release" section plus the rulings.
