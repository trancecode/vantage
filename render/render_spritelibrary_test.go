package render

import (
	"testing"
)

func TestSpriteLibraryAddAndGet(t *testing.T) {
	l := NewSpriteLibrary()
	s := NewSprite()
	l.Add("Character", s)

	got, ok := l.Get("Character")
	if !ok {
		t.Fatal("Get did not find the added sprite")
	}
	if got != s {
		t.Fatal("Get returned a different sprite than the one added")
	}
}

func TestSpriteLibraryGetUnknownName(t *testing.T) {
	l := NewSpriteLibrary()
	if _, ok := l.Get("Missing"); ok {
		t.Fatal("Get reported an unknown name as present")
	}
}

func TestSpriteLibraryAddReturnsSpriteForChaining(t *testing.T) {
	l := NewSpriteLibrary()
	s := NewSprite()
	if got := l.Add("Character", s); got != s {
		t.Fatal("Add did not return the sprite it was given")
	}
}

func TestSpriteLibraryNamesAreSorted(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("Zebra", NewSprite())
	l.Add("Apple", NewSprite())
	l.Add("Mango", NewSprite())

	names := l.Names()
	want := []string{"Apple", "Mango", "Zebra"}
	if len(names) != len(want) {
		t.Fatalf("Names returned %d entries, want %d", len(names), len(want))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("Names()[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestSpriteLibraryLen(t *testing.T) {
	l := NewSpriteLibrary()
	if l.Len() != 0 {
		t.Fatalf("new library Len = %d, want 0", l.Len())
	}
	l.Add("A", NewSprite())
	l.Add("B", NewSprite())
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}
}

func TestSpriteLibraryAddEmptyNamePanics(t *testing.T) {
	l := NewSpriteLibrary()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty sprite name")
		}
	}()
	l.Add("", NewSprite())
}

func TestSpriteLibraryAddNilSpritePanics(t *testing.T) {
	l := NewSpriteLibrary()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil sprite")
		}
	}()
	l.Add("Character", nil)
}

func TestSpriteLibraryAddDuplicateNamePanics(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("Character", NewSprite())
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate sprite name")
		}
	}()
	l.Add("Character", NewSprite())
}

func TestNewSpriteLibraryIsIndependentOfDefault(t *testing.T) {
	l := NewSpriteLibrary()
	l.Add("OnlyInLocalLibrary", NewSprite())
	if _, ok := Sprites.Get("OnlyInLocalLibrary"); ok {
		t.Fatal("adding to a new library leaked into the default Sprites library")
	}
}
