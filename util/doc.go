// Package util provides shared infrastructure used across the codebase.
//
// Key exports:
//   - Logger: global zerolog.Logger instance, initialized via InitLogging
//   - DebugMode: flag toggling debug features at runtime
//   - Time: in-game time type wrapping time.Duration
//   - PriorityQueue: generic min-heap used by pathfinding and scheduling
//   - StartDebugHTTPServer: launches an HTTP server for runtime diagnostics
//   - ScreenLogger: on-screen log overlay for development
//   - Profiler: debug-only accumulator of named wall-time timings (hotspots)
package util
