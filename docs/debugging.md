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
