package tilemap

import (
	"fmt"
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
)

// BenchmarkTileOccupancyChurn measures one reservation moving to the next tile,
// ClearOccupant on its tile and SetOccupant on its neighbour, against how many
// tiles are occupied. Reservations sit on every other column, so a move never
// lands on another reservation; each op moves the next one in turn, and odd
// passes move each back, so the occupied set stays the same size.
func BenchmarkTileOccupancyChurn(b *testing.B) {
	for _, occupied := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("occupied=%d", occupied), func(b *testing.B) {
			world := ecs.NewWorld()
			occupancy := NewTileOccupancyManager()
			side := int(math.Ceil(math.Sqrt(float64(occupied))))
			ids := make([]ecs.EntityId, occupied)
			tiles := make([]TileCoord, occupied)
			for i := range occupied {
				ids[i] = world.NewEntity()
				tiles[i] = TileCoord{X: 2 * (i % side), Y: i / side}
				occupancy.SetOccupant(tiles[i], ids[i])
			}

			b.ReportAllocs()
			b.ResetTimer()
			i := 0
			for b.Loop() {
				n := i % occupied
				next := TileCoord{X: tiles[n].X + 1, Y: tiles[n].Y}
				if (i/occupied)%2 == 1 {
					next.X = tiles[n].X - 1
				}
				occupancy.ClearOccupant(tiles[n])
				occupancy.SetOccupant(next, ids[n])
				tiles[n] = next
				i++
			}
		})
	}
}
