# Movement easing implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a move accelerate and decelerate along an easing curve, as the real simulated position, opt-in per move, without changing arrival times or the existing constant-speed code path.

**Architecture:** A new dependency-free `easing` package owns the curves as a serializable `Curve` enum with an `Apply` method. `geometry.Vector2` gains `Lerp`, the blend. `motion` composes them: `MoveOptions` carries the curve into the `Movement` component, and `ProcessMove` routes each tick either to the untouched incremental `ProcessMovement` (linear) or to a parametric formula (eased).

**Tech Stack:** Go 1.26.4, standard library only, `github.com/trancecode/ecs/ecs`, tests with the standard `testing` package in the style already used in `motion/*_test.go` (plain `t.Errorf`, no testify).

Design spec: `docs/superpowers/specs/2026-07-19-movement-easing-design.md`.

## Global constraints

* Go version comes from `go.mod` (1.26.4). Do not add dependencies; standard library only.
* Environment for every check: `export GOMODCACHE=/tmp/go-mod-cache`.
* Gate commands: `task lint`, `task test:headless`, `go vet ./...`.
* Work directly on `main`, committing as you go. No feature branch, no PR.
* Commit author: `Claude Code <herve.quiroz+claude@gmail.com>`. No `Co-Authored-By` line.
* Style guide: `docs/styleguide.md`. Document every exported type, function and struct field. Enums get a meaningful zero value or a `None` value.
* The linear path (`ProcessMovement`) must keep its current arithmetic byte for byte. Never reformulate it through the parametric path.
* A zero-duration tick moves nothing and never completes a move, on both paths (the v0.1.13 rule, commit `8012b6b`).
* `Movement` stays plain gob-friendly data: no pointers, no interfaces, no function values.
* `easing.Curve` numeric values are savegame ABI: append only, never renumber.

---

### Task 1: The `easing` package

**Files:**
- Create: `easing/easing.go`
- Create: `easing/doc.go`
- Test: `easing/easing_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `easing.Curve` (an `int` enum) with constants `easing.CurveLinear`, `easing.CurveIn`, `easing.CurveOut`, `easing.CurveInOut`, and method `func (c Curve) Apply(t float64) float64`.

- [ ] **Step 1: Write the failing test**

Create `easing/easing_test.go`:

```go
package easing

import (
	"math"
	"testing"
)

var allCurves = []Curve{CurveLinear, CurveIn, CurveOut, CurveInOut}

func TestCurveApply_Endpoints(t *testing.T) {
	for _, c := range allCurves {
		if got := c.Apply(0); got != 0 {
			t.Errorf("curve %d: Apply(0) = %v, want 0", c, got)
		}
		if got := c.Apply(1); got != 1 {
			t.Errorf("curve %d: Apply(1) = %v, want 1", c, got)
		}
	}
}

func TestCurveApply_LinearIsIdentity(t *testing.T) {
	for _, t0 := range []float64{0.1, 0.25, 0.5, 0.75, 0.9} {
		if got := CurveLinear.Apply(t0); got != t0 {
			t.Errorf("CurveLinear.Apply(%v) = %v, want %v", t0, got, t0)
		}
	}
}

// The pinned smoothstep values the consuming game's tests rely on, including
// the exact symmetry at the midpoint.
func TestCurveApply_InOutPinnedValues(t *testing.T) {
	cases := map[float64]float64{
		0.25: 0.15625,
		0.5:  0.5,
		0.75: 0.84375,
	}
	for t0, want := range cases {
		if got := CurveInOut.Apply(t0); math.Abs(got-want) > 1e-12 {
			t.Errorf("CurveInOut.Apply(%v) = %v, want %v", t0, got, want)
		}
	}
	if got := CurveInOut.Apply(0.5); got != 0.5 {
		t.Errorf("CurveInOut.Apply(0.5) = %v, want exactly 0.5", got)
	}
}

// CurveOut covers half the distance earlier than the midpoint of the
// duration: at t = 1 - sqrt(0.5).
func TestCurveApply_OutReachesHalfEarly(t *testing.T) {
	tHalf := 1 - math.Sqrt(0.5)
	if got := CurveOut.Apply(tHalf); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("CurveOut.Apply(%v) = %v, want 0.5", tHalf, got)
	}
	if got := CurveOut.Apply(0.5); got <= 0.5 {
		t.Errorf("CurveOut.Apply(0.5) = %v, want > 0.5 (front-loaded)", got)
	}
}

func TestCurveApply_InReachesHalfLate(t *testing.T) {
	tHalf := math.Sqrt(0.5)
	if got := CurveIn.Apply(tHalf); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("CurveIn.Apply(%v) = %v, want 0.5", tHalf, got)
	}
	if got := CurveIn.Apply(0.5); got >= 0.5 {
		t.Errorf("CurveIn.Apply(0.5) = %v, want < 0.5 (back-loaded)", got)
	}
}

func TestCurveApply_Monotonic(t *testing.T) {
	for _, c := range allCurves {
		previous := c.Apply(0)
		for i := 1; i <= 100; i++ {
			got := c.Apply(float64(i) / 100)
			if got < previous {
				t.Fatalf("curve %d: Apply is not monotonic at t=%v (%v after %v)", c, float64(i)/100, got, previous)
			}
			previous = got
		}
	}
}

func TestCurveApply_ClampsOutOfRange(t *testing.T) {
	for _, c := range allCurves {
		if got := c.Apply(-0.5); got != 0 {
			t.Errorf("curve %d: Apply(-0.5) = %v, want 0", c, got)
		}
		if got := c.Apply(1.5); got != 1 {
			t.Errorf("curve %d: Apply(1.5) = %v, want 1", c, got)
		}
	}
}

func TestCurveApply_UnknownCurveIsLinear(t *testing.T) {
	unknown := Curve(99)
	if got := unknown.Apply(0.3); got != 0.3 {
		t.Errorf("unknown curve: Apply(0.3) = %v, want 0.3", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./easing/
```

Expected: FAIL, build error along the lines of `undefined: Curve` (or "no Go files" if only the test file exists).

- [ ] **Step 3: Write the implementation**

Create `easing/easing.go`:

```go
package easing

// Curve shapes normalized progress over a transition: given how far through a
// transition something is in time, it reports how far through the transition
// it is in value. Every curve satisfies Apply(0) == 0 and Apply(1) == 1, so a
// curve never changes when a transition starts or finishes, only its shape in
// between.
//
// Curve values are persisted by consumers (a Curve rides an in-flight move in
// a savegame), so the numeric values are part of the on-disk format: new
// curves are appended and existing ones are never renumbered or removed.
type Curve int

const (
	// CurveLinear is constant rate: no easing at all. It is the zero value,
	// so an unset Curve behaves the way the engine did before easing
	// existed.
	CurveLinear Curve = iota

	// CurveIn eases the start (t^2): slow off the mark, full rate on
	// arrival. A wind-up.
	CurveIn

	// CurveOut eases the end (1-(1-t)^2): full rate off the mark,
	// decelerating into arrival. An explosive launch.
	CurveOut

	// CurveInOut eases both ends (3t^2-2t^3, smoothstep): slow off the
	// mark, fastest at the midpoint, gentle settle. Symmetric, so it
	// crosses the halfway value at exactly half the duration.
	CurveInOut
)

// Apply maps progress t to eased progress, both in [0,1]. Values of t outside
// [0,1] are clamped. An unrecognized Curve is treated as CurveLinear.
func (c Curve) Apply(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}

	switch c {
	case CurveIn:
		return t * t
	case CurveOut:
		return 1 - (1-t)*(1-t)
	case CurveInOut:
		return t * t * (3 - 2*t)
	}
	return t
}
```

Create `easing/doc.go`:

```go
// Package easing provides easing curves: functions that shape how a value
// progresses from its start to its end over a fixed span of time.
//
// Curve is an enumeration of the standard shapes (linear, ease in, ease out,
// ease in-out) with a single Apply method mapping normalized progress in
// [0,1] to eased progress in [0,1]. Because Apply is scalar to scalar, the
// same curve serves any interpolated value: a position, a camera zoom, an
// alpha fade.
//
// Curve is an integer enumeration rather than a function value so that it can
// be stored in components and persisted in savegames. Its zero value,
// CurveLinear, is constant rate, which makes an unset curve behave as no
// easing at all.
//
// Easing only shapes progress; it does not do the blending. Pair a curve with
// an interpolation such as geometry.Vector2.Lerp:
//
//	position := start.Lerp(destination, curve.Apply(elapsed/total))
package easing
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./easing/ && go vet ./easing/
```

Expected: `ok  github.com/trancecode/vantage/easing`, and `go vet` silent.

- [ ] **Step 5: Commit**

```bash
git add easing/
git commit -m "Add the easing package with the standard curves"
```

---

### Task 2: `geometry.Vector2.Lerp`

**Files:**
- Modify: `geometry/geometry_vector.go` (add after `Scale`, around line 55)
- Test: `geometry/geometry_vector_test.go` (append; the file already exists)

**Interfaces:**
- Consumes: nothing.
- Produces: `func (p Vector2) Lerp(other Vector2, t float64) Vector2`.

- [ ] **Step 1: Write the failing test**

Append to `geometry/geometry_vector_test.go`, matching the assertion style already used in that file:

```go
func TestVector2Lerp_Endpoints(t *testing.T) {
	a := NewVector2(1.0, 2.0)
	b := NewVector2(5.0, 10.0)

	if got := a.Lerp(b, 0); got != a {
		t.Errorf("Lerp(t=0) = %v, want %v", got, a)
	}
	if got := a.Lerp(b, 1); got != b {
		t.Errorf("Lerp(t=1) = %v, want %v", got, b)
	}
}

func TestVector2Lerp_Midpoint(t *testing.T) {
	a := NewVector2(0.0, 0.0)
	b := NewVector2(4.0, -2.0)

	want := NewVector2(2.0, -1.0)
	if got := a.Lerp(b, 0.5); got != want {
		t.Errorf("Lerp(t=0.5) = %v, want %v", got, want)
	}
}

// Lerp is a plain blend: it extrapolates rather than clamping, so callers that
// must stay on the segment clamp their own weight.
func TestVector2Lerp_Extrapolates(t *testing.T) {
	a := NewVector2(0.0, 0.0)
	b := NewVector2(2.0, 0.0)

	if got := a.Lerp(b, 2); got != NewVector2(4.0, 0.0) {
		t.Errorf("Lerp(t=2) = %v, want (4,0)", got)
	}
	if got := a.Lerp(b, -1); got != NewVector2(-2.0, 0.0) {
		t.Errorf("Lerp(t=-1) = %v, want (-2,0)", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./geometry/ -run Lerp
```

Expected: FAIL with `a.Lerp undefined (type Vector2 has no field or method Lerp)`.

- [ ] **Step 3: Write the implementation**

Add to `geometry/geometry_vector.go`, directly after the `Scale` method:

```go
// Lerp linearly interpolates between the current Vector2 and the given
// Vector2: it returns the current vector when t is 0 and the given vector when
// t is 1. It does not clamp t, so values outside [0,1] extrapolate along the
// line through both points. This method uses the form p*(1-t) + other*t to
// ensure the endpoints are exact in floating point arithmetic (at t=0 it
// returns p exactly, at t=1 it returns other exactly).
func (p Vector2) Lerp(other Vector2, t float64) Vector2 {
	return NewVector2(p.x*(1-t)+other.x*t, p.y*(1-t)+other.y*t)
}
```

Note the formula: `p*(1-t) + other*t`, not `p + (other-p)*t`. Only this form returns both endpoints exactly. Measured over 200,000 random coordinate pairs in the +/-100 tile range, `p + (other-p)*t` fails to return `other` exactly at t = 1 for about 31% of pairs (`p = -50000.3, other = 50000.9` gives 50000.90000000001), because the subtraction rounds before the addition can restore it. `1-t` is exactly 0 at t = 1, so the form above cannot drift. Also add a test pinning exactness at both endpoints for such awkward values, since small integer test vectors mask the defect.

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./geometry/ && go vet ./geometry/
```

Expected: `ok  github.com/trancecode/vantage/geometry`, `go vet` silent.

- [ ] **Step 5: Commit**

```bash
git add geometry/
git commit -m "Add Vector2.Lerp for interpolating between positions"
```

---

### Task 3: `Movement` easing state and `ProcessMove`

**Files:**
- Modify: `motion/motion.go` (the `Movement` struct at lines 15-22; add `Progress` and `ProcessMove`; leave `ProcessMovement` at lines 44-82 untouched)
- Test: `motion/motion_test.go` (append; existing `ProcessMovement` tests must stay unmodified and green)

**Interfaces:**
- Consumes: `easing.Curve` and `Curve.Apply` from Task 1; `geometry.Vector2.Lerp` from Task 2.
- Produces:
  - `Movement` with added fields `Ease easing.Curve`, `Start geometry.Vector2`, `Elapsed time.Duration`, `Total time.Duration`.
  - `func (m Movement) Progress() float64`.
  - `func ProcessMove(mc Movement, currentPosition geometry.Vector2, duration time.Duration) (updated Movement, newPosition geometry.Vector2, completed bool)`.

- [ ] **Step 1: Write the failing test**

Append to `motion/motion_test.go`:

```go
// easedMovement builds an eased Movement from start to dest at speed, with
// Total derived the way MoveEntity derives it.
func easedMovement(start, dest geometry.Vector2, speed float64, curve easing.Curve) Movement {
	distance := start.DistanceTo(dest)
	return Movement{
		Destination: dest,
		Speed:       speed,
		Ease:        curve,
		Start:       start,
		Total:       time.Duration(distance / speed * float64(time.Second)),
	}
}

func TestProcessMove_LinearMatchesProcessMovement(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(10.0, 0.0)
	mc := Movement{Destination: dest, Speed: 2.0, Start: start, Total: 5 * time.Second}

	wantPos, wantDone := ProcessMovement(start, dest, 2.0, 500*time.Millisecond)
	updated, gotPos, gotDone := ProcessMove(mc, start, 500*time.Millisecond)

	if gotPos != wantPos || gotDone != wantDone {
		t.Errorf("ProcessMove = (%v, %v), want (%v, %v)", gotPos, gotDone, wantPos, wantDone)
	}
	if updated.Elapsed != 500*time.Millisecond {
		t.Errorf("expected Elapsed 500ms on the linear path, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedMidMovePosition(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut) // Total 4s

	// One second in: t = 0.25, smoothstep(0.25) = 0.15625, so x = 0.625.
	updated, pos, done := ProcessMove(mc, start, time.Second)

	if done {
		t.Error("expected the move to still be in flight")
	}
	if math.Abs(pos.X()-0.625) > 1e-12 || pos.Y() != 0 {
		t.Errorf("expected position (0.625, 0), got %v", pos)
	}
	if updated.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s, got %v", updated.Elapsed)
	}
}

func TestProcessMove_EasedIsIndependentOfTickSlicing(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(4.0, 0.0)

	coarse := easedMovement(start, dest, 1.0, easing.CurveOut)
	_, coarsePos, _ := ProcessMove(coarse, start, 2*time.Second)

	fine := easedMovement(start, dest, 1.0, easing.CurveOut)
	finePos := start
	for range 20 {
		fine, finePos, _ = ProcessMove(fine, finePos, 100*time.Millisecond)
	}

	if coarsePos.DistanceTo(finePos) > 1e-12 {
		t.Errorf("eased position depends on slicing: %v (one tick) vs %v (20 ticks)", coarsePos, finePos)
	}
}

func TestProcessMove_EasedCompletesExactlyAtTotal(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(3.0, 4.0) // distance 5, speed 2 => Total 2.5s
	mc := easedMovement(start, dest, 2.0, easing.CurveInOut)

	pos := start
	ticks := 0
	done := false
	for !done {
		ticks++
		if ticks > 100 {
			t.Fatal("eased move did not complete within 100 ticks")
		}
		mc, pos, done = ProcessMove(mc, pos, 250*time.Millisecond)
	}

	if ticks != 10 {
		t.Errorf("expected completion on tick 10 (2.5s at 250ms), got tick %d", ticks)
	}
	if pos != dest {
		t.Errorf("expected exact arrival at %v, got %v", dest, pos)
	}
	if mc.Elapsed != mc.Total {
		t.Errorf("expected Elapsed to equal Total on completion, got %v of %v", mc.Elapsed, mc.Total)
	}
}

func TestProcessMove_EasedCompletesOnSameTickAsLinear(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(3.0, 0.0)
	const tick = 700 * time.Millisecond // ragged: does not divide the 3s total

	eased := easedMovement(start, dest, 1.0, easing.CurveOut)
	easedPos := start
	easedTicks := 0
	for done := false; !done; {
		easedTicks++
		eased, easedPos, done = ProcessMove(eased, easedPos, tick)
	}

	linearPos := start
	linearTicks := 0
	for done := false; !done; {
		linearTicks++
		linearPos, done = ProcessMovement(linearPos, dest, 1.0, tick)
	}

	if easedTicks != linearTicks {
		t.Errorf("eased move completed on tick %d, linear on tick %d", easedTicks, linearTicks)
	}
}

func TestProcessMove_EasedZeroDurationMakesNoProgress(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(2.0, 0.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut)

	updated, pos, done := ProcessMove(mc, start, 0)

	if done {
		t.Error("a zero-duration tick must not complete a move")
	}
	if pos != start {
		t.Errorf("a zero-duration tick must not move: got %v, want %v", pos, start)
	}
	if updated.Elapsed != 0 {
		t.Errorf("a zero-duration tick must not advance Elapsed, got %v", updated.Elapsed)
	}
}

// A degenerate eased move (no recorded Total) must still respect the
// zero-duration rule before it snaps to its destination.
func TestProcessMove_EasedZeroTotalNeedsAPositiveTick(t *testing.T) {
	start := geometry.NewVector2(0.0, 0.0)
	dest := geometry.NewVector2(2.0, 0.0)
	mc := Movement{Destination: dest, Speed: 1.0, Ease: easing.CurveInOut, Start: start}

	if _, pos, done := ProcessMove(mc, start, 0); done || pos != start {
		t.Errorf("zero-duration tick on a zero-Total move: got (%v, %v), want (%v, false)", pos, done, start)
	}
	if _, pos, done := ProcessMove(mc, start, time.Millisecond); !done || pos != dest {
		t.Errorf("positive tick on a zero-Total move: got (%v, %v), want (%v, true)", pos, done, dest)
	}
}

func TestMovementProgress(t *testing.T) {
	mc := Movement{Total: 4 * time.Second, Elapsed: time.Second}
	if got := mc.Progress(); got != 0.25 {
		t.Errorf("Progress() = %v, want 0.25", got)
	}

	mc.Elapsed = 8 * time.Second
	if got := mc.Progress(); got != 1 {
		t.Errorf("Progress() past Total = %v, want 1", got)
	}

	// A Movement decoded from a save written before easing existed has no
	// recorded Total.
	legacy := Movement{Destination: geometry.NewVector2(1.0, 0.0), Speed: 1.0}
	if got := legacy.Progress(); got != 0 {
		t.Errorf("Progress() without Total = %v, want 0", got)
	}
}
```

Add `"math"` and `"github.com/trancecode/vantage/easing"` to the imports of `motion/motion_test.go`.

- [ ] **Step 2: Run the test to verify it fails**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ -run "ProcessMove|MovementProgress"
```

Expected: FAIL, build errors such as `undefined: ProcessMove` and `unknown field Ease in struct literal`.

- [ ] **Step 3: Write the implementation**

In `motion/motion.go`, replace the `Movement` struct with:

```go
// Movement holds an entity's in-progress move: where it is headed, how fast,
// and how its speed is shaped over the move.
//
// Every field is plain data so consumers can persist an in-flight move in a
// savegame. A Movement decoded without the easing fields is a constant-speed
// move, which is what the engine did before easing existed.
type Movement struct {
	// Destination is the target position the entity is moving towards.
	Destination geometry.Vector2

	// Speed is the movement speed in tiles per second. On an eased move it
	// is the average speed rather than the instantaneous one: the total
	// duration is still the distance divided by Speed.
	Speed float64

	// Ease shapes progress along the move. The zero value, easing.
	// CurveLinear, selects the incremental constant-speed path and ignores
	// the fields below.
	Ease easing.Curve

	// Start is the position the move began from. Eased positions are
	// computed from it, so starting or redirecting a move re-anchors it.
	Start geometry.Vector2

	// Elapsed is the game time spent on this move so far.
	Elapsed time.Duration

	// Total is the game time the move takes end to end, fixed when the move
	// starts as the distance divided by Speed.
	Total time.Duration
}

// Progress reports how far through its duration the move is, from 0 to 1.
// Games use it to drive animation, which cannot read the rate off Speed on an
// eased move. It returns 0 for a move with no recorded Total, such as one
// decoded from a save written before easing existed.
//
// On a constant-speed move Progress is informational: arrival there is decided
// by the incremental overshoot check, so Progress can read slightly under 1 on
// the tick the move completes.
func (m Movement) Progress() float64 {
	if m.Total <= 0 {
		return 0
	}
	if m.Elapsed >= m.Total {
		return 1
	}
	return float64(m.Elapsed) / float64(m.Total)
}
```

Add the `easing` import to `motion/motion.go`.

Then add `ProcessMove` below `ProcessMovement` (leave `ProcessMovement` exactly as it is):

```go
// ProcessMove advances a move by duration and returns the movement with its
// Elapsed advanced, the entity's new position, and whether the move completed.
// It routes constant-speed moves to ProcessMovement and eased moves to the
// parametric formula, where position is a pure function of the move's start,
// destination and progress, and therefore independent of how the elapsed game
// time was sliced into ticks.
//
// A zero or negative duration moves nothing and never completes a move, on
// either path.
func ProcessMove(mc Movement, currentPosition geometry.Vector2, duration time.Duration) (updated Movement, newPosition geometry.Vector2, completed bool) {
	if duration <= 0 {
		return mc, currentPosition, false
	}

	if mc.Ease == easing.CurveLinear {
		newPosition, completed = ProcessMovement(currentPosition, mc.Destination, mc.Speed, duration)
		mc.Elapsed += duration
		return mc, newPosition, completed
	}

	// A move with no duration to spread the curve over has nowhere to be
	// except its destination.
	if mc.Total <= 0 {
		mc.Elapsed = mc.Total
		return mc, mc.Destination, true
	}

	mc.Elapsed += duration
	if mc.Elapsed >= mc.Total {
		mc.Elapsed = mc.Total
		return mc, mc.Destination, true
	}

	eased := mc.Ease.Apply(float64(mc.Elapsed) / float64(mc.Total))
	return mc, mc.Start.Lerp(mc.Destination, eased), false
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ -run "ProcessMove|ProcessMovement|MovementProgress" -v
```

Expected: PASS for the new tests and for every pre-existing `TestProcessMovement_*` test, unmodified.

- [ ] **Step 5: Commit**

```bash
git add motion/motion.go motion/motion_test.go
git commit -m "Add eased movement state and the ProcessMove router"
```

---

### Task 4: Route `System.Tick` through `ProcessMove`

**Files:**
- Modify: `motion/motion_system.go` (the `Tick` loop, lines 63-98)
- Test: `motion/motion_system_test.go` (append; keep the existing helpers and tests)

**Interfaces:**
- Consumes: `ProcessMove` and the `Movement` easing fields from Task 3.
- Produces: no new exported API. `System.Tick` now advances both paths and writes `Elapsed` back to the component.

- [ ] **Step 1: Write the failing test**

Append to `motion/motion_system_test.go`:

```go
// addEasedEntity creates an entity at pos on an eased move toward dest, with
// Total derived the way MoveEntity derives it.
func addEasedEntity(s *System, w *ecs.World, pos, dest geometry.Vector2, speed float64, curve easing.Curve) ecs.EntityId {
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: pos})
	s.Movements.Add(id, Movement{
		Destination: dest,
		Speed:       speed,
		Ease:        curve,
		Start:       pos,
		Total:       time.Duration(pos.DistanceTo(dest) / speed * float64(time.Second)),
	})
	if s.Grid != nil {
		s.Grid.AddEntity(id, pos)
	}
	return id
}

func TestSystemTick_AdvancesEasedPosition(t *testing.T) {
	s, w := newTestSystem()
	// Total 4s; after 1s, smoothstep(0.25) = 0.15625 of 4 tiles = 0.625.
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(4.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(time.Second)

	sc, _ := s.Spatials.Get(id)
	if sc.Position.DistanceTo(geometry.NewVector2(0.625, 0.0)) > 1e-9 {
		t.Errorf("expected eased position near (0.625, 0), got %v", sc.Position)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("expected the Movement to remain in flight")
	}
	if mc.Elapsed != time.Second {
		t.Errorf("expected Elapsed 1s written back to the component, got %v", mc.Elapsed)
	}
}

func TestSystemTick_CompletesEasedMovementAtExactDestination(t *testing.T) {
	s, w := newTestSystem()
	var arrivals []MovementResult
	s.OnArrival = func(r MovementResult) { arrivals = append(arrivals, r) }
	dest := geometry.NewVector2(2.0, 0.0)
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), dest, 1.0, easing.CurveOut)

	// Total is 2s; four 500ms ticks.
	for range 4 {
		s.Tick(500 * time.Millisecond)
	}

	sc, _ := s.Spatials.Get(id)
	if sc.Position != dest {
		t.Errorf("expected exact arrival at %v, got %v", dest, sc.Position)
	}
	if s.Movements.Has(id) {
		t.Error("expected the Movement to be removed on arrival")
	}
	if len(arrivals) != 1 || arrivals[0].EntityId != id {
		t.Errorf("expected one arrival for entity %v, got %+v", id, arrivals)
	}
}

func TestSystemTick_ZeroDurationDoesNotAdvanceEasedMove(t *testing.T) {
	s, w := newTestSystem()
	pos := geometry.NewVector2(0.0, 0.0)
	id := addEasedEntity(s, w, pos, geometry.NewVector2(2.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(0)

	sc, _ := s.Spatials.Get(id)
	if sc.Position != pos {
		t.Errorf("expected no movement on a zero-duration tick, got %v", sc.Position)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("a zero-duration tick must not complete a move")
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed to stay 0, got %v", mc.Elapsed)
	}
}

func TestSystemTick_UpdatesGridOnEasedPath(t *testing.T) {
	s, w := newTestSystem()
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(5.0, 0.0), 1.0, easing.CurveOut)

	s.Tick(time.Second)

	sc, _ := s.Spatials.Get(id)
	around := geometry.NewRectangleFromPoints(sc.Position.X()-0.1, sc.Position.Y()-0.1, sc.Position.X()+0.1, sc.Position.Y()+0.1)
	found := false
	for _, e := range s.Grid.GetRange(around) {
		if e == id {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the grid to track the eased entity at %v", sc.Position)
	}
}

// Consumers cancel a move by removing the component; the body must be left at
// a valid position with nothing owed.
func TestSystemTick_CancelledEasedMoveLeavesPosition(t *testing.T) {
	s, w := newTestSystem()
	id := addEasedEntity(s, w, geometry.NewVector2(0.0, 0.0), geometry.NewVector2(4.0, 0.0), 1.0, easing.CurveInOut)

	s.Tick(time.Second)
	sc, _ := s.Spatials.Get(id)
	cancelled := sc.Position
	s.Movements.Remove(id)
	s.Tick(time.Second)

	sc, _ = s.Spatials.Get(id)
	if sc.Position != cancelled {
		t.Errorf("expected the body to stay at %v after cancelling, got %v", cancelled, sc.Position)
	}
}
```

Add `"github.com/trancecode/vantage/easing"` to the imports of `motion/motion_system_test.go`.

- [ ] **Step 2: Run the test to verify it fails**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ -run "TestSystemTick" -v
```

Expected: FAIL. `TestSystemTick_AdvancesEasedPosition` reports a position near 1.0 instead of 0.625 (the tick still runs the linear formula) and an `Elapsed` of 0.

- [ ] **Step 3: Write the implementation**

In `motion/motion_system.go`, replace the body of the loop in `Tick` (the `original`/`ProcessMovement`/`sc.Position` block at lines 74-76) with:

```go
		original := sc.Position
		updated, newPosition, done := ProcessMove(*mc, sc.Position, elapsed)
		*mc = updated
		sc.Position = newPosition
```

The rest of the loop, the grid sync and the completion handling stay as they are.

Update the `Tick` doc comment to:

```go
// Tick moves every entity that has a Movement by elapsed game time, advancing
// constant-speed and eased moves alike and recording the time spent on each
// move. Entities that reach their destination have their Movement removed and
// are reported through OnArrival. Entities with a Movement but no Spatial are
// skipped.
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ && go vet ./motion/
```

Expected: `ok  github.com/trancecode/vantage/motion` with every pre-existing test still passing.

- [ ] **Step 5: Commit**

```bash
git add motion/motion_system.go motion/motion_system_test.go
git commit -m "Advance eased moves in the motion tick"
```

---

### Task 5: `MoveOptions` on the move entry points

**Files:**
- Modify: `motion/motion_move.go` (add `MoveOptions`; change `MoveEntity`, lines 68-107)
- Modify: `motion/motion_towards.go` (`MoveEntityTowards` line 28, `MoveEntityTowardsArea` line 99, and the three internal `MoveEntity`/`MoveEntityTowards` calls at lines 52, 84, 161)
- Modify: `motion/motion_move_test.go`, `motion/motion_towards_test.go` (call-site updates plus new tests)

**Interfaces:**
- Consumes: `easing.Curve` (Task 1), the `Movement` easing fields (Task 3).
- Produces:
  - `type MoveOptions struct { Speed float64; Ease easing.Curve }`
  - `func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart`
  - `func (s *System) MoveEntityTowards(entityId ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart`
  - `func (s *System) MoveEntityTowardsArea(entityId ecs.EntityId, center geometry.Vector2, radius float64, opts MoveOptions) MoveStart`

- [ ] **Step 1: Write the failing test**

First update every existing call site in the two test files so the package compiles: replace the trailing speed argument with `MoveOptions{Speed: ...}`. For example, in `motion/motion_move_test.go`:

* line 16: `s.MoveEntity(id, geometry.NewVector2(3.0, 4.0), MoveOptions{Speed: 2.0})`
* line 47: `s.MoveEntity(id, dest, MoveOptions{Speed: 1.0})`
* line 67: `s.MoveEntity(id, tilemap.TileToWorldPosition(destTile), MoveOptions{Speed: 1.0})`
* line 88: `s.MoveEntity(id, pos, MoveOptions{Speed: 1.0})`
* line 109: `s.MoveEntity(w.NewEntity(), geometry.NewVector2(1.0, 0.0), MoveOptions{Speed: 1.0})`

and in `motion/motion_towards_test.go`:

* lines 37, 63, 84: `s.MoveEntityTowards(e.id, target, MoveOptions{Speed: 1.0})`
* line 102: `s.MoveEntityTowards(e.id, tilemap.TileToWorldPosition(tilemap.TileCoord{X: 3, Y: 0}), MoveOptions{Speed: 1.0})`
* line 109: `s.MoveEntityTowardsArea(e.id, center, 2.0, MoveOptions{Speed: 1.0})`
* line 120: `s.MoveEntityTowardsArea(e.id, center, 1.0, MoveOptions{Speed: 1.0})`

Then append to `motion/motion_move_test.go`:

```go
func TestMoveEntity_RecordsEasingState(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	origin := geometry.NewVector2(1.0, 1.0)
	s.Spatials.Add(id, Spatial{Position: origin})

	start := s.MoveEntity(id, geometry.NewVector2(4.0, 5.0), MoveOptions{Speed: 2.0, Ease: easing.CurveOut})

	if !start.Started() {
		t.Fatalf("expected move to start, got %+v", start)
	}
	mc, ok := s.Movements.Get(id)
	if !ok {
		t.Fatal("expected a Movement component")
	}
	if mc.Ease != easing.CurveOut {
		t.Errorf("expected Ease CurveOut, got %v", mc.Ease)
	}
	if mc.Start != origin {
		t.Errorf("expected Start %v, got %v", origin, mc.Start)
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed 0, got %v", mc.Elapsed)
	}
	if mc.Total != start.Duration {
		t.Errorf("expected Total to match the reported duration %v, got %v", start.Duration, mc.Total)
	}
}

// A constant-speed move records the same bookkeeping, so Progress works
// uniformly, while position still comes from the incremental path.
func TestMoveEntity_RecordsStateForLinearMoves(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	origin := geometry.NewVector2(0.0, 0.0)
	s.Spatials.Add(id, Spatial{Position: origin})

	s.MoveEntity(id, geometry.NewVector2(4.0, 0.0), MoveOptions{Speed: 1.0})

	mc, _ := s.Movements.Get(id)
	if mc.Ease != easing.CurveLinear {
		t.Errorf("expected the zero curve, got %v", mc.Ease)
	}
	if mc.Start != origin || mc.Total != 4*time.Second {
		t.Errorf("expected Start %v and Total 4s, got %v and %v", origin, mc.Start, mc.Total)
	}
}

func TestMoveEntity_RedirectReAnchors(t *testing.T) {
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	s.MoveEntity(id, geometry.NewVector2(10.0, 0.0), MoveOptions{Speed: 1.0, Ease: easing.CurveInOut})
	s.Tick(2 * time.Second)

	sc, _ := s.Spatials.Get(id)
	redirectFrom := sc.Position
	if redirectFrom.X() == 0 {
		t.Fatal("expected the entity to have moved before the redirect")
	}

	start := s.MoveEntity(id, geometry.NewVector2(0.0, 3.0), MoveOptions{Speed: 1.0, Ease: easing.CurveInOut})

	mc, _ := s.Movements.Get(id)
	if mc.Start != redirectFrom {
		t.Errorf("expected Start re-anchored to %v, got %v", redirectFrom, mc.Start)
	}
	if mc.Elapsed != 0 {
		t.Errorf("expected Elapsed reset to 0, got %v", mc.Elapsed)
	}
	if mc.Total != start.Duration {
		t.Errorf("expected Total %v recomputed from the remaining distance, got %v", start.Duration, mc.Total)
	}

	// No positional jump on the redirecting tick, and arrival exactly one
	// total later.
	s.Tick(0)
	sc, _ = s.Spatials.Get(id)
	if sc.Position != redirectFrom {
		t.Errorf("expected no jump on redirect, got %v want %v", sc.Position, redirectFrom)
	}
	s.Tick(start.Duration)
	sc, _ = s.Spatials.Get(id)
	if sc.Position != geometry.NewVector2(0.0, 3.0) {
		t.Errorf("expected arrival at (0,3), got %v", sc.Position)
	}
}

func TestMoveEntity_PanicsOnNonPositiveSpeed(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for a non-positive speed")
		}
	}()
	s, w := newTestSystem()
	id := w.NewEntity()
	s.Spatials.Add(id, Spatial{Position: geometry.NewVector2(0.0, 0.0)})

	s.MoveEntity(id, geometry.NewVector2(1.0, 0.0), MoveOptions{Speed: 0})
}
```

Append to `motion/motion_towards_test.go`:

```go
func TestMoveEntityTowards_CarriesOptionsThrough(t *testing.T) {
	s, e := newPathTestSystem(t, tilemap.TileCoord{X: 0, Y: 0})
	target := tilemap.TileToWorldPosition(tilemap.TileCoord{X: 3, Y: 0})

	start := s.MoveEntityTowards(e.id, target, MoveOptions{Speed: 1.0, Ease: easing.CurveInOut})

	if !start.Started() {
		t.Fatalf("expected a step to start, got %+v", start)
	}
	mc, ok := s.Movements.Get(e.id)
	if !ok {
		t.Fatal("expected a Movement component")
	}
	if mc.Ease != easing.CurveInOut {
		t.Errorf("expected the curve to reach the Movement, got %v", mc.Ease)
	}
}
```

`newPathTestSystem(t, tile)` is the existing helper in that file: it returns a `*System` on a 10x10 open map plus an `ecsEntity` whose `id` field is the placed entity. Add the `easing` import to both test files.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/
```

Expected: FAIL with build errors such as `undefined: MoveOptions` and `not enough arguments in call to s.MoveEntity`.

- [ ] **Step 3: Write the implementation**

In `motion/motion_move.go`, add above `MoveEntity`:

```go
// MoveOptions describes how a body moves: how fast overall, and how that speed
// is distributed over the move.
type MoveOptions struct {
	// Speed is the average movement speed in tiles per second. It must be
	// positive; the move entry points panic otherwise.
	Speed float64

	// Ease shapes the speed over the move's duration. The zero value,
	// easing.CurveLinear, is constant speed, which is what every move did
	// before easing existed.
	//
	// The curve spans the move as issued, so a single move across many
	// tiles accelerates once over the whole distance; a game wanting a
	// curve per step issues a move per step, as MoveEntityTowards does.
	// Easing suits committed, point-to-point moves: redirecting a move
	// re-anchors its curve, so a body retargeted every tick under a
	// symmetric curve never leaves the slow opening of the curve. Use
	// easing.CurveLinear for continuous steering and pursuit.
	Ease easing.Curve
}
```

Rewrite `MoveEntity`, keeping the occupancy logic untouched, so that it takes `opts`, validates the speed, and re-anchors every parametric field:

```go
// MoveEntity starts moving an entity toward destination under opts (average
// speed in tiles per second, and the easing curve shaping it). When the System
// has an Occupancy manager, the destination tile must be free (or reserved by
// this entity); the entity's reservation moves from its current tile to the
// destination tile as the move starts.
//
// The entity's facing direction is set toward the destination. The entity must
// have a Spatial and opts.Speed must be positive; MoveEntity panics otherwise.
// A move started on an entity that is already moving is re-anchored from its
// current position, so the new move takes its full distance divided by its
// speed. MoveEntity is intended for entities settled on their reserved tile:
// redirecting an entity mid-move can strand its old destination reservation
// and clear a tile it no longer holds.
func (s *System) MoveEntity(id ecs.EntityId, destination geometry.Vector2, opts MoveOptions) MoveStart {
	if opts.Speed <= 0 {
		panic(fmt.Sprintf("moving entity %v: speed must be positive, got %v", id, opts.Speed))
	}

	sc, ok := s.Spatials.Get(id)
	if !ok {
		panic(fmt.Sprintf("moving entity %v: no Spatial component", id))
	}

	// Refuse destinations reserved by another entity.
	if s.Occupancy != nil {
		destTile := tilemap.WorldPositionToTile(destination)
		if occupant, occupied := s.Occupancy.GetOccupant(destTile); occupied && occupant != id {
			return MoveStart{Outcome: MoveOutcomeDestinationOccupied, Destination: destination}
		}
		s.Occupancy.ClearOccupant(tilemap.WorldPositionToTile(sc.Position))
	}

	if sc.Position == destination {
		// Keep the entity's reservation on its current tile.
		if s.Occupancy != nil {
			s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(destination), id)
		}
		return MoveStart{Outcome: MoveOutcomeAtDestination, Destination: destination}
	}

	distance := sc.Position.DistanceTo(destination)
	total := time.Duration(distance / opts.Speed * float64(time.Second))

	// Re-anchor every parametric field: a stale Start or Total from a
	// previous move would make the body jump or arrive at the wrong time.
	mc := s.Movements.GetOrAdd(id, Movement{})
	mc.Destination = destination
	mc.Speed = opts.Speed
	mc.Ease = opts.Ease
	mc.Start = sc.Position
	mc.Elapsed = 0
	mc.Total = total
	sc.Direction = destination.Sub(sc.Position)

	if s.Occupancy != nil {
		s.Occupancy.SetOccupant(tilemap.WorldPositionToTile(destination), id)
	}

	return MoveStart{
		Outcome:     MoveOutcomeStarted,
		Destination: destination,
		Distance:    distance,
		Duration:    total,
	}
}
```

In `motion/motion_towards.go`, change both signatures to take `opts MoveOptions` in place of `speed float64` (`MoveEntityTowardsArea` keeps `radius float64` and gains `opts MoveOptions` as its last parameter), pass `opts` through at the three internal call sites (lines 52, 84 and 161), and update both doc comments to say "under opts" rather than "at speed".

- [ ] **Step 4: Run the tests to verify they pass**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ -v && go vet ./motion/
```

Expected: PASS across the package.

- [ ] **Step 5: Commit**

```bash
git add motion/
git commit -m "Take movement options, including the easing curve, when starting a move"
```

---

### Task 6: Savegame round trip, documentation and release

**Files:**
- Test: `motion/motion_test.go` (append the gob tests)
- Modify: `motion/doc.go`
- Modify: `ARCHITECTURE.md` (the package map table at lines 22-30, and the dependency notes below it)
- Modify: `docs/superpowers/plans/2026-07-19-movement-easing.md` (tick the boxes as you go)

**Interfaces:**
- Consumes: everything from Tasks 1 to 5.
- Produces: no new API.

- [ ] **Step 1: Write the failing test**

Append to `motion/motion_test.go`:

```go
func TestMovement_GobRoundTripMidEasedMove(t *testing.T) {
	start := geometry.NewVector2(1.0, 2.0)
	dest := geometry.NewVector2(5.0, 2.0)
	mc := easedMovement(start, dest, 1.0, easing.CurveInOut) // Total 4s

	advanced, pos, _ := ProcessMove(mc, start, time.Second)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(advanced); err != nil {
		t.Fatalf("encoding a Movement: %v", err)
	}
	var decoded Movement
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decoding a Movement: %v", err)
	}

	if decoded != advanced {
		t.Errorf("round trip changed the movement: %+v, want %+v", decoded, advanced)
	}

	// The restored move continues to the same arrival, at the same tick.
	restoredPos := pos
	ticks := 0
	for done := false; !done; {
		ticks++
		decoded, restoredPos, done = ProcessMove(decoded, restoredPos, time.Second)
	}
	if ticks != 3 || restoredPos != dest {
		t.Errorf("restored move finished in %d ticks at %v, want 3 ticks at %v", ticks, restoredPos, dest)
	}
}

// A Movement encoded before the easing fields existed decodes to a working
// constant-speed move: gob leaves unknown-to-the-encoder fields at their zero
// values.
type legacyMovement struct {
	Destination geometry.Vector2
	Speed       float64
}

func TestMovement_GobDecodesLegacyMovementAsLinear(t *testing.T) {
	legacy := legacyMovement{Destination: geometry.NewVector2(2.0, 0.0), Speed: 1.0}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(legacy); err != nil {
		t.Fatalf("encoding the legacy movement: %v", err)
	}
	var decoded Movement
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decoding into Movement: %v", err)
	}

	if decoded.Ease != easing.CurveLinear || decoded.Total != 0 || decoded.Elapsed != 0 {
		t.Errorf("expected a constant-speed move with no recorded timing, got %+v", decoded)
	}

	pos := geometry.NewVector2(0.0, 0.0)
	_, pos, done := ProcessMove(decoded, pos, time.Second)
	if done || pos.DistanceTo(geometry.NewVector2(1.0, 0.0)) > 1e-9 {
		t.Errorf("expected the legacy move to advance one tile, got %v (done=%v)", pos, done)
	}
}
```

Add `"bytes"` and `"encoding/gob"` to the imports of `motion/motion_test.go`. Gob matches struct fields by name, so `legacyMovement` decodes into `Movement`, leaving the fields it never carried at their zero values. If gob turns out to reject the differing type names, encode a `Movement` carrying only `Destination` and `Speed` instead: gob omits zero-valued fields, so that stream is byte-identical to one written before the easing fields existed. Keep the assertions either way.

- [ ] **Step 2: Run the tests to verify they fail (or pass for the right reason)**

```bash
export GOMODCACHE=/tmp/go-mod-cache && go test ./motion/ -run Gob -v
```

Expected: these tests should pass once they compile, since the design's whole claim is that the component is serialization-safe by construction. If either fails, that is a real defect in the field types: fix `Movement`, do not weaken the test.

- [ ] **Step 3: Update the documentation**

Rewrite `motion/doc.go` so the package summary covers easing:

```go
// Package motion provides components and systems for entity movement.
//
// Spatial holds an entity's current world position and facing direction.
// Movement stores an entity's movement target, speed and easing state.
// MovementResult carries the outcome of a movement tick: the entity ID,
// original position, new position, and whether the destination was reached.
//
// A move runs on one of two paths. Constant-speed moves advance incrementally
// through ProcessMovement, which displaces the body by speed times the tick
// duration. Eased moves are parametric: ProcessMove derives the position from
// the move's start, destination and progress through its total duration, so
// the path is a pure function of elapsed game time and independent of how that
// time is sliced into ticks. Both paths give a move the same total duration
// and the same arrival tick; only the shape in between differs. ProcessMove
// routes between them by the movement's easing.Curve, and a zero-duration tick
// moves nothing and completes nothing on either.
//
// System bundles the component handles and spatial indexes movement operates
// on. Tick advances every entity that has a Movement and satisfies the
// sim.TickSystem interface. MoveEntity starts a single move with tile
// occupancy checks, taking a MoveOptions describing the average speed and the
// easing curve, and MoveEntityTowards and MoveEntityTowardsArea follow A*
// paths one bounded step at a time. Game policy stays with the caller: each
// attempt returns a MoveStart describing what happened so the consuming game
// can update its own entity states, AI scheduling, and logs.
package motion
```

In `ARCHITECTURE.md`, add a row to the package map table immediately after the `geometry` row:

```markdown
| `easing` | Easing curves (`Curve`, `Apply`) for shaping interpolated progress | — |
```

and update the `motion` row's dependency list to include `easing`. Read the surrounding prose (the dependency narrative below the table) and add `easing` wherever `motion`'s dependencies are described, keeping the existing wording style.

- [ ] **Step 4: Run the full gate**

```bash
export GOMODCACHE=/tmp/go-mod-cache && task lint && task test:headless && go vet ./...
```

Expected: all three succeed with no warnings. If `task lint` reports formatting or modernizer findings, fix them and re-run.

- [ ] **Step 5: Commit**

```bash
git add motion/ ARCHITECTURE.md docs/superpowers/plans/2026-07-19-movement-easing.md
git commit -m "Cover eased movement serialization and document the easing feature"
```

- [ ] **Step 6: Tag the release**

Confirm with the user before pushing anything. Then, following the `v0.1.13` precedent (annotated tag whose message is the release summary):

```bash
git push origin main
git tag -a v0.1.14 -m "Add opt-in movement easing

Movements can now follow an easing curve (ease in, ease out, ease in-out)
instead of constant speed. The eased position is the simulated position, and an
eased move keeps the same total duration and arrival tick as the constant-speed
move it replaces. Constant-speed moves keep the existing incremental code path
unchanged. MoveEntity, MoveEntityTowards and MoveEntityTowardsArea now take a
MoveOptions carrying the speed and the curve."
git push origin v0.1.14
```

Report the tagged version so the consuming game can bump its `go.mod`.
