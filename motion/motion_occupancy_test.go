package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestCancelMove_SettlesTheTileReservation(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	s.Grid.AddEntity(id, geometry.NewVector2(0.5, 0.5))
	ledgerOf(s).SetOccupant(tilemap.TileCoord{X: 0, Y: 0}, id)
	s.MoveEntity(id, geometry.NewVector2(2.5, 0.5), MoveOptions{Speed: 1.0})
	s.Tick(time.Second) // now at (1.5, 0.5), tile (1, 0)

	if !s.CancelMove(id) {
		t.Fatal("expected CancelMove to report an in-flight move")
	}

	if s.Movements.Has(id) {
		t.Error("expected the Movement removed")
	}
	if ledgerOf(s).IsOccupied(tilemap.TileCoord{X: 2, Y: 0}) {
		t.Error("expected the cancelled destination released")
	}
	if occupant, _ := ledgerOf(s).GetOccupant(tilemap.TileCoord{X: 1, Y: 0}); occupant != id {
		t.Error("expected the tile the entity stopped on reserved for it")
	}
}

func TestCancelMove_NoMoveInFlight(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})

	if s.CancelMove(id) {
		t.Error("expected CancelMove to report no move")
	}
	if ledgerOf(s).IsOccupied(tilemap.TileCoord{X: 0, Y: 0}) {
		t.Error("expected CancelMove without a move to change nothing")
	}
}

func TestCancelMove_WithoutOccupancy(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	s.MoveEntity(id, geometry.NewVector2(2.5, 0.5), MoveOptions{Speed: 1.0})

	if !s.CancelMove(id) || s.Movements.Has(id) {
		t.Error("expected the move cancelled")
	}
}

func TestMoveEntity_WithCircleReservations(t *testing.T) {
	s, w := newTestSystem()
	circles := tilemap.NewCircleReservations(1.0)
	s.Occupancy = circles
	mover, blocker := w.NewEntity(), w.NewEntity()
	s.Spatials.Add(mover, Spatial{Position: geometry.NewVector2(0.0, 0.0)})
	circles.Place(mover, geometry.NewVector2(0.0, 0.0), 0.25)
	circles.Place(blocker, geometry.NewVector2(2.0, 0.0), 0.25)

	refused := s.MoveEntity(mover, geometry.NewVector2(1.6, 0.0), MoveOptions{Speed: 1.0})
	if refused.Outcome != MoveOutcomeDestinationOccupied {
		t.Fatalf("a leg ending 0.4 from another body's 0.25 circle should be refused, got %+v", refused)
	}

	// A leg through the blocker's circle to a clear end is fine: bodies pass
	// through each other.
	started := s.MoveEntity(mover, geometry.NewVector2(3.0, 0.0), MoveOptions{Speed: 1.0})
	if !started.Started() {
		t.Fatalf("a leg passing through another body should start, got %+v", started)
	}
	if centre, _, _ := circles.Reservation(mover); centre != geometry.NewVector2(3.0, 0.0) {
		t.Errorf("expected the mover's circle at its leg end, got %v", centre)
	}

	s.Tick(1500 * time.Millisecond) // at (1.5, 0), inside the blocker's circle
	s.CancelMove(mover)
	if centre, _, _ := circles.Reservation(mover); centre != geometry.NewVector2(1.5, 0.0) {
		t.Errorf("expected the mover's circle where it stopped, got %v", centre)
	}
}

func TestCanReach_WithCircleReservationsUsesThePoint(t *testing.T) {
	s, w := newTestSystem()
	circles := tilemap.NewCircleReservations(1.0)
	s.Occupancy = circles
	id, other := w.NewEntity(), w.NewEntity()
	circles.Place(id, geometry.NewVector2(8.0, 8.0), 0.25)
	circles.Place(other, geometry.NewVector2(1.5, 1.5), 0.25)

	if s.CanReach(id, geometry.NewVector2(1.6, 1.5)) {
		t.Error("a point inside another body's reservation should not be reachable")
	}
	if !s.CanReach(id, geometry.NewVector2(1.1, 1.1)) {
		t.Error("a point in the same tile but clear of the other circle should be reachable")
	}
}

func TestCancelMove_PanicsWithoutSpatialAndKeepsTheMovement(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Movements.Add(id, Movement{Destination: geometry.NewVector2(2.5, 0.5)})

	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected a panic for a Movement without a Spatial")
			}
		}()
		s.CancelMove(id)
	}()

	if !s.Movements.Has(id) {
		t.Error("expected the Movement left in place by the panicking call")
	}
}
