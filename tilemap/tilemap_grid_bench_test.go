package tilemap

import (
	"fmt"
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
)

// benchWorldSize is the side, in tiles, of the square world the range query
// benchmark populates. It leaves room around the largest query half-width.
const benchWorldSize = 256

// benchHash mixes an index and a salt into a fixed pseudo-random value, so
// benchmark fixtures are identical on every run.
func benchHash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// populatedGrid returns a grid of the given cell size holding density entities
// per tile at hashed positions over a side by side world, with the entities and
// their positions in the order they were added.
func populatedGrid(cellSize float64, side int, density float64) (*SpatialGrid, []ecs.EntityId, []geometry.Vector2) {
	world := ecs.NewWorld()
	grid := NewSpatialGrid(cellSize)
	count := int(float64(side*side) * density)
	ids := make([]ecs.EntityId, count)
	positions := make([]geometry.Vector2, count)
	scale := uint64(side) * 1000
	for i := range count {
		ids[i] = world.NewEntity()
		positions[i] = geometry.NewVector2(
			float64(benchHash(i, 1)%scale)/1000,
			float64(benchHash(i, 2)%scale)/1000,
		)
		grid.AddEntity(ids[i], positions[i])
	}
	return grid, ids, positions
}

// BenchmarkSpatialGridGetRange measures one range query against the query's
// half-width, the entity density and the grid's cell size. The query is a square
// of half-width radius tiles centered in a populated world; entities-found/op
// is how many entities it returns.
func BenchmarkSpatialGridGetRange(b *testing.B) {
	for _, cellSize := range []float64{1, 4, 16} {
		for _, density := range []float64{0.01, 0.1, 0.5} {
			grid, _, _ := populatedGrid(cellSize, benchWorldSize, density)
			for _, radius := range []float64{4, 8, 16, 32, 64} {
				b.Run(fmt.Sprintf("cell=%g/density=%g/radius=%g", cellSize, density, radius), func(b *testing.B) {
					center := float64(benchWorldSize) / 2
					rect := geometry.NewRectangleFromPoints(center-radius, center-radius, center+radius, center+radius)
					found := len(grid.GetRange(rect))
					if expected := density * 4 * radius * radius; expected >= 4 && found == 0 {
						b.Fatalf("query of half-width %g at density %g found no entities, expected about %g", radius, density, expected)
					}

					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						grid.GetRange(rect)
					}
					b.ReportMetric(float64(found), "entities-found/op")
				})
			}
		}
	}
}

// BenchmarkSpatialGridUpdateEntityPosition measures moving one entity one tile
// across a cell boundary, in a grid of cell size 1 populated at density 0.1,
// against the population. Each op moves the next entity in turn; odd passes
// over the population move each entity back, so the population stays in place.
func BenchmarkSpatialGridUpdateEntityPosition(b *testing.B) {
	for _, population := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("population=%d", population), func(b *testing.B) {
			side := int(math.Sqrt(float64(population) / 0.1))
			grid, ids, positions := populatedGrid(1, side, 0.1)
			if len(ids) == 0 {
				b.Fatalf("population %d: fixture holds no entities", population)
			}

			b.ReportAllocs()
			b.ResetTimer()
			i := 0
			for b.Loop() {
				n := i % len(ids)
				step := 1.0
				if (i/len(ids))%2 == 1 {
					step = -1
				}
				from := positions[n]
				to := geometry.NewVector2(from.X()+step, from.Y())
				grid.UpdateEntityPosition(ids[n], from, to)
				positions[n] = to
				i++
			}
		})
	}
}
