package scene

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/ui"
)

// DialogScene is a scene that displays a modal dialog overlay.
// It captures focus when visible and forwards input to the active dialog.
type DialogScene struct {
	BaseScene

	screenWidth  int
	screenHeight int

	// dialog is the currently active dialog, or nil if no dialog is showing.
	dialog *ui.Dialog

	// onDismiss is called when the dialog is dismissed, allowing the game
	// to restore focus to the appropriate scene.
	onDismiss func()

	// skipInputThisFrame is set when the dialog is first shown to avoid
	// the same ESC keypress both opening and closing it within one frame.
	skipInputThisFrame bool
}

// NewDialogScene creates a new dialog scene.
func NewDialogScene() *DialogScene {
	return &DialogScene{}
}

// SceneName returns the scene identifier.
func (s *DialogScene) SceneName() SceneName {
	return DialogSceneName
}

// Init initializes the dialog scene with screen dimensions.
func (s *DialogScene) Init(screenWidth, screenHeight int) {
	s.screenWidth = screenWidth
	s.screenHeight = screenHeight
}

// LayerIndex returns a high layer index so the dialog renders on top of everything.
func (s *DialogScene) LayerIndex() int {
	return 100
}

// ShowDialog displays a dialog and stores the dismiss callback.
func (s *DialogScene) ShowDialog(dialog *ui.Dialog, onDismiss func()) {
	dialog.SetScreenSize(s.screenWidth, s.screenHeight)
	s.dialog = dialog
	s.onDismiss = onDismiss
	s.SetVisible(true)
	s.skipInputThisFrame = true
}

// DismissDialog hides the current dialog and calls the dismiss callback.
func (s *DialogScene) DismissDialog() {
	s.dialog = nil
	s.SetVisible(false)
	if s.onDismiss != nil {
		s.onDismiss()
		s.onDismiss = nil
	}
}

// HasDialog returns true if a dialog is currently being shown.
func (s *DialogScene) HasDialog() bool {
	return s.dialog != nil
}

// Update processes input for the active dialog.
func (s *DialogScene) Update(duration time.Duration) error {
	if !s.HasFocus() || s.dialog == nil {
		return nil
	}

	if s.skipInputThisFrame {
		s.skipInputThisFrame = false
		return nil
	}

	s.dialog.Update()
	return nil
}

// Draw renders the dialog overlay and active dialog.
func (s *DialogScene) Draw(screen *ebiten.Image) {
	if !s.IsVisible() || s.dialog == nil {
		return
	}

	s.dialog.Draw(screen)
}
