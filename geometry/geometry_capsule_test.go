package geometry

import "testing"

func TestCapsuleTouchesRectangle(t *testing.T) {
	// The unit tile at (1, 1). Values are chosen to be exact in binary floating
	// point so the boundary cases do not depend on rounding.
	tile := NewRectangleFromPoints(1.0, 1.0, 2.0, 2.0)

	cases := []struct {
		name   string
		a, b   Vector2
		radius float64
		want   bool
	}{
		{"crosses the middle, both ends far outside", NewVector2(-3.0, 1.5), NewVector2(6.0, 1.5), 0.25, true},
		{"ends inside", NewVector2(-3.0, 1.5), NewVector2(1.5, 1.5), 0.25, true},
		{"parallel to an edge at exactly the radius", NewVector2(-3.0, 0.75), NewVector2(6.0, 0.75), 0.25, true},
		{"parallel to an edge beyond the radius", NewVector2(-3.0, 0.5), NewVector2(6.0, 0.5), 0.25, false},
		// The line x + y = 1.6 passes the corner (1, 1) at 0.4/sqrt(2) = 0.283.
		{"passes a corner just outside the radius", NewVector2(2.6, -1.0), NewVector2(-1.0, 2.6), 0.25, false},
		// The line x + y = 1.7 passes it at 0.3/sqrt(2) = 0.212.
		{"passes a corner just inside the radius", NewVector2(2.7, -1.0), NewVector2(-1.0, 2.7), 0.25, true},
		{"zero length, clear", NewVector2(0.0, 0.0), NewVector2(0.0, 0.0), 0.25, false},
		{"zero length, touching an edge", NewVector2(0.75, 1.5), NewVector2(0.75, 1.5), 0.25, true},
		{"zero radius along an edge", NewVector2(1.0, 0.5), NewVector2(1.0, 2.5), 0, true},
		{"zero radius crossing, ends outside", NewVector2(0.0, 1.5), NewVector2(3.0, 1.75), 0, true},
		{"zero radius missing", NewVector2(0.0, 0.5), NewVector2(3.0, 0.75), 0, false},
		{"segment inside, radius zero", NewVector2(1.25, 1.25), NewVector2(1.75, 1.5), 0, true},
	}
	for _, c := range cases {
		if got := CapsuleTouchesRectangle(c.a, c.b, c.radius, tile); got != c.want {
			t.Errorf("%s: CapsuleTouchesRectangle(%v, %v, %v) = %v, want %v", c.name, c.a, c.b, c.radius, got, c.want)
		}
		// The test is symmetric in the segment's direction.
		if got := CapsuleTouchesRectangle(c.b, c.a, c.radius, tile); got != c.want {
			t.Errorf("%s reversed: got %v, want %v", c.name, got, c.want)
		}
	}
}
