// Package motion provides components and systems for entity movement.
//
// PositionComponent stores an entity's world position and facing direction.
// MovingComponent stores a movement target, direction, and speed.
// ProcessMovement calculates per-tick displacement given a duration, producing
// a MovementResult that callers use to update position and occupancy.
package motion
