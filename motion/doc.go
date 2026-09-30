// Package motion provides components and systems for entity movement.
//
// Spatial holds an entity's current world position and facing direction.
// Movement stores an entity's movement target, speed and easing state.
// MovementResult carries the outcome of a movement tick: the entity ID,
// original position, new position, and whether the destination was reached.
//
// A move runs on one of two paths. Constant-speed moves advance incrementally
// through ProcessMovement, which displaces the body by speed times the tick
// duration. Eased moves are parametric: ProcessMove derives the position from
// the move's start, destination and progress through its total duration, so
// the path is a pure function of elapsed game time and independent of how that
// time is sliced into ticks. Both paths give a move the same nominal
// duration, the distance divided by the speed, but they do not always land on
// the same tick: the eased path completes on the first tick at or after that
// duration, while the constant-speed path completes on a distance tolerance
// and an overshoot test, so under a tick that does not divide the duration
// evenly the two can differ by one tick in either direction. ProcessMove
// routes between them by the movement's easing.Curve and its Timed flag: a
// move started with a fixed MoveOptions.Duration is timed and runs on the
// parametric path even at constant speed. A zero-duration
// tick moves nothing and never completes an in-flight move on either.
//
// System bundles the component handles and spatial indexes movement operates
// on. Tick advances every entity that has a Movement and satisfies the
// sim.TickSystem interface. MoveEntity starts a single move with tile
// occupancy checks, taking a MoveOptions describing the average speed and the
// easing curve, and MoveEntityTowards and MoveEntityTowardsArea follow A*
// paths one bounded step at a time. System.Occupancy reserves through the
// Occupancy interface: a tilemap.TileOccupancyManager for tile-based games, or
// a tilemap.CircleReservations for bodies that stand anywhere. CancelMove stops
// a move where the body stands and settles its reservation there. Game policy
// stays with the caller: each attempt returns a MoveStart describing what
// happened so the consuming game can update its own entity states, AI
// scheduling, and logs.
//
// Continuous movement lets a body stand and walk anywhere rather than hop
// between tile centres. WalkIsClear tests whether a round body can walk
// straight from one point to another without touching a blocked tile.
// System.FindRoute plans a route between two arbitrary points through tile
// centres, never cutting a blocked corner, and keeps every step between its
// points walkable for a body of radius under 0.5. TightenRoute picks the next
// leg along such a route: the farthest point a body can walk to in a straight
// line, capped at a leg length. A game walks by starting each leg with
// MoveEntity, normally with a fixed MoveOptions.Duration, reserving through a
// tilemap.CircleReservations.
package motion
