package render

import (
	"time"

	"github.com/trancecode/vantage/geometry"
)

// Occludes reports whether a drawable covering occluder on screen and standing
// on the line occluderBase hides a subject covering subject and standing on
// subjectBase: their rectangles overlap and the subject stands behind, its base
// higher on the screen (a smaller Y). Games fade occluders that hide subjects
// the player needs to see, such as a tree in front of a character.
func Occludes(occluder geometry.Rectangle, occluderBase float64, subject geometry.Rectangle, subjectBase float64) bool {
	return subjectBase < occluderBase && occluder.Overlaps(subject)
}

// Fader eases the opacity of keyed drawables between opaque and a faded
// opacity, one step per call, so an occluder fades out smoothly while it hides
// something and back in once clear. It is display state only. A struct literal
// such as &Fader[int]{FadedOpacity: 0.35, Duration: 200*time.Millisecond} is
// usable and works the same as a value from NewFader.
type Fader[K comparable] struct {
	// FadedOpacity is the opacity an occluding drawable eases to, in [0, 1].
	FadedOpacity float64

	// Duration is how long a full ease between 1 and FadedOpacity takes.
	Duration time.Duration

	opacity map[K]float64
	seen    map[K]bool
}

// NewFader returns a Fader easing to fadedOpacity over duration.
func NewFader[K comparable](fadedOpacity float64, duration time.Duration) *Fader[K] {
	return &Fader[K]{FadedOpacity: fadedOpacity, Duration: duration, opacity: map[K]float64{}, seen: map[K]bool{}}
}

// Opacity moves key's opacity by elapsed towards FadedOpacity when occluding
// is true and towards 1 otherwise, and returns it. A key seen for the first
// time starts opaque.
func (f *Fader[K]) Opacity(key K, occluding bool, elapsed time.Duration) float64 {
	if f.opacity == nil {
		f.opacity = make(map[K]float64)
	}
	if f.seen == nil {
		f.seen = make(map[K]bool)
	}
	f.seen[key] = true
	current, ok := f.opacity[key]
	if !ok {
		current = 1
	}
	step := 1.0
	if f.Duration > 0 {
		step = (1 - f.FadedOpacity) * float64(elapsed) / float64(f.Duration)
	}
	if occluding {
		current = max(f.FadedOpacity, current-step)
	} else {
		current = min(1, current+step)
	}
	if current == 1 {
		delete(f.opacity, key)
		return 1
	}
	f.opacity[key] = current
	return current
}

// EndFrame forgets every key Opacity was not called for since the previous
// EndFrame, so a drawable that left the view is opaque when it returns.
func (f *Fader[K]) EndFrame() {
	for key := range f.opacity {
		if !f.seen[key] {
			delete(f.opacity, key)
		}
	}
	clear(f.seen)
}
