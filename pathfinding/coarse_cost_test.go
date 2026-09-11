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
