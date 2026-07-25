package app

import (
	"strings"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"github.com/trancecode/vantage/scene"
)

// stubScene is a minimal Scene for exercising scene selection.
type stubScene struct {
	scene.BaseScene
	name scene.SceneName
}

func (s *stubScene) SceneName() scene.SceneName   { return s.name }
func (s *stubScene) Init(w, h int)                {}
func (s *stubScene) LayerIndex() int              { return 0 }
func (s *stubScene) Update(d time.Duration) error { return nil }
func (s *stubScene) Draw(screen *ebiten.Image)    {}

// newTestApp returns an App whose Scene.Show is set to the given names.
func newTestApp(t *testing.T, show ...string) *App {
	t.Helper()
	s, err := LoadSettings("", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Scene.Show = show
	return New(s)
}

func TestApplySceneSelectionEmptyLeavesVisibilityAlone(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	first.SetVisible(true)
	second := &stubScene{name: "second"}
	second.SetVisible(false)
	a.Manager().AddScene(first)
	a.Manager().AddScene(second)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	if !first.IsVisible() {
		t.Fatal("empty selection hid a visible scene")
	}
	if second.IsVisible() {
		t.Fatal("empty selection showed a hidden scene")
	}
}

func TestApplySceneSelectionShowsOnlyRequestedAndFocusesFirst(t *testing.T) {
	a := newTestApp(t, "first", "second")
	first := &stubScene{name: "first"}
	second := &stubScene{name: "second"}
	third := &stubScene{name: "third"}
	third.SetVisible(true)
	a.Manager().AddScene(first)
	a.Manager().AddScene(second)
	a.Manager().AddScene(third)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	if !first.IsVisible() || !second.IsVisible() {
		t.Fatal("requested scenes are not visible")
	}
	if third.IsVisible() {
		t.Fatal("unrequested scene is still visible")
	}
	if !first.HasFocus() {
		t.Fatal("first requested scene does not have focus")
	}
	if second.HasFocus() || third.HasFocus() {
		t.Fatal("focus is not exclusive to the first requested scene")
	}
}

func TestApplySceneSelectionUnknownNameIsAnError(t *testing.T) {
	a := newTestApp(t, "nosuchscene")
	a.Manager().AddScene(&stubScene{name: "first"})

	err := a.applySceneSelection()
	if err == nil {
		t.Fatal("expected an error for an unknown scene name")
	}
	if !strings.Contains(err.Error(), "nosuchscene") {
		t.Fatalf("error does not name the unknown scene: %v", err)
	}
	if !strings.Contains(err.Error(), "first") {
		t.Fatalf("error does not list the registered scenes: %v", err)
	}
}

func TestApplySceneSelectionRegistersShowcaseOnDemand(t *testing.T) {
	a := newTestApp(t, string(scene.SpriteShowcaseSceneName))

	if _, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName); ok {
		t.Fatal("showcase should not be registered before selection")
	}
	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	registered, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName)
	if !ok {
		t.Fatal("showcase was not registered on demand")
	}
	if !registered.IsVisible() {
		t.Fatal("on-demand showcase is not visible")
	}
	if !registered.HasFocus() {
		t.Fatal("on-demand showcase does not have focus")
	}
}

func TestApplySceneSelectionKeepsGameRegisteredShowcase(t *testing.T) {
	a := newTestApp(t, string(scene.SpriteShowcaseSceneName))
	own := &stubScene{name: scene.SpriteShowcaseSceneName}
	a.Manager().AddScene(own)

	if err := a.applySceneSelection(); err != nil {
		t.Fatalf("applySceneSelection returned error: %v", err)
	}
	registered, ok := a.Manager().Scene(scene.SpriteShowcaseSceneName)
	if !ok {
		t.Fatal("scene disappeared")
	}
	if registered != scene.Scene(own) {
		t.Fatal("engine replaced the game's own scene registered under the showcase name")
	}
}
