package motion

import (
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/tilemap"
)

const testMaxMoveActionDistance = math.Sqrt2 + 0.0001

type ecsEntity struct {
	id    ecs.EntityId
	world *ecs.World
}

// newPathTestSystem returns a System on a 10x10 open map with occupancy
// tracking and one entity placed at the given tile.
func newPathTestSystem(t *testing.T, at tilemap.TileCoord) (*System, ecsEntity) {
	t.Helper()
	s, w := newTestSystem()
	s.Terrain = &testTerrain{width: 10, height: 10}
	s.Occupancy = tilemap.NewTileOccupancyManager()
	s.MaxMoveActionDistance = testMaxMoveActionDistance
	id := w.NewEntity()
	pos := tilemap.TileToWorldPosition(at)
	s.Spatials.Add(id, Spatial{Position: pos})
	s.Occupancy.SetOccupant(at, id)
	return s, ecsEntity{id: id, world: w}
}

func TestMoveEntityTowards_TakesOneStepAlongPath(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 0, Y: 0})
	target := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 5, Y: 0})

	start := s.MoveEntityTowards(e.id, target, 1.0)

	if !start.Started() {
		t.Fatalf("expected move to start, got %+v", start)
	}
	if start.Distance > s.MaxMoveActionDistance {
		t.Errorf("expected single step within %v tiles, got %v", s.MaxMoveActionDistance, start.Distance)
	}
	mc, ok := s.Movements.Get(e.id)
	if !ok {
		t.Fatal("expected a Movement to be set")
	}
	want := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 1, Y: 0})
	if mc.Destination != want {
		t.Errorf("expected first waypoint %v, got %v", want, mc.Destination)
	}
}

func TestMoveEntityTowards_RoutesAroundBlockedWaypoint(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 0, Y: 0})
	// Wall off the direct east route so the step must deviate.
	s.Terrain.(*testTerrain).blocked = map[tilemap.TileCoord]bool{
		{X: 1, Y: 0}: true,
	}
	target := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 3, Y: 0})

	start := s.MoveEntityTowards(e.id, target, 1.0)

	if !start.Started() {
		t.Fatalf("expected move to start around the blocked tile, got %+v", start)
	}
	blocked := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 1, Y: 0})
	if start.Destination == blocked {
		t.Error("expected step to avoid the blocked tile")
	}
}

func TestMoveEntityTowards_NoPath(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 0, Y: 0})
	// Box the entity in completely.
	s.Terrain.(*testTerrain).blocked = map[tilemap.TileCoord]bool{
		{X: 1, Y: 0}: true,
		{X: 0, Y: 1}: true,
		{X: 1, Y: 1}: true,
	}
	target := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 5, Y: 5})

	start := s.MoveEntityTowards(e.id, target, 1.0)

	if start.Started() {
		t.Fatalf("expected no move when boxed in, got %+v", start)
	}
	if s.Movements.Has(e.id) {
		t.Error("expected no Movement when boxed in")
	}
}

func TestMoveEntityTowardsArea_AlreadyInside(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 5, Y: 5})
	center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 5, Y: 6})

	start := s.MoveEntityTowardsArea(e.id, center, 2.0, 1.0)

	if start.Outcome != MoveOutcomeAtDestination {
		t.Fatalf("expected MoveOutcomeAtDestination inside the area, got %+v", start)
	}
}

func TestMoveEntityTowardsArea_StepsTowardArea(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 0, Y: 0})
	center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 6, Y: 0})

	start := s.MoveEntityTowardsArea(e.id, center, 1.0, 1.0)

	if !start.Started() {
		t.Fatalf("expected a step toward the area, got %+v", start)
	}
	if start.Distance > s.MaxMoveActionDistance {
		t.Errorf("expected single bounded step, got distance %v", start.Distance)
	}
}
