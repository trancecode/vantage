package render

import (
	"math"

	"github.com/trancecode/vantage/geometry"
)

// AnimationType represents different animation states for sprites. It is an open
// enumeration: games define their own states (casting, harvesting, mining) as
// additional AnimationType values and use them as sprite animation keys. Values
// from AnimationGameBase up are reserved for games, so the engine can add states
// without colliding with them.
//
//go:generate stringer -type=AnimationType -output=render_animation_string.go
type AnimationType int

// AnimationGameBase is the first AnimationType value reserved for consuming games.
// The engine never defines a state at or above it.
const AnimationGameBase AnimationType = 64

const (
	AnimationDefault     AnimationType = 0
	AnimationMoveUp      AnimationType = 1
	AnimationMoveDown    AnimationType = 2
	AnimationMoveLeft    AnimationType = 3
	AnimationMoveRight   AnimationType = 4
	AnimationIdleUp      AnimationType = 5
	AnimationIdleDown    AnimationType = 6
	AnimationIdleLeft    AnimationType = 7
	AnimationIdleRight   AnimationType = 8
	AnimationAttackUp    AnimationType = 9
	AnimationAttackDown  AnimationType = 10
	AnimationAttackLeft  AnimationType = 11
	AnimationAttackRight AnimationType = 12
)

// MoveAnimation returns the appropriate movement animation type for a direction.
func MoveAnimation(direction geometry.Vector2) AnimationType {
	if direction == geometry.Zero2D() {
		return AnimationMoveDown
	}

	// Determine the dominant direction based on the absolute differences.
	if math.Abs(direction.X()) > math.Abs(direction.Y()) {
		// Horizontal movement is dominant.
		if direction.X() > 0 {
			return AnimationMoveRight // Moving right
		}
		if direction.X() < 0 {
			return AnimationMoveLeft // Moving left
		}
	}
	if direction.Y() > 0 {
		return AnimationMoveDown // Moving down
	}
	if direction.Y() < 0 {
		return AnimationMoveUp // Moving up
	}

	return AnimationMoveDown
}

// DirectionalVariant resolves a facing direction to the variant animation registered
// for it in variants, which is keyed by the four AnimationMove* states. Games use it
// to derive their own directional action states (casting, harvesting, mining) without
// the engine needing to know what those actions mean. It returns AnimationDefault when
// variants has no entry for the resolved direction.
func DirectionalVariant(direction geometry.Vector2, variants map[AnimationType]AnimationType) AnimationType {
	return variants[MoveAnimation(direction)]
}

var idleAnimations = map[AnimationType]AnimationType{
	AnimationMoveDown:  AnimationIdleDown,
	AnimationMoveLeft:  AnimationIdleLeft,
	AnimationMoveRight: AnimationIdleRight,
	AnimationMoveUp:    AnimationIdleUp,
}

// IdleAnimation returns the idle animation matching the facing direction
// (for actors without a movement direction, defaults to down).
func IdleAnimation(direction geometry.Vector2) AnimationType {
	return DirectionalVariant(direction, idleAnimations)
}

var attackAnimations = map[AnimationType]AnimationType{
	AnimationMoveDown:  AnimationAttackDown,
	AnimationMoveLeft:  AnimationAttackLeft,
	AnimationMoveRight: AnimationAttackRight,
	AnimationMoveUp:    AnimationAttackUp,
}

// AttackAnimation returns the attack animation matching the facing direction
// (for actors without a movement direction, defaults to down).
func AttackAnimation(direction geometry.Vector2) AnimationType {
	return DirectionalVariant(direction, attackAnimations)
}
