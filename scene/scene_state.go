package scene

import (
	"time"
)

// State contains the state being passed by the Game to each Scene when updating or drawing and debug information.
type State struct {
	lastFrameTime          time.Time
	durationSinceLastFrame time.Duration
}

// Update updates the state of State.
func (s *State) Update() {
	now := time.Now()
	s.durationSinceLastFrame = now.Sub(s.lastFrameTime)
	s.lastFrameTime = now
}

// DurationSinceLastFrame returns the duration since the last frame.
func (s *State) DurationSinceLastFrame() time.Duration {
	return s.durationSinceLastFrame
}
