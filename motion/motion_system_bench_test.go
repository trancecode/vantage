package motion

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/easing"
	"github.com/trancecode/vantage/geometry"
)

// benchFrame is one update at Ebitengine's default 60 updates per second, the
// elapsed time a game's frame hands to Tick.
const benchFrame = time.Second / 60

// benchShuttleTiles is how far east of its spawn tile a BenchmarkSystemTick body
// shuttles.
const benchShuttleTiles = 4

// benchStartFrames is the span of frames over which BenchmarkSystemTick's bodies
// start their first move, half the 240-frame cycle of a body's arrivals at speed
// 1, which the hashed first-leg length spreads over the whole cycle.
const benchStartFrames = 120

// benchWarmupFrames is how many frames BenchmarkSystemTick ticks before timing:
// 15 s of game time, enough for the latest-starting body to have created every
// grid cell its shuttle touches.
const benchWarmupFrames = 900

// benchHash mixes an index and a salt into a fixed pseudo-random value, so
// benchmark fixtures are identical on every run.
func benchHash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// benchShuttle is one BenchmarkSystemTick body's route: the two tile centres it
// shuttles between, and which of them its current leg heads for.
type benchShuttle struct {
	// west is the spawn tile centre.
	west geometry.Vector2
	// east is the tile centre benchShuttleTiles east of west.
	east geometry.Vector2
	// eastbound reports whether the current leg heads east.
	eastbound bool
}

// BenchmarkSystemTick measures one Tick advancing every moving entity by one
// frame, with the spatial grid kept in sync, against the number of moving
// entities, for constant-speed and eased moves.
//
// Entities spawn at tile centres spread at density 0.1 and shuttle at speed 1
// between their spawn tile centre and the tile centre 4 tiles east. Each body
// starts its first move at a hashed frame within the first 120 frames, and the
// first move stops a hashed 1 to 4 tiles east; after that, every arrival turns
// the body around through System.OnArrival, back west after an eastbound leg and
// east after a westbound one. Every leg lasts a whole number of seconds, so
// without the two staggers bodies would arrive in bursts on a few frames of each
// cycle; with them, arrivals spread evenly over the frames. The re-issued
// MoveEntity runs inside Tick, so it is part of the measured per-frame cost, as a
// game's arrivals are.
//
// Before timing, the benchmark ticks 900 frames (15 s of game time), so every
// body has arrived at least once and every grid cell the shuttles touch exists:
// the timed ticks cross only cells a body has crossed before, and the cost per
// tick does not depend on how many ticks the run makes.
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
				shuttles := ecs.Components[benchShuttle](w)
				opts := MoveOptions{Speed: 1, Ease: ease.curve}
				arrivals := 0
				s.OnArrival = func(result MovementResult) {
					arrivals++
					shuttle, _ := shuttles.Get(result.EntityId)
					destination := shuttle.west
					if !shuttle.eastbound {
						destination = shuttle.east
					}
					shuttle.eastbound = !shuttle.eastbound
					if move := s.MoveEntity(result.EntityId, destination, opts); !move.Started() {
						b.Fatalf("entity %v arrived at %v: turning around did not start: %v", result.EntityId, result.NewPosition, move.Outcome)
					}
				}

				// starts holds, per warm-up frame, the bodies that start their first
				// move on it and where that move heads.
				type start struct {
					id    ecs.EntityId
					first geometry.Vector2
				}
				starts := make([][]start, benchStartFrames)
				side := uint64(math.Sqrt(float64(count) / 0.1))
				for i := range count {
					id := w.NewEntity()
					west := geometry.NewVector2(float64(benchHash(i, 1)%side)+0.5, float64(benchHash(i, 2)%side)+0.5)
					s.Spatials.Add(id, Spatial{Position: west})
					s.Grid.AddEntity(id, west)
					shuttles.Add(id, benchShuttle{
						west:      west,
						east:      geometry.NewVector2(west.X()+benchShuttleTiles, west.Y()),
						eastbound: true,
					})
					frame := benchHash(i, 4) % benchStartFrames
					starts[frame] = append(starts[frame], start{
						id:    id,
						first: geometry.NewVector2(west.X()+float64(1+benchHash(i, 3)%benchShuttleTiles), west.Y()),
					})
				}

				for frame := range benchWarmupFrames {
					if frame < benchStartFrames {
						for _, body := range starts[frame] {
							if move := s.MoveEntity(body.id, body.first, opts); !move.Started() {
								b.Fatalf("entity %v: first move at frame %d did not start: %v", body.id, frame, move.Outcome)
							}
						}
					}
					s.Tick(benchFrame)
				}
				if arrivals < count {
					b.Fatalf("warming up for %d frames: %d arrivals, want at least %d", benchWarmupFrames, arrivals, count)
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
