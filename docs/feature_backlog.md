# Feature backlog

Engine features that are planned but not yet implemented. Each entry should be
turned into its own design and implementation plan when picked up.

## Video recording

**Status:** not implemented (deferred).

The engine `App` owns screenshot capture (single shot and PNG frame sequences).
Video recording should live in the same engine capture subsystem, not in
individual games, so every game gets it for free.

**Open decision: output format.**

* Standard-library animated GIF (`image/gif`). Zero external dependency, fully
  self-contained, works headless and in continuous integration. Downsides:
  256-color palette and large files, so it is mediocre for real gameplay
  footage.
* ffmpeg-backed mp4 or webm. Real video quality, ideal for sharing and pull
  request videos. Downside: requires the ffmpeg binary present at runtime (an
  external dependency, needed only while recording).

**Notes.**

* nrg already produces PNG frame sequences via the screenshot path with a
  `%d` verb, which an external tool can assemble into video. In-engine video
  recording would remove that external step.
* Build this onto the `App` capture path (see `app/app_screenshot.go`). The
  capture timing logic (delay, frequency, simulated-time accumulation) is
  already there and can drive frame collection for a recorder.
* When implemented, document the command-line flags and configuration in the
  debugging documentation, and decide how it interacts with `ExitAfter`
  (a recording in progress should be finalized before the app exits).
