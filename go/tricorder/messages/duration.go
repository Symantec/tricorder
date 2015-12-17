package messages

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"math"
	"time"
)

const (
	oneMillion = 1000000
)

func newDuration(d time.Duration) (result Duration) {
	result.Seconds = int64(d / time.Second)
	result.Nanoseconds = int32((d % time.Second) / time.Nanosecond)
	return
}

// SinceEpoch returns the amount of time since unix epoch
func sinceEpoch(t time.Time) (result Duration) {
	result.Seconds = t.Unix()
	result.Nanoseconds = int32(t.Nanosecond())
	if result.Seconds < 0 && result.Nanoseconds > 0 {
		result.Seconds++
		result.Nanoseconds -= 1000000000 // 1 billion
	}
	return
}

func sinceEpochFloat(f float64) (result Duration) {
	ii, ff := math.Modf(f)
	result.Seconds = int64(ii)
	result.Nanoseconds = int32(ff * 1e9)
	return
}

func (d Duration) asGoDuration() time.Duration {
	return time.Second*time.Duration(d.Seconds) + time.Duration(d.Nanoseconds)*time.Nanosecond
}

func (d Duration) asGoTime() time.Time {
	return time.Unix(d.Seconds, int64(d.Nanoseconds))
}

func (d Duration) stringUsingUnits(unit units.Unit) string {
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

func (d Duration) asFloat() float64 {
	return float64(d.Seconds) + float64(d.Nanoseconds)*1e-9
}

func (d Duration) isNegative() bool {
	return d.Nanoseconds < 0 || d.Seconds < 0
}

func (d Duration) prettyFormat() string {
	if d.isNegative() {
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
