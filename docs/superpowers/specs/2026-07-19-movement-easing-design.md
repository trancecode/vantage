# Movement easing design

## Purpose

Let a move accelerate and decelerate instead of running at constant speed, so
that a body reads as stepping tile by tile rather than sliding. The eased
position is the real simulated position, not a render-time remap: rendered and
simulated positions stay identical, and the feature therefore has to be
deterministic, savegame-safe and correct under any tick slicing.

The request originates from `lockstep` (its `docs/design-questions.md`,
"Movement easing", 2026-07-18), which needs per-verb curves: steps settle
symmetrically, dashes and charges launch hard and decelerate into arrival. The
design below deliberately generalizes past that consumer, because `vantage` is
a reusable engine and easing is not universally desirable (see
"Non-goals and documented traps").

## Vocabulary

An *ease* (also called an easing curve) is a function on normalized progress,
`f: [0,1] -> [0,1]`, with `f(0) = 0` and `f(1) = 1`. It answers "given how far
through the move I am in time, how far through the move am I in distance". The
fixed endpoints are what make it purely cosmetic with respect to scheduling: a
curve cannot change when a move starts or finishes, only its shape in between.

A *lerp* (linear interpolation) is the separate blending step,
`source + t * (destination - source)`. Easing shapes `t`; the lerp consumes it.
The two concerns stay in different packages for that reason.

Standard names: *ease in* shapes the start (slow off the mark), *ease out*
shapes the end (decelerating arrival), *ease in-out* does both, and *linear* is
no easing at all.

## Architecture

Three packages, split by concern:

* `easing` (new, top level, no dependencies) owns the curves as a serializable
  enum with behavior.
* `geometry` gains `Vector2.Lerp`, the blend, which belongs with the vector
  type rather than with the curves.
* `motion` composes them: `MoveOptions` carries the chosen curve into the
  `Movement` component, and the tick routes eased moves through a parametric
  formula while leaving the existing incremental formula untouched.

Curves live outside `motion` because they are not movement specific. Camera
pans, fades, dialog slides and sprite timing all want the same four functions,
and none of those should import `motion` to get them. The repository currently
has no easing, lerp or tween code at all, so this sets the convention.

### The `easing` package

```go
// Curve shapes normalized progress over a transition.
type Curve int

const (
	// CurveLinear is constant rate: no easing. It is the zero value, so an
	// unset Curve behaves exactly as the engine did before easing existed.
	CurveLinear Curve = iota

	// CurveIn starts slow and reaches full rate at the end (t^2): wind-up.
	CurveIn

	// CurveOut starts at full rate and decelerates into arrival
	// (1-(1-t)^2): explosive launch.
	CurveOut

	// CurveInOut starts slow, peaks in the middle and settles gently
	// (3t^2-2t^3, smoothstep): symmetric.
	CurveInOut
)

// Apply maps progress t in [0,1] to eased progress in [0,1]. Apply(0) is 0
// and Apply(1) is 1 for every curve. Values outside [0,1] are clamped.
func (c Curve) Apply(t float64) float64
```

`CurveLinear` is named for what it does rather than following the style guide's
default `None` convention, under the guide's explicit "meaningful zero value"
exception (the one that allows `render.AlignLeft`): an unset curve is a
deliberate, safe default, not an uninitialized field.

`Curve` is an integer enum rather than an interface or a function value because
its value is written into savegames. An interface field would require every
consumer to register concrete types with `gob`, would put a nil-able,
pointer-shaped value inside a component, and would record a type name in the
save where an integer suffices. A function value cannot be serialized at all.
The enum still dispatches polymorphically at call sites through `Apply`, so
nothing is lost.

Keeping `Apply` scalar-to-scalar (rather than defining each curve as its own
lerp implementation) means one `CurveInOut` also serves camera zoom, alpha
fades and color transitions, none of which blend a `Vector2`.

**Enum values are savegame ABI.** Once shipped, `CurveInOut = 3` is in players'
saves forever. New curves append; existing values are never renumbered or
removed. This is stated in a comment in the source.

### `geometry.Vector2.Lerp`

```go
// Lerp returns the point at weight t between p and other: p when t is 0,
// other when t is 1. Values outside [0,1] extrapolate.
func (p Vector2) Lerp(other Vector2, t float64) Vector2
```

Deliberately unclamped, since it is a general blend. Callers that must stay on
the segment clamp their own `t`; `motion` is safe because `Curve.Apply` clamps.

### `motion.MoveOptions`

`MoveOptions` replaces the bare `speed` parameter on the three move entry
points:

```go
// MoveOptions describes how a body moves: how fast overall, and how that
// speed is distributed over the move.
type MoveOptions struct {
	// Speed is the average movement speed in tiles per second. It must be
	// positive.
	Speed float64

	// Ease shapes the speed over the move's duration. The zero value is
	// constant speed. The curve spans the move as issued, so a game wanting
	// per-step easing issues per-step moves.
	Ease easing.Curve
}

func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart
func (s *System) MoveEntityTowards(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart
func (s *System) MoveEntityTowardsArea(id ecs.EntityId, center geometry.Vector2, radius float64, opts MoveOptions) MoveStart
```

A struct rather than a fourth parameter or variadic options, for three
reasons. Further knobs are foreseeable (suppressing the facing update for
knockback and slides, opting out of destination reservation for pass-through
bodies, an ease period for long moves) and land as fields without touching
three signatures. A game's movement feels become named, reusable data instead
of literals at call sites, which is exactly the per-verb table the consumer
would otherwise hand-wire:

```go
var (
	moveStep   = motion.MoveOptions{Speed: motion.SpeedWalk, Ease: easing.CurveInOut}
	moveDash   = motion.MoveOptions{Speed: motion.SpeedRun, Ease: easing.CurveOut}
	moveCharge = motion.MoveOptions{Speed: chargeSpeed, Ease: easing.CurveOut}
)
```

And `motion.MoveOptions{Speed: motion.SpeedWalk}` remains a complete, correct
value for a game that never eases anything. Variadic options were rejected
because they suit orthogonal, rarely-set knobs, whereas every field here
describes how the body moves, as `Speed` already did.

Source compatibility is not a constraint: the two consuming games are both
under our control, and `nrg` is pinned to an older `vantage`.

`MoveEntity` panics when `Speed <= 0`. Today the doc comment merely warns that
the returned `Duration` is meaningless; with easing a non-positive speed also
yields a non-positive total duration, which would silently teleport the body.
A non-positive speed is a programming error, and the style guide favors panics
for those.

### The `Movement` component

```go
type Movement struct {
	// Destination is the target position the entity is moving towards.
	Destination geometry.Vector2

	// Speed is the movement speed in tiles per second. On an eased move it
	// is the average speed, not the instantaneous one: the total duration
	// is still Distance/Speed.
	Speed float64

	// Ease shapes progress along the move. The zero value, CurveLinear,
	// selects the incremental constant-speed path.
	Ease easing.Curve

	// Start is where the move began. Eased positions are computed from it,
	// so it is re-anchored whenever a move is (re)started.
	Start geometry.Vector2

	// Elapsed is the game time spent on this move so far.
	Elapsed time.Duration

	// Total is the game time the move takes end to end (Distance/Speed at
	// the moment it started).
	Total time.Duration
}

// Progress reports how far through its duration the move is, in [0,1]. It
// returns 0 for a move with no recorded Total (a move decoded from a save
// written before easing existed).
func (m Movement) Progress() float64
```

All fields are plain, `gob`-friendly data: `geometry.Vector2` already
implements `MarshalBinary`/`UnmarshalBinary`, `time.Duration` is an `int64`,
and `easing.Curve` is an `int`. No pointers, no interfaces, nothing requiring
registration. A move saved mid-flight restores exactly, because everything
needed to recompute its position lives in the component and nothing is derived
from a clock.

Backward compatibility is by construction: a `Movement` decoded from an older
save has `Ease: CurveLinear`, `Total: 0` and a zero `Start`, which routes to
the incremental path that never reads those fields.

`Start`, `Elapsed` and `Total` are populated for linear moves too, even though
the linear path does not derive position from them. They cost nothing, and they
give consumers a uniform `Progress()` for driving walk cycles and footstep
audio, which matters precisely because `Speed` is an average on eased moves.
`Progress()` on a linear move is informational: arrival there is still governed
by the incremental overshoot check, so it can read slightly under 1 at the
moment the move completes. Documented on the method.

### Advancing a move

The linear path keeps the existing `ProcessMovement` byte for byte. It is not
reformulated through the parametric path, so existing users see no float drift
under any tick pattern. A new exported entry point routes between the two and
returns the updated component by value, which keeps it analytically testable
by consumers:

```go
// ProcessMove advances mc by duration from currentPosition. It returns the
// movement with its Elapsed advanced, the new position, and whether the move
// completed.
func ProcessMove(mc Movement, currentPosition geometry.Vector2, duration time.Duration) (updated Movement, newPosition geometry.Vector2, completed bool)
```

Order of checks on the eased path, and it matters:

1. `duration <= 0` returns the current position, not completed, and does not
   advance `Elapsed`. The zero-duration-tick rule (commit `8012b6b`,
   v0.1.13) therefore holds on both paths, and it is checked before anything
   else so that even a degenerate `Total <= 0` move cannot complete on a
   zero-duration tick.
2. `Total <= 0` snaps to the destination and completes. `MoveEntity` cannot
   produce such a move, but a game writing the component directly can.
3. `Elapsed + duration >= Total` sets `Elapsed` to `Total`, the position to
   exactly `Destination`, and completes, so arrival is exact rather than
   tolerance-based.
4. Otherwise `Elapsed += duration` and the position is
   `Start.Lerp(Destination, Ease.Apply(Elapsed/Total))`.

Because step 3 triggers exactly when accumulated elapsed reaches `Total`, an
eased move completes at the same tick as the linear move of the same distance
and speed, whatever the slicing. Mid-move positions are a pure function of
`(Start, Destination, Elapsed/Total)`, independent of how the elapsed time was
sliced.

`System.Tick` keeps its current shape: it calls `ProcessMove`, writes the
returned movement back through the component handle, and syncs the spatial
grid every tick on both paths. Completion handling, `Movement` removal and
`OnArrival` are unchanged. Cancelling a move by removing the component
therefore leaves the body at a valid position with no cleanup owed.

### Mid-flight redirect

`MoveEntity` on an entity that already has a `Movement` re-anchors it
unconditionally: `Start` becomes the current position, `Elapsed` becomes 0, and
`Total` becomes the remaining distance divided by the new speed. Every
parametric field is rewritten, not just `Destination` and `Speed`, because a
stale `Start` or `Total` would make the body jump or arrive at the wrong time.

Carrying the anchor across redirects to preserve momentum was rejected: it
makes mid-move position depend on redirect history rather than on
`(Start, Destination, Elapsed/Total)`, which breaks the determinism contract,
the pinned-position tests, and easy reasoning about savegame round trips.

## Non-goals and documented traps

* **Easing is for committed, point-to-point moves.** Because a redirect
  re-anchors the curve, a game that retargets a pursuer every tick restarts the
  curve every tick and, under a symmetric curve, stays permanently in its slow
  opening and crawls. Continuous steering and pursuit should use
  `CurveLinear`. The engine cannot detect the difference, so `MoveOptions.Ease`
  documents it.
* **The curve spans the whole move, not each tile.** A single 20 tile
  `MoveEntity` eases once across all 20 tiles, which reads as a vehicle rather
  than a walker. Games wanting per-step easing issue per-step moves, which
  `MoveEntityTowards` already does through `MaxMoveActionDistance`. An
  `EasePeriod` field could add per-tile easing later, but a symmetric curve
  would drop velocity to zero at every tile boundary, so a long run would
  stutter rather than stride. Left out until a game asks.
* **No game-supplied curve functions.** They cannot be serialized, so a game
  using one would forfeit savegame support for in-flight moves. None of the
  four standard curves suggests the need.
* **No animation coupling.** The engine exposes `Progress()` and documents that
  `Speed` is an average on eased moves; syncing a walk cycle stays the game's
  job.
* **No new knobs beyond `Ease`.** Suppressing the facing update and opting out
  of destination reservation are foreseeable but unrequested; `MoveOptions`
  exists so they cost nothing later.

## Testing

`easing`:

* `Apply(0) == 0` and `Apply(1) == 1` for every curve, including `CurveLinear`.
* Pinned values: `CurveInOut` at t = 0.25, 0.5 and 0.75 gives 0.15625, exactly
  0.5 (symmetry) and 0.84375; `CurveOut` reaches half distance at
  t = 1 - sqrt(0.5), about 0.2929.
* Monotonicity across a sampled range, and clamping outside [0,1].

`geometry`:

* `Lerp` at t = 0, 0.5 and 1, plus extrapolation outside [0,1].

`motion`:

* Existing tests stay green unmodified apart from the `MoveOptions` call-site
  change, which is the linear path's regression guard.
* An eased move of distance d at speed s completes on exactly the same tick as
  the linear equivalent, both under uniform ticks and under ragged slicing, and
  its final position equals the destination exactly.
* Mid-move positions match the analytic `Start + f(t) * (Destination - Start)`
  for both eased curves, and are identical under two different tick slicings of
  the same total elapsed time.
* A zero-duration tick moves nothing, completes nothing and leaves `Elapsed`
  unchanged, on both paths.
* A mid-flight redirect re-anchors: the body arrives exactly
  `remaining/speed` after the redirect, with no positional jump on the
  redirecting tick.
* Removing the `Movement` mid-flight leaves the body at its last computed
  position.
* The spatial grid is updated every tick on both paths.
* A `Movement` in mid-eased-flight survives a `gob` round trip and continues to
  the same arrival tick and position, and a `Movement` encoded without the new
  fields decodes to a working linear move.

## Documentation and release

* `easing/doc.go` for the new package; `motion/doc.go` updated for
  `MoveOptions` and the two paths.
* `ARCHITECTURE.md`: add `easing` to the package map (no vantage
  dependencies) and record that `motion` depends on it.
* Gate: `task lint`, `task test:headless`, `go vet`, with
  `GOMODCACHE=/tmp/go-mod-cache`.
* Tag `v0.1.14` following the `v0.1.13` flow, and report the version so the
  consumer can bump.
