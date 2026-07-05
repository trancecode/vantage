package motion

import (
	"testing"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// newTestSystem returns a System wired to a fresh ecs world with a spatial
// grid, plus the world for entity allocation. Shared by all motion tests.
func newTestSystem() (*System, *ecs.World) {
	w := ecs.NewWorld()
	s := &System{
		Spatials:  ecs.Components[Spatial](w),
		Movements: ecs.Components[Movement](w),
		Grid:      tilemap.NewSpatialGrid(1.0),
	}
	return s, w
}

// addMovingEntity creates an entity at pos moving toward dest at speed.
func addMovingEntity(s *System, w *ecs.World, pos, dest geometry.Vector2, speed float64) ecs.EntityId {
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: pos})
	s.Movements.Add(id, Movement{Destination: dest, Speed: speed})
	if s.Grid != nil {
		s.Grid.AddEntity(id, pos)
	}
	return id
}

func TestSystemTick_AdvancesPosition(t *testing.T) {
	s, w := newTestSystem()
	id := addMovingEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(10.0, 0.0), 1.0)

	s.Tick(time.Second)

	sc, ok := s.Spatials.Get(id)
	if !ok {
		t.Fatal("expected entity to keep its Spatial")
	}
	want := geometry.NewVector2(1.0, 0.0)
	if sc.Position.DistanceTo(want) > 0.0001 {
		t.Errorf("expected position near %v, got %v", want, sc.Position)
	}
	if !s.Movements.Has(id) {
		t.Error("expected Movement to remain while destination not reached")
	}
}

func TestSystemTick_CompletesMovement(t *testing.T) {
	s, w := newTestSystem()
	var arrivals []MovementResult
	s.OnArrival = func(r MovementResult) { arrivals = append(arrivals, r) }
	id := addMovingEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(1.0, 0.0), 2.0)

	s.Tick(time.Second)

	sc, _ := s.Spatials.Get(id)
	if sc.Position != geometry.NewVector2(1.0, 0.0) {
		t.Errorf("expected position at destination, got %v", sc.Position)
	}
	if s.Movements.Has(id) {
		t.Error("expected Movement to be removed on arrival")
	}
	if len(arrivals) != 1 || arrivals[0].EntityId != id || !arrivals[0].Completed {
		t.Errorf("expected one completed arrival for entity %v, got %+v", id, arrivals)
	}
}

func TestSystemTick_SkipsEntitiesWithoutSpatial(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Movements.Add(id, Movement{Destination: geometry.NewVector2(1.0, 0.0), Speed: 1.0})

	s.Tick(time.Second)

	if !s.Movements.Has(id) {
		t.Error("expected Movement without Spatial to be left untouched")
	}
}

func TestSystemTick_UpdatesGrid(t *testing.T) {
	s, w := newTestSystem()
	id := addMovingEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(5.0, 0.0), 1.0)

	s.Tick(time.Second)

	around := geometry.NewRectangleFromPoints(0.5, -0.5, 1.5, 0.5)
	found := false
	for _, e := range s.Grid.GetRange(around) {
		if e == id {
			found = true
		}
	}
	if !found {
		t.Errorf("expected grid range %v to contain entity after move", around)
	}
}

func TestSystemTick_NilGridIsAllowed(t *testing.T) {
	s, w := newTestSystem()
	s.Grid = nil
	id := addMovingEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(1.0, 0.0), 2.0)

	s.Tick(time.Second)

	if s.Movements.Has(id) {
		t.Error("expected movement to complete without a grid")
	}
}
