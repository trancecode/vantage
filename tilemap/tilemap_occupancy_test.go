package tilemap

import (
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

func twoEntities() (ecs.EntityId, ecs.EntityId) {
	w := ecs.NewWorld()
	return w.NewEntity(), w.NewEntity()
}

func TestTileOccupancyManager_Available(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	tom.SetOccupant(TileCoord{X: 1, Y: 1}, other)
	tom.SetOccupant(TileCoord{X: 2, Y: 2}, id)

	if !tom.Available(id, geometry.NewVector2(0.2, 0.9)) {
		t.Error("a free tile should be available")
	}
	if tom.Available(id, geometry.NewVector2(1.9, 1.1)) {
		t.Error("a tile another entity holds should not be available")
	}
	if !tom.Available(id, geometry.NewVector2(2.5, 2.5)) {
		t.Error("a tile the entity holds itself should be available")
	}
}

func TestTileOccupancyManager_Claim(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	from, dest := geometry.NewVector2(0.5, 0.5), geometry.NewVector2(2.5, 0.5)
	tom.SetOccupant(TileCoord{X: 0, Y: 0}, id)

	if !tom.Claim(id, from, dest) {
		t.Fatal("claiming a free tile should succeed")
	}
	if tom.IsOccupied(TileCoord{X: 0, Y: 0}) {
		t.Error("claiming should release the origin tile")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != id {
		t.Error("claiming should reserve the destination tile")
	}

	if tom.Claim(other, geometry.NewVector2(3.5, 0.5), dest) {
		t.Error("claiming a tile another entity holds should fail")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != id {
		t.Error("a refused claim should change nothing")
	}
}

func TestTileOccupancyManager_ClaimClearsTheOriginWhoeverHoldsIt(t *testing.T) {
	// MoveEntity has always released the tile the entity stands on without
	// checking who holds it; Claim keeps that.
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	tom.SetOccupant(TileCoord{X: 0, Y: 0}, other)

	tom.Claim(id, geometry.NewVector2(0.5, 0.5), geometry.NewVector2(1.5, 0.5))

	if tom.IsOccupied(TileCoord{X: 0, Y: 0}) {
		t.Error("expected the origin tile cleared")
	}
}

func TestTileOccupancyManager_Stop(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, _ := twoEntities()
	dest, position := geometry.NewVector2(2.5, 0.5), geometry.NewVector2(1.5, 0.5)
	tom.SetOccupant(TileCoord{X: 2, Y: 0}, id)

	tom.Stop(id, dest, position)

	if tom.IsOccupied(TileCoord{X: 2, Y: 0}) {
		t.Error("stopping should release the destination the entity held")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 1, Y: 0}); occupant != id {
		t.Error("stopping should claim the tile the entity stands on")
	}
}

func TestTileOccupancyManager_StopLeavesOtherEntitiesAlone(t *testing.T) {
	tom := NewTileOccupancyManager()
	id, other := twoEntities()
	dest, position := geometry.NewVector2(2.5, 0.5), geometry.NewVector2(1.5, 0.5)
	tom.SetOccupant(TileCoord{X: 2, Y: 0}, other)
	tom.SetOccupant(TileCoord{X: 1, Y: 0}, other)

	tom.Stop(id, dest, position)

	if occupant, _ := tom.GetOccupant(TileCoord{X: 2, Y: 0}); occupant != other {
		t.Error("stopping should not release a destination another entity holds")
	}
	if occupant, _ := tom.GetOccupant(TileCoord{X: 1, Y: 0}); occupant != other {
		t.Error("stopping should not take a tile another entity holds")
	}
}
