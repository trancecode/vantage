package tilemap

import (
	"reflect"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

func TestSpatialGrid_AddRemoveEntity(t *testing.T) {
	grid := NewSpatialGrid(10)
	w := ecs.NewWorld()
	entityID := w.NewEntity()
	pos := geometry.NewVector2(5, 5)

	grid.AddEntity(entityID, pos)

	key := grid.cellCoord(pos)
	if _, ok := grid.cells[key][entityID]; !ok {
		t.Errorf("Entity not added to grid correctly")
	}

	grid.RemoveEntity(entityID, pos)

	if _, ok := grid.cells[key][entityID]; ok {
		t.Errorf("Entity not removed from grid correctly")
	}
}

func TestSpatialGrid_UpdateEntityPosition(t *testing.T) {
	grid := NewSpatialGrid(10)
	w := ecs.NewWorld()
	entityID := w.NewEntity()
	oldPos := geometry.NewVector2(5, 5)
	newPos := geometry.NewVector2(15, 15)

	grid.AddEntity(entityID, oldPos)
	oldKey := grid.cellCoord(oldPos)
	newKey := grid.cellCoord(newPos)

	if _, ok := grid.cells[oldKey][entityID]; !ok {
		t.Errorf("Entity not added to grid correctly at old position")
	}

	grid.UpdateEntityPosition(entityID, oldPos, newPos)

	if _, ok := grid.cells[oldKey][entityID]; ok {
		t.Errorf("Entity not removed from old position correctly")
	}

	if _, ok := grid.cells[newKey][entityID]; !ok {
		t.Errorf("Entity not added to new position correctly")
	}
}

func TestSpatialGrid_GetRange(t *testing.T) {
	grid := NewSpatialGrid(10)
	w := ecs.NewWorld()

	// Add some entities to the grid. Entities are allocated in the order they
	// are added so the first two (within the queried range) keep their relative
	// ordering when GetRange iterates cells {0,0} then {1,1}.
	e1 := w.NewEntity()
	e2 := w.NewEntity()
	e3 := w.NewEntity()
	e4 := w.NewEntity()
	e5 := w.NewEntity()

	grid.AddEntity(e1, geometry.NewVector2(5, 5))
	grid.AddEntity(e2, geometry.NewVector2(15, 15))
	grid.AddEntity(e3, geometry.NewVector2(25, 25))
	grid.AddEntity(e4, geometry.NewVector2(35, 35))
	grid.AddEntity(e5, geometry.NewVector2(20, 20))

	// Query rect within cells {0,0} and {1,1} only (Max stays within cell 1)
	rect := geometry.Rectangle{
		Min: geometry.NewVector2(0, 0),
		Max: geometry.NewVector2(19, 19),
	}

	foundEntities := grid.GetRange(rect)

	expectedEntities := []ecs.EntityId{e1, e2}
	if !reflect.DeepEqual(foundEntities, expectedEntities) {
		t.Errorf("Incorrect entities found in range. Expected: %v, Got: %v\nGrid Cells: %v", expectedEntities, foundEntities, grid.cells)
	}
}

func TestSpatialGrid_GetRange_IncludesBoundaryCell(t *testing.T) {
	grid := NewSpatialGrid(10)
	w := ecs.NewWorld()

	e1 := w.NewEntity()
	e2 := w.NewEntity()

	grid.AddEntity(e1, geometry.NewVector2(5, 5))
	grid.AddEntity(e2, geometry.NewVector2(15, 15))

	// Entity 2 is at (15,15) in cell {1,1}. A query rect ending at (15,15)
	// should include cell {1,1} (max.X = floor(15/10) = 1).
	rect := geometry.Rectangle{
		Min: geometry.NewVector2(0, 0),
		Max: geometry.NewVector2(15, 15),
	}

	foundEntities := grid.GetRange(rect)

	expectedEntities := []ecs.EntityId{e1, e2}
	if !reflect.DeepEqual(foundEntities, expectedEntities) {
		t.Errorf("Expected boundary cell to be included. Expected: %v, Got: %v", expectedEntities, foundEntities)
	}
}

// TestSpatialGrid_NegativePosition locks in the math.Floor cell-keying
// semantics for negative coordinates. Flooring sends a position in [-1, 0)
// to cell -1 (whereas truncation toward zero would place it in cell 0).
func TestSpatialGrid_NegativePosition(t *testing.T) {
	grid := NewSpatialGrid(1.0)
	w := ecs.NewWorld()
	entityID := w.NewEntity()

	pos := geometry.NewVector2(-0.5, -0.5)
	if got := grid.cellCoord(pos); got != (TileCoord{X: -1, Y: -1}) {
		t.Errorf("cellCoord(%v) = %v, want {-1 -1}", pos, got)
	}

	grid.AddEntity(entityID, pos)
	if _, ok := grid.cells[TileCoord{X: -1, Y: -1}][entityID]; !ok {
		t.Errorf("Entity not stored in floored negative cell")
	}

	// A range covering negative space must find the entity.
	rect := geometry.Rectangle{
		Min: geometry.NewVector2(-2, -2),
		Max: geometry.NewVector2(0, 0),
	}
	found := grid.GetRange(rect)
	if !reflect.DeepEqual(found, []ecs.EntityId{entityID}) {
		t.Errorf("Expected entity in negative range. Got: %v", found)
	}
}
