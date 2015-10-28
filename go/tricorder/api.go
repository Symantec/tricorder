// Package tricorder provides routines for clients to register metrics
// with the health system.
// This package also uses the go net/http package to register the web
// UI of the health system at path "/tricorder"
// This package also registers static content such as css pages at
// "/tricorderstatic/".
// For now, clients of this package must start an http server within their
// application using the net/http package to make this web UI accessible.
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

// NewUnit returns a Unit from its string representation. It is the inverse
// of String(). If str is not the string representation of a valid Unit,
// returns None and a non-nil error.
func NewUnit(str string) (Unit, error) {
	return newUnit(str)
}

func (u Unit) String() string {
	return u._string()
}

// RegisterMetric registers a single metric with the health system.
// path is the absolute path of the metric e.g "/proc/rpc"
// metric is the metric to register.
// metric can be a pointer to a primitive numeric type or string,
// a function of the form func() NT where NT is a primitive numeric type
// or string, or finally metric can be a *Distribution.
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
	return root.registerMetric(newPathSpec(path), metric, unit, description)
}

// Bucketer represents the organization of buckets for Distribution
// instances. Multiple Distribution instances can share the same Bucketer
// instance.
type Bucketer struct {
	pieces []*bucketPiece
}

// NewBucketerWithScale returns a Bucketer representing buckets on
// a geometric scale. NewBucketerWithScale(25, 3.0, 1.7) means 25 buckets
// starting with <3.0; 3.0 - 5.1; 5.1 - 8.67; 8.67 - 14.739 etc.
// NewBucketerWithScale panics if count < 2 or if start <= 0 or if scale <= 1.
func NewBucketerWithScale(count int, start, scale float64) *Bucketer {
	return newBucketerWithScale(count, start, scale)
}

// NewBucketerWithEndpoints returns a Bucketer representing specific endpoints
// NewBucketerWithEndpoints([]float64{10.0, 20.0, 30.0}) means 4 buckets:
// <10.0; 10.0 - 20.0; 20.0 - 30.0; >= 30.0.
// NewBucketerWithEndpoints panics if len(endpoints) == 0.
// It is the caller's responsibility to ensure that the values in the
// endpoints slice are in ascending order.
func NewBucketerWithEndpoints(endpoints []float64) *Bucketer {
	return newBucketerWithEndpoints(endpoints)
}

// Distribution represents a metric that is a distribution of value.
type Distribution distribution

// NewDistribution creates a new Distribution that uses the given bucketer
// to distribute values.
func NewDistribution(bucketer *Bucketer) *Distribution {
	return (*Distribution)(newDistribution(bucketer))
}

// Add adds a single value to a Distribution instance.
func (d *Distribution) Add(value float64) {
	(*distribution)(d).Add(value)
}

// DirectorySpec represents a specific directory in the heirarchy of
// metrics.
type DirectorySpec directory

// RegisterDirectory returns the DirectorySpec for path.
// RegisterDirectory returns ErrPathInUse if path is already associated
// with a metric.
func RegisterDirectory(path string) (dirSpec *DirectorySpec, err error) {
	r, e := root.registerDirectory(newPathSpec(path))
	return (*DirectorySpec)(r), e
}

// RegisterMetric works just like the package level RegisterMetric
// except that path is relative to this DirectorySpec.
func (d *DirectorySpec) RegisterMetric(
	path string,
	metric interface{},
	unit Unit,
	description string) error {
	return (*directory)(d).registerMetric(newPathSpec(path), metric, unit, description)
}

// RegisterDirectory works just like the package level RegisterDirectory
// except that path is relative to this DirectorySpec.
func (d *DirectorySpec) RegisterDirectory(
	path string) (dirSpec *DirectorySpec, err error) {
	r, e := (*directory)(d).registerDirectory(newPathSpec(path))
	return (*DirectorySpec)(r), e
}

// Returns the absolute path this object represents
func (d *DirectorySpec) AbsPath() string {
	return (*directory)(d).AbsPath()
}
