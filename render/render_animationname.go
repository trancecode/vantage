package render

import (
	"fmt"
	"strings"
)

// enginePrefix is the prefix the generated String() gives the engine's own
// animation type names, which is noise in a label.
const enginePrefix = "Animation"

// animationNames holds display names registered for animation types. Not safe
// for concurrent use: registration happens at init time and reads happen while
// drawing, both on the same goroutine, matching SpriteLibrary.
var animationNames = map[AnimationType]string{}

// RegisterAnimationName gives an animation type a display name. It exists for
// the types the engine's generated String() cannot know: a game defining its
// own types at or above AnimationGameBase gets "AnimationType(64)" and so on,
// which is unreadable in a sprite showcase label.
//
// Registering one of the engine's own types is allowed, for a game that prefers
// its own wording.
//
// It panics on an empty name or a type that is already registered. Both are
// load-time programming errors, and registration runs from init, so failing
// loudly beats a catalog that depends on package initialization order.
func RegisterAnimationName(a AnimationType, name string) {
	if name == "" {
		panic(fmt.Sprintf("animation name for %v must not be empty", a))
	}
	if existing, ok := animationNames[a]; ok {
		panic(fmt.Sprintf("duplicate animation name for %v: %q and %q", a, existing, name))
	}
	animationNames[a] = name
}

// AnimationName returns an animation type's display name: the registered one if
// there is one, and otherwise its String() with the engine's "Animation" prefix
// trimmed, so AnimationIdleDown reads as IdleDown. An unregistered game type has
// no prefix to trim and reads as the stringer's placeholder.
func AnimationName(a AnimationType) string {
	if name, ok := animationNames[a]; ok {
		return name
	}
	s := a.String()
	if strings.Contains(s, "(") {
		// The stringer's fallback for an unrecognized value, e.g.
		// "AnimationType(1065)", which also happens to start with
		// enginePrefix. It has no real prefix to trim.
		return s
	}
	trimmed, _ := strings.CutPrefix(s, enginePrefix)
	return trimmed
}

// unregisterAnimationName removes a registration. It exists only so tests can
// clean up after registering one of the engine's own types, since the registry
// is package-level.
func unregisterAnimationName(a AnimationType) {
	delete(animationNames, a)
}
