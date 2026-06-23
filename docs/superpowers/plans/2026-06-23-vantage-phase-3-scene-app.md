# Vantage extraction — Phase 3: scene framework and App

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift the scene framework and the game-loop plumbing out of nrg into vantage so a game is written by implementing scenes and registering them — never touching Ebiten's `Game` interface. Introduces a typed-string `SceneName`, a `scene.Manager`, a generic `DialogScene`, and an `app.App` that owns window setup, the run loop, screenshot capture, and an injectable global-update hook.

**Architecture:** The `scene` package holds the framework (interface, base, state, manager, dialog scene). A new `app` package holds `App`, which implements `ebiten.Game`, embeds a `scene.Manager`, and owns window/Run/Layout/screenshot/exit plumbing. Games provide scenes plus optional global per-frame logic through `App.OnUpdate`, and request shutdown via `App.RequestExit()`. The nrg-specific `ShowcaseScene` (which uses the removed sprite catalog) and the concrete `SceneName` constants stay in nrg and migrate in Phase 6.

**Tech Stack:** Go 1.26.4, Ebitengine v2, the vantage packages from Phases 1–2 (`geometry`, `util`, `render`, `ui`, `asset`).

## Global Constraints

* Module path: `github.com/trancecode/vantage`. Rewrite every moved file's `github.com/herve-quiroz/nrg/<pkg>` import to `github.com/trancecode/vantage/<pkg>`.
* Go version `1.26.4` (canonical in `go.mod`; do not change).
* Set `GOMODCACHE=/tmp/go-mod-cache` before any Go command. Ebiten-dependent tests run under `xvfb-run -a`.
* `gofmt` must be clean on all new/moved files (`gofmt -l` prints nothing). The repo lint will reject drift.
* Moved files change only by import-path rewrite and the explicit edits named in each task. No unrelated refactor/reformat.
* `SceneName` becomes `type SceneName string`. The engine defines the type and the single generic constant `DialogSceneName`. Game-specific scene-name constants (`SceneRTS`, `SceneShowcase`) are NOT defined in the engine; they stay in nrg.
* Commit author: name `Claude Code`, email `herve.quiroz+claude@gmail.com`. No `Co-Authored-By` line.
* Work directly on `main`; commit and push per task.
* Source of truth for moved packages: nrg at `/home/hqz/src/nrg`.
* Carry-forward: do NOT modify `util`/`geometry` (keep byte-identical to nrg).

## Scope notes

* **Stays in nrg (Phase 6):** `ShowcaseScene` (uses `render.Sprites`/`SpriteID`, removed in Phase 2); the `SceneRTS`/`SceneShowcase` constants; the in-game menu content and the ESC-menu / SPACE-pause global input (these will be wired through `App.OnUpdate` and `DialogScene` in Phase 6).
* **Deferred:** the `util.DebugMode` package flag stays as-is (Phase 4 config). Video recording is a separate follow-up built onto the `App` capture subsystem.

## File structure (Phase 3)

* `scene/scene.go` — `type SceneName string`, `Scene` interface (moved; enum/constants removed).
* `scene/scene_base.go`, `scene/scene_state.go` — moved unchanged but for import paths.
* `scene/scene_dialog.go` — moved; `DialogScene` returns the engine constant `DialogSceneName`.
* `scene/scene_manager.go` — NEW: `Manager` (registry, visibility, focus, layered update/draw, init).
* `scene/scene_manager_test.go` — NEW.
* DELETE (do not copy): `scene/scenename_string.go` (int-enum stringer), `scene/scene_showcase.go` (game content).
* `app/app.go` — NEW: `Config`, `App` (implements `ebiten.Game`), `Run`, `OnUpdate`, `RequestExit`, `ErrExit`.
* `app/app_screenshot.go` — NEW: screenshot capture (ported from nrg `game_screenshot.go`), driven by `App`.
* `app/doc.go`, `app/app_test.go` — NEW.

---

## Task 1: Scene framework (typed-string SceneName, base, state)

**Files:**
- Create (copied from `/home/hqz/src/nrg/scene/`): `scene.go`, `scene_base.go`, `scene_state.go`
- Do NOT copy: `scenename_string.go`, `scene_showcase.go`, `scene_dialog.go` (dialog comes in Task 3)
- Modify after copy: `scene.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/render` (BaseScene holds `*render.Camera`).
- Produces: `github.com/trancecode/vantage/scene` exporting `type SceneName string`, the `Scene` interface, `BaseScene`, `State`.

- [ ] **Step 1: Copy the three framework files**

```bash
cd ~/src/vantage && mkdir -p scene
for f in scene.go scene_base.go scene_state.go; do cp "/home/hqz/src/nrg/scene/$f" scene/; done
```

- [ ] **Step 2: Rewrite import paths in the copied files**

```bash
cd ~/src/vantage && sed -i 's#github.com/herve-quiroz/nrg/#github.com/trancecode/vantage/#g' scene/*.go
```

- [ ] **Step 3: Convert `SceneName` to a typed string in `scene/scene.go`**

In `scene/scene.go`, replace the `SceneName` type declaration, the `//go:generate stringer` directive, and the entire `const ( SceneRTS ... SceneDialog ... )` block with just:
```go
// SceneName identifies a scene within a Manager. Each game defines its own
// SceneName constants; the engine reserves only DialogSceneName.
type SceneName string
```
Leave the `Scene` interface unchanged (its `SceneName() SceneName` method now returns the string type). The `time` and `ebiten` imports stay as used by the interface.

- [ ] **Step 4: Build and vet**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && grep -rn 'herve-quiroz/nrg' scene/ && echo NRG_LEFT || echo clean
go vet ./scene/... && gofmt -l scene/
```
Expected: `clean`; vet exits 0; `gofmt -l` prints nothing. (There are no scene tests yet; the Manager test arrives in Task 2.)

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add scene/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Extract scene framework with typed-string SceneName

Moves the Scene interface, BaseScene, and State into vantage. SceneName
becomes a plain typed string; the nrg-specific int enum and its stringer are
dropped so each game defines its own scene names."
git push origin main
```

---

## Task 2: Scene `Manager`

Lift the registry/visibility/focus/iteration logic out of nrg's `game` package (`game_scenes.go`, plus the scene loops in `game.go`/`game_draw.go`) into a `Manager` type.

**Files:**
- Create: `scene/scene_manager.go`
- Test: `scene/scene_manager_test.go`

**Interfaces:**
- Produces: `scene.Manager` with `NewManager() *Manager`; methods `AddScene(Scene)`, `Init(screenWidth, screenHeight int)`, `Update(duration time.Duration) error`, `Draw(screen *ebiten.Image)`, `SetVisible(SceneName, bool)`, `ShowOnly(map[SceneName]bool)`, `SetExclusiveFocus(SceneName)`, `Scene(SceneName) (Scene, bool)`.

- [ ] **Step 1: Write `scene/scene_manager.go`**

```go
package scene

import (
	"fmt"
	"sort"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Manager owns a set of registered scenes and drives their lifecycle: it
// initializes them, updates them each frame, and draws them in layer order.
// Scenes are keyed by SceneName.
type Manager struct {
	scenes map[SceneName]Scene
}

// NewManager returns an empty scene Manager.
func NewManager() *Manager {
	return &Manager{scenes: map[SceneName]Scene{}}
}

// AddScene registers a scene. It panics if a scene with the same name is
// already registered.
func (m *Manager) AddScene(s Scene) {
	name := s.SceneName()
	if _, ok := m.scenes[name]; ok {
		panic(fmt.Sprintf("duplicate scene name: %s", name))
	}
	m.scenes[name] = s
}

// Scene returns the registered scene with the given name.
func (m *Manager) Scene(name SceneName) (Scene, bool) {
	s, ok := m.scenes[name]
	return s, ok
}

// Init initializes every registered scene with the screen dimensions.
func (m *Manager) Init(screenWidth, screenHeight int) {
	for _, s := range m.scenes {
		s.Init(screenWidth, screenHeight)
	}
}

// Update advances every registered scene by the given duration.
func (m *Manager) Update(duration time.Duration) error {
	for name, s := range m.scenes {
		if err := s.Update(duration); err != nil {
			return fmt.Errorf("updating scene %q: %w", name, err)
		}
	}
	return nil
}

// Draw renders the visible scenes onto screen in ascending layer order.
func (m *Manager) Draw(screen *ebiten.Image) {
	sceneList := make([]Scene, 0, len(m.scenes))
	for _, s := range m.scenes {
		sceneList = append(sceneList, s)
	}
	sort.Slice(sceneList, func(i, j int) bool {
		return sceneList[i].LayerIndex() < sceneList[j].LayerIndex()
	})
	for _, s := range sceneList {
		s.Draw(screen)
	}
}

// SetVisible sets the visibility of a single registered scene.
func (m *Manager) SetVisible(name SceneName, visible bool) {
	s, ok := m.scenes[name]
	if !ok {
		panic(fmt.Sprintf("scene not found: %s", name))
	}
	s.SetVisible(visible)
}

// ShowOnly makes the scenes in the set visible and hides all others.
func (m *Manager) ShowOnly(visible map[SceneName]bool) {
	for name, s := range m.scenes {
		s.SetVisible(visible[name])
	}
}

// SetExclusiveFocus gives focus to the named scene and removes focus from all
// others.
func (m *Manager) SetExclusiveFocus(name SceneName) {
	for n, s := range m.scenes {
		s.SetFocus(n == name)
	}
}
```

Note: nrg's `BaseScene.Draw` does not gate on visibility, and nrg's original draw loop drew all scenes regardless of `IsVisible`; this `Draw` preserves that (draws all in layer order). Visibility is consulted by scenes themselves. Do not add a visibility filter here — it would change behavior.

- [ ] **Step 2: Write `scene/scene_manager_test.go`**

```go
package scene

import (
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// fakeScene is a minimal Scene for exercising the Manager.
type fakeScene struct {
	BaseScene
	name       SceneName
	layer      int
	updates    int
	draws      int
	updateErr  error
}

func (f *fakeScene) SceneName() SceneName { return f.name }
func (f *fakeScene) Init(w, h int)        {}
func (f *fakeScene) LayerIndex() int      { return f.layer }
func (f *fakeScene) Update(d time.Duration) error {
	f.updates++
	return f.updateErr
}
func (f *fakeScene) Draw(screen *ebiten.Image) { f.draws++ }

func TestManagerAddSceneDuplicatePanics(t *testing.T) {
	m := NewManager()
	m.AddScene(&fakeScene{name: "a"})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate scene name")
		}
	}()
	m.AddScene(&fakeScene{name: "a"})
}

func TestManagerUpdateAllAndPropagatesError(t *testing.T) {
	m := NewManager()
	good := &fakeScene{name: "good"}
	m.AddScene(good)
	if err := m.Update(time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if good.updates != 1 {
		t.Fatalf("expected 1 update, got %d", good.updates)
	}
}

func TestManagerSetExclusiveFocus(t *testing.T) {
	m := NewManager()
	a := &fakeScene{name: "a"}
	b := &fakeScene{name: "b"}
	m.AddScene(a)
	m.AddScene(b)
	m.SetExclusiveFocus("a")
	if !a.HasFocus() {
		t.Fatal("scene a should have focus")
	}
	if b.HasFocus() {
		t.Fatal("scene b should not have focus")
	}
}

func TestManagerShowOnly(t *testing.T) {
	m := NewManager()
	a := &fakeScene{name: "a"}
	b := &fakeScene{name: "b"}
	m.AddScene(a)
	m.AddScene(b)
	m.ShowOnly(map[SceneName]bool{"a": true})
	if !a.IsVisible() {
		t.Fatal("scene a should be visible")
	}
	if b.IsVisible() {
		t.Fatal("scene b should be hidden")
	}
}
```

- [ ] **Step 3: Vet, format, and test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./scene/... && gofmt -l scene/ && xvfb-run -a go test ./scene/...
```
Expected: vet clean; `gofmt -l` silent; tests PASS.

- [ ] **Step 4: Commit**

```bash
cd ~/src/vantage
git add scene/scene_manager.go scene/scene_manager_test.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add scene Manager (registry, focus, visibility, layered draw)

Lifts the scene registry and lifecycle out of nrg's game package: AddScene,
Init, Update, layer-ordered Draw, SetVisible, ShowOnly, SetExclusiveFocus."
git push origin main
```

---

## Task 3: Generic `DialogScene`

Move nrg's `DialogScene` into the engine and key it on an engine-defined `SceneName`.

**Files:**
- Create (copied from nrg): `scene/scene_dialog.go`
- Modify after copy: `scene/scene_dialog.go` (SceneName), `scene/scene.go` (add the constant)

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/ui`.
- Produces: `scene.DialogScene` (with `NewDialogScene`, `ShowDialog`, `DismissDialog`, `HasDialog`, etc., unchanged) returning `DialogSceneName`; and `const DialogSceneName SceneName = "dialog"` in `scene.go`.

- [ ] **Step 1: Add the engine dialog constant to `scene/scene.go`**

Directly below the `SceneName` type declaration, add:
```go
// DialogSceneName is the SceneName used by the engine's DialogScene.
const DialogSceneName SceneName = "dialog"
```

- [ ] **Step 2: Copy and rewrite `scene_dialog.go`**

```bash
cd ~/src/vantage && cp /home/hqz/src/nrg/scene/scene_dialog.go scene/
sed -i 's#github.com/herve-quiroz/nrg/#github.com/trancecode/vantage/#g' scene/scene_dialog.go
```
Then in `scene/scene_dialog.go`, change the `SceneName()` method body from `return SceneDialog` to `return DialogSceneName`. Confirm no other `SceneDialog`/`SceneRTS`/`SceneShowcase` references remain in the file:
```bash
grep -n 'SceneDialog\|SceneRTS\|SceneShowcase' scene/scene_dialog.go && echo "STALE CONST REF" || echo "ok"
```
Expected: `ok`. (If a stale reference remains, replace `SceneDialog` with `DialogSceneName`; report any `SceneRTS`/`SceneShowcase` reference as unexpected.)

- [ ] **Step 3: Build, vet, format, test**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && grep -rn 'herve-quiroz/nrg' scene/ && echo NRG_LEFT || echo clean
go vet ./scene/... && gofmt -l scene/ && xvfb-run -a go test ./scene/...
```
Expected: `clean`; vet exits 0; `gofmt -l` silent; tests PASS.

- [ ] **Step 4: Commit**

```bash
cd ~/src/vantage
git add scene/scene_dialog.go scene/scene.go
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Move DialogScene into the engine

The modal dialog overlay scene is generic; it now keys on the engine-defined
DialogSceneName constant instead of nrg's removed SceneDialog enum value."
git push origin main
```

---

## Task 4: The `App` (ebiten.Game owner)

A new `app` package whose `App` implements `ebiten.Game`, embeds a `scene.Manager`, and owns window setup, the run loop, frame timing, the debug watchdog, exit handling, and an injectable global-update hook. Screenshot capture is added in Task 5.

**Files:**
- Create: `app/app.go`, `app/doc.go`
- Test: `app/app_test.go`

**Interfaces:**
- Consumes: `github.com/trancecode/vantage/scene`, `github.com/trancecode/vantage/util`.
- Produces: `github.com/trancecode/vantage/app` exporting `Config`, `App`, `New(Config) *App`, `(*App).Manager() *scene.Manager`, `(*App).Run() error`, `(*App).RequestExit()`, field `OnUpdate func(time.Duration) error`, and `ErrExit`. `App` implements `ebiten.Game` (`Update`, `Draw`, `Layout`).

- [ ] **Step 1: Write `app/app.go`**

```go
package app

import (
	"errors"
	"fmt"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/trancecode/vantage/scene"
	"github.com/trancecode/vantage/util"
)

// ErrExit is returned from Run when the application exits normally, either
// because a game requested exit or the configured ExitAfter elapsed.
var ErrExit = errors.New("application exit requested")

// Config configures an App's window and run behavior. Games fill this in and
// pass it to New; the engine owns the Ebiten window and run loop.
type Config struct {
	// WindowTitle is the OS window title.
	WindowTitle string
	// WindowWidth and WindowHeight set the window size in pixels. When either
	// is zero, the App uses the monitor size and goes fullscreen.
	WindowWidth  int
	WindowHeight int
	// ExitAfter, when non-zero, exits the app after this much wall-clock time.
	// Intended for automated testing and profiling.
	ExitAfter time.Duration
}

// App is the engine's top-level game object. It implements ebiten.Game so that
// games never have to: games register scenes on the Manager and optionally set
// OnUpdate for global per-frame logic.
type App struct {
	config  Config
	manager *scene.Manager

	// OnUpdate, when set, runs once per frame before scenes update. Games use
	// it for global input and logic (menus, pause, hotkeys) without
	// implementing ebiten.Game. Returning a non-nil error stops the loop.
	OnUpdate func(duration time.Duration) error

	screenWidth, screenHeight int
	lastFrameRealTime         time.Time
	watchdog                  *util.Watchdog
	exitRequested             bool
	exitAt                    time.Time
}

// New returns an App with the given configuration and an empty scene Manager.
func New(config Config) *App {
	return &App{
		config:  config,
		manager: scene.NewManager(),
	}
}

// Manager returns the App's scene Manager for registering and controlling scenes.
func (a *App) Manager() *scene.Manager {
	return a.manager
}

// RequestExit asks the app to exit cleanly at the end of the current frame.
func (a *App) RequestExit() {
	a.exitRequested = true
}

// Run sets up the window, initializes scenes, and runs the Ebiten loop. It
// returns nil on a clean exit and any other error from the loop.
func (a *App) Run() error {
	if a.config.WindowWidth > 0 && a.config.WindowHeight > 0 {
		ebiten.SetWindowSize(a.config.WindowWidth, a.config.WindowHeight)
	} else {
		w, h := ebiten.Monitor().Size()
		ebiten.SetWindowSize(w, h)
		ebiten.SetFullscreen(true)
	}
	ebiten.SetWindowTitle(a.config.WindowTitle)

	a.screenWidth, a.screenHeight = ebiten.Monitor().Size()
	a.manager.Init(a.screenWidth, a.screenHeight)

	if a.config.ExitAfter > 0 {
		a.exitAt = time.Now().Add(a.config.ExitAfter)
	}

	if err := ebiten.RunGame(a); err != nil {
		if errors.Is(err, ErrExit) {
			return nil
		}
		return err
	}
	return nil
}

// Update implements ebiten.Game.
func (a *App) Update() error {
	if util.DebugMode {
		if a.watchdog == nil {
			a.watchdog = util.NewReusableWatchdog("app.Update", time.Second)
		}
		a.watchdog.Kick()
		defer a.watchdog.Done()
	}

	if a.lastFrameRealTime.IsZero() {
		a.lastFrameRealTime = time.Now()
	}
	duration := time.Since(a.lastFrameRealTime)
	defer func() { a.lastFrameRealTime = time.Now() }()

	if a.exitRequested {
		return ErrExit
	}
	if !a.exitAt.IsZero() && time.Now().After(a.exitAt) {
		util.Logger.Info().Msg("Automatic exit time reached")
		return ErrExit
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		util.DebugMode = !util.DebugMode
	}

	if a.OnUpdate != nil {
		if err := a.OnUpdate(duration); err != nil {
			return err
		}
	}

	if err := a.manager.Update(duration); err != nil {
		return fmt.Errorf("updating scenes: %w", err)
	}
	return nil
}

// Draw implements ebiten.Game.
func (a *App) Draw(screen *ebiten.Image) {
	util.Log.PrintFpsCounter()
	a.manager.Draw(screen)
	util.Log.Draw(screen)
}

// Layout implements ebiten.Game.
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	scale := ebiten.Monitor().DeviceScaleFactor()
	return int(float64(outsideWidth) * scale), int(float64(outsideHeight) * scale)
}
```

- [ ] **Step 2: Write `app/doc.go`**

```go
// Package app provides the engine's top-level App, which implements
// ebiten.Game so games do not have to. An App owns the window, the run loop,
// frame timing, the debug watchdog, screenshot capture, and exit handling, and
// embeds a scene.Manager. Games register scenes and, optionally, set OnUpdate
// for global per-frame logic.
package app
```

- [ ] **Step 3: Write `app/app_test.go`**

```go
package app

import (
	"testing"
	"time"
)

func TestNewAppHasManager(t *testing.T) {
	a := New(Config{WindowTitle: "test"})
	if a.Manager() == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestRequestExitMakesUpdateReturnErrExit(t *testing.T) {
	a := New(Config{})
	a.RequestExit()
	err := a.Update()
	if err != ErrExit {
		t.Fatalf("expected ErrExit, got %v", err)
	}
}

func TestOnUpdateErrorStopsLoop(t *testing.T) {
	a := New(Config{})
	sentinel := time.Duration(0)
	wantErr := errTest
	a.OnUpdate = func(d time.Duration) error {
		sentinel = d
		return wantErr
	}
	if err := a.Update(); err != wantErr {
		t.Fatalf("expected propagated OnUpdate error, got %v", err)
	}
	_ = sentinel
}

var errTest = &appTestError{}

type appTestError struct{}

func (*appTestError) Error() string { return "test error" }
```

- [ ] **Step 4: Build, vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go mod tidy && go vet ./app/... && gofmt -l app/ && xvfb-run -a go test ./app/...
```
Expected: vet clean; `gofmt -l` silent; tests PASS. (`App.Update` does not create images or a window, so these tests run without a display, but xvfb is harmless.)

- [ ] **Step 5: Commit**

```bash
cd ~/src/vantage
git add app/ go.mod go.sum
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add app.App implementing ebiten.Game

App owns window setup, the run loop, frame timing, the debug watchdog, F12
debug toggle, ExitAfter, and exit handling, and embeds a scene.Manager. Games
register scenes and set the optional OnUpdate hook for global per-frame logic
without touching ebiten.Game."
git push origin main
```

---

## Task 5: Screenshot capture in the `App`

Port nrg's `game_screenshot.go` into the `app` package and drive it from `App` (instead of the external `ScreenshotGame` wrapper), so capture is engine-owned.

**Files:**
- Create: `app/app_screenshot.go`
- Modify: `app/app.go` (Config screenshot fields; capture wiring in `Run`/`Update`/`Draw`)
- Test: `app/app_screenshot_test.go`

**Interfaces:**
- Produces: `Config` gains a `Screenshot ScreenshotConfig` field; `ScreenshotConfig{Path string; Delay, Frequency time.Duration}`. Internal `screenshotCapturer` driven by `App`. Exported helper `SaveScreenshot(img *ebiten.Image, path string) error`.

- [ ] **Step 1: Write `app/app_screenshot.go`**

```go
package app

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// ScreenshotConfig configures automatic screenshot capture. When Path is empty,
// capture is disabled. A Path containing a '%d'-style verb enables frame
// sequences captured every Frequency of simulated time after the initial Delay;
// otherwise a single screenshot is captured once after Delay.
type ScreenshotConfig struct {
	Path      string
	Delay     time.Duration
	Frequency time.Duration
}

// screenshotCapturer tracks simulated time and decides when to capture frames.
type screenshotCapturer struct {
	path      string
	delay     time.Duration
	frequency time.Duration
	sequence  bool

	totalSimulatedTime time.Duration
	captureCount       int
	shouldCapture      bool
	done               bool
}

func newScreenshotCapturer(cfg ScreenshotConfig) *screenshotCapturer {
	return &screenshotCapturer{
		path:      cfg.Path,
		delay:     cfg.Delay,
		frequency: cfg.Frequency,
		sequence:  strings.Contains(cfg.Path, "%"),
	}
}

// tick advances simulated time and sets shouldCapture when a frame is due.
func (s *screenshotCapturer) tick(duration time.Duration) {
	s.totalSimulatedTime += duration
	if s.done || s.totalSimulatedTime < s.delay {
		return
	}
	if s.sequence {
		timeSinceDelay := s.totalSimulatedTime - s.delay
		expectedCaptures := int(timeSinceDelay/s.frequency) + 1
		if expectedCaptures > s.captureCount {
			s.shouldCapture = true
		}
	} else if s.captureCount == 0 {
		s.shouldCapture = true
	}
}

// capture writes a screenshot of screen if one is due this frame.
func (s *screenshotCapturer) capture(screen *ebiten.Image) {
	if !s.shouldCapture {
		return
	}
	s.shouldCapture = false
	s.captureCount++

	path := s.path
	if s.sequence {
		path = fmt.Sprintf(s.path, s.captureCount)
	} else {
		s.done = true
	}

	if err := SaveScreenshot(screen, path); err != nil {
		logger.Error().Err(err).Msgf("Failed to save screenshot to %s", path)
	} else {
		logger.Info().Msgf("Screenshot saved: %s", path)
	}
}

// SaveScreenshot encodes img as a PNG at filePath, creating parent directories.
func SaveScreenshot(img *ebiten.Image, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]byte, width*height*4)
	img.ReadPixels(pixels)

	rgbaImg := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		pixelIndex := i / 4
		x := pixelIndex % width
		y := pixelIndex / width
		rgbaImg.Set(x, y, color.RGBA{R: pixels[i], G: pixels[i+1], B: pixels[i+2], A: pixels[i+3]})
	}

	if err := png.Encode(file, rgbaImg); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode screenshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close screenshot file: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Wire the capturer into `app/app.go`**

1. Add a logger alias near the top of `app.go` (after imports) so `app_screenshot.go` can log without re-importing util:
```go
var logger = util.Logger
```

2. Add the screenshot field to `Config` (after `ExitAfter`):
```go
	// Screenshot configures automatic screenshot capture (disabled when its
	// Path is empty).
	Screenshot ScreenshotConfig
```

3. Add a `screenshot *screenshotCapturer` field to the `App` struct (after `watchdog`).

4. In `Run`, after setting `exitAt` and before `ebiten.RunGame(a)`, initialize the capturer when configured:
```go
	if a.config.Screenshot.Path != "" {
		a.screenshot = newScreenshotCapturer(a.config.Screenshot)
		util.Logger.Info().Msgf("Screenshot capture enabled: path=%s delay=%s frequency=%s",
			a.config.Screenshot.Path, a.config.Screenshot.Delay, a.config.Screenshot.Frequency)
	}
```

5. In `Update`, after computing `duration` (right after the `defer` that records `lastFrameRealTime`), advance the capturer:
```go
	if a.screenshot != nil {
		a.screenshot.tick(duration)
	}
```

6. In `Draw`, after `a.manager.Draw(screen)` and before `util.Log.Draw(screen)`, capture if due:
```go
	if a.screenshot != nil {
		a.screenshot.capture(screen)
	}
```

- [ ] **Step 3: Write `app/app_screenshot_test.go`**

```go
package app

import (
	"testing"
	"time"
)

func TestSingleCaptureAfterDelay(t *testing.T) {
	c := newScreenshotCapturer(ScreenshotConfig{Path: "/tmp/shot.png", Delay: time.Second})
	c.tick(500 * time.Millisecond)
	if c.shouldCapture {
		t.Fatal("should not capture before delay")
	}
	c.tick(600 * time.Millisecond) // total 1.1s >= 1s delay
	if !c.shouldCapture {
		t.Fatal("should capture once after delay")
	}
}

func TestSequenceCapturesAtFrequency(t *testing.T) {
	c := newScreenshotCapturer(ScreenshotConfig{
		Path:      "/tmp/frame-%d.png",
		Delay:     0,
		Frequency: time.Second,
	})
	if !c.sequence {
		t.Fatal("expected sequence mode for %d path")
	}
	c.tick(time.Second)
	if !c.shouldCapture {
		t.Fatal("expected first sequence capture")
	}
	c.shouldCapture = false
	c.captureCount = 1
	c.tick(time.Second) // total 2s, expectedCaptures = 3 > 1
	if !c.shouldCapture {
		t.Fatal("expected next sequence capture")
	}
}
```

- [ ] **Step 4: Build, vet, format, test under xvfb**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go vet ./app/... && gofmt -l app/ && xvfb-run -a go test ./app/...
```
Expected: vet clean; `gofmt -l` silent; all app tests PASS (capturer timing tests do not need a display).

- [ ] **Step 5: Full-module sanity build and test**

```bash
cd ~/src/vantage && export GOMODCACHE=/tmp/go-mod-cache && go build ./... && xvfb-run -a go test ./...
```
Expected: every package (`app`, `asset`, `geometry`, `render`, `scene`, `ui`, `util`) builds and tests PASS.

- [ ] **Step 6: Commit**

```bash
cd ~/src/vantage
git add app/
git -c user.name="Claude Code" -c user.email="herve.quiroz+claude@gmail.com" \
  commit -m "Add engine-owned screenshot capture to App

Ports nrg's screenshot capture into the app package, driven by App rather than
an external wrapper. Supports a single capture after a delay or a frame
sequence at a fixed frequency via a '%d' path. SaveScreenshot is exported for
direct use."
git push origin main
```

---

## Self-review (Phase 3)

* **Spec coverage:** Implements the design spec's scene-management section — typed-string `SceneName`, the `Manager` (registry, visibility/focus, layered update/draw), the thin `App` implementing `ebiten.Game` with window setup + `Run` + screenshot capture + the `ExitAfter` test hook, so games never touch Ebiten. `DialogScene` moves to the engine as the generic candidate; `ShowcaseScene` and the concrete scene-name constants stay in nrg per the spec's "details to resolve."
* **Placeholders:** none. Code for `Manager`, `App`, and the screenshot capturer is complete. The `OnUpdate` hook and `RequestExit`/`ErrExit` are the named home for nrg's ESC-menu/SPACE-pause global input, wired in Phase 6.
* **Type consistency:** `scene.NewManager`/`Manager` methods used by `App` match Task 2's definitions; `Config`/`ScreenshotConfig` field names are consistent between `app.go` and `app_screenshot.go`; `DialogSceneName` defined in Task 3 Step 1 is what `DialogScene.SceneName()` returns in Step 2.
* **Behavior preservation:** the `Manager.Draw` layer-sort and the screenshot timing logic mirror nrg's originals exactly; `App.Update`'s watchdog/timing/exit/F12 mirror nrg's `game.Update` minus the game-specific ESC/SPACE handling (relocated to `OnUpdate` for Phase 6).
* **Deferred:** `util.DebugMode` flag (Phase 4); video recording (follow-up onto the App capture subsystem); nrg migration of the menu/pause logic, `ShowcaseScene`, and scene-name constants (Phase 6).
