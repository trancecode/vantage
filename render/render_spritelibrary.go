package render

import (
	"fmt"
	"slices"
)

// SpriteLibrary maps display names to sprites. Games register their sprites
// into the package-level Sprites library at init time, and engine tooling such
// as the sprite showcase scene reads them back.
//
// A SpriteLibrary is not safe for concurrent use: registration happens at init
// time and reads happen while drawing, both on the same goroutine.
type SpriteLibrary struct {
	sprites map[string]*Sprite
}

// NewSpriteLibrary returns an empty SpriteLibrary. Most games register into the
// package-level Sprites library instead; this is for tests and for tooling that
// needs a library of its own.
func NewSpriteLibrary() *SpriteLibrary {
	return &SpriteLibrary{sprites: map[string]*Sprite{}}
}

// Add registers a sprite under a display name and returns it, so that
// registration composes with the chainable Sprite setters:
//
//	Sprites.Add("Character", MustLoadSprite(img, 6, 10, indexes, nil)).
//		SetType(SpriteTypeActor)
//
// It panics on an empty name, a nil sprite, or a name that is already
// registered. All three are load-time programming errors, and registration
// runs from init, so failing loudly beats yielding a silently wrong catalog.
func (l *SpriteLibrary) Add(name string, s *Sprite) *Sprite {
	if name == "" {
		panic("sprite name must not be empty")
	}
	if s == nil {
		panic(fmt.Sprintf("sprite %q must not be nil", name))
	}
	if _, ok := l.sprites[name]; ok {
		panic(fmt.Sprintf("duplicate sprite name: %q", name))
	}
	l.sprites[name] = s
	return s
}

// Get returns the sprite registered under a name. Unlike Add, a miss is a
// plausible runtime condition rather than a programming error, so it reports
// absence instead of panicking.
func (l *SpriteLibrary) Get(name string) (*Sprite, bool) {
	s, ok := l.sprites[name]
	return s, ok
}

// Names returns every registered name, sorted lexicographically, so callers
// that iterate the library get a stable order without sorting themselves.
func (l *SpriteLibrary) Names() []string {
	names := make([]string, 0, len(l.sprites))
	for name := range l.sprites {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Len returns the number of registered sprites.
func (l *SpriteLibrary) Len() int {
	return len(l.sprites)
}

// Sprites is the default sprite library. Games register their sprites here, and
// engine tooling reads them back.
var Sprites = NewSpriteLibrary()
