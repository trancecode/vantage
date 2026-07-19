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
// routes between them by the movement's easing.Curve, and a zero-duration
// tick moves nothing and never completes an in-flight move on either.
//
// System bundles the component handles and spatial indexes movement operates
// on. Tick advances every entity that has a Movement and satisfies the
// sim.TickSystem interface. MoveEntity starts a single move with tile
// occupancy checks, taking a MoveOptions describing the average speed and the
// easing curve, and MoveEntityTowards and MoveEntityTowardsArea follow A*
// paths one bounded step at a time. Game policy stays with the caller: each
// attempt returns a MoveStart describing what happened so the consuming game
// can update its own entity states, AI scheduling, and logs.
package motion
