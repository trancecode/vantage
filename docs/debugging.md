# Debugging and development tools

The engine's debug tooling lives in `util`, with wiring points in `app`
(settings, key bindings) and `sim` (driver profiling). Everything here is
diagnostics only and never affects the simulation.

## Debug mode

`util.DebugMode` is the global switch the other tools consult. It defaults to
`true` and is normally driven by settings rather than set directly.

* Settings: `[debug] enabled` in the game's TOML settings, or the `-debug`
  command-line flag (registered by `app.RegisterFlags`). `app.Apply` copies the
  setting into `util.DebugMode`.
* Keyboard: F12 toggles debug mode at runtime (handled in `app.App.Update`).
* When enabled, `app.App.Update` also runs under a one-second
  `util.Watchdog` that logs a warning if an update stalls.

## Forcing scenes

`[scene] show` in the game's TOML settings, or the `--scene` command-line flag,
overrides which registered scenes are visible at startup. The named scenes are
shown, every other scene is hidden, and the first name listed gets exclusive
focus.

```bash
# Show one scene
./game --scene rts

# Show two, focusing the first. Repetition and commas both work.
./game --scene rts --scene dialog
./game --scene rts,dialog
```

The equivalent settings entry:

```toml
[scene]
show = ["rts", "dialog"]
```

The three forms differ in how several scenes are written:

* `--scene` accepts repetition and comma-separated values, both split by the
  flag itself.
* `[scene] show` in a settings file is a TOML array.
* The `section.key=value` config override (see the `config` package) takes a
  TOML array too, quoting included: `scene.show=["rts","dialog"]`. The bare
  `scene.show=rts` form is not accepted, because letting overrides guess at
  malformed values would weaken every `[]string` setting a game registers.

An empty `show`, which is the default, leaves scene visibility entirely to the
game. A name that is not registered is a startup error listing the scenes that
are, since a silently ignored typo would otherwise render a black window.

The flag selects among scenes the game has already registered; it does not
construct them. The one exception is the engine's own sprite showcase below,
which `app` registers on demand.

## Sprite showcase

`--scene sprite_showcase` displays every sprite in the engine's sprite library,
animating, labelled with the sprite name and the name of each animation. Use it
to inspect art without building a level around it.

Sprites with more than one animation get a row each, with one column per
animation. Sprites with a single animation pack into a grid below them, ten to
a row.

The grid is a contact sheet: every cell is the same fixed size, whatever the
art in it measures, so sprites stay comparable side by side and one outsized
sprite cannot spread the whole grid. Each cell gives its art a slot one tile
square. A sprite whose largest frame, multiplied by its own `Scale`, is bigger
than that slot is scaled down to fit it; art at or below one tile is left at
its natural size rather than blown up, so it stays pixel honest.

The scaling is per draw. The showcase never changes a sprite's `Scale`, since
the library hands the game the same sprite pointer, and rescaling it here would
rescale it everywhere.

```bash
./game --scene sprite_showcase
```

The scene needs no wiring: `app` registers it when the flag names it. What it
shows is whatever the game registered into `render.Sprites`:

```go
render.Sprites.Add("Character", render.MustLoadSprite(
    data.ImagePlayer, 6, 10,
    map[render.AnimationType][]int{
        render.AnimationIdleDown:  {0, 1, 2, 3, 4, 5},
        render.AnimationIdleRight: {6, 7, 8, 9, 10, 11},
    },
    nil,
)).SetZeroPosition(geometry.NewVector2(24, 40)).SetType(render.SpriteTypeActor)
```

`Add` returns the sprite, so registration chains onto the sprite setters. It
panics on an empty name, a nil sprite, or a duplicate name, since all three are
load-time mistakes. If the library is empty when the showcase is shown, the
engine logs a warning rather than leaving a blank screen unexplained.

Camera controls, from `render.CameraController`:

* `W` / `A` / `S` / `D` pan.
* `Q` / `E` zoom out and in.
* The mouse wheel also zooms, alongside `Q` / `E`.
* Dragging with the middle mouse button pans.

### Running the showcase without a game

`cmd/showcasedemo` runs the showcase against procedurally generated placeholder
art, so the scene can be exercised from this repository with no game and no
asset files. It defaults to showing the showcase in an 800 by 600 window, so it
needs no flags:

```bash
go run ./cmd/showcasedemo
```

Pass `--width` and `--height` for a different size. Unlike the engine default,
this command does not go fullscreen when they are unset.

The generated sprites deliberately differ in size, from 8 to 64 pixels, and one
uses `Sprite.Scale` rather than large frames. That is what makes the fixed slots
and the scale-down-to-fit behavior visible: the 24, 32, 48 and 64 pixel tiles
all render at the size of the 16 pixel ones, while the 8 pixel tile stays small.

Every engine flag applies, so the same command also exercises scene selection
and screenshot capture. To capture a frame without a display:

```bash
xvfb-run -a go run ./cmd/showcasedemo \
    --screenshot_path shot.png \
    --screenshot_delay 600ms \
    --run_for 1500ms
```

## Screen logger (on-screen overlay)

`util.ScreenLogger` buffers debug lines each frame and draws them as an
overlay; `util.Log` is the shared instance. `Printf` and `Print` queue lines
(no-ops unless `util.DebugMode` is on), `Draw` renders and clears the buffer,
so lines must be re-queued every frame.

```go
util.Log.PrintFpsCounter()                 // "FPS: ..."
util.Log.Printf("entities: %d", n)
util.Log.Draw(screen)                      // from the game's Draw
```

Note: `ScreenLogger` has a `//go:build race` variant in `util_debug_race.go`.
New methods must be added to both files or the `-race` gate fails.

## Profiler

`util.Profiler` accumulates named wall-time timings. `Record(name, d)` adds a
sample; `Snapshot()` returns per-phase totals, averages, and call counts
sorted by total time descending.

The sim driver profiles itself when a profiler is attached: it records each
registered tick system (labelled by concrete type name) and the event drain.
A nil profiler (the default) disables profiling with zero overhead.

```go
profiler := util.NewProfiler()
driver.SetProfiler(profiler)              // before RunUntil
// ...
util.Log.PrintProfiler(profiler)          // one overlay line per phase
```

`ScreenLogger.PrintProfiler` renders a snapshot on the debug overlay: name,
total, average, and call count per phase.

## Debug HTTP server

`util.StartDebugHTTPServer(port, debugMode)` serves pprof and expvar endpoints
for live inspection. It returns `nil, nil` immediately when `debugMode` is
false. Otherwise it binds the listener before returning and hands back the
`*http.Server`; the caller owns its lifetime and must shut it down (for
example via `Shutdown` or `Close`) when it is no longer needed.

* Settings: `[debug] http_enabled` and `[debug] http_port`, or the
  `-enable_debug_http_server` and `-debug_http_port` flags.
* Endpoints: `/debug/pprof/` (CPU, heap, goroutines) and `/debug/vars`
  (expvar), plus an index page at `/`.

## Watchdogs

`util.NewWatchdog(name, timeout)` returns a stop function and logs a warning
if the stop does not happen within the timeout, for one-shot stall detection.
`util.NewReusableWatchdog` is the per-frame variant (`Kick`/`Done`) that
`app.App.Update` uses in debug mode.

## Automatic screenshots

`app` can capture screenshots on a schedule for visual verification:
`[screenshot] path`, `delay`, and `frequency` settings (see
`app.ScreenshotSettings`). Game-time advance is clamped so captures land on
exact game-time targets, which keeps screenshot sequences deterministic.

## Visual-regression testing

The `visualtest` package and its `visualtest/capture` companion give a
consuming game deterministic visual-regression testing: capture a frame
sequence, then diff it pixel-for-pixel against a committed golden set.

### Capturing a deterministic sequence

`capture.StepCapturer` advances a game-supplied simulation by a fixed game-time
step once per frame and saves a screenshot every N frames. The scheduling and
PNG-saving loop is generic; the game supplies the simulation advance. Wire its
`Draw` into the game's `Draw`, after the game has rendered the screen, and let
the `Advance` hook be the only thing that advances the simulation, so the
sequence is a pure function of the step count.

```go
capturer, err := capture.NewStepCapturer(capture.StepCaptureConfig{
	Advance:     func(step time.Duration) { world.Advance(step) },
	Step:        16 * time.Millisecond, // fixed game-time step per frame
	Every:       10,                    // screenshot every 10 frames
	Count:       12,                    // stop after 12 screenshots
	PathPattern: "captures/frame_%03d.png",
})
// in Draw, after the game has drawn to screen:
if err := capturer.Draw(screen); err != nil { /* handle */ }
// quit once capturer.Done() reports true
```

Captures land on frames 0, `Every`, `2*Every`, and so on; `Done` reports when
`Count` screenshots have been taken. A `Count` of zero or less captures
indefinitely. `Save` defaults to `capture.SavePNG` and can be overridden. This
package depends on Ebitengine and needs a display for its tests (run under the
`task test:headless` target).

### Diffing against a golden set

`visualtest` is display-free, so the diff runs anywhere, including headless CI.
`visualtest.CompareImages`, `ComparePNGFiles`, and `CompareSequences` do a
bounds check then a pixel-for-pixel compare and report the first difference as
a `*Mismatch`: a size mismatch, or the coordinates and colors of the first
differing pixel. `PNGSequence` lists a directory's `.png` files sorted by name,
matching the zero-padded frame names the capturer produces.

The `cmd/visualdiff` command is a thin CLI over the library:

```sh
# two directories: compare the PNG sequences frame by frame
visualdiff testdata/golden captures

# two files: compare single images
visualdiff golden.png candidate.png
```

It prints the first difference and exits non-zero on any mismatch, so it drops
straight into a test or CI step.
