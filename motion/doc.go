// Package motion provides components and systems for entity movement.
//
// Spatial holds an entity's current world position and facing direction.
// MovingComponent stores a movement target, direction, and speed.
// MovementResult carries the outcome of a movement tick: the entity ID,
// original position, new position, and whether the destination was reached.
// ProcessMovement calculates per-tick displacement given a duration and returns
// the new position and a completion flag indicating whether the entity has
// reached its destination.
package motion
