// Package pathfinding implements A* search with terrain awareness.
//
// The algorithm supports 8-directional movement with diagonal cost weighting
// and terrain speed multipliers. Callers provide a TerrainProvider interface
// that answers IsInBounds, IsWalkable, and GetTerrainSpeedMultiplier queries.
// The package has no internal dependencies and operates on its own Coord type.
//
// On open terrain at speed 1.0, a search that succeeds costs one node expansion
// per tile of the path it returns, independently of the size of the map. Terrain
// faster or slower than that makes octile distance a poor estimate, so FindPath
// takes a Heuristic and a game chooses its strategy:
//
//   - nil, octile distance: exact on open grass, but routes miss roads faster
//     than 1.0 and searches flood over slow ground such as forest.
//   - ScaledOctile: optimal routes, at about 0.8 x length² expansions on open
//     grass when the declared fastest speed is 2.0.
//   - CoarseCost: near-optimal routes and corridor-shaped searches over both
//     fast and slow ground, learned from the terrain a cell at a time; the
//     first search over new ground pays for reading it.
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
