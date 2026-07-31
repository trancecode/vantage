package pathfinding

import (
	"fmt"
	"testing"
)

// benchTerrain is a square, uniform-speed map backed by a dense walkability
// slice. Benchmarks use it rather than the map-backed mockTerrain so the
// numbers reflect search cost instead of the terrain provider's own lookups.
type benchTerrain struct {
	size     int
	walkable []bool
}

// newBenchTerrain builds a fully walkable size x size map.
func newBenchTerrain(size int) *benchTerrain {
	terrain := &benchTerrain{size: size, walkable: make([]bool, size*size)}
	for i := range terrain.walkable {
		terrain.walkable[i] = true
	}
	return terrain
}

func (t *benchTerrain) IsInBounds(x, y int) bool {
	return x >= 0 && x < t.size && y >= 0 && y < t.size
}

func (t *benchTerrain) IsWalkable(x, y int) bool {
	return t.IsInBounds(x, y) && t.walkable[y*t.size+x]
}

func (t *benchTerrain) GetTerrainSpeedMultiplier(x, y int) float64 {
	if !t.IsWalkable(x, y) {
		return 0
	}
	return 1
}

func (t *benchTerrain) setWalkable(x, y int, walkable bool) {
	t.walkable[y*t.size+x] = walkable
}

// benchMapSize matches the open map used by the game-side scale harness that
// motivated these benchmarks.
const benchMapSize = 304

// benchDistances are the straight-line journey lengths measured against cost.
// The largest one spans most of a benchMapSize map, which is the caravan-style
// journey long-distance routing has to support.
var benchDistances = []int{2, 8, 32, 64, 128, 256}

// benchMargin keeps a benchmark's origin off the map edge while still leaving
// room for the longest journey in benchDistances.
const benchMargin = 16

// Expectations for runFindPath, named so that call sites read clearly.
const (
	wantPath   = true
	wantNoPath = false
)

// runFindPath times FindPath over the given query and reports the number of
// nodes the search expanded alongside the usual cost and allocation figures.
// Expansions are what distinguish a search that walked a narrow corridor to the
// goal from one that flooded the reachable map. It fails the benchmark when the
// query does not resolve as wantPath says it should, so a mis-specified query
// cannot quietly measure an instant rejection instead of a real search.
func runFindPath(b *testing.B, terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker, wantPath bool) {
	b.Helper()

	path, expanded := findPath(terrain, start, goal, isOccupied)
	if gotPath := path != nil; gotPath != wantPath {
		b.Fatalf("path from %v to %v: got path %t, want %t", start, goal, gotPath, wantPath)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		FindPath(terrain, start, goal, isOccupied)
	}
	b.ReportMetric(float64(expanded), "expansions/op")
}

// BenchmarkFindPathByPathLength measures A* cost and node expansions against
// journey length on open terrain with no other agents, separately for cardinal
// and diagonal journeys. Octile distance is exact on an obstacle-free uniform
// grid, so the search should walk a corridor rather than a region: expansions
// staying at one per path tile is what confirms it.
func BenchmarkFindPathByPathLength(b *testing.B) {
	terrain := newBenchTerrain(benchMapSize)

	for _, direction := range []struct {
		name   string
		origin Coord
		dx, dy int
	}{
		{name: "cardinal", origin: Coord{X: benchMargin, Y: benchMapSize / 2}, dx: 1, dy: 0},
		{name: "diagonal", origin: Coord{X: benchMargin, Y: benchMargin}, dx: 1, dy: 1},
	} {
		origin := direction.origin
		for _, distance := range benchDistances {
			goal := Coord{X: origin.X + direction.dx*distance, Y: origin.Y + direction.dy*distance}
			b.Run(fmt.Sprintf("%s/distance=%d", direction.name, distance), func(b *testing.B) {
				runFindPath(b, terrain, origin, goal, nil, wantPath)
			})
		}
	}
}

// BenchmarkFindPathByMapSize measures whether cost depends on the size of the
// map rather than on the length of the journey, for both a two-tile hop and a
// journey of fixed length.
func BenchmarkFindPathByMapSize(b *testing.B) {
	for _, size := range []int{64, 152, 304, 608} {
		terrain := newBenchTerrain(size)
		origin := Coord{X: size / 4, Y: size / 2}

		for _, distance := range []int{2, 32} {
			goal := Coord{X: origin.X + distance, Y: origin.Y}
			b.Run(fmt.Sprintf("size=%d/distance=%d", size, distance), func(b *testing.B) {
				runFindPath(b, terrain, origin, goal, nil, wantPath)
			})
		}
	}
}

// BenchmarkFindPathOccupiedGoal measures the converging-agents case: many
// agents route toward one shared destination, which is occupied as soon as the
// first of them arrives. Every later agent then asks for a path to a tile it
// can never enter.
func BenchmarkFindPathOccupiedGoal(b *testing.B) {
	terrain := newBenchTerrain(benchMapSize)
	origin := Coord{X: benchMapSize / 4, Y: benchMapSize / 2}
	goal := Coord{X: origin.X + 128, Y: origin.Y}
	isOccupied := func(coord Coord) bool { return coord == goal }

	runFindPath(b, terrain, origin, goal, isOccupied, wantNoPath)
}

// BenchmarkFindPathWalledGoal measures a goal that is reachable-looking but
// sealed off by terrain, the other way a search ends up exhausting the map.
func BenchmarkFindPathWalledGoal(b *testing.B) {
	terrain := newBenchTerrain(benchMapSize)
	origin := Coord{X: benchMapSize / 4, Y: benchMapSize / 2}
	goal := Coord{X: origin.X + 128, Y: origin.Y}

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx != 0 || dy != 0 {
				terrain.setWalkable(goal.X+dx, goal.Y+dy, false)
			}
		}
	}

	runFindPath(b, terrain, origin, goal, nil, wantNoPath)
}
