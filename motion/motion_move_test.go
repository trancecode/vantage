package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestMoveEntity_StartsMove(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	start := s.MoveEntity(id, geometry.NewVector2(3.0, 4.0), 2.0)

	if !start.Started() {
		t.Fatalf("expected move to start, got %+v", start)
	}
	if start.Distance != 5.0 {
		t.Errorf("expected distance 5.0, got %v", start.Distance)
	}
	if start.Duration != 2500*time.Millisecond {
		t.Errorf("expected duration 2.5s, got %v", start.Duration)
	}
	mc, ok := s.Movements.Get(id)
	if !ok || mc.Destination != geometry.NewVector2(3.0, 4.0) || mc.Speed != 2.0 {
		t.Errorf("expected Movement toward (3,4) at speed 2.0, got %+v (ok=%v)", mc, ok)
	}
	sc, _ := s.Spatials.Get(id)
	if sc.Direction != geometry.NewVector2(3.0, 4.0) {
		t.Errorf("expected direction toward destination, got %v", sc.Direction)
	}
}

func TestMoveEntity_DestinationOccupied(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	origin := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 0, Y: 0})
	s.Spatials.Add(id, Spatial{Position: origin})
	other := w.NewEntity()
	dest := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 2, Y: 0})
	s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(dest), other)

	start := s.MoveEntity(id, dest, 1.0)

	if start.Outcome != MoveOutcomeDestinationOccupied {
		t.Fatalf("expected MoveOutcomeDestinationOccupied, got %+v", start)
	}
	if s.Movements.Has(id) {
		t.Error("expected no Movement when destination is occupied")
	}
}

func TestMoveEntity_MovesReservation(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	id := w.NewEntity()
	originTile := tilemap.TileCoord{X: 0, Y: 0}
	destTile := tilemap.TileCoord{X: 2, Y: 0}
	origin := tilemap.TileToWorldPosition(originTile)
	s.Spatials.Add(id, Spatial{Position: origin})
	s.Occupancy.SetOccupant(originTile, id)

	start := s.MoveEntity(id, tilemap.TileToWorldPosition(destTile), 1.0)

	if !start.Started() {
		t.Fatalf("expected move to start, got %+v", start)
	}
	if s.Occupancy.IsOccupied(originTile) {
		t.Error("expected origin tile reservation to be cleared")
	}
	occupant, occupied := s.Occupancy.GetOccupant(destTile)
	if !occupied || occupant != id {
		t.Error("expected destination tile to be reserved by the mover")
	}
}

func TestMoveEntity_AlreadyAtDestination(t *testing.T) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	pos := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 1, Y: 1})
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: pos})

	start := s.MoveEntity(id, pos, 1.0)

	if start.Outcome != MoveOutcomeAtDestination {
		t.Fatalf("expected MoveOutcomeAtDestination, got %+v", start)
	}
	occupant, occupied := s.Occupancy.GetOccupant(tilemap.WorldPositionToTile(pos))
	if !occupied || occupant != id {
		t.Error("expected entity to keep its tile reservation")
	}
	if s.Movements.Has(id) {
		t.Error("expected no Movement when already at destination")
	}
}

func TestMoveEntity_PanicsWithoutSpatial(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for entity without Spatial")
		}
	}()
	s, w := newTestSystem()
	s.MoveEntity(w.NewEntity(), geometry.NewVector2(1.0, 0.0), 1.0)
}

func TestFaceDirection_SetsDirection(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	s.FaceDirection(id, geometry.NewVector2(0.0, -1.0))

	sc, _ := s.Spatials.Get(id)
	if sc.Direction != geometry.NewVector2(0.0, -1.0) {
		t.Errorf("expected direction (0,-1), got %v", sc.Direction)
	}
}
