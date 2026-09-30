package motion

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func TestTightenRoute_CapsAtMaxLength(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5), v(3.5, 1.5), v(4.5, 1.5), v(5.5, 1.5), v(6.5, 1.5)}

	got, ok := TightenRoute(terrain, v(1.5, 1.5), route, 0.25, 1.5)

	// The first point beyond 1.5 is (3.5, 1.5), 2 away; the cap scales the leg
	// toward it by 0.75, which is exact in floating point.
	if !ok || got != v(3.0, 1.5) {
		t.Errorf("expected the leg capped at (3.0, 1.5), got %v, %v", got, ok)
	}
}

func TestTightenRoute_UncappedReturnsTheLastPoint(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5), v(3.5, 2.5), v(6.2, 2.9)}

	got, ok := TightenRoute(terrain, v(1.5, 1.5), route, 0.25, math.Inf(1))

	if !ok || got != v(6.2, 2.9) {
		t.Errorf("expected the whole route in one leg on open ground, got %v, %v", got, ok)
	}
}

func TestTightenRoute_StopsWhereTheWallBlocksTheView(t *testing.T) {
	// A wall at x = 3 from y = 0 to y = 5 forces the route up and around.
	blocked := map[tilemap.TileCoord]bool{}
	for y := range 6 {
		blocked[tilemap.TileCoord{X: 3, Y: y}] = true
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	s := newRouteSystem(terrain)
	from := v(1.5, 1.5)
	route, ok := s.FindRoute(from, v(5.5, 1.5))
	if !ok {
		t.Fatal("test setup: expected a route around the wall")
	}

	got, ok := TightenRoute(terrain, from, route, 0.25, math.Inf(1))

	if !ok || !WalkIsClear(terrain, from, got, 0.25) {
		t.Fatalf("expected a clear leg, got %v, %v", got, ok)
	}
	i := slices.Index(route, got)
	if i < 0 {
		t.Fatalf("expected an uncapped leg to end on a route point, got %v", got)
	}
	if i+1 < len(route) && WalkIsClear(terrain, from, route[i+1], 0.25) {
		t.Errorf("expected the next route point %v to be out of view", route[i+1])
	}
}

func TestTightenRoute_ReportsFalseWhenNothingIsClear(t *testing.T) {
	terrain := oneBlockedTile()

	if _, ok := TightenRoute(terrain, v(4.5, 5.5), []geometry.Vector2{v(6.5, 5.5)}, 0.25, math.Inf(1)); ok {
		t.Error("a route whose first point lies behind a blocked tile has no leg")
	}
	if _, ok := TightenRoute(terrain, v(4.5, 5.5), nil, 0.25, math.Inf(1)); ok {
		t.Error("an empty route has no leg")
	}
	if _, ok := TightenRoute(terrain, v(4.5, 4.5), []geometry.Vector2{v(4.5, 4.5)}, 0.25, math.Inf(1)); ok {
		t.Error("a route leading only to where the body stands has no leg")
	}
}

// countingTerrain counts the tile lookups WalkIsClear makes.
type countingTerrain struct {
	*testTerrain
	lookups int
}

func (c *countingTerrain) IsInBounds(x, y int) bool {
	c.lookups++
	return c.testTerrain.IsInBounds(x, y)
}

func TestTightenRoute_ScanIsBoundedByMaxLength(t *testing.T) {
	terrain := &countingTerrain{testTerrain: &testTerrain{width: 2000, height: 3}}
	route := make([]geometry.Vector2, 0, 1000)
	for x := 1; x <= 1000; x++ {
		route = append(route, v(float64(x)+0.5, 1.5))
	}

	got, ok := TightenRoute(terrain, v(0.5, 1.5), route, 0.25, 1.0)

	if !ok || got != v(1.5, 1.5) {
		t.Fatalf("expected a one-tile leg, got %v, %v", got, ok)
	}
	if terrain.lookups > 100 {
		t.Errorf("expected a handful of tile lookups for a 1-tile leg, got %d", terrain.lookups)
	}
}

func TestTightenRoute_LargeBodyNeverWalksThroughAWall(t *testing.T) {
	// A one-tile corridor at y = 5 through a wall at x = 5. A 0.6 body does not
	// fit through it.
	blocked := map[tilemap.TileCoord]bool{}
	for y := range 10 {
		if y != 5 {
			blocked[tilemap.TileCoord{X: 5, Y: y}] = true
		}
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	s := newRouteSystem(terrain)
	from := v(2.5, 5.5)
	route, ok := s.FindRoute(from, v(8.5, 5.5))
	if !ok {
		t.Fatal("test setup: expected a route through the corridor")
	}

	got, ok := TightenRoute(terrain, from, route, 0.6, math.Inf(1))

	if !ok || !WalkIsClear(terrain, from, got, 0.6) {
		t.Fatalf("expected a walkable leg toward the corridor, got %v, %v", got, ok)
	}
	// On y = 5.5 a 0.6 body keeps 0.6 from the corners (5, 5) and (5, 6), so
	// its centre stays below x = 5 - sqrt(0.6² - 0.5²) = 4.668.
	if got.X() > 4.668 {
		t.Errorf("a 0.6 body cannot reach x = %v, inside the corridor mouth", got.X())
	}

	// From there the next point is inside the corridor: the body stops short.
	route, ok = s.FindRoute(got, v(8.5, 5.5))
	if !ok {
		t.Fatal("expected a route from the corridor mouth")
	}
	if next, ok := TightenRoute(terrain, got, route, 0.6, math.Inf(1)); ok {
		t.Errorf("expected no leg through the narrow corridor, got one to %v", next)
	}
}

func TestTightenRoute_ShortLegsStillReachTheGoal(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	s := newRouteSystem(terrain)
	position, goal := v(1.5, 1.5), v(5.5, 5.5)

	for leg := 0; position != goal; leg++ {
		if leg > 100 {
			t.Fatalf("no arrival at %v after 100 legs, stuck at %v", goal, position)
		}
		route, ok := s.FindRoute(position, goal)
		if !ok {
			t.Fatalf("route lost from %v", position)
		}
		next, ok := TightenRoute(terrain, position, route, 0.25, 0.3)
		if !ok {
			t.Fatalf("no leg from %v along %v", position, route)
		}
		position = next
	}
}

func TestTightenRoute_WalksEveryRouteToItsEnd(t *testing.T) {
	for _, maxLength := range []float64{0.3, 0.7, 1.0} {
		t.Run(fmt.Sprintf("maxLength %v", maxLength), func(t *testing.T) {
			walksEveryRouteToItsEnd(t, maxLength)
		})
	}
}

func walksEveryRouteToItsEnd(t *testing.T, maxLength float64) {
	t.Helper()
	rng := rand.New(rand.NewPCG(7, 8))
	walks := 0
	for trial := range 500 {
		terrain := &testTerrain{width: 12, height: 12, blocked: map[tilemap.TileCoord]bool{}}
		for range 28 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(12), Y: rng.IntN(12)}] = true
		}
		s := newRouteSystem(terrain)
		radius := 0.05 + rng.Float64()*0.44
		position, goal := legalPoint(rng, terrain, radius), legalPoint(rng, terrain, radius)
		if _, ok := s.FindRoute(position, goal); !ok {
			continue
		}
		walks++

		for leg := 0; position != goal; leg++ {
			if leg > 1000 {
				t.Fatalf("trial %d, radius %v: no arrival at %v after 1000 legs, stuck at %v", trial, radius, goal, position)
			}
			route, ok := s.FindRoute(position, goal)
			if !ok {
				t.Fatalf("trial %d: route lost from %v", trial, position)
			}
			next, ok := TightenRoute(terrain, position, route, radius, maxLength)
			if !ok {
				t.Fatalf("trial %d, radius %v: no leg from %v along %v", trial, radius, position, route)
			}
			if !WalkIsClear(terrain, position, next, radius) {
				t.Fatalf("trial %d: leg from %v to %v is not walkable", trial, position, next)
			}
			position = next
		}
	}
	if walks < 150 {
		t.Fatalf("only %d walks ran; the test is not exercising routes", walks)
	}
}

func TestTightenRoute_Panics(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}
	route := []geometry.Vector2{v(2.5, 1.5)}
	for name, call := range map[string]func(){
		"negative radius": func() { TightenRoute(terrain, v(1.5, 1.5), route, -0.1, 1.0) },
		"zero max length": func() { TightenRoute(terrain, v(1.5, 1.5), route, 0.25, 0) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected a panic", name)
				}
			}()
			call()
		}()
	}
}
