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

* The screenshot path already produces PNG frame sequences with a
  `%d` verb, which an external tool can assemble into video. In-engine video
  recording would remove that external step.
* Build this onto the `App` capture path (see `app/app_screenshot.go`). The
  capture timing logic (delay, frequency, simulated-time accumulation) is
  already there and can drive frame collection for a recorder.
* When implemented, document the command-line flags and configuration in the
  debugging documentation, and decide how it interacts with `ExitAfter`
  (a recording in progress should be finalized before the app exits).

## Candidates surveyed from consuming games (2026-07-07)

Generic mechanisms observed in nrg and lockstep that are engine-shaped but
need a design pass before promotion. Higher-value candidates already have
issues (world-anchored overlays, depth-sorted draw list, visual-regression
harness).

* **Observer/event-listener registry** (lockstep `core/core_events.go`):
  registration-ordered listener lists with reflective fan-out of a listener to
  every interface it implements. The dispatch mechanism is game-agnostic; the
  event payload types are not. Needs a decision on how far to generalize.
* **Deterministic spatial placement** (lockstep `core/core_scenario_spawn.go`):
  breadth-first search for N reachable, unclaimed tiles within a radius of an
  anchor, closest-first with a deterministic tiebreak. Generic over
  TerrainProvider plus occupancy; the scenario schema stays in the game.
* **Expiring spatial markers** (lockstep `core/core_party_marker.go`, nrg
  `rts/rts_party_marker.go`): entity annotations with created/expires times,
  per-owner limits (evict oldest), and a maintenance tick pruning expired ones
  against the sim clock. The mechanism is engine-worthy; the marker semantics
  are game content. Needs a clean split.
* **Rule/behavior framework** (lockstep `core/core_ai_rule.go`, `_condition.go`,
  `_directive.go`): `When().Then()` builder, `And`/`Or`/`Not` combinators,
  first-successful-directive execution. Reusable but hard-bound to the game's
  `World`; promotion means parameterizing over a game context type with
  generics. Design-heavy; easy to over-generalize.
* **Per-frame timing distribution** (nrg `rts/rts_metrics.go`): min/max/avg
  over an N-frame circular buffer, a feature `util.Profiler` lacks. Fold in
  only if a game actually needs the distribution, not just totals.
