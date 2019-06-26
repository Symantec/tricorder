// Package units contains the various units of measurement for a metric.
package units

// Unit represents a unit of measurement
type Unit string

const (
	Unknown       Unit = ""
	None          Unit = "None"
	Millisecond   Unit = "Milliseconds"
	Second        Unit = "Seconds"
	Celsius       Unit = "Celsius"
	Byte          Unit = "Bytes"
	BytePerSecond Unit = "BytesPerSecond"
)

func (u Unit) String() string {
	if u == Unknown {
		return "Unknown"
	}
	return string(u)
}

// FromSeconds returns the conversion factor between seconds and u.
// For example FromSeconds(Millisecond) returns 1000.
// Returns 1.0 if u is not a time unit.
func FromSeconds(u Unit) float64 {
	n, d := FromSecondsRational(u)
	return float64(n) / float64(d)
}

// FromSecondsRational is the same as FromSeconds but returns answer as a
// rational number.
func FromSecondsRational(u Unit) (numerator, denominator int64) {
	switch u {
	case Millisecond:
		return 1000, 1
	default:
		return 1, 1
	}
}
