// Package messages provides the types needed to collect metrics via
// the go rpc calls or the REST API mentioned in the tricorder package.
package messages

import (
	"errors"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
)

var (
	// The MetricServer.GetMetric RPC call returns this if no
	// metric with given path exists.
	ErrMetricNotFound = errors.New("messages: No metric found.")
)

// RangeWithCount represents the number of values within a particular range
type RangeWithCount struct {
	// If non nil, represents the lower bound of the range inclusive.
	// nil means no lower bound
	Lower *float64 `json:"lower,omitempty"`
	// If non nil, represents the upper bound of the range exclusive.
	// nil means no upper bound
	Upper *float64 `json:"upper,omitempty"`
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
	// The total number of values
	Count uint64 `json:"count"`
	// The number of values within each range
	Ranges []*RangeWithCount `json:"ranges,omitempty"`
}

// Value represents the value of a metric.
type Value struct {
	// The value's type
	Kind types.Type `json:"kind"`
	// bool values stored here
	BoolValue *bool `json:"boolValue,omitempty"`
	// int values stored here
	IntValue *int64 `json:"intValue,omitempty"`
	// uint values stored here
	UintValue *uint64 `json:"uintValue,omitempty"`
	// float values stored here
	FloatValue *float64 `json:"floatValue,omitempty"`
	// string values are stored here. Also time values are stored here
	// as seconds after Jan 1, 1970 GMT in this format:
	// 1234567890.987654321
	StringValue *string `json:"stringValue,omitempty"`
	// Distributions stored here
	DistributionValue *Distribution `json:"distributionValue,omitempty"`
}

// Metric represents a single metric
type Metric struct {
	// The absolute path to this metric
	Path string `json:"path"`
	// The description of this metric
	Description string `json:"description"`
	// The unit of measurement this metric represents
	Unit units.Unit `json:"unit"`
	// The value of this metric
	Value *Value `json:"value"`
}

// MetricList represents a list of metrics.
type MetricList []*Metric
