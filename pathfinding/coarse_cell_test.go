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

// openFiniteTerrain is a finite map of the given size, walkable everywhere at
// speed 1.0.
func openFiniteTerrain(width, height int) *mockTerrain {
	terrain := newMockTerrain(width, height)
	for y := range height {
		for x := range width {
			terrain.setWalkable(x, y, true)
		}
	}
	return terrain
}

// TestBuildCellRatesStraddlingMapEdge tests that a cell straddling a finite
// map's east or south edge is measured over its in-bounds part, so its
// out-of-bounds tiles do not make it uncrossable.
func TestBuildCellRatesStraddlingMapEdge(t *testing.T) {
	terrain := openFiniteTerrain(40, 40)

	east := buildCellRates(terrain, Coord{1, 0}, 32)
	south := buildCellRates(terrain, Coord{0, 1}, 32)

	assert.InDelta(t, 1.0, east.westEast, 1e-9, "Crossing the eight in-bounds columns")
	assert.InDelta(t, 1.0, east.northSouth, 1e-9)
	assert.InDelta(t, 1.0, south.northSouth, 1e-9, "Crossing the eight in-bounds rows")
	assert.InDelta(t, 1.0, south.westEast, 1e-9)
}

// TestBuildCellRatesSingleInBoundsColumn tests that a cell whose in-bounds part
// is a single column crosses west to east at the inverse of its fastest
// walkable tile's speed, and not at all when none of its tiles is walkable.
func TestBuildCellRatesSingleInBoundsColumn(t *testing.T) {
	terrain := openFiniteTerrain(33, 32)

	assert.InDelta(t, 1.0, buildCellRates(terrain, Coord{1, 0}, 32).westEast, 1e-9)
	assert.True(t, buildCellRates(newMockTerrain(33, 32), Coord{1, 0}, 32).uncrossable())
	assert.True(t, buildCellRates(terrain, Coord{2, 0}, 32).uncrossable(), "A cell entirely out of bounds")
}

// TestBuildCellRatesReadsEachTileOnce tests that building a cell reads each of
// its tiles' speed exactly once.
func TestBuildCellRatesReadsEachTileOnce(t *testing.T) {
	terrain := &countingTerrain{TerrainProvider: uniformTerrain(1)}

	buildCellRates(terrain, Coord{0, 0}, 32)

	assert.Equal(t, 32*32, terrain.speedQueries)
}
