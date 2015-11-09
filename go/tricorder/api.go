package tricorder

import (
	"errors"
	"github.com/Symantec/tricorder/go/tricorder/units"
)

var (
	// RegisterMetric returns this if given path is already in use.
	ErrPathInUse = errors.New("tricorder: Path in use")
)

// A region represents a collection of variables for metrics that are all
// updated by a common function. When a client sends a request for one or
// more metrics backed by variables within a particular region, tricorder
// calls that region’s update function one time before reading any of the
// variables in that region to to respond to the client. However, to provide
// a consistent view of the variables within a region, tricorder will never
// call a region’s update function once it has begun reading variables in that
// region to service an in-process request.  If tricorder does happen to
// receive an incoming request for metrics from a given region after tricorder
// has begun reading variables in that same region to service another
// in-process request, tricorder will skip calling the region’s update
// function for the incoming request. In this case, the two requests will
// read the same data from that region.
type Region region

// NewRegion creates a new region with a particular update function
func RegisterRegion(updateFunc func()) *Region {
	return (*Region)(newRegion(updateFunc))
}

// RegisterMetric registers a single metric with the health system.
// path is the absolute path of the metric e.g "/proc/rpc";
// metric is the metric to register;
// unit is the unit of measurement for the metric;
// description is the description of the metric.
// RegisterMetric returns an error if unsuccessful such as if path
// already represents a metric or a directory.
// RegisterMetric panics if metric is not of a valid type.
func RegisterMetric(
	path string,
	metric interface{},
	unit units.Unit,
	description string) error {
	return root.registerMetric(
		newPathSpec(path), metric, nil, unit, description)
}

// RegisterMetricWithRegion works just like RegisterMetrics but allows
// the caller to specify the region to which the variable being registered
// belongs. RegisterMetricWithRegion ignores the region parameter when
// registering a callback function or distribution.
func RegisterMetricInRegion(
	path string,
	metric interface{},
	r *Region,
	unit units.Unit,
	description string) error {
	return root.registerMetric(newPathSpec(path), metric, (*region)(r), unit, description)
}

// Bucketer represents the organization of buckets for Distribution
// instances. Because bucketer instances are immutable, multiple distribution
// instances can share the same Bucketer instance.
type Bucketer struct {
	pieces []*bucketPiece
}

var (
	// Ranges in powers of two
	PowersOfTwo = NewExponentialBucketer(20, 1.0, 2.0)
	// Ranges in powers of four
	PowersOfFour = NewExponentialBucketer(11, 1.0, 4.0)
	// Ranges in powers of 10
	PowersOfTen = NewExponentialBucketer(7, 1.0, 10.0)
)

// NewExponentialBucketer returns a Bucketer representing buckets on
// a geometric scale. NewExponentialBucketer(25, 3.0, 1.7) means 25 buckets
// starting with <3.0; 3.0 - 5.1; 5.1 - 8.67; 8.67 - 14.739 etc.
// NewExponentialBucketer panics if count < 2 or if start <= 0 or if scale <= 1.
func NewExponentialBucketer(count int, start, scale float64) *Bucketer {
	return newBucketerFromStream(
		newExponentialBucketerStream(count, start, scale))
}

// NewLinearBucketer returns a Bucketer representing bucktes on
// a linear scale. NewLinearBucketer(5, 0, 10) means 5 buckets
// starting with <0; 0-10; 10-20; 20-30; >=30.
// NewLinearBucketer panics if count < 2 or if increment <= 0.
func NewLinearBucketer(count int, start, increment float64) *Bucketer {
	return newBucketerFromStream(
		newLinearBucketerStream(count, start, increment))
}

// NewArbitraryBucketer returns a Bucketer representing specific endpoints
// NewArbitraryBucketer([]float64{10.0, 20.0, 30.0}) means 4 buckets:
// <10.0; 10.0 - 20.0; 20.0 - 30.0; >= 30.0.
// NewArbitraryBucketer panics if len(endpoints) == 0.
// It is the caller's responsibility to ensure that the values in the
// endpoints slice are in ascending order.
func NewArbitraryBucketer(endpoints []float64) *Bucketer {
	return newBucketerFromStream(
		newArbitraryBucketerStream(endpoints))
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
	unit units.Unit,
	description string) error {
	return (*directory)(d).registerMetric(newPathSpec(path), metric, nil, unit, description)
}

// RegisterMetricWithRegion works just like the package level
// RegisterMetricWithRegion except that path is relative to this
// DirectorySpec.
func (d *DirectorySpec) RegisterMetricInRegion(
	path string,
	metric interface{},
	r *Region,
	unit units.Unit,
	description string) error {
	return (*directory)(d).registerMetric(newPathSpec(path), metric, (*region)(r), unit, description)
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
