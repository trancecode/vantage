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

// Grid geometry for the showcase, in tiles. The pitches and label offsets are
// the values for art that fits inside one tile; showcaseMetricsFor scales them
// up for larger art, so they act as floors rather than fixed values.
const (
	// showcaseMinColumnPitch is the horizontal distance between two cells
	// holding one-tile-wide art.
	showcaseMinColumnPitch = 2.0
	// showcaseMinRowPitch is the vertical distance between two rows of
	// one-tile-tall art, leaving room for the sprite and both labels.
	showcaseMinRowPitch = 4.0
	// showcaseOriginX is the left edge of the grid.
	showcaseOriginX = 1.0
	// showcaseOriginY is the top edge of the grid.
	showcaseOriginY = 1.0
	// showcaseMinLabelCenterX offsets a label to the horizontal center of
	// one-tile-wide art.
	showcaseMinLabelCenterX = 0.5
	// showcaseMinSpriteLabelY offsets the sprite name below one-tile-tall art.
	showcaseMinSpriteLabelY = 1.2
	// showcaseMinAnimationLabelY offsets the animation name below the sprite
	// name, for one-tile-tall art.
	showcaseMinAnimationLabelY = 1.8
	// showcaseColumnsPerRow caps how many single-animation sprites share a row,
	// so their labels do not overlap.
	showcaseColumnsPerRow = 10
)

// showcaseMetrics is the grid geometry for one library, in tiles. Every field
// is derived from the largest frame in that library, so art bigger than a tile
// gets proportionally more room instead of overlapping its neighbours.
type showcaseMetrics struct {
	// ColumnPitch is the horizontal distance between two cells.
	ColumnPitch float64
	// RowPitch is the vertical distance between two rows.
	RowPitch float64
	// LabelCenterX offsets a label to the horizontal center of its cell.
	LabelCenterX float64
	// SpriteLabelY offsets the sprite name below the sprite.
	SpriteLabelY float64
	// AnimationLabelY offsets the animation name below the sprite name.
	AnimationLabelY float64
}

// showcaseMetricsFor measures the largest frame in a library and scales the
// grid geometry to it. A frame's footprint is its bounds multiplied by its
// sprite's Scale, converted to tiles with render.TileSize; the largest width
// scales the horizontal geometry and the largest height the vertical geometry,
// each floored at one tile so a library of tile-sized art (or an empty one)
// lays out exactly as the showcaseMin constants describe.
//
// It reads only image bounds, which is metadata available without a running
// game loop, so it stays testable headlessly. Sprite.VisibleBounds would be a
// tighter measurement but scans pixels, which is not.
func showcaseMetricsFor(library *render.SpriteLibrary) showcaseMetrics {
	widthTiles, heightTiles := 1.0, 1.0
	for _, name := range library.Names() {
		sprite, ok := library.Get(name)
		if !ok {
			continue
		}
		for _, animation := range sprite.Animations {
			for _, image := range animation.Images {
				if image == nil {
					continue
				}
				bounds := image.Bounds()
				widthTiles = max(widthTiles, float64(bounds.Dx())*sprite.Scale/render.TileSize)
				heightTiles = max(heightTiles, float64(bounds.Dy())*sprite.Scale/render.TileSize)
			}
		}
	}
	return showcaseMetrics{
		ColumnPitch:     showcaseMinColumnPitch * widthTiles,
		RowPitch:        showcaseMinRowPitch * heightTiles,
		LabelCenterX:    showcaseMinLabelCenterX * widthTiles,
		SpriteLabelY:    showcaseMinSpriteLabelY * heightTiles,
		AnimationLabelY: showcaseMinAnimationLabelY * heightTiles,
	}
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
}

// showcaseLayout computes the ordered cells to draw for a library. It touches
// no camera and no screen, so tests can assert exact positions without a
// display: sprites with more than one animation come first, one row each with
// one column per animation sorted by String(); then single-animation sprites
// pack left to right, wrapping into a new row every showcaseColumnsPerRow
// cells. A name whose Get misses, or a sprite with zero animations, is
// skipped defensively. Cell spacing comes from showcaseMetricsFor, so a
// library of large art spreads out instead of overlapping.
func showcaseLayout(library *render.SpriteLibrary) []showcaseCell {
	metrics := showcaseMetricsFor(library)

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

		x := showcaseOriginX
		for _, animation := range animations {
			cells = append(cells, showcaseCell{Sprite: sprite, Name: name, Animation: animation, X: x, Y: y})
			x += metrics.ColumnPitch
		}
		y += metrics.RowPitch
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

		cells = append(cells, showcaseCell{Sprite: sprite, Name: name, Animation: animations[0], X: x, Y: y})

		x += metrics.ColumnPitch
		column++
		if column >= showcaseColumnsPerRow {
			x = showcaseOriginX
			y += metrics.RowPitch
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
// camera panning and zoom.
func (s *SpriteShowcaseScene) Update(duration time.Duration) error {
	s.durationSinceInit += duration
	if s.HasFocus() {
		s.cameraController.HandleInput()
	}
	return nil
}

// Draw renders every cell cellsToDraw returns, labelled at the offsets the
// library's own art size calls for.
func (s *SpriteShowcaseScene) Draw(screen *ebiten.Image) {
	metrics := showcaseMetricsFor(s.library)
	for _, cell := range s.cellsToDraw() {
		s.drawCell(screen, metrics, cell)
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

// drawCell draws one cell's animation at its tile position, with the sprite
// name and the animation name centered beneath it.
func (s *SpriteShowcaseScene) drawCell(screen *ebiten.Image, metrics showcaseMetrics, cell showcaseCell) {
	position := geometry.NewVector2(cell.X, cell.Y)
	cell.Sprite.DrawAnimation(screen, s.Camera, position, cell.Animation, s.durationSinceInit)

	labelX := cell.X + metrics.LabelCenterX
	s.drawLabel(screen, cell.Name, labelX, cell.Y+metrics.SpriteLabelY)

	animationName, _ := strings.CutPrefix(cell.Animation.String(), "Animation")
	s.drawLabel(screen, animationName, labelX, cell.Y+metrics.AnimationLabelY)
}

// drawLabel draws centered text on a dark backdrop at a tile position.
func (s *SpriteShowcaseScene) drawLabel(screen *ebiten.Image, msg string, x, y float64) {
	render.TextDefault.
		WithAlignment(render.AlignCenter).
		WithBackground(showcaseLabelBackground).
		Text(msg).
		Draw(screen, s.Camera, geometry.NewVector2(x, y))
}
