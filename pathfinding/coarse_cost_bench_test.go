package pathfinding

import (
	"runtime"
	"testing"
)

// BenchmarkCoarseCostCellBuild measures building one cell of
// DefaultCoarseCellSize over each kind of ground, from the tile reads to both
// crossing rates.
func BenchmarkCoarseCostCellBuild(b *testing.B) {
	for _, ground := range []struct {
		name    string
		terrain speedFuncTerrain
	}{
		{name: "grass", terrain: grassBenchTerrain(0)},
		{name: "grid", terrain: gridBenchTerrain(0)},
		{name: "reaches", terrain: reachesBenchTerrain(0)},
	} {
		b.Run("ground="+ground.name, func(b *testing.B) {
			b.ReportAllocs()
			cell := 0
			for b.Loop() {
				buildCellRates(ground.terrain, Coord{X: cell, Y: benchOrigin / DefaultCoarseCellSize}, DefaultCoarseCellSize)
				cell++
			}
		})
	}
}

// BenchmarkCoarseCostRetainedPerCell measures the heap memory a CoarseCost keeps
// per built cell, over 4,096 cells.
func BenchmarkCoarseCostRetainedPerCell(b *testing.B) {
	const cellCount = 4096
	terrain := grassBenchTerrain(0)
	var retained float64
	for b.Loop() {
		field := NewCoarseCost(terrain, coarseBenchConfig())
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for i := range cellCount {
			field.rates(Coord{X: i, Y: 0})
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		retained = float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / cellCount
		runtime.KeepAlive(field)
	}
	b.ReportMetric(retained, "retained-bytes/cell")
}
