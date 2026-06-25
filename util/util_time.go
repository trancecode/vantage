package util

import (
	"fmt"
	"time"
)

// DurationString returns a string representation of a duration in the format "12h34m56s".
func DurationString(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000
	/*
		microseconds := int(d.Microseconds()) % 1000
		nanoseconds := int(d.Nanoseconds()) % 1000
	*/

	result := ""

	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 {
		result += fmt.Sprintf("%ds", seconds)
	}
	if milliseconds > 0 {
		result += fmt.Sprintf("%dms", milliseconds)
	}
	/*
		if microseconds > 0 {
			result += fmt.Sprintf("%dµs", microseconds)
		}
		if nanoseconds > 0 {
			result += fmt.Sprintf("%dns", nanoseconds)
		}
	*/

	return result
}

type durationStringer struct {
	d time.Duration
}

func (d *durationStringer) String() string {
	return DurationString(d.d)
}

// DurationStringer returns a fmt.Stringer that formats d using DurationString.
func DurationStringer(d time.Duration) fmt.Stringer {
	return &durationStringer{d: d}
}

// Time is used as the in-game time type.
// It is a wrapper around time.Duration to provide a more readable string representation.
// It allows for easy addition and subtraction of durations, making it suitable for game time management.
type Time time.Duration

func (t Time) String() string {
	if t == 0 {
		return "0s"
	}
	return DurationString(time.Duration(t))
}

// Add adds a duration to the Time instance and returns a new Time instance.
func (t Time) Add(d time.Duration) Time {
	return Time(time.Duration(t) + d)
}

// Sub returns the duration elapsed from other to t.
func (t Time) Sub(other Time) time.Duration {
	return time.Duration(t) - time.Duration(other)
}
