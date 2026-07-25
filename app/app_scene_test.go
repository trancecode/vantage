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

	// initCalls records every (width, height) Init was called with, so tests
	// can assert scenes are initialized against the real render size.
	initCalls [][2]int
}

func (s *stubScene) SceneName() scene.SceneName   { return s.name }
func (s *stubScene) Init(w, h int)                { s.initCalls = append(s.initCalls, [2]int{w, h}) }
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
	if _, isShowcase := registered.(*scene.SpriteShowcaseScene); !isShowcase {
		t.Fatalf("scene registered under the showcase name is a %T, want *scene.SpriteShowcaseScene", registered)
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

func TestShowcaseRequestedTrueOnlyWhenNamed(t *testing.T) {
	if showcaseRequested([]scene.SceneName{"rts"}) {
		t.Fatal("showcaseRequested is true without the showcase name among the requested names")
	}
	if showcaseRequested(nil) {
		t.Fatal("showcaseRequested is true for an empty selection")
	}
	if !showcaseRequested([]scene.SceneName{"rts", scene.SpriteShowcaseSceneName}) {
		t.Fatal("showcaseRequested is false despite the showcase name being requested")
	}
}

// TestInitScenesUsesTheRenderSizeNotTheMonitor pins the fix for cameras that
// sized themselves to the monitor while rendering into a smaller window, which
// scaled their zoom by the wrong factor.
func TestInitScenesUsesTheRenderSize(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	a.Manager().AddScene(first)

	a.initScenes(800, 600)

	if len(first.initCalls) != 1 {
		t.Fatalf("Init called %d times, want 1", len(first.initCalls))
	}
	if first.initCalls[0] != [2]int{800, 600} {
		t.Fatalf("Init called with %v, want [800 600]", first.initCalls[0])
	}
}

func TestInitScenesSkipsAnUnchangedSize(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	a.Manager().AddScene(first)

	a.initScenes(800, 600)
	a.initScenes(800, 600)
	a.initScenes(800, 600)

	if len(first.initCalls) != 1 {
		t.Fatalf("Init called %d times for an unchanged size, want 1", len(first.initCalls))
	}
}

// TestInitScenesReinitializesOnResize covers the contract scene.Scene
// documents: Init runs again every time the screen resolution changes.
func TestInitScenesReinitializesOnResize(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	a.Manager().AddScene(first)

	a.initScenes(800, 600)
	a.initScenes(1024, 768)

	want := [][2]int{{800, 600}, {1024, 768}}
	if len(first.initCalls) != len(want) {
		t.Fatalf("Init calls = %v, want %v", first.initCalls, want)
	}
	for i := range want {
		if first.initCalls[i] != want[i] {
			t.Fatalf("Init call %d = %v, want %v", i, first.initCalls[i], want[i])
		}
	}
}

// TestLayoutInitializesScenesWithWhatItReturns ties the two together: whatever
// Layout reports as the render size is what scenes are initialized against.
func TestLayoutInitializesScenesWithWhatItReturns(t *testing.T) {
	a := newTestApp(t)
	first := &stubScene{name: "first"}
	a.Manager().AddScene(first)

	width, height := a.Layout(800, 600)

	if len(first.initCalls) != 1 {
		t.Fatalf("Init called %d times, want 1", len(first.initCalls))
	}
	if first.initCalls[0] != [2]int{width, height} {
		t.Fatalf("Init called with %v, but Layout returned [%d %d]",
			first.initCalls[0], width, height)
	}
}
