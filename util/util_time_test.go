package util

import (
	"testing"
	"time"
)

func TestDurationString(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, ""},
		{time.Second, "1s"},
		{2 * time.Second, "2s"},
		{time.Minute, "1m"},
		{2 * time.Minute, "2m"},
		{time.Hour, "1h"},
		{5 * time.Second, "5s"},
		{5 * time.Minute, "5m"},
		{5 * time.Hour, "5h"},
		{time.Minute + time.Second, "1m1s"},
		{time.Hour + time.Second, "1h1s"},
		{time.Hour + time.Minute, "1h1m"},
		{time.Hour + time.Minute + time.Second, "1h1m1s"},
	}
	for _, test := range tests {
		actual := DurationString(test.duration)
		if actual != test.expected {
			t.Errorf("DurationString(%v) = %v, expected %v", test.duration, actual, test.expected)
		}
	}
}
