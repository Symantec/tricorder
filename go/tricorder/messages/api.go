// Package messages provides the types needed to collect metrics via
// the go rpc calls and the REST API mentioned in the tricorder package.
package messages

import (
	"encoding/gob"
	"errors"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"time"
)

var (
	// The MetricServer.GetMetric RPC call returns this if no
	// metric with given path exists.
	ErrMetricNotFound = errors.New("messages: No metric found.")
)

// RangeWithCount represents the number of values within a
// particular range.
type RangeWithCount struct {
	// Represents the lower bound of the range inclusive.
	// Ignore for the lowest range which never has a lower bound.
	Lower float64 `json:"lower"`
	// Represents the upper bound of the range exclusive.
	// Ignore for the highest range which never has a upper bound.
	Upper float64 `json:"upper"`
	// The number of values falling within the range.
	Count uint64 `json:"count"`
}

// Distribution represents a distribution of values.
type Distribution struct {
	// The minimum value
	Min float64 `json:"min"`
	// The maximum value
	Max float64 `json:"max"`
	// The average value
	Average float64 `json:"average"`
	// The approximate median value
	Median float64 `json:"median"`
	// The sum
	Sum float64 `json:"sum"`
	// The total number of values
	Count uint64 `json:"count"`
	// The number of values within each range
	Ranges []*RangeWithCount `json:"ranges,omitempty"`
}

// Duration represents a duration of time
// For negative durations, both Seconds and Nanoseconds are negative.
// Internal use only for now.
type Duration struct {
	Seconds     int64
	Nanoseconds int32
}

func NewDuration(d time.Duration) Duration {
	return newDuration(d)
}

// SinceEpoch returns the amount of time since unix epoch
func SinceEpoch(t time.Time) Duration {
	return sinceEpoch(t)
}

// AsGoDuration converts this duration to a go duration
func (d Duration) AsGoDuration() time.Duration {
	return d.asGoDuration()
}

// AsGoTime Converts this duration to a go time.
// This is the inverse of SinceEpoch.
func (d Duration) AsGoTime() time.Time {
	return d.asGoTime()
}

// String shows in seconds
func (d Duration) String() string {
	return d.stringUsingUnits(units.Second)
}

// StringUsingUnits shows in specified time unit.
// If unit not a time, shows in seconds.
func (d Duration) StringUsingUnits(unit units.Unit) string {
	return d.stringUsingUnits(unit)
}

// IsNegative returns true if this duration is negative.
func (d Duration) IsNegative() bool {
	return d.isNegative()
}

// PrettyFormat pretty formats this duration.
// PrettyFormat panics if this duration is negative.
func (d Duration) PrettyFormat() string {
	return d.prettyFormat()
}

// Metric represents a single metric
// The type of the actual value stored in the Value field depends on the
// value of the Kind field.
//
// The chart below lists what type Value contains for each value of the
// Kind field:
//
// 	types.Bool	bool
//	types.Int	int64
//	types.Uint	uint64
//	types.Float	float64
//	types.String	string
//	types.Dist	*messages.Distribution
//	types.Time	string: Seconds since Jan 1, 1970 GMT. 9 digits after the decimal point.
//	types.Duration	string: Seconds with 9 digits after the decimal point.
//	types.GoTime	time.Time (Go RPC only)
//	types.GoDuration	time.Duration (Go RPC only)
type Metric struct {
	// The absolute path to this metric
	Path string `json:"path"`
	// The description of this metric
	Description string `json:"description"`
	// The unit of measurement this metric represents
	Unit units.Unit `json:"unit"`
	// The metric's type
	Kind types.Type `json:"kind"`
	// The size in bits of metric's value if Kind is
	// types.Int, types.Uint, or types.Float. This is size in bits used
	// on the process serving the metrics and may be smaller than the
	// number of bits used in the Value field of this struct.
	Bits int `json:"bits,omitempty"`
	// value stored here
	Value interface{} `json:"value"`
}

// IsJson returns true if this metric is json compatible
func (m *Metric) IsJson() bool {
	return m.isJson(false)
}

// ConvertToJson changes this metric in place to be json compatible.
func (m *Metric) ConvertToJson() {
	m.isJson(true)
}

// MetricList represents a list of metrics. Clients should treat MetricList
// instances as immutable. In particular, clients should not modify contained
// Metric instances in place.
type MetricList []*Metric

// AsJson returns a MetricList like this one that is Json compatible.
func (m MetricList) AsJson() MetricList {
	return m.asJson()
}

func init() {
	var tm time.Time
	var dur time.Duration
	var dist *Distribution
	gob.Register(tm)
	gob.Register(dur)
	gob.Register(dist)
}
