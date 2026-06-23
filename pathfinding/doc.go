// Package pathfinding implements A* search with terrain awareness.
//
// The algorithm supports 8-directional movement with diagonal cost weighting
// and terrain speed multipliers. Callers provide a TerrainProvider interface
// that answers IsInBounds, IsWalkable, and GetTerrainSpeedMultiplier queries.
// The package has no internal dependencies and operates on its own Coord type.
package pathfinding
