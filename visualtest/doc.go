// Package visualtest provides a deterministic, pixel-exact PNG comparator for
// visual-regression testing that any consuming game can reuse.
//
// [CompareImages], [ComparePNGFiles], and [CompareSequences] do a bounds check
// followed by a pixel-for-pixel compare and report the first difference with a
// useful reason (a size mismatch, or the coordinates and colors of a differing
// pixel). They operate on the standard image types and have no display
// dependency, so the diff and the command in cmd/visualdiff run anywhere,
// including headless CI.
//
// The companion visualtest/capture package produces the frame sequences to
// diff: it advances a game-supplied simulation by a fixed game-time step and
// screenshots every N frames. That helper depends on Ebitengine and so is kept
// in a separate package to keep this one display-free.
package visualtest
