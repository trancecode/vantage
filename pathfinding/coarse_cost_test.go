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

// shoreTestTerrain is half-speed forest south of a straight shoreline at row
// shoreY, with open water north of it.
func shoreTestTerrain(shoreY int) speedFuncTerrain {
	return speedFuncTerrain{speed: func(x, y int) float64 {
		if y < shoreY {
			return 0
		}
		return 0.5
	}}
}

// TestCoarseSearchLeavesShoreCells tests that the coarse search reaches the
// ground beyond a row of shore cells. Water along their northern side leaves
// them no north-south crossing, yet a route leaves them through their southern
// side without crossing them edge to edge.
func TestCoarseSearchLeavesShoreCells(t *testing.T) {
	size := DefaultCoarseCellSize
	field := NewCoarseCost(shoreTestTerrain(8), testCoarseConfig(0.5))
	// The goal sits in the upper half of its cell, so every center the search
	// starts from is water or shore.
	goal := Coord{15, 10}
	start := Coord{15, 10 + 10*size}
	require.True(t, math.IsInf(field.rates(field.cellOf(goal)).northSouth, 1), "The goal's cell should have no north-south crossing")
	search := field.newSearch(start, goal)

	value, ok := search.value(Coord{0, 10})

	require.True(t, ok, "The cell ten rows south of the shore should settle")
	assert.InDelta(t, float64(10*size)/0.5, value, float64(size), "The value should cross ten rows of half-speed forest")
	assert.Greater(t, search.estimate(start), 1.5*octile(start, goal), "The estimate should see the half-speed forest octile distance misses")
}

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

	optimal, _ := FindPath(terrain, start, goal, nil, budget, ScaledOctile{MaxSpeed: 2})
	coarse, _ := FindPath(terrain, start, goal, nil, budget, NewCoarseCost(terrain, testCoarseConfig(2)))
	direct, _ := FindPath(terrain, start, goal, nil, budget, nil)
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

	optimal, _ := FindPath(terrain, start, goal, nil, budget, ScaledOctile{MaxSpeed: 0.5})
	_, octileExpanded := FindPath(terrain, start, goal, nil, budget, nil)
	coarse, coarseExpanded := FindPath(terrain, start, goal, nil, budget, NewCoarseCost(terrain, testCoarseConfig(2)))
	require.NotNil(t, optimal)
	require.NotNil(t, coarse)

	assert.Less(t, coarseExpanded, 30*len(coarse), "The search should stay a corridor")
	assert.Less(t, coarseExpanded*5, octileExpanded, "The field should expand far less than octile distance")
	assert.LessOrEqual(t, pathCost(terrain, coarse), pathCost(terrain, optimal)*1.02)
}

// TestCoarseCostReachesGoalPastPartialEdgeCell tests that on a finite forest
// map whose width leaves a partial last cell, a goal past that cell's center
// keeps the search close to its route: the partial cell must be crossable, or
// the coarse search empties at once and every estimate falls back to octile.
func TestCoarseCostReachesGoalPastPartialEdgeCell(t *testing.T) {
	const width, height = 120, 96 // the last cell covers x 96 to 119, centered on 111.5
	terrain := newMockTerrain(width, height)
	for y := range height {
		for x := range width {
			terrain.setWalkable(x, y, true)
			terrain.setSpeed(x, y, 0.5)
		}
	}
	start, goal := Coord{0, 48}, Coord{115, 48}
	const budget = 1_000_000

	optimal, _ := FindPath(terrain, start, goal, nil, budget, ScaledOctile{MaxSpeed: 0.5})
	_, octileExpanded := FindPath(terrain, start, goal, nil, budget, nil)
	coarse, coarseExpanded := FindPath(terrain, start, goal, nil, budget, NewCoarseCost(terrain, testCoarseConfig(2)))
	require.NotNil(t, optimal)
	require.NotNil(t, coarse)

	assert.Equal(t, goal, coarse[len(coarse)-1])
	assert.Less(t, coarseExpanded, 5*len(coarse), "The search should stay a corridor")
	assert.Less(t, coarseExpanded*5, octileExpanded, "The field should expand far less than octile distance")
	assert.LessOrEqual(t, pathCost(terrain, coarse), pathCost(terrain, optimal)*1.02)
}

// TestCoarseCostFallsBackPastCellBudget tests that a search whose cell budget
// runs out still reaches the goal, through the octile fallback.
func TestCoarseCostFallsBackPastCellBudget(t *testing.T) {
	terrain := poolTestTerrain()
	config := testCoarseConfig(2)
	config.CellBudget = 1

	path, _ := FindPath(terrain, Coord{0, 0}, Coord{200, 0}, nil, 1_000_000, NewCoarseCost(terrain, config))

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

	path, _ := FindPath(terrain, Coord{10, 10}, goal, nil, 1000, NewCoarseCost(terrain, testCoarseConfig(1)))

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
	firstPath, _ := FindPath(terrain, start, goal, nil, 1_000_000, field)
	require.NotNil(t, firstPath)

	blocked = true
	path, _ := FindPath(terrain, start, goal, nil, 1_000_000, field)

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

	want, _ := FindPath(terrain, start, goal, nil, 1_000_000, field)
	require.NotNil(t, want)

	gotWarm, _ := FindPath(terrain, start, goal, nil, 1_000_000, field)
	assert.Equal(t, want, gotWarm, "warm field")
	gotFresh, _ := FindPath(terrain, start, goal, nil, 1_000_000, NewCoarseCost(terrain, testCoarseConfig(2)))
	assert.Equal(t, want, gotFresh, "fresh field")
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

	path, _ := FindPath(terrain, start, goal, nil, testMaxExpansions, NewCoarseCost(terrain, config))

	require.NotNil(t, path)
	assert.Less(t, pathCost(terrain, path), 39.0, "The route should use the road")
}
