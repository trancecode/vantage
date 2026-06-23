package ui

import "testing"

func TestButtonContainsPoint(t *testing.T) {
	b := Button{
		X:      100,
		Y:      200,
		Width:  150,
		Height: 40,
	}

	tests := []struct {
		name     string
		x, y     float64
		expected bool
	}{
		{"inside", 150, 220, true},
		{"top-left corner", 100, 200, true},
		{"just outside left", 99, 220, false},
		{"just outside top", 150, 199, false},
		{"just outside right", 250, 220, false},
		{"just outside bottom", 150, 240, false},
		{"bottom-right edge", 249, 239, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.containsPoint(tt.x, tt.y)
			if got != tt.expected {
				t.Errorf("containsPoint(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.expected)
			}
		})
	}
}
