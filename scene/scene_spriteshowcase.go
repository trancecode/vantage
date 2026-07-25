package scene

import (
	"image/color"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/util"
)

// SpriteShowcaseSceneName is the SceneName used by the engine's
// SpriteShowcaseScene.
const SpriteShowcaseSceneName SceneName = "sprite_showcase"

// Grid geometry for the showcase, in tiles.
const (
	// showcaseColumnPitch is the horizontal distance between two cells.
	showcaseColumnPitch = 2.0
	// showcaseRowPitch is the vertical distance between two rows, leaving room
	// for the sprite and both labels.
	showcaseRowPitch = 4.0
	// showcaseOriginX is the left edge of the grid.
	showcaseOriginX = 1.0
	// showcaseOriginY is the top edge of the grid.
	showcaseOriginY = 1.0
	// showcaseLabelCenterX offsets a label to the horizontal center of its cell.
	showcaseLabelCenterX = 0.5
	// showcaseSpriteLabelY offsets the sprite name below the sprite.
	showcaseSpriteLabelY = 1.2
	// showcaseAnimationLabelY offsets the animation name below the sprite name.
	showcaseAnimationLabelY = 1.8
	// showcaseColumnsPerRow caps how many single-animation sprites share a row,
	// so their labels do not overlap.
	showcaseColumnsPerRow = 10
)

// showcaseLabelBackground is the semi-transparent backdrop that keeps labels
// readable over sprite art.
var showcaseLabelBackground = color.RGBA{R: 0, G: 0, B: 0, A: 180}

// SpriteShowcaseScene draws every sprite in a render.SpriteLibrary, animating,
// labelled with the sprite name and the name of each animation. It is a
// read-only inspection surface for checking art without building a level around
// it, reached with the --scene sprite_showcase flag.
type SpriteShowcaseScene struct {
	BaseScene

	// library is the sprite library to display.
	library *render.SpriteLibrary

	// durationSinceInit accumulates elapsed time and drives the animation phase.
	durationSinceInit time.Duration

	// cameraController maps W/A/S/D and Q/E onto the scene camera.
	cameraController *render.CameraController
}

// NewSpriteShowcaseScene returns a showcase for the package-level
// render.Sprites library, which is what a game registers its sprites into.
func NewSpriteShowcaseScene() *SpriteShowcaseScene {
	return NewSpriteShowcaseSceneFor(render.Sprites)
}

// NewSpriteShowcaseSceneFor returns a showcase for an explicit library. Tests
// use this so they can exercise the layout without mutating render.Sprites.
func NewSpriteShowcaseSceneFor(library *render.SpriteLibrary) *SpriteShowcaseScene {
	return &SpriteShowcaseScene{library: library}
}

// SceneName returns the name of the sprite showcase scene.
func (s *SpriteShowcaseScene) SceneName() SceneName {
	return SpriteShowcaseSceneName
}

// Init builds a camera anchored at the top-left corner so the grid starts there,
// and resets the animation clock.
func (s *SpriteShowcaseScene) Init(screenWidth, screenHeight int) {
	s.durationSinceInit = 0
	s.Camera = render.NewCamera(screenWidth, screenHeight)
	s.Camera.SetZeroAsTopLeft()
	s.cameraController = render.NewCameraController(s.Camera)
	util.Logger.Debug().Msg(s.Camera.CameraDebugInfo())
}

// Update advances the animation clock and, when the scene has focus, applies
// camera panning and zoom.
func (s *SpriteShowcaseScene) Update(duration time.Duration) error {
	s.durationSinceInit += duration
	if s.HasFocus() {
		s.cameraController.HandleInput()
	}
	return nil
}

// Draw renders the sprite grid.
func (s *SpriteShowcaseScene) Draw(screen *ebiten.Image) {
	if !s.IsVisible() {
		return
	}
	s.drawAllSprites(screen)
}

// LayerIndex returns the bottom layer. The showcase is a full-screen scene, not
// an overlay.
func (s *SpriteShowcaseScene) LayerIndex() int {
	return 0
}

// drawAllSprites lays out the library: sprites with several animations get a
// row each with one column per animation, then single-animation sprites pack
// into a grid wrapping at showcaseColumnsPerRow.
func (s *SpriteShowcaseScene) drawAllSprites(screen *ebiten.Image) {
	var multiAnimation, singleAnimation []string
	for _, name := range s.library.Names() {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		if len(sprite.AllAnimations()) > 1 {
			multiAnimation = append(multiAnimation, name)
		} else {
			singleAnimation = append(singleAnimation, name)
		}
	}

	y := showcaseOriginY

	for _, name := range multiAnimation {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		sort.Slice(animations, func(i, j int) bool {
			return animations[i].String() < animations[j].String()
		})

		x := showcaseOriginX
		for _, animation := range animations {
			s.drawCell(screen, sprite, name, animation, x, y)
			x += showcaseColumnPitch
		}
		y += showcaseRowPitch
	}

	x := showcaseOriginX
	column := 0
	for _, name := range singleAnimation {
		sprite, ok := s.library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		if len(animations) == 0 {
			continue
		}

		s.drawCell(screen, sprite, name, animations[0], x, y)

		x += showcaseColumnPitch
		column++
		if column >= showcaseColumnsPerRow {
			x = showcaseOriginX
			y += showcaseRowPitch
			column = 0
		}
	}
}

// drawCell draws one animation at the given tile position, with the sprite name
// and the animation name centered beneath it.
func (s *SpriteShowcaseScene) drawCell(
	screen *ebiten.Image,
	sprite *render.Sprite,
	name string,
	animation render.AnimationType,
	x, y float64,
) {
	sprite.DrawAnimation(screen, s.Camera, geometry.NewVector2(x, y), animation, s.durationSinceInit)

	labelX := x + showcaseLabelCenterX
	s.drawLabel(screen, name, labelX, y+showcaseSpriteLabelY)

	animationName, _ := strings.CutPrefix(animation.String(), "Animation")
	s.drawLabel(screen, animationName, labelX, y+showcaseAnimationLabelY)
}

// drawLabel draws centered text on a dark backdrop at a tile position.
func (s *SpriteShowcaseScene) drawLabel(screen *ebiten.Image, msg string, x, y float64) {
	render.TextDefault.
		WithAlignment(render.AlignCenter).
		WithBackground(showcaseLabelBackground).
		Text(msg).
		Draw(screen, s.Camera, geometry.NewVector2(x, y))
}
