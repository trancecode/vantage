package render

import (
	"math"

	"github.com/trancecode/vantage/geometry"
)

// AnimationType represents different animation states for sprites.
//
//go:generate stringer -type=AnimationType -output=render_animation_string.go
type AnimationType int

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

var IdleAnimations = map[AnimationType]AnimationType{
	AnimationMoveDown:  AnimationIdleDown,
	AnimationMoveLeft:  AnimationIdleLeft,
	AnimationMoveRight: AnimationIdleRight,
	AnimationMoveUp:    AnimationIdleUp,
}

func IdleAnimation(direction geometry.Vector2) AnimationType {
	return IdleAnimations[MoveAnimation(direction)]
}

var AttackAnimations = map[AnimationType]AnimationType{
	AnimationMoveDown:  AnimationAttackDown,
	AnimationMoveLeft:  AnimationAttackLeft,
	AnimationMoveRight: AnimationAttackRight,
	AnimationMoveUp:    AnimationAttackUp,
}

// AttackAnimation returns the attack animation matching the facing direction
// (for actors without a movement direction, defaults to down).
func AttackAnimation(direction geometry.Vector2) AnimationType {
	return AttackAnimations[MoveAnimation(direction)]
}
