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

// Metric represents a single metric
type Metric struct {
	// The absolute path to this metric
	Path string `json:"path"`
	// The description of this metric
	Description string `json:"description"`
	// The unit of measurement this metric represents
	Unit units.Unit `json:"unit"`
	// The metric's type
	Kind types.Type `json:"kind"`
	// The size in bits of metric's value if Int, Uint, or float
	Bits int `json:"bits,omitempty"`
	// value stored here
	Value interface{} `json:"value"`
}

// MetricList represents a list of metrics.
type MetricList []*Metric

func init() {
	var tm time.Time
	var dur time.Duration
	var dist *Distribution
	gob.Register(tm)
	gob.Register(dur)
	gob.Register(dist)
}
