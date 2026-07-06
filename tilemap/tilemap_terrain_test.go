package tilemap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trancecode/vantage/pathfinding"
)

func TestTileGridStartsZeroValued(t *testing.T) {
	g := NewTileGrid[int](3, 2)
	assert.Equal(t, 3, g.Width())
	assert.Equal(t, 2, g.Height())
	assert.Zero(t, g.At(2, 1))
}

func TestTileGridSetAtRoundTrip(t *testing.T) {
	g := NewTileGrid[string](4, 3)
	g.Set(1, 2, "grass")
	assert.Equal(t, "grass", g.At(1, 2))
	assert.Zero(t, g.At(2, 1), "neighbouring tiles stay untouched")
}

func TestTileGridIsInBounds(t *testing.T) {
	g := NewTileGrid[int](4, 3)
	assert.True(t, g.IsInBounds(0, 0))
	assert.True(t, g.IsInBounds(3, 2))
	assert.False(t, g.IsInBounds(-1, 0))
	assert.False(t, g.IsInBounds(0, -1))
	assert.False(t, g.IsInBounds(4, 0))
	assert.False(t, g.IsInBounds(0, 3))
}

func TestTileGridPanicsOutOfBounds(t *testing.T) {
	g := NewTileGrid[int](2, 2)
	assert.Panics(t, func() { g.At(2, 0) })
	assert.Panics(t, func() { g.Set(0, -1, 7) })
}

func TestTileGridFill(t *testing.T) {
	g := NewTileGrid[int](2, 2)
	g.Fill(9)
	for y := range 2 {
		for x := range 2 {
			assert.Equal(t, 9, g.At(x, y))
		}
	}
}

// terrainSpeed maps the test's tile payload (a plain int) to a speed
// multiplier: 0 is impassable, anything else walkable at that speed.
func terrainSpeed(v int) float64 { return float64(v) }

func TestTerrainImplementsTerrainProvider(t *testing.T) {
	var _ pathfinding.TerrainProvider = Terrain[int]{}
}

func TestTerrainQueries(t *testing.T) {
	g := NewTileGrid[int](2, 1)
	g.Set(0, 0, 2) // fast
	// (1, 0) stays 0: impassable
	terrain := Terrain[int]{Tiles: g, Speed: terrainSpeed}

	assert.True(t, terrain.IsInBounds(1, 0))
	assert.False(t, terrain.IsInBounds(2, 0))

	assert.True(t, terrain.IsWalkable(0, 0))
	assert.False(t, terrain.IsWalkable(1, 0), "zero speed means impassable")
	assert.False(t, terrain.IsWalkable(2, 0), "out of bounds is not walkable")

	assert.Equal(t, 2.0, terrain.GetTerrainSpeedMultiplier(0, 0))
	assert.Equal(t, 0.0, terrain.GetTerrainSpeedMultiplier(1, 0))
	assert.Equal(t, 0.0, terrain.GetTerrainSpeedMultiplier(-1, 0), "out of bounds has zero speed")
}

func TestTerrainPathfindsAroundWalls(t *testing.T) {
	// 3×3, all walkable except a wall through the middle column's top two rows:
	//   . # .
	//   . # .
	//   . . .
	g := NewTileGrid[int](3, 3)
	g.Fill(1)
	g.Set(1, 0, 0)
	g.Set(1, 1, 0)
	terrain := Terrain[int]{Tiles: g, Speed: terrainSpeed}

	path := pathfinding.FindPath(terrain, pathfinding.Coord{X: 0, Y: 0}, pathfinding.Coord{X: 2, Y: 0}, nil)
	require.NotEmpty(t, path, "a path around the wall must exist")
	for _, c := range path {
		assert.True(t, terrain.IsWalkable(c.X, c.Y), "path must avoid the wall: %v", c)
	}
	assert.Equal(t, pathfinding.Coord{X: 2, Y: 0}, path[len(path)-1])
}
