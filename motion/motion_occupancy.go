package motion

import (
	"github.com/trancecode/ecs/ecs"
	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

// Occupancy records where each body stands or is headed, and refuses a move
// whose end another body holds. System.Occupancy reserves through it.
// tilemap.TileOccupancyManager reserves whole tiles, one entity per tile;
// tilemap.CircleReservations reserves a circle per body, for bodies that stand
// anywhere.
type Occupancy interface {
	// Available reports whether id could claim position: nothing another
	// entity holds is there.
	Available(id ecs.EntityId, position geometry.Vector2) bool

	// Claim reserves destination for id as it sets off from from, releasing
	// what id held at from. It reports false, and changes nothing, when
	// another entity holds destination.
	Claim(id ecs.EntityId, from, destination geometry.Vector2) bool

	// Stop records that id halted at position before reaching destination: it
	// drops id's claim on destination and claims position.
	Stop(id ecs.EntityId, destination, position geometry.Vector2)
}

var _ Occupancy = (*tilemap.TileOccupancyManager)(nil)

var _ Occupancy = (*tilemap.CircleReservations)(nil)

// tileLedger is the per-tile lookup the tile route helpers use to route around
// reserved tiles. A tilemap.TileOccupancyManager provides it; an Occupancy
// without it leaves those helpers ignoring bodies.
type tileLedger interface {
	GetOccupant(tile tilemap.TileCoord) (ecs.EntityId, bool)
}

// ledger returns the System's Occupancy as a per-tile ledger, when it is
// one.
func (s *System) ledger() (tileLedger, bool) {
	if s.Occupancy == nil {
		return nil, false
	}
	ledger, ok := s.Occupancy.(tileLedger)
	return ledger, ok
}
