# Performance optimization opportunities

This document tracks potential performance optimizations that were deliberately
not applied, to keep code clear and faithful to its origin, per the project's
performance guidance (optimize for clarity unless there is a measured need).

## Screenshot pixel conversion (app/app_screenshot.go)

`SaveScreenshot` converts the frame to an `*image.RGBA` with a per-pixel
`rgbaImg.Set(...)` loop. Ebiten's `Image.ReadPixels` already returns bytes in
RGBA order matching `image.RGBA.Pix`, so the loop could be replaced with a
single `copy(rgbaImg.Pix, pixels)`, which is both simpler and substantially
faster for large frames (the per-pixel path does ~2M bounds-checked calls at
1080p). Left as-is for now because capture is not on the hot path; revisit if
high-frequency frame-sequence capture becomes a bottleneck.
