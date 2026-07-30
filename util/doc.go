// Package util provides shared infrastructure used across the codebase.
//
// It has no graphics dependency, so the simulation packages built on it link
// and run with no display; the on-screen debug overlay lives in render instead.
//
// Key exports:
//   - Logger: global zerolog.Logger instance, initialized via InitLogging
//   - DebugMode: flag toggling debug features at runtime
//   - Time: in-game time type wrapping time.Duration
//   - PriorityQueue: generic min-heap used by pathfinding and scheduling
//   - StartDebugHTTPServer: launches an HTTP server for runtime diagnostics
//   - Profiler: debug-only accumulator of named wall-time timings (hotspots)
//   - Rng: seedable deterministic random source with marshalable state, for
//     simulations whose random sequence must survive a savegame reload
package util
