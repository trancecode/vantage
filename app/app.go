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
