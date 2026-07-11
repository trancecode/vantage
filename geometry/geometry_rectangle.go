package geometry

import (
	"fmt"

	"github.com/trancecode/vantage/util"
)

// Rectangle represents a rectangular area defined by its minimum and maximum positions.
type Rectangle struct {
	Min Vector2
	Max Vector2
}

// NewRectangle creates and returns a new Rectangle with the given minimum and maximum positions.
func NewRectangle(min, max Vector2) Rectangle {
	return Rectangle{Min: min, Max: max}
}

func NewRectangleFromPoints[T util.Number](minX, minY, maxX, maxY T) Rectangle {
	min := NewVector2(minX, minY)
	max := NewVector2(maxX, maxY)
	return NewRectangle(min, max)
}

// Width returns the width of the rectangle.
func (r Rectangle) Width() float64 {
	return r.Max.X() - r.Min.X()
}

// Height returns the height of the rectangle.
func (r Rectangle) Height() float64 {
	return r.Max.Y() - r.Min.Y()
}

func (r Rectangle) String() string {
	return fmt.Sprintf("(%v, %v)", r.Min, r.Max)
}

func SquareWithCenter(center Vector2, size float64) Rectangle {
	halfSize := size / 2
	min := NewVector2(center.X()-halfSize, center.Y()-halfSize)
	max := NewVector2(center.X()+halfSize, center.Y()+halfSize)
	return NewRectangle(min, max)
}

// RandomPointInRectangle returns a random point within the given rectangle.
func RandomPointInRectangle(r Rectangle, rng util.Float64Source) Vector2 {
	x := r.Min.X() + (r.Max.X()-r.Min.X())*rng.Float64()
	y := r.Min.Y() + (r.Max.Y()-r.Min.Y())*rng.Float64()
	return NewVector2(x, y)
}
