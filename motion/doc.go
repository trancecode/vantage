// Package motion provides components and systems for entity movement.
//
// Spatial holds an entity's current world position and facing direction.
// Movement stores an entity's movement target and speed.
// MovementResult carries the outcome of a movement tick: the entity ID,
// original position, new position, and whether the destination was reached.
// ProcessMovement calculates per-tick displacement given a duration and returns
// the new position and a completion flag indicating whether the entity has
// reached its destination.
//
// System bundles the component handles and spatial indexes movement operates
// on. Tick advances every entity that has a Movement and satisfies the
// sim.TickSystem interface. MoveEntity starts a single move with tile
// occupancy checks, and MoveEntityTowards and MoveEntityTowardsArea follow
// A* paths one bounded step at a time. Game policy stays with the caller:
// each attempt returns a MoveStart describing what happened so the consuming
// game can update its own entity states, AI scheduling, and logs.
package motion
