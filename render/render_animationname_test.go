package render

import "testing"

// testAnimationType returns a game-range animation type well clear of both the
// engine's own values and of other tests' registrations, since the registry is
// package-level.
func testAnimationType(offset int) AnimationType {
	return AnimationGameBase + 1000 + AnimationType(offset)
}

func TestAnimationNameUsesTheRegisteredName(t *testing.T) {
	a := testAnimationType(0)
	RegisterAnimationName(a, "South-east")
	if got := AnimationName(a); got != "South-east" {
		t.Fatalf("AnimationName = %q, want %q", got, "South-east")
	}
}

func TestAnimationNameTrimsTheEnginePrefixWhenUnregistered(t *testing.T) {
	// The engine's own names all start with "Animation", which is noise in a
	// label.
	if got := AnimationName(AnimationIdleDown); got != "IdleDown" {
		t.Fatalf("AnimationName = %q, want %q", got, "IdleDown")
	}
	if got := AnimationName(AnimationDefault); got != "Default" {
		t.Fatalf("AnimationName = %q, want %q", got, "Default")
	}
}

func TestAnimationNameFallsBackToStringForUnknownTypes(t *testing.T) {
	// A game type with no registration has no prefix to trim, so it reads as
	// the stringer's placeholder. Ugly, which is why registering is worth it.
	a := testAnimationType(1)
	want := a.String()
	if got := AnimationName(a); got != want {
		t.Fatalf("AnimationName = %q, want %q", got, want)
	}
}

func TestRegisterAnimationNameAcceptsAnEngineType(t *testing.T) {
	// A game may prefer its own wording for the engine's types.
	RegisterAnimationName(AnimationAttackUp, "Attack up")
	defer unregisterAnimationName(AnimationAttackUp)
	if got := AnimationName(AnimationAttackUp); got != "Attack up" {
		t.Fatalf("AnimationName = %q, want %q", got, "Attack up")
	}
}

func TestRegisterAnimationNameEmptyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on an empty animation name")
		}
	}()
	RegisterAnimationName(testAnimationType(2), "")
}

func TestRegisterAnimationNameDuplicatePanics(t *testing.T) {
	a := testAnimationType(3)
	RegisterAnimationName(a, "First")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on a duplicate animation type")
		}
	}()
	RegisterAnimationName(a, "Second")
}
