// Package units contains the various units of measurement for a metric.
package units

// Unit represents a unit of measurement
type Unit string

const (
	None          Unit = "None"
	Millisecond   Unit = "Milliseconds"
	Second        Unit = "Seconds"
	Celsius       Unit = "Celsius"
	Byte          Unit = "Bytes"
	BytePerSecond Unit = "BytesPerSecond"
)

// Returns the conversion factor between seconds and u.
// For example FromSeconds(Millisecond) returns 1000.
// Returns 1.0 if u is not a time unit.
func FromSeconds(u Unit) float64 {
	switch u {
	case Millisecond:
		return 1000.0
	default:
		return 1.0
	}
}
