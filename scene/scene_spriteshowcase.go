package scene

import (
	"image/color"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/geometry"
	"github.com/trancecode/vantage/render"
)

// SpriteShowcaseSceneName is the SceneName used by the engine's
// SpriteShowcaseScene.
const SpriteShowcaseSceneName SceneName = "sprite_showcase"

// Grid geometry for the showcase, in tiles. The grid is a uniform contact
// sheet: every cell gets the same slot whatever its art measures, so sprites
// stay comparable side by side and one outsized sprite cannot spread the
// whole grid. Art too big for its slot is scaled down by showcaseFitScale.
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
	// showcaseSlotPixels is the side of the square each cell gives its art, in
	// pixels. One tile, which is the size the pitches and label offsets above
	// leave room for.
	showcaseSlotPixels = render.TileSize
)

// showcaseFitScale returns the display scale that fits a sprite's art inside
// one cell's slot: the largest frame dimension across its animations,
// multiplied by the sprite's own Scale, divided into showcaseSlotPixels.
//
// The result is capped at 1, so oversized art shrinks but art at or below a
// slot is left at its natural size rather than blown up. Magnifying it would
// misrepresent it: a showcase is for judging art as it will be drawn, and an
// upscaled 8x8 sprite is not what the game shows.
//
// The measurement reads only image bounds, which is metadata available without
// a running game loop, so it stays testable headlessly. Sprite.VisibleBounds
// would be a tighter measurement but scans pixels, which is not.
//
// The scale is a property of this view, never of the sprite. It is passed to
// render.Sprite.DrawAnimationScaled per draw rather than set with SetScale,
// because a library hands the same pointer to the game and setting it here
// would rescale that sprite everywhere.
func showcaseFitScale(sprite *render.Sprite) float64 {
	artPixels := 0.0
	for _, animation := range sprite.Animations {
		for _, image := range animation.Images {
			if image == nil {
				continue
			}
			bounds := image.Bounds()
			artPixels = max(artPixels, float64(max(bounds.Dx(), bounds.Dy()))*sprite.Scale)
		}
	}
	if artPixels <= 0 {
		return 1.0
	}
	return min(1.0, showcaseSlotPixels/artPixels)
}

// showcaseLabelBackground is the semi-transparent backdrop that keeps labels
// readable over sprite art.
var showcaseLabelBackground = color.RGBA{R: 0, G: 0, B: 0, A: 180}

// showcaseCell is one sprite animation to draw at a tile position, as computed
// by showcaseLayout.
type showcaseCell struct {
	// Sprite is the sprite to draw.
	Sprite *render.Sprite
	// Name is the sprite's display name in the library, used for its label.
	Name string
	// Animation is the animation to draw and label.
	Animation render.AnimationType
	// X is the cell's horizontal tile position.
	X float64
	// Y is the cell's vertical tile position.
	Y float64
	// FitScale is the display scale that fits this sprite's art in the cell's
	// slot, as returned by showcaseFitScale.
	FitScale float64
}

// showcaseLayout computes the ordered cells to draw for a library. It touches
// no camera and no screen, so tests can assert exact positions without a
// display: sprites with more than one animation come first, one row each with
// one column per animation sorted by String(); then single-animation sprites
// pack left to right, wrapping into a new row every showcaseColumnsPerRow
// cells. A name whose Get misses, or a sprite with zero animations, is
// skipped defensively. Cell spacing is the same for every library; each cell
// carries the fit scale that shrinks its own art into that fixed slot.
func showcaseLayout(library *render.SpriteLibrary) []showcaseCell {
	var multiAnimation, singleAnimation []string
	for _, name := range library.Names() {
		sprite, ok := library.Get(name)
		if !ok {
			continue
		}
		if len(sprite.AllAnimations()) > 1 {
			multiAnimation = append(multiAnimation, name)
		} else {
			singleAnimation = append(singleAnimation, name)
		}
	}

	var cells []showcaseCell
	y := showcaseOriginY

	for _, name := range multiAnimation {
		sprite, ok := library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		sort.Slice(animations, func(i, j int) bool {
			return animations[i].String() < animations[j].String()
		})

		fitScale := showcaseFitScale(sprite)
		x := showcaseOriginX
		for _, animation := range animations {
			cells = append(cells, showcaseCell{
				Sprite: sprite, Name: name, Animation: animation, X: x, Y: y, FitScale: fitScale,
			})
			x += showcaseColumnPitch
		}
		y += showcaseRowPitch
	}

	x := showcaseOriginX
	column := 0
	for _, name := range singleAnimation {
		sprite, ok := library.Get(name)
		if !ok {
			continue
		}
		animations := sprite.AllAnimations()
		if len(animations) == 0 {
			continue
		}

		cells = append(cells, showcaseCell{
			Sprite: sprite, Name: name, Animation: animations[0], X: x, Y: y, FitScale: showcaseFitScale(sprite),
		})

		x += showcaseColumnPitch
		column++
		if column >= showcaseColumnsPerRow {
			x = showcaseOriginX
			y += showcaseRowPitch
			column = 0
		}
	}

	return cells
}

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
}

// Update advances the animation clock and, when the scene has focus, applies
// camera panning and zoom. The camera controller is only built by Init, so
// updates arriving before it are clock-only.
func (s *SpriteShowcaseScene) Update(duration time.Duration) error {
	s.durationSinceInit += duration
	if s.HasFocus() && s.cameraController != nil {
		s.cameraController.HandleInput()
	}
	return nil
}

// Draw renders every cell cellsToDraw returns.
func (s *SpriteShowcaseScene) Draw(screen *ebiten.Image) {
	for _, cell := range s.cellsToDraw() {
		s.drawCell(screen, cell)
	}
}

// LayerIndex returns the bottom layer. The showcase is a full-screen scene, not
// an overlay.
func (s *SpriteShowcaseScene) LayerIndex() int {
	return 0
}

// cellsToDraw returns the cells Draw renders: the laid-out library when the
// scene is visible, and nothing when it is hidden. Tests call it directly to
// check the visibility guard, since an ebiten.Image cannot have its pixels
// read back outside a running ebiten.RunGame loop ("ui: ReadPixels cannot be
// called before the game starts"), which this package's tests do not run.
func (s *SpriteShowcaseScene) cellsToDraw() []showcaseCell {
	if !s.IsVisible() {
		return nil
	}
	return showcaseLayout(s.library)
}

// drawCell draws one cell's animation at its tile position, scaled down to the
// cell's slot, with the sprite name and the animation name centered beneath it.
// The scale is per draw, so the sprite the game shares with this scene is left
// untouched.
func (s *SpriteShowcaseScene) drawCell(screen *ebiten.Image, cell showcaseCell) {
	position := geometry.NewVector2(cell.X, cell.Y)
	cell.Sprite.DrawAnimationScaled(screen, s.Camera, position, cell.Animation, s.durationSinceInit, cell.FitScale)

	labelX := cell.X + showcaseLabelCenterX
	s.drawLabel(screen, cell.Name, labelX, cell.Y+showcaseSpriteLabelY)

	animationName, _ := strings.CutPrefix(cell.Animation.String(), "Animation")
	s.drawLabel(screen, animationName, labelX, cell.Y+showcaseAnimationLabelY)
}

// drawLabel draws centered text on a dark backdrop at a tile position.
func (s *SpriteShowcaseScene) drawLabel(screen *ebiten.Image, msg string, x, y float64) {
	render.TextDefault.
		WithAlignment(render.AlignCenter).
		WithBackground(showcaseLabelBackground).
		Text(msg).
		Draw(screen, s.Camera, geometry.NewVector2(x, y))
}
