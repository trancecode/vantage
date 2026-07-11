//go:build !race

package render

import (
	"testing"

	"github.com/trancecode/vantage/geometry"
)

func TestMoveAnimation(t *testing.T) {
	tests := []struct {
		name     string
		from     geometry.Vector2
		to       geometry.Vector2
		expected AnimationType
	}{
		{
			name:     "No movement",
			from:     geometry.NewVector2(0, 0),
			to:       geometry.NewVector2(0, 0),
			expected: AnimationMoveDown,
		},
		{
			name:     "Move right",
			from:     geometry.NewVector2(0, 0),
			to:       geometry.NewVector2(1, 0),
			expected: AnimationMoveRight,
		},
		{
			name:     "Move left",
			from:     geometry.NewVector2(1, 0),
			to:       geometry.NewVector2(0, 0),
			expected: AnimationMoveLeft,
		},
		{
			name:     "Move down",
			from:     geometry.NewVector2(0, 0),
			to:       geometry.NewVector2(0, 1),
			expected: AnimationMoveDown,
		},
		{
			name:     "Move up",
			from:     geometry.NewVector2(0, 1),
			to:       geometry.NewVector2(0, 0),
			expected: AnimationMoveUp,
		},
		{
			name:     "Diagonal move (dominant X)",
			from:     geometry.NewVector2(0, 0),
			to:       geometry.NewVector2(2, 1),
			expected: AnimationMoveRight,
		},
		{
			name:     "Diagonal move (dominant Y)",
			from:     geometry.NewVector2(0, 0),
			to:       geometry.NewVector2(1, 2),
			expected: AnimationMoveDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MoveAnimation(tt.to.Sub(tt.from)); got != tt.expected {
				t.Errorf("MoveAnimation(%v, %v) = %v, expected %v", tt.from, tt.to, got, tt.expected)
			}
		})
	}
}

func TestDirectionalVariantResolvesGameDefinedStates(t *testing.T) {
	const (
		castDown AnimationType = AnimationGameBase + iota
		castRight
	)

	variants := map[AnimationType]AnimationType{
		AnimationMoveDown:  castDown,
		AnimationMoveRight: castRight,
	}

	if got := DirectionalVariant(geometry.NewVector2(0, 1), variants); got != castDown {
		t.Errorf("DirectionalVariant(down) = %v, expected castDown", got)
	}
	if got := DirectionalVariant(geometry.NewVector2(1, 0), variants); got != castRight {
		t.Errorf("DirectionalVariant(right) = %v, expected castRight", got)
	}
}

func TestDirectionalVariantWithoutEntryReturnsDefault(t *testing.T) {
	variants := map[AnimationType]AnimationType{AnimationMoveDown: AnimationIdleDown}

	if got := DirectionalVariant(geometry.NewVector2(0, -1), variants); got != AnimationDefault {
		t.Errorf("DirectionalVariant(up) = %v, expected AnimationDefault", got)
	}
}

func TestIdleAndAttackAnimationsUseDirectionalVariant(t *testing.T) {
	if got := IdleAnimation(geometry.NewVector2(-1, 0)); got != AnimationIdleLeft {
		t.Errorf("IdleAnimation(left) = %v, expected AnimationIdleLeft", got)
	}
	if got := AttackAnimation(geometry.NewVector2(0, -1)); got != AnimationAttackUp {
		t.Errorf("AttackAnimation(up) = %v, expected AnimationAttackUp", got)
	}
}
