package easing

// Curve shapes normalized progress over a transition: given how far through a
// transition something is in time, it reports how far through the transition
// it is in value. Every curve satisfies Apply(0) == 0 and Apply(1) == 1, so a
// curve never changes when a transition starts or finishes, only its shape in
// between.
//
// Curve values are persisted by consumers (a Curve rides an in-flight move in
// a savegame), so the numeric values are part of the on-disk format: new
// curves are appended and existing ones are never renumbered or removed.
type Curve int

const (
	// CurveLinear is constant rate: no easing at all. It is the zero value,
	// so an unset Curve behaves the way the engine did before easing
	// existed.
	CurveLinear Curve = iota

	// CurveIn eases the start (t^2): slow off the mark, full rate on
	// arrival. A wind-up.
	CurveIn

	// CurveOut eases the end (1-(1-t)^2): full rate off the mark,
	// decelerating into arrival. An explosive launch.
	CurveOut

	// CurveInOut eases both ends (3t^2-2t^3, smoothstep): slow off the
	// mark, fastest at the midpoint, gentle settle. Symmetric, so it
	// crosses the halfway value at exactly half the duration.
	CurveInOut
)

// Apply maps progress t to eased progress, both in [0,1]. Values of t outside
// [0,1] are clamped. An unrecognized Curve is treated as CurveLinear.
func (c Curve) Apply(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}

	switch c {
	case CurveIn:
		return t * t
	case CurveOut:
		return 1 - (1-t)*(1-t)
	case CurveInOut:
		return t * t * (3 - 2*t)
	}
	return t
}
