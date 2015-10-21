// Package tricorder provides routines for clients to register metrics
// with the health system.
package tricorder

import (
	"errors"
)

// Unit represents an unit of measurement.
type Unit int

const (
	None Unit = iota
	Millisecond
	Second
	Celsius
)

var (
	// RegisterMetric returns this if given path is already in use.
	ErrPathInUse = errors.New("tricorder: Path in use")
)

func (u Unit) String() string {
	return ""
}

// RegisterMetric registers a single metric with the health system.
// path is the absolute path of the metric e.g "/proc/rpc"
// metric is the metric to register.
// metric can be a pointer to a primitive numeric type,
// a function of the form func() (foo NT, err error)
// where NT is a primitive numeric type, or a *Distribution.
// RegisterMetric panics if metric is not of a valid type.
// unit is the unit of measurement for the metric.
// description is the description of the metric.
// RegisterMetric returns an error if unsuccessful such as if path
// already represents a metric or a directory.
func RegisterMetric(
	path string,
	metric interface{},
	unit Unit,
	description string) error {
	return nil
}

// Bucketer represents the organization of buckets for Distribution
// instances. Multiple Distribution instances can share the same Bucketer
// instance.
type Bucketer struct {
}

// NewBucketerWithScale returns a Bucketer representing buckets on
// a geometric scale. NewBucketerWithScale(25, 3.0, 1.7) means 25 buckets
// starting with <3.0; 3.0 - 5.1; 5.1 - 8.67; 8.67 - 14.739 etc.
// NewBucketerWithScale panics if count < 2 or if stat <= 0 or if scale <= 1.
func NewBucketerWithScale(count int, stat, scale float64) *Bucketer {
	return nil
}

// NewBucketerWithEndpoints returns a Bucketer representing specific endpoints
// NewBucketerWithEndpoints([]float64{10.0, 20.0, 30.0}) means 4 buckets:
// <10.0; 10.0 - 20.0; 20.0 - 30.0; >= 30.0.
// NewBucketerWithEndpoints panics if len(endpoints) == 0.
func NewBucketerWithEndpoints(endpoints []float64) *Bucketer {
	return nil
}

// Distribution represents a metric that is a distribution of value.
type Distribution struct {
}

// NewDistribution creates a new Distribution that uses the given bucketer
// to distribute values.
func NewDistribution(bucketer *Bucketer) *Distribution {
	return nil
}

// Add adds a single value to a Distribution instance.
func (d *Distribution) Add(value float64) {
}

// DirectorySpec represents a specific directory in the heirarchy of
// metrics.
type DirectorySpec struct {
}

// RegisterDirectory returns the DirectorySpec for path.
// RegisterDirectory returns ErrPathInUse if path is already associated
// with a metric.
func RegisterDirectory(path string) (dirSpec *DirectorySpec, err error) {
	return nil, nil
}

// RegisterMetric works just like the package level RegisterMetric
// except that path is relative to this DirectorySpec.
func (d *DirectorySpec) RegisterMetric(
	path string,
	metric interface{},
	unit Unit,
	description string) error {
	return nil
}
