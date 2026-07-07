package geometry

import (
	"bytes"
	"encoding/gob"
	"testing"
)

func TestVector2Add(t *testing.T) {
	a := NewVector2(1, 2)
	b := NewVector2(3, 4)
	got := a.Add(b)
	want := NewVector2(4, 6)
	if got != want {
		t.Fatalf("Add() = %v, want %v", got, want)
	}
}

func TestVector2BinaryRoundTrip(t *testing.T) {
	v := NewVector2(3.25, -7.5)
	b, err := v.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var got Vector2
	if err := got.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got != v {
		t.Fatalf("round trip = %v, want %v", got, v)
	}
}

func TestVector2UnmarshalRejectsShortData(t *testing.T) {
	var v Vector2
	if err := v.UnmarshalBinary([]byte{1, 2, 3}); err == nil {
		t.Fatal("UnmarshalBinary must fail on data that is not 16 bytes")
	}
}

func TestVector2GobRoundTrip(t *testing.T) {
	// Games persist engine components holding Vector2 values with encoding/gob,
	// which relies on the BinaryMarshaler implementation for opaque structs.
	var buf bytes.Buffer
	v := NewVector2(1.5, 2.5)
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("gob encode: %v", err)
	}
	var got Vector2
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("gob decode: %v", err)
	}
	if got != v {
		t.Fatalf("gob round trip = %v, want %v", got, v)
	}
}
