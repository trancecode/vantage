package scene

import (
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// SceneName identifies a scene within a Manager. Each game defines its own
// SceneName constants; the engine reserves only DialogSceneName.
type SceneName string

// Scene defines the interface for game scenes.
type Scene interface {
	// SceneName returns the name of the scene
	SceneName() SceneName

	// Init is called once before the first Update() or Draw() call, and every time the screen resolution changes.
	Init(screenWidth, screenHeight int)
	// Update updates the scene state. Called every frame with the duration to advance the scene state for.
	Update(duration time.Duration) error

	// Draw draws the scene. Called every frame.
	Draw(screen *ebiten.Image)

	// LayerIndex returns the index that is used to determine the order in which to draw the scenes (from lowest to highest).
	LayerIndex() int

	// SetVisible specifies whether the scene should be visible on screen.
	SetVisible(v bool)

	// IsVisible returns whether the scene is visible on screen.
	IsVisible() bool

	// SetFocus specifies whether the scene should have focus.
	SetFocus(f bool)

	// HasFocus returns whether the scene has focus.
	HasFocus() bool
}
