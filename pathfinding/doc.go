// Package pathfinding implements A* search with terrain awareness.
//
// The algorithm supports 8-directional movement with diagonal cost weighting
// and terrain speed multipliers. Callers provide a TerrainProvider interface
// that answers IsInBounds, IsWalkable, and GetTerrainSpeedMultiplier queries.
// The package has no internal dependencies and operates on its own Coord type.
//
// A search that succeeds costs one node expansion per tile of the path it
// returns, independently of the size of the map. A search that fails has to
// expand every reachable tile to establish that, so the two ways a goal is
// commonly unenterable — its own tile occupied, or every tile next to it
// unenterable — are answered without searching at all. Any other unreachable
// goal still costs a full flood. See docs/pathfinding_performance.md for the
// measurements behind this.
package pathfinding
