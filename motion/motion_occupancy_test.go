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
