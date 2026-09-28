package render

import (
	"math"
	"testing"
	"time"

	"github.com/trancecode/vantage/geometry"
)

func TestOccludes(t *testing.T) {
	tree := geometry.NewRectangleFromPoints(0.0, 0.0, 100.0, 200.0)
	behind := geometry.NewRectangleFromPoints(40.0, 100.0, 60.0, 160.0)
	cases := []struct {
		name        string
		subject     geometry.Rectangle
		subjectBase float64
		want        bool
	}{
		{"overlapping and behind", behind, 160, true},
		{"overlapping and in front", geometry.NewRectangleFromPoints(40.0, 150.0, 60.0, 210.0), 210, false},
		{"level with the base", behind, 200, false},
		{"clear of it", geometry.NewRectangleFromPoints(300.0, 100.0, 320.0, 160.0), 160, false},
	}
	for _, c := range cases {
		if got := Occludes(tree, 200, c.subject, c.subjectBase); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFaderEasesDownAndBack(t *testing.T) {
	f := NewFader[int](0.35, 200*time.Millisecond)
	if got := f.Opacity(1, true, 100*time.Millisecond); !near(got, 1-0.65/2) {
		t.Fatalf("after half the duration occluding: got %v, want %v", got, 1-0.65/2)
	}
	if got := f.Opacity(1, true, time.Second); !near(got, 0.35) {
		t.Fatalf("long after: got %v, want 0.35", got)
	}
	if got := f.Opacity(1, false, 100*time.Millisecond); !near(got, 0.35+0.65/2) {
		t.Fatalf("half way back: got %v, want %v", got, 0.35+0.65/2)
	}
	if got := f.Opacity(1, false, time.Second); !near(got, 1) {
		t.Fatalf("fully back: got %v, want 1", got)
	}
}

func TestFaderNewKeyStartsOpaqueAndEndFrameForgets(t *testing.T) {
	f := NewFader[int](0.35, 200*time.Millisecond)
	if got := f.Opacity(7, false, 0); got != 1 {
		t.Fatalf("new key: got %v, want 1", got)
	}
	f.Opacity(7, true, time.Second)
	f.EndFrame()
	f.EndFrame() // key 7 was not asked for during this frame, so it is forgotten
	if got := f.Opacity(7, false, 0); got != 1 {
		t.Fatalf("forgotten key: got %v, want 1", got)
	}
}

func TestFaderStructLiteralEasesLikeNewFader(t *testing.T) {
	// A struct-literal Fader should work and produce the same results as NewFader.
	literal := &Fader[int]{FadedOpacity: 0.35, Duration: 200 * time.Millisecond}
	fromNew := NewFader[int](0.35, 200*time.Millisecond)

	// Both should handle the first call (100ms occluding) identically.
	literalStep1 := literal.Opacity(1, true, 100*time.Millisecond)
	newStep1 := fromNew.Opacity(1, true, 100*time.Millisecond)
	if !near(literalStep1, newStep1) {
		t.Errorf("step 1 (literal): got %v, want %v", literalStep1, newStep1)
	}

	// Both should handle the second call (another 100ms occluding) identically.
	literalStep2 := literal.Opacity(1, true, 100*time.Millisecond)
	newStep2 := fromNew.Opacity(1, true, 100*time.Millisecond)
	if !near(literalStep2, newStep2) {
		t.Errorf("step 2 (literal): got %v, want %v", literalStep2, newStep2)
	}
}
