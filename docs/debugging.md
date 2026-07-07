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
for live inspection. It returns immediately when `debugMode` is false.

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
