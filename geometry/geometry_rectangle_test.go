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
