package motion

import (
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
)

func TestMoveEntity_WithDuration(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	start := s.MoveEntity(id, geometry.NewVector2(3.0, 4.0), MoveOptions{Duration: 2 * time.Second})

	if !start.Started() || start.Duration != 2*time.Second || start.Distance != 5.0 {
		t.Fatalf("expected a 2s move over 5 tiles, got %+v", start)
	}
	mc, _ := s.Movements.Get(id)
	if !mc.Timed || mc.Total != 2*time.Second || mc.Speed != 2.5 {
		t.Errorf("expected a timed Movement of 2s at 2.5 tiles/s, got %+v", mc)
	}
}

func TestTimedMove_EndsExactlyAtItsDuration(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.5, 0.5)})
	dest := geometry.NewVector2(1.7, 1.3)
	s.MoveEntity(id, dest, MoveOptions{Duration: 2 * time.Second})

	s.Tick(1999 * time.Millisecond)
	if !s.Movements.Has(id) {
		t.Fatal("expected the move still in flight a millisecond before its end")
	}
	s.Tick(time.Millisecond)
	if s.Movements.Has(id) {
		t.Fatal("expected the move complete at its duration")
	}
	if sc, _ := s.Spatials.Get(id); sc.Position != dest {
		t.Errorf("expected the body exactly at %v, got %v", dest, sc.Position)
	}
}

func TestTimedMove_IsIndependentOfTickSlicing(t *testing.T) {
	from, dest := geometry.NewVector2(0.5, 0.5), geometry.NewVector2(1.7, 1.3)
	newMover := func() (*System, ecsEntity) {
		s, w := newTestSystem()
		id := w.NewEntity()
		s.Spatials.Add(id, Spatial{Position: from})
		s.MoveEntity(id, dest, MoveOptions{Duration: 1300 * time.Millisecond})
		return s, ecsEntity{id: id, world: w}
	}
	whole, a := newMover()
	sliced, b := newMover()

	whole.Tick(700 * time.Millisecond)
	for range 7 {
		sliced.Tick(100 * time.Millisecond)
	}
	wholePos, _ := whole.Spatials.Get(a.id)
	slicedPos, _ := sliced.Spatials.Get(b.id)
	if wholePos.Position != slicedPos.Position {
		t.Fatalf("mid-leg positions differ: one tick %v, seven ticks %v", wholePos.Position, slicedPos.Position)
	}

	whole.Tick(600 * time.Millisecond)
	for range 6 {
		sliced.Tick(100 * time.Millisecond)
	}
	if whole.Movements.Has(a.id) || sliced.Movements.Has(b.id) {
		t.Error("expected both moves complete at 1.3s")
	}
}

func TestMoveEntity_PanicsWithNeitherSpeedNorDuration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic with neither a speed nor a duration")
		}
	}()
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})
	s.MoveEntity(id, geometry.NewVector2(1.0, 0.0), MoveOptions{})
}
