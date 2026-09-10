package pathfinding

import (
	"fmt"
	"math"
	"testing"
)

// Speed multipliers of the terrain kinds on the road benchmark maps.
const (
	roadSpeed   = 2.0
	grassSpeed  = 1.0
	forestSpeed = 0.5
)

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

// roadOrigin places every journey far from zero, so the widest search stays on
// positive coordinates. The maps have no edge, so nothing else depends on it.
// It is a multiple of gridSpacing, which puts the grid map's roads through it.
const roadOrigin = 10_000

// uncappedExpansions is a budget no journey in BenchmarkFindPathRoads comes
// near, so a search run with it returns the route the heuristic leads to and
// the full expansion count needed to get there.
const uncappedExpansions = 50_000_000

// roadJourneyLengths are the straight-line journey lengths, in tiles.
var roadJourneyLengths = []int{250, 500, 1000, 2000}

// roadTerrain is an edgeless, fully walkable map whose speed at each tile comes
// from a function. Having no edge means a search that spreads wide is never cut
// short by the map, and computing speeds means no tile storage.
type roadTerrain struct {
	speed func(x, y int) float64
}

func (t roadTerrain) IsInBounds(x, y int) bool { return true }

func (t roadTerrain) IsWalkable(x, y int) bool { return true }

func (t roadTerrain) GetTerrainSpeedMultiplier(x, y int) float64 { return t.speed(x, y) }

// onRoad reports whether coordinate c lies on a road centred on center.
func onRoad(c, center int) bool {
	return max(c-center, center-c) <= roadWidth/2
}

// floorMod returns a modulo m in [0, m), whatever the sign of a.
func floorMod(a, m int) int {
	return (a%m + m) % m
}

// isForest reports whether (x, y) falls in a forest block, picking blocks by a
// fixed hash so the map is the same on every run.
func isForest(x, y int) bool {
	blockX := (x - floorMod(x, forestBlockSize)) / forestBlockSize
	blockY := (y - floorMod(y, forestBlockSize)) / forestBlockSize
	h := uint64(blockX)*0x9E3779B97F4A7C15 ^ uint64(blockY)*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h%100 < forestPercent
}

// grassTerrain is grass everywhere. Declaring roadSpeed on it measures what the
// scaled heuristic costs a journey that has no road anywhere near it.
func grassTerrain(int) roadTerrain {
	return roadTerrain{speed: func(x, y int) float64 { return grassSpeed }}
}

// offsetRoadTerrain is grass with one road running parallel to the journey, a
// tenth of the journey's length off to one side. Reaching it means heading
// away from the goal, which octile distance never considers worthwhile.
func offsetRoadTerrain(length int) roadTerrain {
	roadY := roadOrigin + length/10
	return roadTerrain{speed: func(x, y int) float64 {
		if onRoad(y, roadY) {
			return roadSpeed
		}
		return grassSpeed
	}}
}

// gridTerrain is a road grid gridSpacing tiles apart, with blocks of forest
// scattered over the grass between the roads.
func gridTerrain(int) roadTerrain {
	return roadTerrain{speed: func(x, y int) float64 {
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

// roadScenarios are the maps and journey directions BenchmarkFindPathRoads
// measures. Grid journeys start in the middle of a cell, as far from any road
// as the grid allows.
var roadScenarios = []struct {
	name    string
	terrain func(length int) roadTerrain
	origin  Coord
	// dx and dy are the journey's unit direction.
	dx, dy float64
}{
	{name: "grass/cardinal", terrain: grassTerrain, origin: Coord{X: roadOrigin, Y: roadOrigin}, dx: 1, dy: 0},
	{name: "offset-road/cardinal", terrain: offsetRoadTerrain, origin: Coord{X: roadOrigin, Y: roadOrigin}, dx: 1, dy: 0},
	{name: "grid/cardinal", terrain: gridTerrain, origin: Coord{X: roadOrigin + gridSpacing/2, Y: roadOrigin + gridSpacing/2}, dx: 1, dy: 0},
	{name: "grid/oblique", terrain: gridTerrain, origin: Coord{X: roadOrigin + gridSpacing/2, Y: roadOrigin + gridSpacing/2}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5)},
}

// BenchmarkFindPathRoads measures what dividing the heuristic by the terrain's
// fastest speed (MaxSpeedProvider) costs in expansions, and what leaving it
// undivided costs in route quality, on maps where roads at roadSpeed make some
// steps cheaper than their distance. heuristic=octile is the terrain searched
// as is; heuristic=scaled is the same terrain declaring roadSpeed, whose route
// is the optimum because the divided heuristic never overestimates.
//
// Each case reports the expansions the search needs to reach the goal, with no
// budget in the way; whether that fits within benchMaxExpansions, which is
// what a caller under that budget gets; and how far the route found lies above
// the optimum, in percent. ns/op times a call under benchMaxExpansions, so a
// case that does not fit times a search the budget gives up on. The optimal
// routes on the longest journeys take seconds to compute; run this benchmark on
// its own, with -benchtime 1x when only the counts are wanted.
func BenchmarkFindPathRoads(b *testing.B) {
	for _, scenario := range roadScenarios {
		for _, length := range roadJourneyLengths {
			b.Run(fmt.Sprintf("%s/length=%d", scenario.name, length), func(b *testing.B) {
				terrain := scenario.terrain(length)
				scaled := speedBoundedTerrain{TerrainProvider: terrain, maxSpeed: roadSpeed}
				start := scenario.origin
				goal := Coord{
					X: start.X + int(math.Round(float64(length)*scenario.dx)),
					Y: start.Y + int(math.Round(float64(length)*scenario.dy)),
				}

				optimalPath, optimalExpanded := findPath(scaled, start, goal, nil, uncappedExpansions)
				if optimalPath == nil {
					b.Fatalf("path from %v to %v: no route with the scaled heuristic", start, goal)
				}
				optimalCost := pathCost(terrain, optimalPath)

				b.Run("heuristic=octile", func(b *testing.B) {
					path, expanded := findPath(terrain, start, goal, nil, uncappedExpansions)
					runRoadJourney(b, terrain, start, goal, path, expanded, optimalCost)
				})
				b.Run("heuristic=scaled", func(b *testing.B) {
					runRoadJourney(b, scaled, start, goal, optimalPath, optimalExpanded, optimalCost)
				})
			})
		}
	}
}

// runRoadJourney reports the metrics of BenchmarkFindPathRoads for a route
// already found with uncappedExpansions, then times FindPath over the same
// journey under benchMaxExpansions. The budget check follows the goal check in
// the search, so a search that reached the goal in N expansions reaches it
// under any budget of at least N: comparing the uncapped count against the
// budget is the same as searching under it.
func runRoadJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, path []Coord, expanded int, optimalCost float64) {
	b.Helper()

	if path == nil {
		b.Fatalf("path from %v to %v: no route", start, goal)
	}
	aboveOptimal := (pathCost(terrain, path) - optimalCost) / optimalCost * 100
	if aboveOptimal < -1e-9 {
		b.Fatalf("path from %v to %v: route is %.6f%% below the scaled heuristic's, which should be optimal", start, goal, -aboveOptimal)
	}
	foundWithinBudget := 0.0
	if expanded <= benchMaxExpansions {
		foundWithinBudget = 1
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		FindPath(terrain, start, goal, nil, benchMaxExpansions)
	}
	b.ReportMetric(float64(expanded), "expansions/op")
	b.ReportMetric(foundWithinBudget, "found-within-budget/op")
	b.ReportMetric(aboveOptimal, "pct-above-optimal/op")
}
