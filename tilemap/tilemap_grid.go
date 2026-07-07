package tilemap

import (
	"math"
	"slices"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// EntitySet is a set of Entity IDs.
type EntitySet map[ecs.EntityId]struct{}

// SpatialGrid divides the game world into cells, allowing for efficient spatial queries.
type SpatialGrid struct {
	// cellSize is the size of each cell in world units.
	cellSize float64
	// cells maps a TileCoord to the set of entities within that cell.
	cells map[TileCoord]EntitySet
}

// NewSpatialGrid creates a new SpatialGrid with the specified cell size.
// cellSize determines the size of each cell in world units.
func NewSpatialGrid(cellSize float64) *SpatialGrid {
	return &SpatialGrid{
		cellSize: cellSize,
		cells:    make(map[TileCoord]EntitySet),
	}
}

// AddEntity adds an entity to the SpatialGrid at the given position.
// entity is the ID of the entity, and position is its world position.
func (sg *SpatialGrid) AddEntity(entity ecs.EntityId, position geometry.Vector2) {
	key := sg.cellCoord(position)
	if _, exists := sg.cells[key]; !exists {
		sg.cells[key] = make(EntitySet)
	}
	sg.cells[key][entity] = struct{}{}
}

// RemoveEntity removes an entity from the SpatialGrid at the given position.
// entity is the ID of the entity, and position is its world position.
func (sg *SpatialGrid) RemoveEntity(entity ecs.EntityId, position geometry.Vector2) {
	key := sg.cellCoord(position)
	if _, exists := sg.cells[key]; !exists {
		return
	}
	delete(sg.cells[key], entity)
}

// UpdateEntityPosition moves an entity from its old position to a new position within the SpatialGrid.
// entity is the ID of the entity, oldPosition is the entity's previous position,
// and newPosition is the entity's updated position.
func (sg *SpatialGrid) UpdateEntityPosition(entity ecs.EntityId, oldPosition, newPosition geometry.Vector2) {
	oldKey := sg.cellCoord(oldPosition)
	newKey := sg.cellCoord(newPosition)
	if oldKey != newKey {
		sg.RemoveEntity(entity, oldPosition)
		sg.AddEntity(entity, newPosition)
	}
}

// GetRange returns all entities within the specified rectangular area, in
// EntityId (allocation) order. The order is part of the contract: cells hold
// entities in sets, and leaking map iteration order to callers would make
// consuming decisions (nearest-candidate ties, processing order) differ
// between identical runs, breaking simulation determinism.
//
// Parameters:
//   - rect: The rectangular area to search within.
//
// Returns:
//   - A slice of EntityId containing all entities within the rectangle.
func (sg *SpatialGrid) GetRange(rect geometry.Rectangle) []ecs.EntityId {
	min := sg.cellCoord(rect.Min)
	max := sg.cellCoord(rect.Max)

	var entities []ecs.EntityId

	for x := min.X; x <= max.X; x++ {
		for y := min.Y; y <= max.Y; y++ {
			key := TileCoord{X: x, Y: y}
			if cell, exists := sg.cells[key]; exists {
				for entity := range cell {
					entities = append(entities, entity)
				}
			}
		}
	}

	slices.SortFunc(entities, ecs.EntityId.Compare)
	return entities
}

// cellCoord calculates the cell TileCoord for a given world position.
// position is the world position to convert into a cell coordinate.
func (sg *SpatialGrid) cellCoord(position geometry.Vector2) TileCoord {
	return TileCoord{
		X: int(math.Floor(position.X() / sg.cellSize)),
		Y: int(math.Floor(position.Y() / sg.cellSize)),
	}
}
