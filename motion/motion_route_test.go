package motion

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func newRouteSystem(terrain *testTerrain) *System {
	s, _ := newTestSystem()
	s.Terrain = terrain
	s.MaxPathExpansions = testMaxPathExpansions
	return s
}

func TestFindRoute_Shapes(t *testing.T) {
	s := newRouteSystem(&testTerrain{width: 10, height: 10})
	cases := []struct {
		name     string
		from, to geometry.Vector2
		want     []geometry.Vector2
	}{
		{"off-centre to off-centre", v(1.2, 1.3), v(4.7, 1.6),
			[]geometry.Vector2{v(1.5, 1.5), v(2.5, 1.5), v(3.5, 1.5), v(4.5, 1.5), v(4.7, 1.6)}},
		{"from a centre", v(1.5, 1.5), v(3.7, 1.6),
			[]geometry.Vector2{v(2.5, 1.5), v(3.5, 1.5), v(3.7, 1.6)}},
		{"to a centre", v(1.2, 1.3), v(3.5, 1.5),
			[]geometry.Vector2{v(1.5, 1.5), v(2.5, 1.5), v(3.5, 1.5)}},
		{"within one tile", v(1.2, 1.3), v(1.8, 1.1),
			[]geometry.Vector2{v(1.5, 1.5), v(1.8, 1.1)}},
		{"within one tile from its centre", v(1.5, 1.5), v(1.8, 1.1),
			[]geometry.Vector2{v(1.8, 1.1)}},
		{"same point", v(1.2, 1.3), v(1.2, 1.3), []geometry.Vector2{}},
	}
	for _, c := range cases {
		got, ok := s.FindRoute(c.from, c.to)
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("%s: FindRoute(%v, %v) = %v, %v; want %v, true", c.name, c.from, c.to, got, ok, c.want)
		}
	}
}

func TestFindRoute_NoRoute(t *testing.T) {
	blocked := map[tilemap.TileCoord]bool{{X: 7, Y: 7}: true}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				blocked[tilemap.TileCoord{X: 2 + dx, Y: 2 + dy}] = true
			}
		}
	}
	s := newRouteSystem(&testTerrain{width: 10, height: 10, blocked: blocked})

	if route, ok := s.FindRoute(v(0.5, 5.5), v(7.5, 7.5)); ok || route != nil {
		t.Errorf("an unwalkable goal should have no route, got %v, %v", route, ok)
	}
	if route, ok := s.FindRoute(v(0.5, 5.5), v(2.5, 2.5)); ok || route != nil {
		t.Errorf("a sealed-off goal should have no route, got %v, %v", route, ok)
	}
}

func TestFindRoute_DoesNotCutABlockedCorner(t *testing.T) {
	s := newRouteSystem(oneBlockedTile())

	got, ok := s.FindRoute(v(4.5, 5.5), v(5.5, 4.5))

	want := []geometry.Vector2{v(4.5, 4.5), v(5.5, 4.5)}
	if !ok || !slices.Equal(got, want) {
		t.Errorf("expected the route around the corner %v, got %v, %v", want, got, ok)
	}
}

func TestFindRoute_IncludesTheGoalTileCentre(t *testing.T) {
	// The goal sits near the corner of its tile that touches the blocked tile
	// (4, 4). A straight walk from the west neighbour's centre to it grazes
	// that tile: the route that ends there stalls a body one tile short. The
	// goal tile's centre in between avoids it.
	s := newRouteSystem(&testTerrain{width: 10, height: 10, blocked: map[tilemap.TileCoord]bool{{X: 4, Y: 4}: true}})
	west, goal := v(4.5, 5.5), v(5.21, 5.01)
	const radius = 0.2
	if !WalkIsClear(s.Terrain, goal, goal, radius) {
		t.Fatal("test setup: the goal should be a legal position")
	}
	if WalkIsClear(s.Terrain, west, goal, radius) {
		t.Fatal("test setup: the straight walk from the west centre should graze the blocked tile")
	}

	got, ok := s.FindRoute(west, goal)

	want := []geometry.Vector2{v(5.5, 5.5), goal}
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v, %v", want, got, ok)
	}
	if !WalkIsClear(s.Terrain, west, got[0], radius) || !WalkIsClear(s.Terrain, got[0], got[1], radius) {
		t.Error("every step of the route should be walkable")
	}
}

func TestFindRoute_EveryStepIsWalkable(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	found := 0
	for trial := range 2000 {
		terrain := &testTerrain{width: 12, height: 12, blocked: map[tilemap.TileCoord]bool{}}
		for range 28 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(12), Y: rng.IntN(12)}] = true
		}
		s := newRouteSystem(terrain)
		radius := 0.05 + rng.Float64()*0.44
		from, to := legalPoint(rng, terrain, radius), legalPoint(rng, terrain, radius)

		route, ok := s.FindRoute(from, to)
		if !ok {
			continue
		}
		found++
		previous := from
		for i, point := range route {
			if !WalkIsClear(terrain, previous, point, radius) {
				t.Fatalf("trial %d, radius %v: step %d from %v to %v is not walkable (route %v)", trial, radius, i, previous, point, route)
			}
			previous = point
		}
	}
	if found < 500 {
		t.Fatalf("only %d of 2000 trials found a route; the test is not exercising routes", found)
	}
}

// legalPoint draws a random point where a body of radius fits.
func legalPoint(rng *rand.Rand, terrain *testTerrain, radius float64) geometry.Vector2 {
	for {
		p := v(rng.Float64()*float64(terrain.width), rng.Float64()*float64(terrain.height))
		if WalkIsClear(terrain, p, p, radius) {
			return p
		}
	}
}

func TestFindRoute_RecordsPhase(t *testing.T) {
	s := newRouteSystem(&testTerrain{width: 10, height: 10})
	var phases []string
	s.RecordPhase = func(name string, _ time.Duration) { phases = append(phases, name) }

	s.FindRoute(v(1.2, 1.3), v(4.7, 1.6))

	if !slices.Equal(phases, []string{"pathfinding"}) {
		t.Errorf("expected one pathfinding phase, got %v", phases)
	}
}

func TestFindRoute_PanicsWithoutConfiguration(t *testing.T) {
	for name, s := range map[string]*System{
		"no terrain":          {MaxPathExpansions: testMaxPathExpansions},
		"no expansion budget": {Terrain: &testTerrain{width: 10, height: 10}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			s.FindRoute(v(1.2, 1.3), v(4.7, 1.6))
		}()
	}
}
