package pathfinding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMaxExpansions is a budget above the tile count of every finite map the
// tests build, so it never changes the outcome of a search on one of them.
const testMaxExpansions = 10_000

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, isOccupied, testMaxExpansions)

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
	path := FindPath(terrain, start, goal, nil, testMaxExpansions)

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
	path := FindPath(terrain, start, start, nil, testMaxExpansions)
	assert.Nil(t, path)

	// Test out of bounds goal
	outOfBounds := Coord{15, 15}
	path = FindPath(terrain, start, outOfBounds, nil, testMaxExpansions)
	assert.Nil(t, path)

	// Test unwalkable goal
	terrain.setWalkable(7, 7, false)
	unwalkableGoal := Coord{7, 7}
	path = FindPath(terrain, start, unwalkableGoal, nil, testMaxExpansions)
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
	path := FindPath(terrain, start, goal, isOccupied, testMaxExpansions)

	assert.Nil(t, path, "Should not find path to occupied destination")
}

// ringOccupancy returns an occupancy checker marking the eight tiles around
// center as taken.
func ringOccupancy(center Coord) OccupancyChecker {
	occupied := make(map[Coord]bool)
	for _, dir := range directions {
		occupied[Coord{center.X + dir.X, center.Y + dir.Y}] = true
	}
	return func(coord Coord) bool { return occupied[coord] }
}

// TestFindPathGoalRingedByOccupants tests that a free goal whose every
// approach is taken is rejected rather than searched for.
func TestFindPathGoalRingedByOccupants(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	goal := Coord{5, 5}
	path := FindPath(terrain, Coord{0, 0}, goal, ringOccupancy(goal), testMaxExpansions)

	assert.Nil(t, path, "Should not find path to a goal with no free approach")
}

// TestFindPathGoalRingedButAdjacentStart tests that the tile the search starts
// from still counts as an approach to the goal even when occupancy reports it
// taken, which it does whenever the reservation belongs to the moving entity.
func TestFindPathGoalRingedButAdjacentStart(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
		}
	}

	goal := Coord{5, 5}
	start := Coord{4, 4}
	path := FindPath(terrain, start, goal, ringOccupancy(goal), testMaxExpansions)

	require.NotNil(t, path)
	assert.Equal(t, []Coord{start, goal}, path)
}

// TestFindPathImpassableSpeed tests that a walkable tile reporting zero speed
// cannot be crossed, since entering it from another such tile has no finite
// cost.
func TestFindPathImpassableSpeed(t *testing.T) {
	terrain := newMockTerrain(10, 10)

	for y := range 10 {
		for x := range 10 {
			terrain.setWalkable(x, y, true)
			terrain.setSpeed(x, y, 1.0)
		}
	}

	// A two-column band of walkable but zero-speed tiles spanning the map.
	// Stepping into the band still averages against speed 1, but stepping from
	// one band tile to the next averages zero, so the band cannot be crossed.
	for y := range 10 {
		terrain.setSpeed(4, y, 0)
		terrain.setSpeed(5, y, 0)
	}

	path := FindPath(terrain, Coord{0, 5}, Coord{9, 5}, nil, testMaxExpansions)

	assert.Nil(t, path, "Should not cross a band of zero-speed terrain")
}

// TestFindPathDeterministic tests that repeated searches over identical inputs
// return identical paths, which the simulation relies on for reproducibility.
func TestFindPathDeterministic(t *testing.T) {
	terrain := newMockTerrain(30, 30)

	for y := range 30 {
		for x := range 30 {
			terrain.setWalkable(x, y, true)
		}
	}

	// Scattered obstacles, so the search has genuine choices to make.
	for y := 5; y < 25; y++ {
		terrain.setWalkable(10, y, false)
		terrain.setWalkable(20, 29-y, false)
	}

	occupied := map[Coord]bool{{15, 12}: true, {15, 13}: true, {15, 14}: true}
	isOccupied := func(coord Coord) bool { return occupied[coord] }

	start := Coord{0, 0}
	goal := Coord{29, 29}
	want := FindPath(terrain, start, goal, isOccupied, testMaxExpansions)
	require.NotNil(t, want)

	for range 5 {
		assert.Equal(t, want, FindPath(terrain, start, goal, isOccupied, testMaxExpansions))
	}
}

// unboundedTerrain is an edgeless, uniformly walkable map with one square ring
// of impassable tiles at Chebyshev distance ringRadius around pocketCenter. It
// stands in for a procedurally generated world with no edge, where a search for
// a goal sealed inside the ring can never empty its open set.
type unboundedTerrain struct {
	pocketCenter Coord
	ringRadius   int
}

func (t unboundedTerrain) IsInBounds(x, y int) bool { return true }

func (t unboundedTerrain) IsWalkable(x, y int) bool {
	dx := max(x-t.pocketCenter.X, t.pocketCenter.X-x)
	dy := max(y-t.pocketCenter.Y, t.pocketCenter.Y-y)
	return max(dx, dy) != t.ringRadius
}

func (t unboundedTerrain) GetTerrainSpeedMultiplier(x, y int) float64 {
	if !t.IsWalkable(x, y) {
		return 0
	}
	return 1
}

// TestFindPathBudgetExhausted tests that a goal sealed inside a pocket of an
// edgeless map is given up on once the search has expanded maxExpansions
// nodes, since the open set on such a map never empties. The ring sits two
// tiles out so that the goal's own neighbours are free and the up-front
// approachability check lets the search run.
func TestFindPathBudgetExhausted(t *testing.T) {
	goal := Coord{0, 0}
	terrain := unboundedTerrain{pocketCenter: goal, ringRadius: 2}
	const budget = 1000

	path, expanded := findPath(terrain, Coord{10, 10}, goal, nil, budget)

	assert.Nil(t, path, "Should give up on a goal sealed inside a pocket")
	assert.Equal(t, budget, expanded, "The budget, not an emptied open set, should have stopped the search")
}

// TestFindPathAroundPocket tests that a reachable goal on the edgeless map is
// still found under the same budget, routing around the sealed pocket that
// lies on the straight line to it.
func TestFindPathAroundPocket(t *testing.T) {
	terrain := unboundedTerrain{pocketCenter: Coord{0, 0}, ringRadius: 2}

	path := FindPath(terrain, Coord{10, 10}, Coord{-10, -10}, nil, 1000)

	require.NotNil(t, path, "Should find a path around the pocket")
	assert.Equal(t, Coord{-10, -10}, path[len(path)-1])
	for _, coord := range path {
		assert.True(t, terrain.IsWalkable(coord.X, coord.Y), "Path must avoid the ring: %v", coord)
	}
}

// TestFindPathInsidePocket tests that a goal sealed inside a pocket is still
// reached from a start inside the same pocket.
func TestFindPathInsidePocket(t *testing.T) {
	goal := Coord{0, 0}
	terrain := unboundedTerrain{pocketCenter: goal, ringRadius: 2}

	path := FindPath(terrain, Coord{1, 1}, goal, nil, 1000)

	assert.Equal(t, []Coord{{1, 1}, {0, 0}}, path)
}

// TestFindPathBudgetIsInclusive tests the budget's boundary: a journey that
// needs exactly maxExpansions expansions (one per path tile, the goal
// included) is found, and one expansion fewer loses it. A budget therefore
// means "at most this many expansions".
func TestFindPathBudgetIsInclusive(t *testing.T) {
	terrain := unboundedTerrain{pocketCenter: Coord{100, 100}, ringRadius: 2}
	start := Coord{0, 0}
	goal := Coord{20, 0}
	const pathTiles = 21

	found, expanded := findPath(terrain, start, goal, nil, pathTiles)
	require.Len(t, found, pathTiles)
	assert.Equal(t, pathTiles, expanded)

	assert.Nil(t, FindPath(terrain, start, goal, nil, pathTiles-1), "One expansion short of the goal should return no path")
}

// TestFindPathRequiresPositiveBudget tests that a budget that is not positive
// is a programming error rather than a request for an unbounded search, which
// on an edgeless map would never return.
func TestFindPathRequiresPositiveBudget(t *testing.T) {
	terrain := newMockTerrain(3, 3)
	for y := range 3 {
		for x := range 3 {
			terrain.setWalkable(x, y, true)
		}
	}

	for _, budget := range []int{0, -1} {
		assert.Panics(t, func() { FindPath(terrain, Coord{0, 0}, Coord{2, 2}, nil, budget) }, "budget %d", budget)
	}
}
