package tricorder

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"time"
)

const (
	oneMillion = 1000000
)

// duration represents a duration of time
// For negative durations, both Seconds and Nanoseconds are negative.
type duration struct {
	Seconds     int64
	Nanoseconds int32
}

func newDuration(d time.Duration) (result duration) {
	result.Seconds = int64(d / time.Second)
	result.Nanoseconds = int32((d % time.Second) / time.Nanosecond)
	return
}

// sinceEpoch returns the amount of time since unix epoch
func durationSinceEpoch(t time.Time) (result duration) {
	result.Seconds = t.Unix()
	result.Nanoseconds = int32(t.Nanosecond())
	if result.Seconds < 0 && result.Nanoseconds > 0 {
		result.Seconds++
		result.Nanoseconds -= 1000000000 // 1 billion
	}
	return
}

// AsGoDuration converts this duration to a go duration
func (d duration) AsGoDuration() time.Duration {
	return time.Second*time.Duration(d.Seconds) + time.Duration(d.Nanoseconds)*time.Nanosecond
}

// AsGoTime Converts this duration to a go time.
// This is the inverse of SinceEpoch.
func (d duration) AsGoTime() time.Time {
	return time.Unix(d.Seconds, int64(d.Nanoseconds))
}

// String shows in seconds
func (d duration) String() string {
	return d.StringUsingUnits(units.Second)
}

// StringUsingUnits shows in specified time unit.
// If unit not a time, shows in seconds.
func (d duration) StringUsingUnits(unit units.Unit) string {
	formattedNs := d.Nanoseconds
	if formattedNs < 0 {
		formattedNs = -formattedNs
	}
	switch unit {
	case units.Millisecond:
		return fmt.Sprintf(
			"%d%03d.%06d",
			d.Seconds,
			formattedNs/oneMillion,
			formattedNs%oneMillion)
	default: // second
		return fmt.Sprintf("%d.%09d", d.Seconds, formattedNs)
	}

}

// IsNegative returns true if this duration is negative.
func (d duration) IsNegative() bool {
	return d.Nanoseconds < 0 || d.Seconds < 0
}

// PrettyFormat pretty formats this duration.
// PrettyFormat panics if this duration is negative.
func (d duration) PrettyFormat() string {
	if d.IsNegative() {
		panic("Cannot pretty format negative durations")
	}
	switch {
	case d.Seconds == 0 && d.Nanoseconds < 10000:
		return fmt.Sprintf("%dns", d.Nanoseconds)
	case d.Seconds == 0 && d.Nanoseconds < 10000000:
		return fmt.Sprintf("%dμs", d.Nanoseconds/1000)
	case d.Seconds == 0:
		return fmt.Sprintf("%dms", d.Nanoseconds/1000000)
	case d.Seconds < 60:
		return fmt.Sprintf("%d.%03ds", d.Seconds, d.Nanoseconds/1000000)
	case d.Seconds < 60*60:
		return fmt.Sprintf(
			"%dm %d.%03ds",
			d.Seconds/60,
			d.Seconds%60,
			d.Nanoseconds/1000000)
	case d.Seconds < 24*60*60:
		return fmt.Sprintf(
			"%dh %dm %ds",
			d.Seconds/(60*60),
			(d.Seconds%(60*60))/60,
			d.Seconds%60)
	default:
		return fmt.Sprintf(
			"%dd %dh %dm %ds",
			d.Seconds/(24*60*60),
			(d.Seconds%(24*60*60))/(60*60),
			(d.Seconds%(60*60))/60,
			d.Seconds%60)

	}
}
