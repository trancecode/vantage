package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// testTerrain is a rectangular map with optional blocked tiles.
type testTerrain struct {
	width, height int
	blocked       map[tilemap.TileCoord]bool
}

func (tt *testTerrain) IsInBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < tt.width && y < tt.height
}

func (tt *testTerrain) IsWalkable(x, y int) bool {
	return !tt.blocked[tilemap.TileCoord{X: x, Y: y}]
}

func (tt *testTerrain) GetTerrainSpeedMultiplier(x, y int) float64 {
	if tt.IsWalkable(x, y) {
		return 1.0
	}
	return 0.0
}

func TestCanReach_ChecksTerrainAndOccupancy(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	s.Terrain = &testTerrain{width: 10, height: 10, blocked: map[tilemap.TileCoord]bool{{X: 3, Y: 3}: true}}
	id := w.NewEntity()
	other := w.NewEntity()
	s.Occupancy.SetOccupant(tilemap.TileCoord{X: 5, Y: 5}, other)
	s.Occupancy.SetOccupant(tilemap.TileCoord{X: 6, Y: 6}, id)

	cases := []struct {
		name string
		tile tilemap.TileCoord
		want bool
	}{
		{"free walkable tile", tilemap.TileCoord{X: 1, Y: 1}, true},
		{"blocked tile", tilemap.TileCoord{X: 3, Y: 3}, false},
		{"out of bounds", tilemap.TileCoord{X: -1, Y: 0}, false},
		{"occupied by other", tilemap.TileCoord{X: 5, Y: 5}, false},
		{"occupied by self", tilemap.TileCoord{X: 6, Y: 6}, true},
	}
	for _, c := range cases {
		got := s.CanReach(id, tilemap.TileToWorldPosition(c.tile))
		if got != c.want {
			t.Errorf("%s: expected CanReach=%v, got %v", c.name, c.want, got)
		}
	}
}

func TestCanReach_NilTerrainIsAllWalkable(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()

	if !s.CanReach(id, geometry.NewVector2(100.0, 100.0)) {
		t.Error("expected any tile to be reachable with nil Terrain and nil Occupancy")
	}
}

func TestFindTilePath_StraightLine(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}

	path := s.FindTilePath(tilemap.TileCoord{X: 0, Y: 0}, tilemap.TileCoord{X: 3, Y: 0})

	if len(path) == 0 {
		t.Fatal("expected a path on open terrain")
	}
	last := path[len(path)-1]
	if last != (tilemap.TileCoord{X: 3, Y: 0}) {
		t.Errorf("expected path to end at goal, got %v", last)
	}
}

func TestFindTilePath_AvoidsOccupiedTiles(t *testing.T) {
	s, w := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	s.Occupancy = tilemap.NewTileOccupancyManager()
	blocker := w.NewEntity()
	s.Occupancy.SetOccupant(tilemap.TileCoord{X: 1, Y: 0}, blocker)

	path := s.FindTilePath(tilemap.TileCoord{X: 0, Y: 0}, tilemap.TileCoord{X: 3, Y: 0})

	for _, tile := range path {
		if tile == (tilemap.TileCoord{X: 1, Y: 0}) {
			t.Error("expected path to route around the occupied tile")
		}
	}
}

func TestFindPathBetween_EndsAtTileCenter(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	origin := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 0, Y: 0})
	destination := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 2})

	path := s.FindPathBetween(origin, destination)

	if len(path) == 0 {
		t.Fatal("expected a path on open terrain")
	}
	if path[len(path)-1] != destination {
		t.Errorf("expected final waypoint at destination tile center %v, got %v", destination, path[len(path)-1])
	}
	if path[0] == origin {
		t.Errorf("expected origin tile to be skipped, got first waypoint %v", path[0])
	}
}

func TestFindPathBetween_RecordsPhase(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	var phases []string
	s.RecordPhase = func(name string, _ time.Duration) { phases = append(phases, name) }

	s.FindPathBetween(tilemap.TileToWorldPosition(tilemap.TileCoord{X: 0, Y: 0}), tilemap.TileToWorldPosition(tilemap.TileCoord{X: 2, Y: 0}))

	if len(phases) != 1 || phases[0] != "pathfinding" {
		t.Errorf("expected one 'pathfinding' phase record, got %v", phases)
	}
}

func TestFindPathBetween_SameTileReturnsTileCenter(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 2, Y: 2})
	origin := center.Add(geometry.NewVector2(0.2, 0.1))

	path := s.FindPathBetween(origin, center)

	if len(path) != 1 || path[0] != center {
		t.Errorf("expected single waypoint at tile center %v, got %v", center, path)
	}
}

func TestFindPathBetween_SameTileAtCenterReturnsEmpty(t *testing.T) {
	s, _ := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 2, Y: 2})
	destination := center.Add(geometry.NewVector2(0.2, 0.1))

	path := s.FindPathBetween(center, destination)

	if len(path) != 0 {
		t.Errorf("expected empty path when already at the tile center, got %v", path)
	}
}
