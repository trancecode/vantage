package motion

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/tilemap"
)

func v(x, y float64) geometry.Vector2 { return geometry.NewVector2(x, y) }

// oneBlockedTile is a 10x10 map whose only blocked tile is (5, 5), the box
// [5, 6] x [5, 6].
func oneBlockedTile() *testTerrain {
	return &testTerrain{width: 10, height: 10, blocked: map[tilemap.TileCoord]bool{{X: 5, Y: 5}: true}}
}

func TestWalkIsClear_BlockedCorners(t *testing.T) {
	terrain := oneBlockedTile()
	// Each pair of legs passes one corner of the blocked tile, first at
	// 0.4/sqrt(2) = 0.283 (clear of a 0.25 body), then at 0.3/sqrt(2) = 0.212
	// (touching). Both ends of every leg stand well clear of the tile.
	cases := []struct {
		name     string
		from, to geometry.Vector2
		want     bool
	}{
		{"corner (5, 5) clear", v(4.1, 5.5), v(5.5, 4.1), true},
		{"corner (5, 5) touching", v(4.15, 5.55), v(5.55, 4.15), false},
		{"corner (6, 5) clear", v(5.5, 4.1), v(6.9, 5.5), true},
		{"corner (6, 5) touching", v(5.45, 4.15), v(6.85, 5.55), false},
		{"corner (6, 6) clear", v(5.5, 6.9), v(6.9, 5.5), true},
		{"corner (6, 6) touching", v(5.45, 6.85), v(6.85, 5.45), false},
		{"corner (5, 6) clear", v(4.1, 5.5), v(5.5, 6.9), true},
		{"corner (5, 6) touching", v(4.15, 5.45), v(5.55, 6.85), false},
		// A diagonal step between two tile centres runs through the shared
		// corner of the tiles beside it: the corner cut FindBodyPath refuses.
		{"diagonal step past the corner", v(4.5, 5.5), v(5.5, 4.5), false},
		{"cardinal step beside the tile", v(4.5, 4.5), v(6.5, 4.5), true},
	}
	for _, c := range cases {
		if got := WalkIsClear(terrain, c.from, c.to, 0.25); got != c.want {
			t.Errorf("%s: WalkIsClear(%v, %v) = %v, want %v", c.name, c.from, c.to, got, c.want)
		}
		if got := WalkIsClear(terrain, c.to, c.from, 0.25); got != c.want {
			t.Errorf("%s reversed: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWalkIsClear_OneTileHole(t *testing.T) {
	blocked := map[tilemap.TileCoord]bool{}
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx != 0 || dy != 0 {
				blocked[tilemap.TileCoord{X: 5 + dx, Y: 5 + dy}] = true
			}
		}
	}
	terrain := &testTerrain{width: 10, height: 10, blocked: blocked}
	centre := v(5.5, 5.5)

	if !WalkIsClear(terrain, centre, centre, 0.25) {
		t.Error("a 0.25 body should fit at the centre of a one-tile hole")
	}
	if !WalkIsClear(terrain, centre, v(5.625, 5.5), 0.25) {
		t.Error("a walk ending with the body's edge at x = 5.875 should be clear")
	}
	if WalkIsClear(terrain, centre, v(5.75, 5.5), 0.25) {
		t.Error("a walk ending with the body's edge exactly on the wall should be blocked")
	}
	if WalkIsClear(terrain, centre, v(5.8, 5.5), 0.25) {
		t.Error("a walk carrying the body into the hole's wall should be blocked")
	}
}

func TestWalkIsClear_OutOfBoundsIsBlocked(t *testing.T) {
	terrain := &testTerrain{width: 10, height: 10}

	if WalkIsClear(terrain, v(0.5, 0.5), v(0.25, 0.5), 0.25) {
		t.Error("a body whose edge reaches x = 0 exactly touches the out-of-bounds tile and should be blocked")
	}
	if !WalkIsClear(terrain, v(0.5, 0.5), v(0.375, 0.5), 0.25) {
		t.Error("a body whose edge stays at x = 0.125 should be clear")
	}
}

func TestWalkIsClear_BodyAlreadyTouchingCannotWalk(t *testing.T) {
	terrain := oneBlockedTile()

	// (4.9, 5.5) is 0.1 from the blocked tile's west edge.
	if WalkIsClear(terrain, v(4.9, 5.5), v(3.5, 5.5), 0.25) {
		t.Error("a body starting within its radius of a blocked tile should not be able to walk")
	}
}

func TestWalkIsClear_MatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for trial := range 3000 {
		terrain := &testTerrain{width: 16, height: 16, blocked: map[tilemap.TileCoord]bool{}}
		for range 40 {
			terrain.blocked[tilemap.TileCoord{X: rng.IntN(16), Y: rng.IntN(16)}] = true
		}
		// Half the points sit on tile boundaries or quarter marks, to catch
		// rounding at the column and row limits.
		point := func() geometry.Vector2 {
			if trial%2 == 0 {
				return v(float64(rng.IntN(64))/4, float64(rng.IntN(64))/4)
			}
			return v(rng.Float64()*16, rng.Float64()*16)
		}
		from, to := point(), point()
		radius := float64(rng.IntN(5)) / 8 // 0 to 0.5 in eighths

		want := bruteForceWalkIsClear(terrain, from, to, radius)
		if got := WalkIsClear(terrain, from, to, radius); got != want {
			t.Fatalf("trial %d: WalkIsClear(%v, %v, %v) = %v, brute force says %v", trial, from, to, radius, got, want)
		}
	}
}

// bruteForceWalkIsClear tests every tile of the capsule's bounding box, grown
// by a tile on every side.
func bruteForceWalkIsClear(terrain *testTerrain, from, to geometry.Vector2, radius float64) bool {
	minX := int(math.Floor(math.Min(from.X(), to.X())-radius)) - 1
	maxX := int(math.Floor(math.Max(from.X(), to.X())+radius)) + 1
	minY := int(math.Floor(math.Min(from.Y(), to.Y())-radius)) - 1
	maxY := int(math.Floor(math.Max(from.Y(), to.Y())+radius)) + 1
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			if terrain.IsInBounds(x, y) && terrain.IsWalkable(x, y) {
				continue
			}
			tile := geometry.NewRectangleFromPoints(x, y, x+1, y+1)
			if geometry.CapsuleTouchesRectangle(from, to, radius, tile) {
				return false
			}
		}
	}
	return true
}

func TestWalkIsClear_PanicsOnNegativeRadius(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for a negative radius")
		}
	}()
	WalkIsClear(&testTerrain{width: 10, height: 10}, v(1, 1), v(2, 2), -0.1)
}
