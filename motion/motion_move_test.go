package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestMoveEntity_StartsMove(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	start := s.MoveEntity(id, geometry.NewVector2(3.0, 4.0), MoveOptions{Speed: 2.0})

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

	start := s.MoveEntity(id, dest, MoveOptions{Speed: 1.0})

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

	start := s.MoveEntity(id, tilemap.TileToWorldPosition(destTile), MoveOptions{Speed: 1.0})

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

	start := s.MoveEntity(id, pos, MoveOptions{Speed: 1.0})

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
	s.MoveEntity(w.NewEntity(), geometry.NewVector2(1.0, 0.0), MoveOptions{Speed: 1.0})
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

func TestMoveEntity_RecordsEasingState(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	origin := geometry.NewVector2(1.0, 1.0)
	s.Spatials.Add(id, Spatial{Position: origin})

	start := s.MoveEntity(id, geometry.NewVector2(4.0, 5.0), MoveOptions{Speed: 2.0, Ease: easing.CurveOut})

	if !start.Started() {
		t.Fatalf("expected move to start, got %+v", start)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("expected a Movement component")
	}
	if mc.Ease != easing.CurveOut {
		t.Errorf("expected Ease CurveOut, got %v", mc.Ease)
	}
	if mc.Start != origin {
		t.Errorf("expected Start %v, got %v", origin, mc.Start)
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed 0, got %v", mc.Elapsed)
	}
	if mc.Total != start.Duration {
		t.Errorf("expected Total to match the reported duration %v, got %v", start.Duration, mc.Total)
	}
}

// A constant-speed move records the same bookkeeping, so Progress works
// uniformly, while position still comes from the incremental path.
func TestMoveEntity_RecordsStateForLinearMoves(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	origin := geometry.NewVector2(0.0, 0.0)
	s.Spatials.Add(id, Spatial{Position: origin})

	s.MoveEntity(id, geometry.NewVector2(4.0, 0.0), MoveOptions{Speed: 1.0})

	mc, _ := s.Movements.Get(id)
	if mc.Ease != easing.CurveLinear {
		t.Errorf("expected the zero curve, got %v", mc.Ease)
	}
	if mc.Start != origin || mc.Total != 4*time.Second {
		t.Errorf("expected Start %v and Total 4s, got %v and %v", origin, mc.Start, mc.Total)
	}
}

func TestMoveEntity_RedirectReAnchors(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	s.MoveEntity(id, geometry.NewVector2(10.0, 0.0), MoveOptions{Speed: 1.0, Ease: easing.CurveInOut})
	s.Tick(2 * time.Second)

	sc, _ := s.Spatials.Get(id)
	redirectFrom := sc.Position
	if redirectFrom.X() == 0 {
		t.Fatal("expected the entity to have moved before the redirect")
	}

	start := s.MoveEntity(id, geometry.NewVector2(0.0, 3.0), MoveOptions{Speed: 1.0, Ease: easing.CurveInOut})

	mc, _ := s.Movements.Get(id)
	if mc.Start != redirectFrom {
		t.Errorf("expected Start re-anchored to %v, got %v", redirectFrom, mc.Start)
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed reset to 0, got %v", mc.Elapsed)
	}
	if mc.Total != start.Duration {
		t.Errorf("expected Total %v recomputed from the remaining distance, got %v", start.Duration, mc.Total)
	}

	// No positional jump on the redirecting tick, and arrival exactly one
	// total later.
	s.Tick(0)
	sc, _ = s.Spatials.Get(id)
	if sc.Position != redirectFrom {
		t.Errorf("expected no jump on redirect, got %v want %v", sc.Position, redirectFrom)
	}
	s.Tick(start.Duration)
	sc, _ = s.Spatials.Get(id)
	if sc.Position != geometry.NewVector2(0.0, 3.0) {
		t.Errorf("expected arrival at (0,3), got %v", sc.Position)
	}
}

func TestMoveEntity_PanicsOnNonPositiveSpeed(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for a non-positive speed")
		}
	}()
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	s.MoveEntity(id, geometry.NewVector2(1.0, 0.0), MoveOptions{Speed: 0})
}
