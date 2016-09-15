package duration

import (
	"errors"
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	oneBillion = 1000000000
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
		result.Nanoseconds -= oneBillion
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

func (d Duration) toString() string {
	formattedNs := d.Nanoseconds
	if formattedNs < 0 {
		formattedNs = -formattedNs
	}
	return fmt.Sprintf("%d.%09d", d.Seconds, formattedNs)
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

func (d Duration) mult(scalar int64) (result Duration) {
	fracprod := int64(d.Nanoseconds) * scalar
	wholeprod := d.Seconds*scalar + fracprod/oneBillion
	fracprod = fracprod % oneBillion
	result.Seconds = wholeprod
	result.Nanoseconds = int32(fracprod)
	return
}

func (d Duration) div(scalar int64) (result Duration) {
	wholequot := d.Seconds / scalar
	wholerem := d.Seconds % scalar
	fracquot := (int64(d.Nanoseconds) + wholerem*oneBillion) / scalar
	result.Seconds = wholequot
	result.Nanoseconds = int32(fracquot)
	return
}

func (d Duration) convert(from, to units.Unit) Duration {
	fromNum, fromDen := units.FromSecondsRational(from)
	toNum, toDen := units.FromSecondsRational(to)
	num := toNum * fromDen
	den := toDen * fromNum
	return d.mult(num).div(den)
}

func parse(str string) (result Duration, err error) {
	var bNegative bool
	if strings.HasPrefix(str, "-") {
		bNegative = true
		str = str[1:]
	}
	number := strings.SplitN(str, ".", 2)
	whole, err := strconv.ParseInt(number[0], 10, 64)
	if err != nil {
		return
	}
	if whole < 0 {
		err = errors.New("Double negative while parsing")
		return
	}
	var frac uint64
	if len(number) == 2 {
		fracStr := number[1]
		if len(fracStr) < 9 {
			fracStr = fracStr + strings.Repeat("0", 9-len(fracStr))
		} else {
			fracStr = fracStr[:9]
		}
		frac, err = strconv.ParseUint(fracStr, 10, 32)
		if err != nil {
			return
		}
	}
	if frac > 999999999 {
		err = errors.New("Unexpected error")
		return
	}
	if bNegative {
		result.Seconds = -1 * whole
		result.Nanoseconds = -1 * int32(frac)
	} else {
		result.Seconds = whole
		result.Nanoseconds = int32(frac)
	}
	return
}
