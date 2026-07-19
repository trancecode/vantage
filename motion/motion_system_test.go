package motion

import (
	"testing"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/easing"
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

// addEasedEntity creates an entity at pos on an eased move toward dest, with
// Total derived the way MoveEntity derives it.
func addEasedEntity(s *System, w *ecs.World, pos, dest geometry.Vector2, speed float64, curve easing.Curve) ecs.EntityId {
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: pos})
	s.Movements.Add(id, Movement{
		Destination: dest,
		Speed:       speed,
		Ease:        curve,
		Start:       pos,
		Total:       time.Duration(pos.DistanceTo(dest) / speed * float64(time.Second)),
	})
	if s.Grid != nil {
		s.Grid.AddEntity(id, pos)
	}
	return id
}

func TestSystemTick_AdvancesEasedPosition(t *testing.T) {
	s, w := newTestSystem()
	// Total 4s; after 1s, smoothstep(0.25) = 0.15625 of 4 tiles = 0.625.
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(4.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(time.Second)

	sc, _ := s.Spatials.Get(id)
	if sc.Position.DistanceTo(geometry.NewVector2(0.625, 0.0)) > 1e-9 {
		t.Errorf("expected eased position near (0.625, 0), got %v", sc.Position)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("expected the Movement to remain in flight")
	}
	if mc.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s written back to the component, got %v", mc.Elapsed)
	}
}

func TestSystemTick_CompletesEasedMovementAtExactDestination(t *testing.T) {
	s, w := newTestSystem()
	var arrivals []MovementResult
	s.OnArrival = func(r MovementResult) { arrivals = append(arrivals, r) }
	dest := geometry.NewVector2(2.0, 0.0)
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), dest, 1.0, easing.CurveOut)

	// Total is 2s; four 500ms ticks.
	for range 4 {
		s.Tick(500 * time.Millisecond)
	}

	sc, _ := s.Spatials.Get(id)
	if sc.Position != dest {
		t.Errorf("expected exact arrival at %v, got %v", dest, sc.Position)
	}
	if s.Movements.Has(id) {
		t.Error("expected the Movement to be removed on arrival")
	}
	if len(arrivals) != 1 || arrivals[0].EntityId != id {
		t.Errorf("expected one arrival for entity %v, got %+v", id, arrivals)
	}
}

func TestSystemTick_ZeroDurationDoesNotAdvanceEasedMove(t *testing.T) {
	s, w := newTestSystem()
	pos := geometry.NewVector2(0.0, 0.0)
	id := addEasedEntity(s, w, pos, geometry.NewVector2(2.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(0)

	sc, _ := s.Spatials.Get(id)
	if sc.Position != pos {
		t.Errorf("expected no movement on a zero-duration tick, got %v", sc.Position)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("a zero-duration tick must not complete a move")
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed to stay 0, got %v", mc.Elapsed)
	}
}

func TestSystemTick_UpdatesGridOnEasedPath(t *testing.T) {
	s, w := newTestSystem()
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(5.0, 0.0), 1.0, easing.CurveOut)

	s.Tick(time.Second)

	sc, _ := s.Spatials.Get(id)
	around := geometry.NewRectangleFromPoints(sc.Position.X()-0.1, sc.Position.Y()-0.1, sc.Position.X()+0.1, sc.Position.Y()+0.1)
	found := false
	for _, e := range s.Grid.GetRange(around) {
		if e == id {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the grid to track the eased entity at %v", sc.Position)
	}
}

// Consumers cancel a move by removing the component; the body must be left at
// a valid position with nothing owed.
func TestSystemTick_CancelledEasedMoveLeavesPosition(t *testing.T) {
	s, w := newTestSystem()
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(4.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(time.Second)
	sc, _ := s.Spatials.Get(id)
	cancelled := sc.Position
	s.Movements.Remove(id)
	s.Tick(time.Second)

	sc, _ = s.Spatials.Get(id)
	if sc.Position != cancelled {
		t.Errorf("expected the body to stay at %v after cancelling, got %v", cancelled, sc.Position)
	}
}
