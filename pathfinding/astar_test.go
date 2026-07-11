package pathfinding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTerrain implements TerrainProvider for testing
type mockTerrain struct {
	width            int
	height           int
	walkable         map[Coord]bool
	speedMultipliers map[Coord]float64
}

func newMockTerrain(width, height int) *mockTerrain {
	return &mockTerrain{
		width:            width,
		height:           height,
		walkable:         make(map[Coord]bool),
		speedMultipliers: make(map[Coord]float64),
	}
}

func (m *mockTerrain) IsInBounds(x, y int) bool {
	return x >= 0 && x < m.width && y >= 0 && y < m.height
}

func (m *mockTerrain) IsWalkable(x, y int) bool {
	if !m.IsInBounds(x, y) {
		return false
	}
	coord := Coord{x, y}
	walkable, exists := m.walkable[coord]
	return exists && walkable
}

func (m *mockTerrain) GetTerrainSpeedMultiplier(x, y int) float64 {
	if !m.IsInBounds(x, y) {
		return 0.0
	}
	coord := Coord{x, y}
	if speed, exists := m.speedMultipliers[coord]; exists {
		return speed
	}
	return 1.0 // Default speed
}

func (m *mockTerrain) setWalkable(x, y int, walkable bool) {
	m.walkable[Coord{x, y}] = walkable
}

func (m *mockTerrain) setSpeed(x, y int, speed float64) {
	m.speedMultipliers[Coord{x, y}] = speed
}

// TestFindPathStraightLine tests pathfinding in a straight line
func TestFindPathStraightLine(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Test horizontal path
	start := Coord{0, 5}
	goal := Coord{5, 5}
	path := FindPath(terrain, start, goal, nil)

	require.NotNil(t, path)
	assert.Equal(t, 6, len(path)) // Start + 4 intermediate + goal
	assert.Equal(t, start, path[0])
	assert.Equal(t, goal, path[len(path)-1])

	// Verify path is straight
	for i := range path {
		assert.Equal(t, 5, path[i].Y)
		assert.Equal(t, i, path[i].X)
	}
}

// TestFindPathDiagonal tests diagonal pathfinding
func TestFindPathDiagonal(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Test diagonal path
	start := Coord{0, 0}
	goal := Coord{3, 3}
	path := FindPath(terrain, start, goal, nil)

	require.NotNil(t, path)
	assert.Equal(t, 4, len(path)) // Optimal diagonal path
	assert.Equal(t, start, path[0])
	assert.Equal(t, goal, path[len(path)-1])

	// Verify diagonal movement
	for i := range path {
		assert.Equal(t, i, path[i].X)
		assert.Equal(t, i, path[i].Y)
	}
}

// TestFindPathObstacles tests pathfinding around obstacles
func TestFindPathObstacles(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Create a wall of unwalkable tiles
	for y := 2; y <= 7; y++ {
		terrain.setWalkable(5, y, false)
	}

	// Test path that must go around the wall
	start := Coord{0, 5}
	goal := Coord{8, 5}
	path := FindPath(terrain, start, goal, nil)

	require.NotNil(t, path)
	assert.Equal(t, start, path[0])
	assert.Equal(t, goal, path[len(path)-1])

	// Verify path doesn't go through the wall
	for _, coord := range path {
		if coord.X == 5 {
			assert.True(t, coord.Y < 2 || coord.Y > 7, "Path should not go through wall")
		}
	}
}

// TestFindPathNoPath tests when no path exists
func TestFindPathNoPath(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Create an island surrounded by unwalkable tiles
	for x := 4; x <= 6; x++ {
		terrain.setWalkable(x, 3, false)
		terrain.setWalkable(x, 7, false)
	}
	for y := 4; y <= 6; y++ {
		terrain.setWalkable(3, y, false)
		terrain.setWalkable(7, y, false)
	}

	// Test path from outside to inside the island
	start := Coord{0, 0}
	goal := Coord{5, 5}
	path := FindPath(terrain, start, goal, nil)

	assert.Nil(t, path, "Should return nil when no path exists")
}

// TestFindPathDiagonalCornerCutting tests diagonal movement rules
func TestFindPathDiagonalCornerCutting(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Create a corner obstacle
	terrain.setWalkable(5, 5, false) // Unwalkable corner
	terrain.setWalkable(6, 5, false) // Block one adjacent
	terrain.setWalkable(5, 6, false) // Block other adjacent

	// Test diagonal movement that would cut through the corner
	start := Coord{4, 4}
	goal := Coord{6, 6}
	path := FindPath(terrain, start, goal, nil)

	require.NotNil(t, path)

	// The path should not cut through the corner at (5,5)
	for i := 1; i < len(path); i++ {
		prev := path[i-1]
		curr := path[i]

		// Check if this is a diagonal move through the blocked corner
		if prev.X == 4 && prev.Y == 4 && curr.X == 5 && curr.Y == 5 {
			t.Error("Path should not cut through corner")
		}
	}
}

// TestFindPathWithOccupancy tests pathfinding with occupancy checker
func TestFindPathWithOccupancy(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Create occupancy map
	occupied := map[Coord]bool{
		{5, 5}: true,
		{5, 6}: true,
	}

	isOccupied := func(coord Coord) bool {
		return occupied[coord]
	}

	// Test path that must go around occupied tiles
	start := Coord{0, 5}
	goal := Coord{9, 5}
	path := FindPath(terrain, start, goal, isOccupied)

	require.NotNil(t, path)

	// Verify path doesn't go through occupied tiles
	for _, coord := range path {
		assert.False(t, occupied[coord], "Path should not go through occupied tiles")
	}
}

// TestFindPathTerrainSpeed tests terrain speed affecting path cost
func TestFindPathTerrainSpeed(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
			terrain.setSpeed(x, y, 1.0)
		}
	}

	// Create a slow path in the middle (direct route)
	for x := 1; x <= 3; x++ {
		terrain.setSpeed(x, 1, 0.1) // Very slow terrain
	}

	// Fast path around the top (indirect but faster)
	for x := 1; x <= 3; x++ {
		terrain.setSpeed(x, 0, 2.0) // Fast terrain
	}

	// Test that pathfinding prefers the faster route
	start := Coord{0, 1}
	goal := Coord{4, 1}
	path := FindPath(terrain, start, goal, nil)

	require.NotNil(t, path)

	// The optimal path should go around via y=0 due to faster terrain
	// At least one step should have y=0
	hasTopPath := false
	for _, coord := range path {
		if coord.Y == 0 {
			hasTopPath = true
			break
		}
	}
	assert.True(t, hasTopPath, "Path should prefer faster terrain route")
}

// TestFindPathEdgeCases tests various edge cases
func TestFindPathEdgeCases(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Test same start and goal - should return nil (no path needed)
	start := Coord{5, 5}
	path := FindPath(terrain, start, start, nil)
	assert.Nil(t, path)

	// Test out of bounds goal
	outOfBounds := Coord{15, 15}
	path = FindPath(terrain, start, outOfBounds, nil)
	assert.Nil(t, path)

	// Test unwalkable goal
	terrain.setWalkable(7, 7, false)
	unwalkableGoal := Coord{7, 7}
	path = FindPath(terrain, start, unwalkableGoal, nil)
	assert.Nil(t, path)
}

// TestFindPathOccupiedGoal tests that pathfinding rejects occupied destinations
func TestFindPathOccupiedGoal(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	// Make all tiles walkable
	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Mark goal as occupied
	goal := Coord{5, 5}
	isOccupied := func(coord Coord) bool {
		return coord == goal
	}

	// Path should NOT be found because occupied tiles are not reachable
	start := Coord{0, 0}
	path := FindPath(terrain, start, goal, isOccupied)

	assert.Nil(t, path, "Should not find path to occupied destination")
}
