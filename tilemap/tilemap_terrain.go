package tilemap

import "fmt"

// TileGrid is a dense width×height grid of per-tile values, the storage for
// map layers such as terrain types, fog, or decoration. The value type is
// game-defined; the engine only stores and addresses it. Tiles start as the
// zero value of T. Contrast with SpatialGrid, which is a sparse index of
// entities by cell, not per-tile storage.
type TileGrid[T any] struct {
	width  int
	height int
	tiles  []T
}

// NewTileGrid returns a width×height grid with every tile at the zero value.
func NewTileGrid[T any](width, height int) *TileGrid[T] {
	return &TileGrid[T]{
		width:  width,
		height: height,
		tiles:  make([]T, width*height),
	}
}

// Width returns the grid width in tiles.
func (g *TileGrid[T]) Width() int { return g.width }

// Height returns the grid height in tiles.
func (g *TileGrid[T]) Height() int { return g.height }

// IsInBounds reports whether (x, y) addresses a tile of the grid.
func (g *TileGrid[T]) IsInBounds(x, y int) bool {
	return x >= 0 && x < g.width && y >= 0 && y < g.height
}

// At returns the value of the tile at (x, y). Like slice indexing, it panics
// out of bounds; guard with IsInBounds when the coordinates are not known to
// be valid.
func (g *TileGrid[T]) At(x, y int) T {
	g.mustBeInBounds(x, y)
	return g.tiles[y*g.width+x]
}

// Set stores the value of the tile at (x, y). Like slice indexing, it panics
// out of bounds.
func (g *TileGrid[T]) Set(x, y int, v T) {
	g.mustBeInBounds(x, y)
	g.tiles[y*g.width+x] = v
}

// Fill sets every tile to v.
func (g *TileGrid[T]) Fill(v T) {
	for i := range g.tiles {
		g.tiles[i] = v
	}
}

func (g *TileGrid[T]) mustBeInBounds(x, y int) {
	if !g.IsInBounds(x, y) {
		panic(fmt.Sprintf("tilemap.TileGrid: (%d, %d) out of bounds (%d×%d)", x, y, g.width, g.height))
	}
}

// Terrain adapts a TileGrid to terrain queries: it satisfies
// pathfinding.TerrainProvider, so it plugs into the pathfinding helpers and
// motion.System directly. Speed maps a tile value to its movement-speed
// multiplier; 0 (or below) means impassable, so walkability needs no second
// function. Out-of-bounds tiles are impassable with zero speed.
type Terrain[T any] struct {
	// Tiles is the underlying per-tile storage.
	Tiles *TileGrid[T]

	// Speed maps a tile value to its movement-speed multiplier, 0 or below
	// meaning impassable.
	Speed func(T) float64
}

// IsInBounds reports whether (x, y) addresses a tile.
func (t Terrain[T]) IsInBounds(x, y int) bool {
	return t.Tiles.IsInBounds(x, y)
}

// IsWalkable reports whether (x, y) is in bounds with a positive speed.
func (t Terrain[T]) IsWalkable(x, y int) bool {
	return t.GetTerrainSpeedMultiplier(x, y) > 0
}

// GetTerrainSpeedMultiplier returns the movement-speed multiplier at (x, y),
// or 0 when the tile is out of bounds.
func (t Terrain[T]) GetTerrainSpeedMultiplier(x, y int) float64 {
	if !t.Tiles.IsInBounds(x, y) {
		return 0
	}
	return t.Speed(t.Tiles.At(x, y))
}
