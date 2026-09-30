package geometry

import "math"

// CapsuleTouchesRectangle reports whether the capsule of radius around the
// segment from a to b, the shape a circle sweeps when moved along it, touches
// the closed rectangle r: whether some point of r lies within radius of the
// segment. Touching counts, so a capsule whose edge meets r exactly touches
// it. A zero-length segment is a circle.
//
// The test is exact. Two disjoint convex shapes are closest at a vertex of one
// of them, so it checks both ends of the segment against r, the corners of r
// against the segment, and whether the segment crosses an edge of r.
func CapsuleTouchesRectangle(a, b Vector2, radius float64, r Rectangle) bool {
	r2 := radius * radius
	if pointRectangleDistanceSquared(a, r) <= r2 || pointRectangleDistanceSquared(b, r) <= r2 {
		return true
	}

	corners := [4]Vector2{r.Min, NewVector2(r.Max.X(), r.Min.Y()), r.Max, NewVector2(r.Min.X(), r.Max.Y())}
	for _, corner := range corners {
		if pointSegmentDistanceSquared(corner, a, b) <= r2 {
			return true
		}
	}
	for i := range corners {
		if segmentsIntersect(a, b, corners[i], corners[(i+1)%len(corners)]) {
			return true
		}
	}
	return false
}

// pointRectangleDistanceSquared returns the squared distance from p to the
// closest point of r, zero when p is inside.
func pointRectangleDistanceSquared(p Vector2, r Rectangle) float64 {
	dx := math.Max(math.Max(r.Min.X()-p.X(), 0), p.X()-r.Max.X())
	dy := math.Max(math.Max(r.Min.Y()-p.Y(), 0), p.Y()-r.Max.Y())
	return dx*dx + dy*dy
}

// pointSegmentDistanceSquared returns the squared distance from p to the
// closest point of the segment from a to b.
func pointSegmentDistanceSquared(p, a, b Vector2) float64 {
	abx, aby := b.X()-a.X(), b.Y()-a.Y()
	apx, apy := p.X()-a.X(), p.Y()-a.Y()
	length2 := abx*abx + aby*aby
	dot := apx*abx + apy*aby
	if length2 == 0 || dot <= 0 {
		return apx*apx + apy*apy
	}
	if dot >= length2 {
		bpx, bpy := p.X()-b.X(), p.Y()-b.Y()
		return bpx*bpx + bpy*bpy
	}
	cross := apx*aby - apy*abx
	return cross * cross / length2
}

// orientation reports which side of the line through a and b the point c is
// on: 1 for counter-clockwise, -1 for clockwise, 0 for on the line.
func orientation(a, b, c Vector2) int {
	v := (b.X()-a.X())*(c.Y()-a.Y()) - (b.Y()-a.Y())*(c.X()-a.X())
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// withinBounds reports whether p lies in the bounding box of the segment from
// a to b, which for a p on the segment's line means on the segment.
func withinBounds(a, p, b Vector2) bool {
	return p.X() >= math.Min(a.X(), b.X()) && p.X() <= math.Max(a.X(), b.X()) &&
		p.Y() >= math.Min(a.Y(), b.Y()) && p.Y() <= math.Max(a.Y(), b.Y())
}

// segmentsIntersect reports whether the segments ab and cd share a point,
// touching and collinear overlap included.
func segmentsIntersect(a, b, c, d Vector2) bool {
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	if o1*o2 < 0 && o3*o4 < 0 {
		return true
	}
	return (o1 == 0 && withinBounds(a, c, b)) ||
		(o2 == 0 && withinBounds(a, d, b)) ||
		(o3 == 0 && withinBounds(c, a, d)) ||
		(o4 == 0 && withinBounds(c, b, d))
}
