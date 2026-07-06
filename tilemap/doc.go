// Package tilemap provides tile-based coordinate conversion and occupancy tracking.
//
// TileCoord represents integer tile coordinates. WorldPositionToTile and
// TileToWorldPosition convert between continuous world space (Vector2) and
// discrete tile space. TileOccupancyManager tracks which entity occupies
// each tile, used by movement and pathfinding to avoid collisions.
//
// SpatialGrid partitions the world into configurable-size cells keyed by
// TileCoord for efficient spatial neighbor queries.
//
// TileGrid is dense per-tile storage for map layers over a game-defined value
// type. Terrain adapts a TileGrid to pathfinding.TerrainProvider through a
// game-supplied speed function, so a game's tile types plug into pathfinding
// and motion without engine knowledge of them.
package tilemap
