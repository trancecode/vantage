package render

import (
	"fmt"
	"testing"
)

// drawListBenchHash mixes an index into a fixed pseudo-random value, so the
// benchmark's layers and Y values are identical on every run.
func drawListBenchHash(i int) uint64 {
	h := uint64(i) * 0x9E3779B97F4A7C15
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// BenchmarkDrawListOrdering measures ordering one frame's drawables: Clear, then
// one Add per drawable with a hashed layer (0 to 3) and Y value, then one Each
// over the payloads in painter's order, against the number of drawables.
func BenchmarkDrawListOrdering(b *testing.B) {
	for _, count := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("drawables=%d", count), func(b *testing.B) {
			layers := make([]int, count)
			ys := make([]float64, count)
			for i := range count {
				h := drawListBenchHash(i)
				layers[i] = int(h % 4)
				ys[i] = float64((h>>8)%100_000) / 100
			}
			var list DrawList[int]
			visited := 0

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				list.Clear()
				for i := range count {
					list.Add(layers[i], ys[i], i)
				}
				visited = 0
				list.Each(func(int) { visited++ })
			}
			if visited != count {
				b.Fatalf("Each visited %d of %d drawables", visited, count)
			}
		})
	}
}
