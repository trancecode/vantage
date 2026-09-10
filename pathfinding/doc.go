// Package pathfinding implements A* search with terrain awareness.
//
// The algorithm supports 8-directional movement with diagonal cost weighting
// and terrain speed multipliers. Callers provide a TerrainProvider interface
// that answers IsInBounds, IsWalkable, and GetTerrainSpeedMultiplier queries.
// The package has no internal dependencies and operates on its own Coord type.
//
// On open terrain no faster than 1.0, a search that succeeds costs one node
// expansion per tile of the path it returns, independently of the size of the
// map. A terrain with faster tiles, such as roads, can implement
// MaxSpeedProvider so that routes onto them are found, at the price of a
// search that expands a region rather than a corridor.
//
// A search that fails has to expand every reachable tile to establish that, so
// the two ways a goal is commonly unenterable — its own tile occupied, or every
// tile next to it unenterable — are answered without searching at all. Any
// other unreachable goal costs a flood, bounded by the expansion budget the
// caller passes to FindPath. The budget is also what makes a search return at
// all on a terrain with no edge, where IsInBounds is always true and a flood
// would otherwise never run out of tiles. See docs/pathfinding_performance.md
// for the measurements behind this.
package pathfinding
