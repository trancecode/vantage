package tilemap

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

func TestCircleReservations_ConflictIsStrictlyLessThanTheSumOfRadii(t *testing.T) {
	c := NewCircleReservations(1.0)
	small, large := twoEntities()
	c.Place(small, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(large, geometry.NewVector2(5.0, 5.0), 0.5)

	if c.Claim(large, geometry.NewVector2(5.0, 5.0), geometry.NewVector2(0.7, 0.0)) {
		t.Error("centres 0.7 apart with radii 0.25 and 0.5 overlap; the claim should fail")
	}
	if centre, _, _ := c.Reservation(large); centre != geometry.NewVector2(5.0, 5.0) {
		t.Error("a refused claim should leave the reservation where it was")
	}
	if !c.Claim(large, geometry.NewVector2(5.0, 5.0), geometry.NewVector2(0.75, 0.0)) {
		t.Error("circles that only touch do not conflict; the claim should succeed")
	}
}

func TestCircleReservations_FindsALargeBodyFromASmallOnesQuery(t *testing.T) {
	c := NewCircleReservations(1.0)
	large, small := twoEntities()
	c.Place(large, geometry.NewVector2(0.0, 0.0), 2.0)
	c.Place(small, geometry.NewVector2(10.0, 10.0), 0.25)

	if c.Available(small, geometry.NewVector2(2.1, 0.0)) {
		t.Error("2.1 from a radius 2 circle is inside it for a radius 0.25 body")
	}
	if !c.Available(small, geometry.NewVector2(2.25, 0.0)) {
		t.Error("exactly the sum of radii away should be available")
	}
}

func TestCircleReservations_AvailableIgnoresTheEntitysOwnCircle(t *testing.T) {
	c := NewCircleReservations(1.0)
	id, _ := twoEntities()
	c.Place(id, geometry.NewVector2(0.0, 0.0), 0.25)

	if !c.Available(id, geometry.NewVector2(0.1, 0.0)) {
		t.Error("an entity's own reservation should not block it")
	}
}

func TestCircleReservations_StopRecordsWhereTheBodyIsEvenOnAnotherCircle(t *testing.T) {
	c := NewCircleReservations(1.0)
	a, b := twoEntities()
	c.Place(a, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(b, geometry.NewVector2(3.0, 0.0), 0.25)

	c.Stop(b, geometry.NewVector2(3.0, 0.0), geometry.NewVector2(0.1, 0.0))

	if centre, _, _ := c.Reservation(b); centre != geometry.NewVector2(0.1, 0.0) {
		t.Errorf("expected b reserved where it stopped, got %v", centre)
	}
}

func TestCircleReservations_PlaceAndRemove(t *testing.T) {
	c := NewCircleReservations(1.0)
	a, b := twoEntities()
	c.Place(a, geometry.NewVector2(0.0, 0.0), 0.25)
	c.Place(b, geometry.NewVector2(3.0, 0.0), 0.25)

	c.Place(a, geometry.NewVector2(5.0, 5.0), 0.25)
	if !c.Available(b, geometry.NewVector2(0.0, 0.0)) {
		t.Error("placing an entity again should move it")
	}

	c.Remove(a)
	if _, _, ok := c.Reservation(a); ok {
		t.Error("expected a removed")
	}
	if !c.Available(b, geometry.NewVector2(5.0, 5.0)) {
		t.Error("a removed entity should block nothing")
	}
}

func TestCircleReservations_Panics(t *testing.T) {
	cases := map[string]func(c *CircleReservations){
		"negative radius": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Place(id, geometry.NewVector2(0.0, 0.0), -0.1)
		},
		"available for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Available(id, geometry.NewVector2(0.0, 0.0))
		},
		"claim for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Claim(id, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(1.0, 0.0))
		},
		"stop for an entity never placed": func(c *CircleReservations) {
			id, _ := twoEntities()
			c.Stop(id, geometry.NewVector2(1.0, 0.0), geometry.NewVector2(0.0, 0.0))
		},
	}
	for name, call := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			call(NewCircleReservations(1.0))
		}()
	}
}
