// Package easing provides easing curves: functions that shape how a value
// progresses from its start to its end over a fixed span of time.
//
// Curve is an enumeration of the standard shapes (linear, ease in, ease out,
// ease in-out) with a single Apply method mapping normalized progress in
// [0,1] to eased progress in [0,1]. Because Apply is scalar to scalar, the
// same curve serves any interpolated value: a position, a camera zoom, an
// alpha fade.
//
// Curve is an integer enumeration rather than a function value so that it can
// be stored in components and persisted in savegames. Its zero value,
// CurveLinear, is constant rate, which makes an unset curve behave as no
// easing at all.
//
// Easing only shapes progress; it does not do the blending. Pair a curve with
// an interpolation such as geometry.Vector2.Lerp:
//
//	position := start.Lerp(destination, curve.Apply(elapsed/total))
package easing
