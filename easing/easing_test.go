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
