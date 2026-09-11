// Package drawbench measures how many sprites and text labels vantage draws per
// frame. It is a package of its own, containing only test files, because
// Ebitengine permits one ebiten.RunGame per process and draws only execute
// inside the game loop: the benchmark runs one game that steps through every
// count. It needs a display; run it through task bench, which wraps it in
// xvfb-run.
//
// Under a virtual display the GPU work runs on Mesa's software rasterizer, so
// absolute frame times describe that machine rather than a game's hardware.
// What transfers is the scaling across counts and the CPU-side cost; a game
// re-measures on its own machine with the same benchmark.
package drawbench

import (
	"fmt"
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
)

const (
	// canvasWidth and canvasHeight are the window size the benchmark draws into.
	canvasWidth  = 1280
	canvasHeight = 720

	// spritePixels is the side of the synthetic sprite, one tile at the default
	// tile size.
	spritePixels = 16

	// warmupFrames are drawn at each count before timing starts, so the frame
	// times exclude uploading textures and building glyph caches.
	warmupFrames = 10

	// measuredFrames are timed at each count.
	measuredFrames = 60
)

// drawCase is one count of one kind of drawable.
type drawCase struct {
	// name is the metric suffix, e.g. "sprites-1k".
	name string
	// draw draws this case's drawables for one frame.
	draw func(screen *ebiten.Image)
}

// hash mixes an index and a salt into a fixed pseudo-random value, so drawables
// land at the same positions on every run.
func hash(i int, salt uint64) uint64 {
	h := uint64(i)*0x9E3779B97F4A7C15 ^ salt*0xC2B2AE3D27D4EB4F
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return h
}

// positions returns count hashed positions inside the canvas.
func positions(count int, salt uint64) []geometry.Vector2 {
	out := make([]geometry.Vector2, count)
	for i := range count {
		out[i] = geometry.NewVector2(float64(hash(i, salt)%canvasWidth), float64(hash(i, salt+1)%canvasHeight))
	}
	return out
}

// countLabel renders a count as the metric suffix spells it: 100, 1k, 50k.
func countLabel(count int) string {
	if count >= 1000 {
		return fmt.Sprintf("%dk", count/1000)
	}
	return fmt.Sprintf("%d", count)
}

// buildCases returns every case in the order the benchmark runs them. It creates
// images, so it runs inside the game loop.
func buildCases() []drawCase {
	img := ebiten.NewImage(spritePixels, spritePixels)
	img.Fill(color.White)
	sprite := render.NewSprite()
	sprite.AddImage(render.AnimationDefault, img)
	camera := render.NewScreenCamera(canvasWidth, canvasHeight)
	label := render.NewTextWriter().Text("label")

	var cases []drawCase
	for _, count := range []int{100, 1_000, 10_000, 50_000} {
		at := positions(count, 1)
		cases = append(cases, drawCase{
			name: "sprites-" + countLabel(count),
			draw: func(screen *ebiten.Image) {
				for _, p := range at {
					sprite.Draw(screen, camera, p, render.AnimationDefault)
				}
			},
		})
	}
	for _, count := range []int{10, 100, 1_000} {
		at := positions(count, 3)
		cases = append(cases, drawCase{
			name: "labels-" + countLabel(count),
			draw: func(screen *ebiten.Image) {
				for _, p := range at {
					label.Draw(screen, camera, p)
				}
			},
		})
	}
	return cases
}

// throughputGame draws each case for warmupFrames and then measuredFrames
// frames, and records the mean wall time between consecutive frames while
// measuring. With vsync off and ticks synchronized with frames, that interval
// is the whole frame: the draw calls, the GPU flush and the loop's own work.
type throughputGame struct {
	cases     []drawCase
	current   int
	frame     int
	lastFrame time.Time
	elapsed   time.Duration
	results   []time.Duration
}

// Update builds the cases on the first frame and ends the game once every case
// has been measured.
func (g *throughputGame) Update() error {
	if g.cases == nil {
		g.cases = buildCases()
	}
	if g.current >= len(g.cases) {
		return ebiten.Termination
	}
	return nil
}

// Draw draws the current case and times the frame.
func (g *throughputGame) Draw(screen *ebiten.Image) {
	if g.current >= len(g.cases) {
		return
	}
	now := time.Now()
	if g.frame > warmupFrames {
		g.elapsed += now.Sub(g.lastFrame)
	}
	g.lastFrame = now
	g.cases[g.current].draw(screen)
	g.frame++
	if g.frame > warmupFrames+measuredFrames {
		g.results = append(g.results, g.elapsed/measuredFrames)
		g.current++
		g.frame = 0
		g.elapsed = 0
	}
}

// Layout keeps the canvas at its fixed size.
func (g *throughputGame) Layout(int, int) (int, int) {
	return canvasWidth, canvasHeight
}

// gameRun holds the one game loop's outcome, shared by every call to the
// benchmark in this process.
var (
	gameRun     sync.Once
	gameResults []time.Duration
	gameNames   []string
	gameErr     error
)

// BenchmarkDrawThroughput measures one frame's wall time drawing a synthetic
// 16-pixel sprite 100, 1,000, 10,000 and 50,000 times, and a short text label
// 10, 100 and 1,000 times, at hashed positions on a 1280x720 canvas, with vsync
// off. It reports ms-per-frame-<case>/op for each case. The game loop runs once
// per process, the first time the benchmark is called; later calls report the
// same results, so b.N and -benchtime do not change what it measures.
func BenchmarkDrawThroughput(b *testing.B) {
	gameRun.Do(func() {
		ebiten.SetWindowSize(canvasWidth, canvasHeight)
		ebiten.SetVsyncEnabled(false)
		ebiten.SetTPS(ebiten.SyncWithFPS)
		game := &throughputGame{}
		gameErr = ebiten.RunGame(game)
		gameResults = game.results
		for _, c := range game.cases {
			gameNames = append(gameNames, c.name)
		}
	})
	if gameErr != nil {
		b.Fatalf("running the draw throughput game: %v", gameErr)
	}
	if len(gameResults) != len(gameNames) || len(gameNames) == 0 {
		b.Fatalf("draw throughput game measured %d of %d cases", len(gameResults), len(gameNames))
	}

	for b.Loop() {
	}
	for i, name := range gameNames {
		b.ReportMetric(float64(gameResults[i].Microseconds())/1000, "ms-per-frame-"+name+"/op")
	}
}
