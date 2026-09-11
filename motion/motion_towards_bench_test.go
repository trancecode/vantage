package motion

import (
	"fmt"
	"math"
	"testing"

	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// decisionTerrainSize is the side, in tiles, of the open terrain the decision
// benchmarks run on.
const decisionTerrainSize = 256

// newDecisionSystem returns a System over open terrain decisionTerrainSize tiles
// wide, with occupancy and a one-tile step, its world, and one entity standing on
// its reservation at start.
func newDecisionSystem(start geometry.Vector2) (*System, *ecs.World, ecs.EntityId) {
	s, w := newTestSystem()
	s.Occupancy = tilemap.NewTileOccupancyManager()
	s.Terrain = &testTerrain{width: decisionTerrainSize, height: decisionTerrainSize}
	s.MaxPathExpansions = 100_000
	s.MaxMoveActionDistance = 1.5
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: start})
	s.Grid.AddEntity(id, start)
	s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
	return s, w, id
}

// BenchmarkMoveEntityTowards measures one movement decision, planning a path to
// a destination journey tiles east and issuing the next step, on open terrain
// with occupancy and no heuristic configured. The entity is never ticked, so
// every op plans the same journey from the same tile: after each decision, the
// timed loop releases the step tile it reserved and reserves the start tile
// again, restoring the fixture before the next op.
func BenchmarkMoveEntityTowards(b *testing.B) {
	for _, journey := range []int{8, 32, 128} {
		b.Run(fmt.Sprintf("journey=%d", journey), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			destination := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4 + journey, Y: 64})
			s, _, id := newDecisionSystem(start)
			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				move := s.MoveEntityTowards(id, destination, opts)
				if !move.Started() {
					b.Fatalf("journey of %d tiles, decision %d: started no move: %v", journey, attempt, move.Outcome)
				}
				s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(move.Destination))
				s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				move := s.MoveEntityTowards(id, destination, opts)
				if !move.Started() {
					b.Fatalf("journey of %d tiles: started no move: %v", journey, move.Outcome)
				}
				s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(move.Destination))
				s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
			}
		})
	}
}

// reserveInnerRings reserves for occupant every tile of the circular area
// (center, radius in tiles) that lies inside its outermost ring, a ring being a
// tile's Chebyshev distance from the center tile, as a crowd standing at a
// gather point would.
func reserveInnerRings(s *System, center geometry.Vector2, radius float64, occupant ecs.EntityId) {
	centerTile := tilemap.WorldPositionToTile(center)
	inner := int(math.Ceil(radius)) - 1
	for dx := -inner; dx <= inner; dx++ {
		for dy := -inner; dy <= inner; dy++ {
			tile := tilemap.TileCoord{X: centerTile.X + dx, Y: centerTile.Y + dy}
			if tilemap.TileToWorldPosition(tile).DistanceTo(center) <= radius {
				s.Occupancy.SetOccupant(tile, occupant)
			}
		}
	}
}

// BenchmarkMoveEntityTowardsArea measures one decision toward a circular area of
// the given radius whose center lies 32 tiles east: finding the nearest reachable
// tile in the area, planning to it and issuing the next step. Another entity
// reserves every tile of the area inside its outermost ring, as a crowd standing
// at a gather point would, so the decision scans the reserved inner rings by
// lookup and plans to the nearest free tile on the outer ring; the radius sets
// both how many rings are scanned and how far the journey runs. The entity is
// never ticked, so every op makes the same decision: after each decision, the
// timed loop releases the step tile it reserved and reserves the start tile
// again, restoring the fixture before the next op, and leaves the crowd's
// reservations untouched.
func BenchmarkMoveEntityTowardsArea(b *testing.B) {
	for _, radius := range []float64{1, 4, 8} {
		b.Run(fmt.Sprintf("radius=%g", radius), func(b *testing.B) {
			start := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 4, Y: 64})
			center := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 36, Y: 64})
			s, w, id := newDecisionSystem(start)
			reserveInnerRings(s, center, radius, w.NewEntity())

			outerRing := int(math.Ceil(radius))
			target, _, found := s.findAreaTarget(start, center, radius)
			offset := tilemap.WorldPositionToTile(target)
			centerTile := tilemap.WorldPositionToTile(center)
			dx, dy := offset.X-centerTile.X, offset.Y-centerTile.Y
			if ring := max(dx, -dx, dy, -dy); !found || ring != outerRing {
				b.Fatalf("area of radius %g: decision targets %v on ring %d (found %t), want ring %d", radius, target, ring, found, outerRing)
			}

			opts := MoveOptions{Speed: 1}
			for attempt := range 2 {
				move := s.MoveEntityTowardsArea(id, center, radius, opts)
				if !move.Started() {
					b.Fatalf("area of radius %g, decision %d: started no move: %v", radius, attempt, move.Outcome)
				}
				s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(move.Destination))
				s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				move := s.MoveEntityTowardsArea(id, center, radius, opts)
				if !move.Started() {
					b.Fatalf("area of radius %g: started no move: %v", radius, move.Outcome)
				}
				s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(move.Destination))
				s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(start), id)
			}
		})
	}
}
