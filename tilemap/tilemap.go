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

// Available reports whether id could claim the tile holding position: the tile
// is unreserved or already id's.
func (tom *TileOccupancyManager) Available(id ecs.EntityId, position geometry.Vector2) bool {
	occupant, occupied := tom.occupancy[WorldPositionToTile(position)]
	return !occupied || occupant == id
}

// Claim reserves the tile holding destination for id as it sets off from from,
// and reports false, changing nothing, when another entity holds that tile.
// The tile holding from is cleared whoever holds it, which is how a move has
// always released its origin, so redirecting an entity mid-move strands the
// old destination's reservation and can clear a tile the entity never held.
func (tom *TileOccupancyManager) Claim(id ecs.EntityId, from, destination geometry.Vector2) bool {
	if !tom.Available(id, destination) {
		return false
	}
	tom.ClearOccupant(WorldPositionToTile(from))
	tom.SetOccupant(WorldPositionToTile(destination), id)
	return true
}

// Stop records that id halted at position on its way to destination: the
// destination tile is released if id holds it, and the tile holding position
// becomes id's unless another entity holds it.
func (tom *TileOccupancyManager) Stop(id ecs.EntityId, destination, position geometry.Vector2) {
	destinationTile := WorldPositionToTile(destination)
	if occupant, occupied := tom.occupancy[destinationTile]; occupied && occupant == id {
		tom.ClearOccupant(destinationTile)
	}
	if tom.Available(id, position) {
		tom.SetOccupant(WorldPositionToTile(position), id)
	}
}
