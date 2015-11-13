package messages

import (
	"fmt"
	"time"
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

func (d Duration) asGoDuration() time.Duration {
	return time.Second*time.Duration(d.Seconds) + time.Duration(d.Nanoseconds)*time.Nanosecond
}

func (d Duration) AsSeconds() float64 {
	return float64(d.Seconds) + float64(d.Nanoseconds)*1e-9
}

func (d Duration) asGoTime() time.Time {
	return time.Unix(d.Seconds, int64(d.Nanoseconds))
}

func (d Duration) _string() string {
	formattedNs := d.Nanoseconds
	if formattedNs < 0 {
		formattedNs = -formattedNs
	}
	return fmt.Sprintf("%d.%09d", d.Seconds, formattedNs)
}
