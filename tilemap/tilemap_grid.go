package tilemap

import (
	"math"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// EntitySet is a set of Entity IDs.
type EntitySet map[ecs.EntityId]struct{}

// SpatialGrid divides the game world into cells, allowing for efficient spatial queries.
type SpatialGrid struct {
	// CellSize is the size of each cell in world units.
	CellSize float64
	// Cells is a map of TileCoord to a set of entities within that cell.
	Cells map[TileCoord]EntitySet
}

// NewSpatialGrid creates a new SpatialGrid with the specified cell size.
// cellSize determines the size of each cell in world units.
func NewSpatialGrid(cellSize float64) *SpatialGrid {
	return &SpatialGrid{
		CellSize: cellSize,
		Cells:    make(map[TileCoord]EntitySet),
	}
}

// AddEntity adds an entity to the SpatialGrid at the given position.
// entity is the ID of the entity, and position is its world position.
func (sg *SpatialGrid) AddEntity(entity ecs.EntityId, position geometry.Vector2) {
	key := sg.cellCoord(position)
	if _, exists := sg.Cells[key]; !exists {
		sg.Cells[key] = make(EntitySet)
	}
	sg.Cells[key][entity] = struct{}{}
}

// RemoveEntity removes an entity from the SpatialGrid at the given position.
// entity is the ID of the entity, and position is its world position.
func (sg *SpatialGrid) RemoveEntity(entity ecs.EntityId, position geometry.Vector2) {
	key := sg.cellCoord(position)
	if _, exists := sg.Cells[key]; !exists {
		return
	}
	delete(sg.Cells[key], entity)
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

// GetRange returns all entities within the specified rectangular area.
// It iterates through the cells that intersect with the rectangle and collects entities from those cells.
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
			if cell, exists := sg.Cells[key]; exists {
				for entity := range cell {
					entities = append(entities, entity)
				}
			}
		}
	}

	return entities
}

// cellCoord calculates the cell TileCoord for a given world position.
// position is the world position to convert into a cell coordinate.
func (sg *SpatialGrid) cellCoord(position geometry.Vector2) TileCoord {
	return TileCoord{
		X: int(math.Floor(position.X() / sg.CellSize)),
		Y: int(math.Floor(position.Y() / sg.CellSize)),
	}
}
