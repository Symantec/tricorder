// Package units contains the various units of measurement for a metric.
package units

// Unit represents a unit of measurement
type Unit string

const (
	None        Unit = "None"
	Millisecond Unit = "Milliseconds"
	Second      Unit = "Seconds"
	Celsius     Unit = "Celsius"
	Byte        Unit = "Bytes"
)
