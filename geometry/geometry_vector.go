package geometry

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/trancecode/vantage/util"
)

// Zero2D returns a zero Vector2, which is a vector with both x and y set to 0.
// It is a convenient way to create a zero vector without needing to specify the values.
func Zero2D() Vector2 {
	return zero2D

}

// Zero2D is a constant representing the zero vector in 2D space.
var zero2D = NewVector2(0, 0)

// Vector2 represents a 2D point in space.
type Vector2 struct {
	x float64
	y float64
}

// NewVector2 creates and returns a new Vector2 with the given x and y values.
func NewVector2[T util.Number](x, y T) Vector2 {
	return Vector2{x: float64(x), y: float64(y)}
}

// Add returns a new Vector2 that is the sum of the current Vector2 and the given Vector2.
func (p Vector2) Add(other Vector2) Vector2 {
	return NewVector2(p.x+other.x, p.y+other.y)
}

// Sub returns a new Vector2 that is the difference of the current Vector2 and the given Vector2.
func (p Vector2) Sub(other Vector2) Vector2 {
	return NewVector2(p.x-other.x, p.y-other.y)
}

// Min returns a new Vector2 that is the minimum of the current Vector2 and the given Vector2.
func (p Vector2) Min(other Vector2) Vector2 {
	return NewVector2(min(p.x, other.x), min(p.y, other.y))
}

// Max returns a new Vector2 that is the maximum of the current Vector2 and the given Vector2.
func (p Vector2) Max(other Vector2) Vector2 {
	return NewVector2(max(p.x, other.x), max(p.y, other.y))
}

// Scale returns a new Vector2 that is the multiplication of the current Vector2 and the given scalar value.
func (p Vector2) Scale(scalar float64) Vector2 {
	return NewVector2(p.x*scalar, p.y*scalar)
}

// Lerp linearly interpolates between the current Vector2 and the given
// Vector2: it returns the current vector when t is 0 and the given vector when
// t is 1. It does not clamp t, so values outside [0,1] extrapolate along the
// line through both points.
func (p Vector2) Lerp(other Vector2, t float64) Vector2 {
	return NewVector2(p.x+(other.x-p.x)*t, p.y+(other.y-p.y)*t)
}

// DistanceTo calculates the Euclidean distance between two Vector2s.
func (p Vector2) DistanceTo(other Vector2) float64 {
	return other.Sub(p).Magnitude()
}

// String returns a string representation of the Vector2.
func (p Vector2) String() string {
	return fmt.Sprintf("(%.3f, %.3f)", p.x, p.y)
}

// X returns the x component of the vector.
func (p Vector2) X() float64 { return p.x }

// Y returns the y component of the vector.
func (p Vector2) Y() float64 { return p.y }

// Unit returns the normalized (unit) vector of the Vector2.
func (p Vector2) Unit() Vector2 {
	magnitude := p.Magnitude()
	if magnitude == 0 {
		return NewVector2(0, 0) // Avoid division by zero; return a zero vector.
	}
	return NewVector2(p.x/magnitude, p.y/magnitude)
}

// Magnitude calculates the length (magnitude) of the Vector2 vector.
func (p Vector2) Magnitude() float64 {
	return math.Sqrt(p.x*p.x + p.y*p.y)
}

// IsZero returns true if the vector is the zero vector.
func (p Vector2) IsZero() bool {
	return p.x == 0 && p.y == 0
}

// AsInts returns the vector components as integers.
func (p Vector2) AsInts() (int, int) {
	return int(p.x), int(p.y)
}

// AsFloats returns the vector components as floats.
func (p Vector2) AsFloats() (float64, float64) {
	return p.x, p.y
}

// MarshalBinary encodes the vector as 16 big-endian bytes (x then y, IEEE 754
// bits). It implements encoding.BinaryMarshaler so vectors, and the engine
// components holding them, can be persisted in savegames (encoding/gob uses
// this implementation automatically).
func (v Vector2) MarshalBinary() ([]byte, error) {
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b[0:8], math.Float64bits(v.x))
	binary.BigEndian.PutUint64(b[8:16], math.Float64bits(v.y))
	return b, nil
}

// UnmarshalBinary decodes 16 bytes produced by MarshalBinary.
func (v *Vector2) UnmarshalBinary(data []byte) error {
	if len(data) != 16 {
		return fmt.Errorf("geometry.Vector2.UnmarshalBinary: expected 16 bytes, got %d", len(data))
	}
	v.x = math.Float64frombits(binary.BigEndian.Uint64(data[0:8]))
	v.y = math.Float64frombits(binary.BigEndian.Uint64(data[8:16]))
	return nil
}
