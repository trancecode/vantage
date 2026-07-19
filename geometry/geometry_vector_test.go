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

func TestVector2Lerp_EndpointsAreExactForAwkwardValues(t *testing.T) {
	// The endpoint-exact form p*(1-t) + other*t is used to guarantee exact
	// endpoints in floating point arithmetic. These values exhibit precision
	// issues with the naive p + (other-p)*t formula, so we verify exactness here.
	a := NewVector2(-50000.3, 0.7)
	b := NewVector2(50000.9, -1234.5678)

	if got := a.Lerp(b, 1); got != b {
		t.Errorf("Lerp(t=1) = %v, want exactly %v", got, b)
	}
	if got := a.Lerp(b, 0); got != a {
		t.Errorf("Lerp(t=0) = %v, want exactly %v", got, a)
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
