// Command showcasedemo runs the engine's sprite showcase scene against
// procedurally generated placeholder art, so the scene can be exercised without
// a game and without any asset files.
//
// The generated sprites deliberately differ in size, from 8 to 64 pixels, and
// one declares a source tile size smaller than a tile rather than using large
// frames, which is what makes the showcase's fixed slots and its
// scale-down-to-fit behavior visible. Another declares a SourceTileSize larger
// than a single tile and carries four directional animations registered
// through render.RegisterAnimationName, so both the tile ratio and the
// animation naming path have visual coverage here too: raising
// --scene_showcase_slot_tiles shows its art at its true size instead of
// shrunk to fit a one tile slot, and its labels read "S", "E", "N", "W"
// rather than the generated AnimationType(64) placeholder.
//
// Usage:
//
//	showcasedemo [flags]
//
// It defaults to showing the sprite showcase in a window, so no flags are
// needed. Every engine flag applies, so it also serves as a way to exercise
// scene selection and screenshot capture:
//
//	showcasedemo --width 1920 --height 1080
//	showcasedemo --screenshot_path shot.png --screenshot_delay 600ms --run_for 1500ms
package main

import (
	"fmt"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	flag "github.com/spf13/pflag"

	"github.com/trancecode/vantage/app"
	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/scene"
	"github.com/trancecode/vantage/util"
)

// frameInset divides a frame's size to get its border thickness, so each
// generated frame shows its own extent once the showcase scales it.
const frameInset = 8

// Directional animation types for the demo's own SourceTileSize sprite below,
// defined at render.AnimationGameBase and up like any game-defined animation
// state. Their names are registered in init, which is well before the first
// frame reads them.
const (
	animationSouth render.AnimationType = render.AnimationGameBase + iota
	animationEast
	animationNorth
	animationWest
)

// init registers display names for this demo's own animation types, the same
// way a game registers names for its facings. Names are read when a label is
// drawn, not when a sprite is built, so any point before the first frame would
// do.
func init() {
	render.RegisterAnimationName(animationSouth, "S")
	render.RegisterAnimationName(animationEast, "E")
	render.RegisterAnimationName(animationNorth, "N")
	render.RegisterAnimationName(animationWest, "W")
}

// framesPerAnimation is how many frames each generated animation cycles
// through, enough for the animation to be visibly moving.
const framesPerAnimation = 4

// Default window size. The engine treats a zero width or height as "fullscreen
// at monitor size", which is a poor fit for a demo you want beside an editor, so
// this command picks a window unless --width or --height says otherwise.
const (
	defaultWindowWidth  = 800
	defaultWindowHeight = 600
)

// frame returns a square of the given size: a white border around a solid fill,
// so a sprite's real extent stays visible after the showcase scales it down.
func frame(size int, fill color.RGBA) *ebiten.Image {
	inset := max(size/frameInset, 1)
	img := ebiten.NewImage(size, size)
	img.Fill(color.RGBA{R: 255, G: 255, B: 255, A: 255})

	inner := ebiten.NewImage(size-2*inset, size-2*inset)
	inner.Fill(fill)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(inset), float64(inset))
	img.DrawImage(inner, op)
	return img
}

// shaded darkens a color by a factor, used to make successive animation frames
// distinguishable from each other.
func shaded(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
		A: 255,
	}
}

// demoSprite builds a sprite whose frames are all size pixels square, with one
// animation per requested type.
func demoSprite(size int, fill color.RGBA, animations ...render.AnimationType) *render.Sprite {
	s := render.NewSprite()
	for i, a := range animations {
		for f := range framesPerAnimation {
			s.AddImage(a, frame(size, shaded(fill, 0.45+0.15*float64((i+f)%framesPerAnimation))))
		}
	}
	return s
}

// registerSprites fills the default sprite library with placeholder art chosen
// to exercise the showcase: sprites with several animations, sprites with one,
// enough single-animation sprites to wrap a row, and a spread of sizes.
func registerSprites() {
	red := color.RGBA{R: 220, G: 70, B: 70, A: 255}
	green := color.RGBA{R: 70, G: 200, B: 90, A: 255}
	blue := color.RGBA{R: 80, G: 130, B: 230, A: 255}
	amber := color.RGBA{R: 230, G: 180, B: 60, A: 255}
	purple := color.RGBA{R: 160, G: 90, B: 200, A: 255}

	// Several animations each: one row per sprite, one column per animation.
	render.Sprites.Add("HeroNormal16", demoSprite(16, red,
		render.AnimationIdleDown, render.AnimationIdleRight,
		render.AnimationMoveDown, render.AnimationMoveRight))
	render.Sprites.Add("BossHuge64", demoSprite(64, blue,
		render.AnimationIdleDown, render.AnimationMoveDown, render.AnimationAttackDown))
	// 16 pixel art drawn for 4 pixel tiles, so it covers four tiles.
	render.Sprites.Add("GiantSourceTile16x4", demoSprite(16, amber,
		render.AnimationIdleDown, render.AnimationMoveDown).SetSourceTileSize(4))

	// A 64 pixel frame declaring the tile size it was drawn for, so it is two
	// tiles at the default tile size, with directional animations carrying
	// registered names instead of the generated AnimationType placeholder.
	render.Sprites.Add("DirectionalTiles64x2", demoSprite(64, purple,
		animationSouth, animationEast, animationNorth, animationWest,
	).SetSourceTileSize(32))

	// One animation each: packed ten to a row, so twelve of them wrap.
	for i, size := range []int{16, 16, 32, 8, 16, 64, 16, 24, 16, 48, 16, 16} {
		render.Sprites.Add(
			fmt.Sprintf("Tile%02d_%dpx", i+1, size),
			demoSprite(size, green, render.AnimationDefault),
		)
	}
}

func main() {
	settings, err := app.LoadSettings("", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "loading settings:", err)
		os.Exit(1)
	}
	settings.RegisterFlags(flag.CommandLine)
	flag.Parse()
	util.InitLogging()

	// Showing the showcase is the whole point of this command, so default to it
	// while still letting --scene pick something else.
	if len(settings.Scene.Show) == 0 {
		settings.Scene.Show = []string{string(scene.SpriteShowcaseSceneName)}
	}

	// Each dimension is filled in separately, since the engine only stays
	// windowed when both are non-zero.
	if settings.Window.Width == 0 {
		settings.Window.Width = defaultWindowWidth
	}
	if settings.Window.Height == 0 {
		settings.Window.Height = defaultWindowHeight
	}

	registerSprites()

	if err := app.New(settings).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
}
