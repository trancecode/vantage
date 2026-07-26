package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/trancecode/vantage/render"
	"github.com/trancecode/vantage/scene"
	"github.com/trancecode/vantage/util"
)

// ErrExit is returned from Run when the application exits normally, either
// because a game requested exit or the configured run-for duration elapsed.
var ErrExit = errors.New("application exit requested")

var logger = util.Logger

// App is the engine's top-level game object. It implements ebiten.Game so that
// games never have to: games register scenes on the Manager and optionally set
// OnUpdate for global per-frame logic.
type App struct {
	settings *Settings
	manager  *scene.Manager

	// OnUpdate, when set, runs once per frame before scenes update. Games use
	// it for global input and logic (menus, pause, hotkeys) without
	// implementing ebiten.Game. Returning a non-nil error stops the loop.
	OnUpdate func(duration time.Duration) error

	screenWidth, screenHeight int
	lastFrameRealTime         time.Time
	watchdog                  *util.Watchdog
	screenshot                *screenshotCapturer
	exitRequested             bool
	exitAt                    time.Time
}

// New returns an App driven by the given settings, with an empty scene Manager.
func New(settings *Settings) *App {
	return &App{
		settings: settings,
		manager:  scene.NewManager(),
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

// Run applies settings, sets up the window, initializes scenes, and runs the
// Ebiten loop. It returns nil on a clean exit.
func (a *App) Run() error {
	if err := a.settings.validate(); err != nil {
		return err
	}
	a.settings.Apply()

	if a.settings.Window.Width > 0 && a.settings.Window.Height > 0 {
		ebiten.SetWindowSize(a.settings.Window.Width, a.settings.Window.Height)
	} else {
		w, h := ebiten.Monitor().Size()
		ebiten.SetWindowSize(w, h)
		ebiten.SetFullscreen(true)
	}
	ebiten.SetWindowTitle(a.settings.Window.Title)

	if err := a.applySceneSelection(); err != nil {
		return err
	}

	// Scenes are initialized from Layout, which is the only place the real
	// render size is known. Ebiten calls it before the first Update and Draw.

	if a.settings.Run.For.Duration > 0 {
		a.exitAt = time.Now().Add(a.settings.Run.For.Duration)
	}

	if a.settings.Screenshot.Path != "" {
		a.screenshot = newScreenshotCapturer(
			a.settings.Screenshot.Path,
			a.settings.Screenshot.Delay.Duration,
			a.settings.Screenshot.Frequency.Duration,
		)
		util.Logger.Info().Msgf("Screenshot capture enabled: path=%s delay=%s frequency=%s",
			a.settings.Screenshot.Path, a.settings.Screenshot.Delay.Duration, a.settings.Screenshot.Frequency.Duration)
	}

	if err := ebiten.RunGame(a); err != nil {
		if errors.Is(err, ErrExit) {
			return nil
		}
		return err
	}
	return nil
}

// applySceneSelection honours the [scene] show setting and the --scene flag: it
// shows exactly the named scenes and focuses the first. An empty selection
// leaves visibility to the game.
//
// It registers the engine's own SpriteShowcaseScene on demand, since no game
// registers it, and validates the requested names, because Manager.ShowOnly and
// Manager.SetExclusiveFocus silently ignore names that match nothing: a typo
// would otherwise blank the window with no diagnostic.
func (a *App) applySceneSelection() error {
	show := a.settings.Scene.Show
	if len(show) == 0 {
		return nil
	}

	names := make([]scene.SceneName, 0, len(show))
	for _, name := range show {
		names = append(names, scene.SceneName(name))
	}

	for _, name := range names {
		if name != scene.SpriteShowcaseSceneName {
			continue
		}
		if _, ok := a.manager.Scene(name); !ok {
			a.manager.AddScene(scene.NewSpriteShowcaseScene())
		}
	}

	for _, name := range names {
		if _, ok := a.manager.Scene(name); !ok {
			return fmt.Errorf("unknown scene %q, registered scenes: %s",
				name, strings.Join(a.registeredSceneNames(), ", "))
		}
	}

	a.manager.ShowOnly(names...)
	a.manager.SetExclusiveFocus(names[0])

	if showcaseRequested(names) && render.Sprites.Len() == 0 {
		logger.Warn().Msg(
			"Sprite showcase requested but no sprites are registered; register them with render.Sprites.Add")
	}

	return nil
}

// showcaseRequested reports whether names include the engine's sprite
// showcase scene, i.e. whether it ends up shown by this selection.
func showcaseRequested(names []scene.SceneName) bool {
	return slices.Contains(names, scene.SpriteShowcaseSceneName)
}

// registeredSceneNames returns every registered scene name, sorted, for error
// messages.
func (a *App) registeredSceneNames() []string {
	names := a.manager.SceneNames()
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, string(n))
	}
	return out
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

	if a.screenshot != nil {
		// Clamp the game-time advance so screenshots land on exact game-time
		// targets; this may hold the simulation (advance 0) for the frame a
		// capture is taken.
		duration = a.screenshot.advance(duration)
	}

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
	if a.screenshot != nil {
		a.screenshot.capture(screen)
	}
	util.Log.Draw(screen)
}

// Layout implements ebiten.Game.
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	scale := ebiten.Monitor().DeviceScaleFactor()
	width := int(float64(outsideWidth) * scale)
	height := int(float64(outsideHeight) * scale)
	a.initScenes(width, height)
	return width, height
}

// initScenes initializes every scene for a render size, and does so again
// whenever that size changes, which is the contract scene.Scene documents.
//
// The size has to come from Layout. It is the size of the screen image handed
// to Draw, and therefore what a scene's camera must be built against. The
// monitor size is not a substitute: a windowed game renders into something
// smaller, and a camera sized to the monitor scales its zoom by the wrong
// factor.
func (a *App) initScenes(width, height int) {
	if width == a.screenWidth && height == a.screenHeight {
		return
	}
	a.screenWidth, a.screenHeight = width, height
	a.manager.Init(width, height)
}
