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

// The shore map's journeys end in the cell holding benchOrigin, whose top row
// is shoreCellTop. Water covers that cell's top four rows and everything north
// of them, and the goal sits on row shoreGoalY, in the upper half of the cell,
// so every coarse center a search starts from is water or shore.
const (
	shoreCellTop = benchOrigin / DefaultCoarseCellSize * DefaultCoarseCellSize
	shoreY       = shoreCellTop + 4
	shoreGoalY   = shoreCellTop + 10
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

// shoreBenchTerrain is half-speed forest south of a straight shoreline with
// open water north of it, standing in for a pool shore in nrg's Blighted
// Reaches: the goal's row of cells has water along its northern edge, so none
// of them can be crossed north to south.
func shoreBenchTerrain(int) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if y < shoreY {
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
	// towardOrigin makes the journey end at the origin rather than start there,
	// so its goal sits on whatever the map places at the origin.
	towardOrigin bool
}

var benchMaps = []benchMap{
	{name: "grass/cardinal", terrain: grassBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: grassSpeed},
	{name: "offset-road/cardinal", terrain: offsetRoadBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: roadSpeed},
	{name: "grid/cardinal", terrain: gridBenchTerrain, origin: Coord{X: benchOrigin + gridSpacing/2, Y: benchOrigin + gridSpacing/2}, dx: 1, fastestSpeed: roadSpeed},
	{name: "grid/oblique", terrain: gridBenchTerrain, origin: Coord{X: benchOrigin + gridSpacing/2, Y: benchOrigin + gridSpacing/2}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5), fastestSpeed: roadSpeed},
	{name: "reaches/cardinal", terrain: reachesBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 1, fastestSpeed: forestSpeed},
	{name: "reaches/oblique", terrain: reachesBenchTerrain, origin: Coord{X: benchOrigin, Y: benchOrigin}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5), fastestSpeed: forestSpeed},
	{name: "shore/cardinal", terrain: shoreBenchTerrain, origin: Coord{X: benchOrigin, Y: shoreGoalY}, dy: 1, fastestSpeed: forestSpeed, towardOrigin: true},
	{name: "shore/oblique", terrain: shoreBenchTerrain, origin: Coord{X: benchOrigin, Y: shoreGoalY}, dx: 2 / math.Sqrt(5), dy: 1 / math.Sqrt(5), fastestSpeed: forestSpeed, towardOrigin: true},
}

// benchJourney returns the start and goal of a journey of length tiles on a map:
// from the map's origin along its direction, each moved off a pool when one
// sits on it, and swapped when the map's journeys run toward its origin.
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
	if m.towardOrigin {
		return goal, start
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
// budget on a fresh field; cells-built/op, the cells that call built;
// cells-settled/op, the cells that call's coarse search settled, which is what
// DefaultCoarseCellBudget bounds; and extra-chunks/op, the benchChunkSize chunks
// those cells read that the tile search did not, which is what a lazily
// generated world pays to materialize.
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
				optimalPath, _ := FindPath(terrain, start, goal, nil, uncappedExpansions, ScaledOctile{MaxSpeed: m.fastestSpeed})
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

	path, expanded := FindPath(terrain, start, goal, nil, uncappedExpansions, heuristic)

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

// coarseSearchRecorder is a Heuristic that serves a CoarseCost's estimates and
// keeps the coarse search of the last search it served, so a benchmark can read
// how many cells that search settled without the field keeping such state.
type coarseSearchRecorder struct {
	field *CoarseCost
	last  *coarseSearch
}

// ForSearch starts the field's coarse search for start and goal, keeps it, and
// returns its estimate.
func (r *coarseSearchRecorder) ForSearch(start, goal Coord) Estimate {
	r.last = r.field.newSearch(start, goal)
	return r.last.estimate
}

// runCoarseJourney reports CoarseCost's journey metrics, cold and warm, and
// times FindPath over the journey on a warm field under benchMaxExpansions.
func runCoarseJourney(b *testing.B, terrain TerrainProvider, start, goal Coord, optimalCost float64) {
	b.Helper()

	coldField := NewCoarseCost(terrain, coarseBenchConfig())
	recorder := &coarseSearchRecorder{field: coldField}
	coldStart := time.Now()
	FindPath(terrain, start, goal, nil, benchMaxExpansions, recorder)
	cold := time.Since(coldStart)
	if recorder.last == nil {
		b.Fatalf("path from %v to %v: the cold call ran no search", start, goal)
	}
	cellsBuilt := len(coldField.cells)
	cellsSettled := recorder.last.settled
	extraChunks := coarseExtraChunks(terrain, start, goal)

	field := NewCoarseCost(terrain, coarseBenchConfig())
	path, expanded := FindPath(terrain, start, goal, nil, uncappedExpansions, field)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		FindPath(terrain, start, goal, nil, benchMaxExpansions, field)
	}
	reportJourney(b, terrain, start, goal, path, expanded, optimalCost)
	b.ReportMetric(float64(cold.Microseconds())/1000, "cold-ms/op")
	b.ReportMetric(float64(cellsBuilt), "cells-built/op")
	b.ReportMetric(float64(cellsSettled), "cells-settled/op")
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
