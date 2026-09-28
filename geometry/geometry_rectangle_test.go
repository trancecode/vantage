package geometry

import (
	"math/rand/v2"
	"testing"

	"github.com/trancecode/vantage/util"
)

func TestRandomPointInRectangleAcceptsRandRand(t *testing.T) {
	r := NewRectangleFromPoints(0, 0, 10, 10)
	rng := rand.New(rand.NewPCG(1, 2))
	p := RandomPointInRectangle(r, rng)
	if p.X() < r.Min.X() || p.X() > r.Max.X() || p.Y() < r.Min.Y() || p.Y() > r.Max.Y() {
		t.Fatalf("RandomPointInRectangle() = %v, want a point within %v", p, r)
	}
}

func TestRandomPointInRectangleReproducibleAcrossSaveReload(t *testing.T) {
	r := NewRectangleFromPoints(0, 0, 100, 100)
	rng := util.NewRng(1, 2)

	// Draw a few points before "saving" so the sequence is not at its start.
	RandomPointInRectangle(r, rng)
	RandomPointInRectangle(r, rng)

	state, err := rng.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	want := RandomPointInRectangle(r, rng)

	restored := util.NewRng(0, 0) // Seeded differently; UnmarshalBinary must override this state.
	if err := restored.UnmarshalBinary(state); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	got := RandomPointInRectangle(r, restored)

	if got != want {
		t.Fatalf("point drawn after save/reload = %v, want %v (sequence diverged)", got, want)
	}
}

func TestRectangleOverlaps(t *testing.T) {
	a := NewRectangleFromPoints(0.0, 0.0, 10.0, 10.0)
	cases := []struct {
		name string
		b    Rectangle
		want bool
	}{
		{"inside", NewRectangleFromPoints(2.0, 2.0, 4.0, 4.0), true},
		{"crossing an edge", NewRectangleFromPoints(8.0, 8.0, 12.0, 12.0), true},
		{"touching an edge only", NewRectangleFromPoints(10.0, 0.0, 12.0, 10.0), false},
		{"apart", NewRectangleFromPoints(20.0, 20.0, 30.0, 30.0), false},
	}
	for _, c := range cases {
		if got := a.Overlaps(c.b); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		if got := c.b.Overlaps(a); got != c.want {
			t.Errorf("%s (reversed): got %v, want %v", c.name, got, c.want)
		}
	}
}
