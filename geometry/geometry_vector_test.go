package geometry

import "testing"

func TestVector2Add(t *testing.T) {
	a := NewVector2(1, 2)
	b := NewVector2(3, 4)
	got := a.Add(b)
	want := NewVector2(4, 6)
	if got != want {
		t.Fatalf("Add() = %v, want %v", got, want)
	}
}
