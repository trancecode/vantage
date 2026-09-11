package motion

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
)

// benchFrame is one update at Ebitengine's default 60 updates per second, the
// elapsed time a game's frame hands to Tick.
const benchFrame = time.Second / 60

// benchHash mixes an index and a salt into a fixed pseudo-random value, so
// benchmark fixtures are identical on every run.
func benchHash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// BenchmarkSystemTick measures one Tick advancing every moving entity by one
// frame, with the spatial grid kept in sync, against the number of moving
// entities, for constant-speed and eased moves. Entities are spread at density
// 0.1 and head for destinations 100,000 tiles away, so none arrives during the
// benchmark.
func BenchmarkSystemTick(b *testing.B) {
	for _, ease := range []struct {
		name  string
		curve easing.Curve
	}{
		{name: "linear", curve: easing.CurveLinear},
		{name: "eased", curve: easing.CurveInOut},
	} {
		for _, count := range []int{1_000, 10_000, 100_000} {
			b.Run(fmt.Sprintf("ease=%s/entities=%d", ease.name, count), func(b *testing.B) {
				s, w := newTestSystem()
				side := uint64(math.Sqrt(float64(count) / 0.1))
				for i := range count {
					id := w.NewEntity()
					position := geometry.NewVector2(float64(benchHash(i, 1)%side), float64(benchHash(i, 2)%side))
					s.Spatials.Add(id, Spatial{Position: position})
					s.Grid.AddEntity(id, position)
					destination := geometry.NewVector2(position.X()+100_000, position.Y())
					if move := s.MoveEntity(id, destination, MoveOptions{Speed: 1, Ease: ease.curve}); !move.Started() {
						b.Fatalf("entity %d: move did not start: %v", i, move.Outcome)
					}
				}

				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					s.Tick(benchFrame)
				}
			})
		}
	}
}
