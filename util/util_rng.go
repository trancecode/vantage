package util

import "math/rand/v2"

// Rng is a seedable, deterministic random source whose state can be marshaled,
// so a deterministic simulation's random sequence resumes exactly after a
// savegame reload. It wraps a rand/v2 PCG source; the same seed always yields
// the same sequence. Games use this instead of a bare *rand.Rand, whose source
// state cannot be saved and restored.
type Rng struct {
	pcg *rand.PCG
	r   *rand.Rand
}

// NewRng returns an Rng seeded with (seed1, seed2). The same seeds produce the
// same sequence.
func NewRng(seed1, seed2 uint64) *Rng {
	pcg := rand.NewPCG(seed1, seed2)
	return &Rng{pcg: pcg, r: rand.New(pcg)}
}

// Float64 returns a random float in [0.0, 1.0).
func (g *Rng) Float64() float64 { return g.r.Float64() }

// IntN returns a random int in [0, n). It panics if n <= 0.
func (g *Rng) IntN(n int) int { return g.r.IntN(n) }

// Uint64 returns a random 64-bit value.
func (g *Rng) Uint64() uint64 { return g.r.Uint64() }

// MarshalBinary encodes the generator's current state.
func (g *Rng) MarshalBinary() ([]byte, error) { return g.pcg.MarshalBinary() }

// UnmarshalBinary restores generator state from data produced by MarshalBinary;
// draws after restore continue the saved sequence. The wrapped source is mutated
// in place, so values drawn through this Rng reflect the restored state.
func (g *Rng) UnmarshalBinary(data []byte) error { return g.pcg.UnmarshalBinary(data) }

// Float64Source is a random source that can produce a float in [0.0, 1.0).
// It exists so callers can pass either a bare *math/rand/v2.Rand or the
// engine's savegame-safe *Rng, whose state can be marshaled and restored
// across a reload; a bare *rand.Rand cannot.
type Float64Source interface {
	Float64() float64
}
