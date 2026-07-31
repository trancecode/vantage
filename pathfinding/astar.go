package pathfinding

import (
	"container/heap"
	"math"
	"slices"
)

// Coord represents a 2D coordinate in the pathfinding grid.
type Coord struct {
	X, Y int
}

// TerrainProvider is an interface for querying terrain properties.
// Any terrain implementation can provide pathfinding by implementing this interface.
type TerrainProvider interface {
	// IsInBounds checks if the coordinates are within valid map bounds.
	IsInBounds(x, y int) bool

	// IsWalkable checks if the tile at the coordinates is walkable.
	IsWalkable(x, y int) bool

	// GetTerrainSpeedMultiplier returns the movement speed multiplier for the tile.
	// Higher values indicate faster movement, 0.0 indicates impassable terrain.
	GetTerrainSpeedMultiplier(x, y int) float64
}

// pathNode represents a node in the A* pathfinding algorithm
type pathNode struct {
	coord  Coord
	g      float64   // Cost from start to this node
	h      float64   // Heuristic cost from this node to goal
	f      float64   // Total cost (g + h)
	parent *pathNode // Parent node in the path
	index  int       // Index in the priority queue, -1 when not queued
	closed bool      // Expanded already, so no better route to it remains
}

// pathNodeQueue implements a priority queue for A* pathfinding
type pathNodeQueue []*pathNode

func (pq pathNodeQueue) Len() int { return len(pq) }

func (pq pathNodeQueue) Less(i, j int) bool {
	return pq[i].f < pq[j].f
}

func (pq pathNodeQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *pathNodeQueue) Push(x any) {
	n := len(*pq)
	node := x.(*pathNode)
	node.index = n
	*pq = append(*pq, node)
}

func (pq *pathNodeQueue) Pop() any {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.index = -1
	*pq = old[0 : n-1]
	return node
}

// Direction vectors for 8-directional movement (N, NE, E, SE, S, SW, W, NW)
var directions = []Coord{
	{0, -1},  // N
	{1, -1},  // NE
	{1, 0},   // E
	{1, 1},   // SE
	{0, 1},   // S
	{-1, 1},  // SW
	{-1, 0},  // W
	{-1, -1}, // NW
}

// Movement costs: cardinal = 1.0, diagonal = sqrt(2)
const (
	cardinalCost = 1.0
	diagonalCost = math.Sqrt2
)

// isCardinalDirection returns true if the direction is cardinal (N, E, S, W)
func isCardinalDirection(dx, dy int) bool {
	return (dx == 0 && dy != 0) || (dx != 0 && dy == 0)
}

// heuristic calculates the estimated cost from one coord to another using octile distance.
func heuristic(from, to Coord) float64 {
	dx := math.Abs(float64(to.X - from.X))
	dy := math.Abs(float64(to.Y - from.Y))

	// Octile distance: diagonal moves cost sqrt(2), cardinal moves cost 1
	return cardinalCost*math.Max(dx, dy) + (diagonalCost-cardinalCost)*math.Min(dx, dy)
}

// calculateTerrainSpeedMultiplier calculates the average terrain speed multiplier
// between two coords.
func calculateTerrainSpeedMultiplier(terrain TerrainProvider, origin, dest Coord) float64 {
	originSpeed := terrain.GetTerrainSpeedMultiplier(origin.X, origin.Y)
	destSpeed := terrain.GetTerrainSpeedMultiplier(dest.X, dest.Y)
	return (originSpeed + destSpeed) / 2.0
}

// calculateMovementCost calculates the actual movement cost between two coords,
// accounting for distance and terrain speed.
func calculateMovementCost(terrain TerrainProvider, origin, dest Coord, distance float64) float64 {
	terrainMultiplier := calculateTerrainSpeedMultiplier(terrain, origin, dest)
	if terrainMultiplier <= 0 {
		return math.Inf(1) // Infinite cost for impassable terrain
	}

	// Higher speed multiplier means lower movement cost for pathfinding
	return distance / terrainMultiplier
}

// canMoveDiagonally checks if diagonal movement is allowed from one coord to another.
// Diagonal movement is allowed if at least one adjacent cardinal path is traversable.
func canMoveDiagonally(terrain TerrainProvider, from, to Coord) bool {
	dx := to.X - from.X
	dy := to.Y - from.Y

	// Not a diagonal move
	if isCardinalDirection(dx, dy) {
		return true
	}

	// Check the two adjacent cardinal coords
	adjacent1 := Coord{from.X + dx, from.Y}
	adjacent2 := Coord{from.X, from.Y + dy}

	// At least one adjacent coord must be walkable to allow diagonal movement
	walkable1 := terrain.IsInBounds(adjacent1.X, adjacent1.Y) && terrain.IsWalkable(adjacent1.X, adjacent1.Y)
	walkable2 := terrain.IsInBounds(adjacent2.X, adjacent2.Y) && terrain.IsWalkable(adjacent2.X, adjacent2.Y)

	return walkable1 || walkable2
}

// OccupancyChecker is an optional interface for checking if a coordinate is occupied.
// If provided, the pathfinding algorithm will avoid occupied coordinates.
type OccupancyChecker func(coord Coord) bool

// isGoalApproachable reports whether any tile the search could step to the goal
// from is enterable. It answers conservatively: it ignores the diagonal
// corner-cutting rule, so a goal it accepts may still turn out to be sealed off.
//
// The start tile counts as approachable even when occupancy reports it taken,
// because occupancy normally includes the moving entity's own reservation and
// the search never applies the occupancy check to its start tile.
func isGoalApproachable(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker) bool {
	for _, dir := range directions {
		neighbor := Coord{X: goal.X + dir.X, Y: goal.Y + dir.Y}
		if neighbor == start {
			return true
		}
		if !terrain.IsInBounds(neighbor.X, neighbor.Y) || !terrain.IsWalkable(neighbor.X, neighbor.Y) {
			continue
		}
		if isOccupied != nil && isOccupied(neighbor) {
			continue
		}
		return true
	}
	return false
}

// FindPath finds a path between two coordinates using A* pathfinding algorithm.
// It uses the terrain provider to query terrain properties and an optional
// occupancy checker to avoid occupied coordinates. It returns nil when no path
// exists, when start and goal are the same, and when the goal is out of bounds,
// unwalkable, occupied or has no enterable tile next to it — the last three
// being answered without searching, since a search would have to flood the map
// to reach them.
func FindPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker) []Coord {
	path, _ := findPath(terrain, start, goal, isOccupied)
	return path
}

// findPath implements FindPath and additionally reports how many nodes the
// search expanded (popped from the open set). The count is what tells apart a
// search that walked a narrow corridor to the goal from one that flooded the
// map, so the benchmarks measure it alongside wall-clock cost.
func findPath(terrain TerrainProvider, start, goal Coord, isOccupied OccupancyChecker) (path []Coord, expanded int) {
	// Quick checks
	if start == goal {
		return nil, 0 // No path needed when already at destination
	}

	if !terrain.IsInBounds(goal.X, goal.Y) || !terrain.IsWalkable(goal.X, goal.Y) {
		return nil, 0 // Goal is not reachable
	}

	// Reject a goal the search could never enter before searching for it.
	// Establishing the same answer by search costs a full flood of every
	// reachable tile: on a 304x304 open map that is ~92k expansions and ~130 ms,
	// against ~370 us for the longest journey that does succeed there. Both
	// rejections below are routine once many agents converge on one destination.
	if isOccupied != nil && isOccupied(goal) {
		return nil, 0 // Goal tile is taken
	}

	if !isGoalApproachable(terrain, start, goal, isOccupied) {
		return nil, 0 // Goal is sealed off by its immediate surroundings
	}

	// Initialize A* data structures
	openSet := &pathNodeQueue{}
	heap.Init(openSet)

	// nodeMap holds every node the search has touched, open and closed alike.
	// Carrying the closed flag on the node rather than in a second map keeps the
	// neighbour loop down to one hash lookup per neighbour.
	nodeMap := make(map[Coord]*pathNode)

	// Create and add the start node
	startNode := &pathNode{
		coord: start,
		g:     0,
		h:     heuristic(start, goal),
		index: -1,
	}
	startNode.f = startNode.g + startNode.h

	heap.Push(openSet, startNode)
	nodeMap[start] = startNode

	// A* main loop
	for openSet.Len() > 0 {
		current := heap.Pop(openSet).(*pathNode)
		expanded++

		// Check if we reached the goal
		if current.coord == goal {
			// Reconstruct path (append then reverse for O(n) instead of prepend O(n²))
			for node := current; node != nil; node = node.parent {
				path = append(path, node.coord)
			}
			slices.Reverse(path)
			return path, expanded
		}

		current.closed = true

		// Explore neighbors
		for _, dir := range directions {
			neighbor := Coord{
				X: current.coord.X + dir.X,
				Y: current.coord.Y + dir.Y,
			}

			// Look the node up once: both the closed check and the relaxation
			// below need it.
			neighborNode, visited := nodeMap[neighbor]
			if visited && neighborNode.closed {
				continue
			}

			// Skip if neighbor is not walkable
			if !terrain.IsInBounds(neighbor.X, neighbor.Y) || !terrain.IsWalkable(neighbor.X, neighbor.Y) {
				continue
			}

			// Skip if neighbor is occupied
			if isOccupied != nil && isOccupied(neighbor) {
				continue
			}

			// Skip if diagonal movement is not allowed
			if !isCardinalDirection(dir.X, dir.Y) && !canMoveDiagonally(terrain, current.coord, neighbor) {
				continue
			}

			// Calculate movement cost
			moveCost := cardinalCost
			if !isCardinalDirection(dir.X, dir.Y) {
				moveCost = diagonalCost
			}

			// Account for terrain speed. A walkable tile can still report a
			// speed of zero, in which case there is no finite cost to enter it.
			actualCost := calculateMovementCost(terrain, current.coord, neighbor, moveCost)
			if math.IsInf(actualCost, 1) {
				continue
			}

			// Skip unless this path to the neighbor beats the best one so far
			tentativeG := current.g + actualCost
			if visited && tentativeG >= neighborNode.g {
				continue
			}

			if !visited {
				neighborNode = &pathNode{
					coord: neighbor,
					h:     heuristic(neighbor, goal),
					index: -1, // Not in the open set yet
				}
				nodeMap[neighbor] = neighborNode
			}

			neighborNode.g = tentativeG
			neighborNode.f = neighborNode.g + neighborNode.h
			neighborNode.parent = current

			if neighborNode.index == -1 {
				heap.Push(openSet, neighborNode)
			} else {
				heap.Fix(openSet, neighborNode.index)
			}
		}
	}

	// No path found
	return nil, expanded
}
