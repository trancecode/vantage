// Package pathfinding implements A* search with terrain awareness.
//
// The algorithm supports 8-directional movement with diagonal cost weighting
// and terrain speed multipliers. Callers provide a TerrainProvider interface
// that answers IsInBounds, IsWalkable, and GetTerrainSpeedMultiplier queries.
// The package has no internal dependencies and operates on its own Coord type.
//
// Cost scales with the length of the path found, not with the size of the map,
// and a goal that cannot be entered is rejected without searching rather than
// costing a full flood of the map. See docs/pathfinding_performance.md for the
// measurements behind that and for the cases still left to the search.
package pathfinding
