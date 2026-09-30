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
