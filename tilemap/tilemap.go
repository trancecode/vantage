package tilemap

import (
	"math"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// TileCoord represents integer tile coordinates in the game world
type TileCoord struct {
	X, Y int
}

// TileOccupancyManager tracks which entity occupies each tile
type TileOccupancyManager struct {
	occupancy map[TileCoord]ecs.EntityId // Which entity occupies each tile
}

// WorldPositionToTile converts world coordinates to tile coordinates (1.0-unit tiles)
func WorldPositionToTile(pos geometry.Vector2) TileCoord {
	return TileCoord{
		X: int(math.Floor(pos.X())),
		Y: int(math.Floor(pos.Y())),
	}
}

// TileToWorldPosition converts tile coordinates to world coordinates (tile centers)
func TileToWorldPosition(tile TileCoord) geometry.Vector2 {
	return geometry.NewVector2(
		float64(tile.X)+0.5, // Center of 1.0-unit tile
		float64(tile.Y)+0.5,
	)
}

// NewTileOccupancyManager creates a new tile occupancy manager
func NewTileOccupancyManager() *TileOccupancyManager {
	return &TileOccupancyManager{
		occupancy: make(map[TileCoord]ecs.EntityId),
	}
}

// GetOccupant returns the entity ID occupying the given tile and whether it's occupied
func (tom *TileOccupancyManager) GetOccupant(tile TileCoord) (ecs.EntityId, bool) {
	entityId, occupied := tom.occupancy[tile]
	return entityId, occupied
}

// SetOccupant sets the entity occupying the given tile
func (tom *TileOccupancyManager) SetOccupant(tile TileCoord, entityId ecs.EntityId) {
	tom.occupancy[tile] = entityId
}

// ClearOccupant removes any entity from the given tile
func (tom *TileOccupancyManager) ClearOccupant(tile TileCoord) {
	delete(tom.occupancy, tile)
}

// IsOccupied returns true if the tile is occupied by any entity
func (tom *TileOccupancyManager) IsOccupied(tile TileCoord) bool {
	_, occupied := tom.occupancy[tile]
	return occupied
}
