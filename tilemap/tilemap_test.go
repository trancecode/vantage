//go:build !race

package tilemap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

func TestWorldPositionToTile(t *testing.T) {
	tests := []struct {
		name     string
		pos      geometry.Vector2
		expected TileCoord
	}{
		{
			name:     "Origin",
			pos:      geometry.NewVector2(0.0, 0.0),
			expected: TileCoord{X: 0, Y: 0},
		},
		{
			name:     "Positive integers",
			pos:      geometry.NewVector2(3.0, 4.0),
			expected: TileCoord{X: 3, Y: 4},
		},
		{
			name:     "Positive decimals",
			pos:      geometry.NewVector2(2.7, 3.9),
			expected: TileCoord{X: 2, Y: 3},
		},
		{
			name:     "Negative integers",
			pos:      geometry.NewVector2(-2.0, -3.0),
			expected: TileCoord{X: -2, Y: -3},
		},
		{
			name:     "Negative decimals",
			pos:      geometry.NewVector2(-1.5, -2.3),
			expected: TileCoord{X: -2, Y: -3},
		},
		{
			name:     "Mixed positive and negative",
			pos:      geometry.NewVector2(1.5, -2.7),
			expected: TileCoord{X: 1, Y: -3},
		},
		{
			name:     "Tile center",
			pos:      geometry.NewVector2(0.5, 0.5),
			expected: TileCoord{X: 0, Y: 0},
		},
		{
			name:     "Near tile boundary",
			pos:      geometry.NewVector2(0.999, 0.999),
			expected: TileCoord{X: 0, Y: 0},
		},
		{
			name:     "Exactly on tile boundary",
			pos:      geometry.NewVector2(1.0, 1.0),
			expected: TileCoord{X: 1, Y: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WorldPositionToTile(tt.pos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTileToWorldPosition(t *testing.T) {
	tests := []struct {
		name     string
		tile     TileCoord
		expected geometry.Vector2
	}{
		{
			name:     "Origin tile",
			tile:     TileCoord{X: 0, Y: 0},
			expected: geometry.NewVector2(0.5, 0.5),
		},
		{
			name:     "Positive tile",
			tile:     TileCoord{X: 3, Y: 4},
			expected: geometry.NewVector2(3.5, 4.5),
		},
		{
			name:     "Negative tile",
			tile:     TileCoord{X: -2, Y: -3},
			expected: geometry.NewVector2(-1.5, -2.5),
		},
		{
			name:     "Mixed tile",
			tile:     TileCoord{X: 1, Y: -2},
			expected: geometry.NewVector2(1.5, -1.5),
		},
		{
			name:     "Large coordinates",
			tile:     TileCoord{X: 100, Y: 200},
			expected: geometry.NewVector2(100.5, 200.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TileToWorldPosition(tt.tile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Test that converting from tile to world and back gives the same tile
	tiles := []TileCoord{
		{X: 0, Y: 0},
		{X: 5, Y: 10},
		{X: -3, Y: -7},
		{X: 100, Y: -50},
	}

	for _, tile := range tiles {
		worldPos := TileToWorldPosition(tile)
		resultTile := WorldPositionToTile(worldPos)
		assert.Equal(t, tile, resultTile, "Round trip conversion failed for tile %v", tile)
	}
}

func TestTileOccupancyManager(t *testing.T) {
	tom := NewTileOccupancyManager()
	assert.NotNil(t, tom)

	tile1 := TileCoord{X: 0, Y: 0}
	tile2 := TileCoord{X: 1, Y: 1}
	tile3 := TileCoord{X: -1, Y: -1}

	w := ecs.NewWorld()
	entity1 := w.NewEntity()
	entity2 := w.NewEntity()

	// Test initial state - tiles should be unoccupied
	occupied, ok := tom.GetOccupant(tile1)
	assert.False(t, ok)
	assert.Equal(t, ecs.EntityId{}, occupied)
	assert.False(t, tom.IsOccupied(tile1))

	// Test setting occupant
	tom.SetOccupant(tile1, entity1)
	occupied, ok = tom.GetOccupant(tile1)
	assert.True(t, ok)
	assert.Equal(t, entity1, occupied)
	assert.True(t, tom.IsOccupied(tile1))

	// Test setting different occupants on different tiles
	tom.SetOccupant(tile2, entity2)
	occupied, ok = tom.GetOccupant(tile2)
	assert.True(t, ok)
	assert.Equal(t, entity2, occupied)

	// Verify tile1 still has entity1
	occupied, ok = tom.GetOccupant(tile1)
	assert.True(t, ok)
	assert.Equal(t, entity1, occupied)

	// Test overwriting occupant
	tom.SetOccupant(tile1, entity2)
	occupied, ok = tom.GetOccupant(tile1)
	assert.True(t, ok)
	assert.Equal(t, entity2, occupied)

	// Test clearing occupant
	tom.ClearOccupant(tile1)
	occupied, ok = tom.GetOccupant(tile1)
	assert.False(t, ok)
	assert.Equal(t, ecs.EntityId{}, occupied)
	assert.False(t, tom.IsOccupied(tile1))

	// Test clearing non-existent occupant (should not panic)
	tom.ClearOccupant(tile3)
	assert.False(t, tom.IsOccupied(tile3))

	// Test negative coordinates
	tom.SetOccupant(tile3, entity1)
	assert.True(t, tom.IsOccupied(tile3))
	occupied, ok = tom.GetOccupant(tile3)
	assert.True(t, ok)
	assert.Equal(t, entity1, occupied)
}

func TestTileOccupancyManagerMultipleEntities(t *testing.T) {
	tom := NewTileOccupancyManager()
	w := ecs.NewWorld()

	// Create a grid of tiles and entities
	entities := make(map[TileCoord]ecs.EntityId)
	for x := range 3 {
		for y := range 3 {
			tile := TileCoord{X: x, Y: y}
			entityId := w.NewEntity()
			entities[tile] = entityId
			tom.SetOccupant(tile, entityId)
		}
	}

	// Verify all tiles are occupied with correct entities
	for x := range 3 {
		for y := range 3 {
			tile := TileCoord{X: x, Y: y}
			expectedEntity := entities[tile]

			occupied, ok := tom.GetOccupant(tile)
			assert.True(t, ok, "Tile (%d, %d) should be occupied", x, y)
			assert.Equal(t, expectedEntity, occupied, "Tile (%d, %d) has wrong entity", x, y)
		}
	}

	// Clear some tiles
	tom.ClearOccupant(TileCoord{X: 1, Y: 1})
	tom.ClearOccupant(TileCoord{X: 0, Y: 2})

	// Verify cleared tiles are empty
	assert.False(t, tom.IsOccupied(TileCoord{X: 1, Y: 1}))
	assert.False(t, tom.IsOccupied(TileCoord{X: 0, Y: 2}))

	// Verify other tiles are still occupied
	assert.True(t, tom.IsOccupied(TileCoord{X: 0, Y: 0}))
	assert.True(t, tom.IsOccupied(TileCoord{X: 2, Y: 2}))
}
