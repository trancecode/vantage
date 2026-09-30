package tilemap

import (
	"fmt"
	"math"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// CircleReservations reserves a circle of floor per body, each body with its
// own radius, at the end of its current move or where it stands. It satisfies
// motion.Occupancy, so a motion.System reserves moves through it. Two
// reservations conflict when their centres are closer than the sum of their
// radii; circles that only touch do not. Only where a move ends is checked:
// bodies pass through each other's reservations on the way.
//
// Lookups search a SpatialGrid of the reservation centres, separate from any
// grid of current positions, over the entity's radius plus the largest radius
// the table has held. That largest radius never shrinks, which can widen a
// search after a large body leaves but never misses a conflict.
type CircleReservations struct {
	grid      *SpatialGrid
	circles   map[ecs.EntityId]reservedCircle
	maxRadius float64
}

// reservedCircle is one entity's reservation.
type reservedCircle struct {
	centre geometry.Vector2
	radius float64
}

// NewCircleReservations returns an empty table whose grid has cells of
// cellSize tiles.
func NewCircleReservations(cellSize float64) *CircleReservations {
	return &CircleReservations{
		grid:    NewSpatialGrid(cellSize),
		circles: make(map[ecs.EntityId]reservedCircle),
	}
}

// Place reserves a circle of radius at position for id unconditionally, moving
// it if id already holds one. It is for spawning, teleports and loading saves;
// moves claim through Claim. It panics on a negative radius.
func (c *CircleReservations) Place(id ecs.EntityId, position geometry.Vector2, radius float64) {
	if radius < 0 {
		panic(fmt.Sprintf("placing entity %v in circle reservations: radius must not be negative, got %v", id, radius))
	}
	if old, ok := c.circles[id]; ok {
		c.grid.RemoveEntity(id, old.centre)
	}
	c.circles[id] = reservedCircle{centre: position, radius: radius}
	c.grid.AddEntity(id, position)
	c.maxRadius = math.Max(c.maxRadius, radius)
}

// Remove drops id's reservation, if it holds one.
func (c *CircleReservations) Remove(id ecs.EntityId) {
	old, ok := c.circles[id]
	if !ok {
		return
	}
	c.grid.RemoveEntity(id, old.centre)
	delete(c.circles, id)
}

// Reservation returns the centre and radius of id's reservation, and false
// when id holds none.
func (c *CircleReservations) Reservation(id ecs.EntityId) (centre geometry.Vector2, radius float64, ok bool) {
	held, ok := c.circles[id]
	return held.centre, held.radius, ok
}

// Available reports whether id's circle at position would conflict with no
// other entity's reservation. id must have been placed; Available panics
// otherwise, since the table cannot know its radius.
func (c *CircleReservations) Available(id ecs.EntityId, position geometry.Vector2) bool {
	own := c.mustHold(id, "checking reservation availability")
	return !c.conflicts(id, position, own.radius)
}

// Claim moves id's reservation to destination, and reports false, changing
// nothing, when it would conflict with another entity's. The origin needs no
// release, since an entity holds one circle. id must have been placed; Claim
// panics otherwise.
func (c *CircleReservations) Claim(id ecs.EntityId, _, destination geometry.Vector2) bool {
	own := c.mustHold(id, "claiming reservation")
	if c.conflicts(id, destination, own.radius) {
		return false
	}
	c.move(id, destination)
	return true
}

// Stop moves id's reservation to position, where it halted, unconditionally: a
// body stopped mid-move may stand on another's reservation, since bodies pass
// through each other, and the table records where it is. id must have been
// placed; Stop panics otherwise.
func (c *CircleReservations) Stop(id ecs.EntityId, _, position geometry.Vector2) {
	c.mustHold(id, "stopping reservation")
	c.move(id, position)
}

// mustHold returns id's reservation, panicking with the action being attempted
// when id was never placed.
func (c *CircleReservations) mustHold(id ecs.EntityId, action string) reservedCircle {
	held, ok := c.circles[id]
	if !ok {
		panic(fmt.Sprintf("%s for entity %v: entity was never placed", action, id))
	}
	return held
}

// conflicts reports whether a circle of radius at position overlaps any
// reservation other than id's own.
func (c *CircleReservations) conflicts(id ecs.EntityId, position geometry.Vector2, radius float64) bool {
	reach := radius + c.maxRadius
	area := geometry.NewRectangleFromPoints(position.X()-reach, position.Y()-reach, position.X()+reach, position.Y()+reach)
	for _, other := range c.grid.GetRange(area) {
		if other == id {
			continue
		}
		held := c.circles[other]
		if position.DistanceTo(held.centre) < radius+held.radius {
			return true
		}
	}
	return false
}

// move relocates id's reservation to position.
func (c *CircleReservations) move(id ecs.EntityId, position geometry.Vector2) {
	held := c.circles[id]
	c.grid.UpdateEntityPosition(id, held.centre, position)
	held.centre = position
	c.circles[id] = held
}
