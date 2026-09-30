// Package geometry provides geometric types and operations for 2D space.
//
// Vector2 is the primary type — a generic 2D point/vector supporting
// addition, subtraction, scaling, distance, magnitude, and unit-vector
// operations. Rectangle represents an axis-aligned bounding box.
// These types are used throughout the codebase for world coordinates.
//
// CapsuleTouchesRectangle is the exact test of whether a circle swept along a
// segment touches a rectangle, which movement uses to test straight walks
// against blocked tiles.
package geometry
