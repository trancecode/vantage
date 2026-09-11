package motion

import (
	"fmt"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// decisionTerrainSize is the side, in tiles, of the open terrain the decision
// benchmarks run on.
const decisionTerrainSize = 256

// newDecisionSystem returns a System over open terrain decisionTerrainSize tiles
// wide, with occupancy and a one-tile step, and one entity standing on its
// reservation at start.
func newDecisionSystem(start geometry.Vector2) (*System, ecs.EntityId) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	s.Terrain = &testTerrain{width: decisionTerrainSize, height: decisionTerrainSize}
	s.MaxPathExpansions = 100_000
	s.MaxMoveActionDistance = 1.5
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: start})
	s.Grid.AddEntity(id, start)
	s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
	return s, id
}

// BenchmarkMoveEntityTowards measures one movement decision, planning a path to
// a destination journey tiles east and issuing the next step, on open terrain
// with occupancy and no heuristic configured. The entity is never ticked, so
// every op plans the same journey from the same tile.
func BenchmarkMoveEntityTowards(b *testing.B) {
	for _, journey := range []int{8, 32, 128} {
		b.Run(fmt.Sprintf("journey=%d", journey), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			destination := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4 + journey, Y: 64})
			s, id := newDecisionSystem(start)
			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				if move := s.MoveEntityTowards(id, destination, opts); !move.Started() {
					b.Fatalf("journey of %d tiles, decision %d: started no move: %v", journey, attempt, move.Outcome)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.MoveEntityTowards(id, destination, opts)
			}
		})
	}
}

// BenchmarkMoveEntityTowardsArea measures one decision toward a circular area of
// the given radius whose center lies 32 tiles east: finding the nearest reachable
// tile in the area, planning to it and issuing the next step. The entity is never
// ticked, so every op makes the same decision.
func BenchmarkMoveEntityTowardsArea(b *testing.B) {
	for _, radius := range []float64{1, 4, 8} {
		b.Run(fmt.Sprintf("radius=%g", radius), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 36, Y: 64})
			s, id := newDecisionSystem(start)
			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				if move := s.MoveEntityTowardsArea(id, center, radius, opts); !move.Started() {
					b.Fatalf("area of radius %g, decision %d: started no move: %v", radius, attempt, move.Outcome)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.MoveEntityTowardsArea(id, center, radius, opts)
			}
		})
	}
}
