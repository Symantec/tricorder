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
// updated by a common function. Each time a client sends a request for one or
// more metrics backed by variables within a particular region, tricorder
// calls that region’s update function one time before reading any of the
// variables in that region to respond to the client. However, to provide
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
// RegisterMetric returns ErrPathInUse if path already represents a metric
// or a directory.
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
// the caller to specify the region to which the variable or callback function
// being registered belongs. RegisterMetricWithRegion ignores the region
// parameter when registering a distribution.
func RegisterMetricInRegion(
	path string,
	metric interface{},
	r *Region,
	unit units.Unit,
	description string) error {
	return root.registerMetric(newPathSpec(path), metric, (*region)(r), unit, description)
}

// UnregisterPath unregisters the metric or DirectorySpec at the given path.
// UnregisterPath ignores requests to unregister the root path.
func UnregisterPath(path string) {
	root.unregisterPath(newPathSpec(path))
}

// Bucketer represents the organization of values into buckets for
// distributions. Bucketer instances are immutable.
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
	return newBucketerFromEndpoints(
		newExponentialBucketerStream(count, start, scale))
}

// NewLinearBucketer returns a Bucketer representing bucktes on
// a linear scale. NewLinearBucketer(5, 0, 10) means 5 buckets
// starting with <0; 0-10; 10-20; 20-30; >=30.
// NewLinearBucketer panics if count < 2 or if increment <= 0.
func NewLinearBucketer(count int, start, increment float64) *Bucketer {
	return newBucketerFromEndpoints(
		newLinearBucketerStream(count, start, increment))
}

// NewArbitraryBucketer returns a Bucketer representing specific endpoints
// NewArbitraryBucketer(10.0, 20.0, 30.0) means 4 buckets:
// <10.0; 10.0 - 20.0; 20.0 - 30.0; >= 30.0.
// NewArbitraryBucketer panics if it is called with no arguments.
// It is the caller's responsibility to ensure that the arguments are in
// ascending order.
func NewArbitraryBucketer(endpoints ...float64) *Bucketer {
	return newBucketerFromEndpoints(endpoints)
}

// NewGeometricBucketer returns a Bucketer representing endpoints
// of the form 10^k, 2*10^k, 5*10^k. lower is the lower bound of
// the endpoints; upper is the upper bound of the endpoints.
// NewGeometricBucker(0.5, 50) ==>
// <0.5; 0.5-1; 1-2; 2-5; 5-10; 10-20; 20-50; >=50
func NewGeometricBucketer(lower, upper float64) *Bucketer {
	return newBucketerFromEndpoints(
		newGeometricBucketerStream(lower, upper))
}

// NewCumulativeDistribution creates a new CumulativeDistribution that uses
// this bucketer to distribute values.
func (b *Bucketer) NewCumulativeDistribution() *CumulativeDistribution {
	return (*CumulativeDistribution)(newDistribution(b, false))
}

// NewNonCumulativeDistribution creates a new NonCumulativeDistribution that
// uses this bucketer to distribute values.
func (b *Bucketer) NewNonCumulativeDistribution() *NonCumulativeDistribution {
	return (*NonCumulativeDistribution)(newDistribution(b, true))
}

// CumulativeDistribution represents a metric that is a distribution of
// values. Cumulative distributions only receive new values.
type CumulativeDistribution distribution

// Add adds a single value to this CumulativeDistribution instance.
// value can be a float32, float64, or a time.Duration.
// If a time.Duration, Add converts it to the same unit of time specified in
// the RegisterMetric call made to register this instance.
func (c *CumulativeDistribution) Add(value interface{}) {
	(*distribution)(c).Add(value)
}

// Snapshot fetches a snapshot of this instance
func (c *CumulativeDistribution) Snapshot() *Snapshot {
	return nil
}

// Unlike in CumulativeDistributions,values in NonCumulativeDistributions
// can change shifting from bucket to bucket.
type NonCumulativeDistribution distribution

// Add adds a single value to this NonCumulativeDistribution instance.
// value can be a float32, float64, or a time.Duration.
// If a time.Duration, Add converts it to the same unit of time specified in
// the RegisterMetric call made to register this instance.
func (c *NonCumulativeDistribution) Add(value interface{}) {
	(*distribution)(c).Add(value)
}

// Update updates a value in this NonCumulativeDistribution instance.
// oldValue and newValue can be a float32, float64, or a time.Duration.
// If a time.Duration, Update converts them to the same unit of time specified
// in the RegisterMetric call made to register this instance.
// The reliability of Update() depends on the caller providing the correct
// old value of what is being changed. Failure to do this results in
// undefined behavior.
// Update updates all distribution statistics in the expected way; however,
// it updates min and max such that min only gets smaller and max only gets
// larger. If update Updates values such that they fall into a narrower
// range than before, min and max remain unchanged to indicate the all-time
// min and all-time max. To have min and max reflect the current min and max
// instead of the all-time min and max, see UpdateMinMax().
func (d *NonCumulativeDistribution) Update(oldValue, newValue interface{}) {
	(*distribution)(d).Update(oldValue, newValue)
}

// UpdateMinMax() estimates the current min and max of this distribution.
// and updates min and max accordingly.
// As this call may be expensive, clients need not use unless both are
// true:
// 1) the client has made calls to Update which narrowed the current
// min and max.
// 2) The client wants min and max to reflect the current min and max
// instead of the all-time min and max.
// This method only estimates. The only guarantees that it makes
// upon returning are:
// original_min <= min <= current_min and old_max >= max >= current_max.
// In fact, calling this method may do nothing at all which would still be
// correct behavior.
func (d *NonCumulativeDistribution) UpdateMinMax() {
}

// Remove removes a value from this NonCumulativeDistribution instance.
// valueToBeRemoved can be a float32, float64, or a time.Duration.
// If a time.Duration, Remove converts it to the same unit of time
// specified in the RegisterMetric call made to register this instance.
// The reliability of Remove() depends on the caller providing a value
// already in the distribution. Failure to do this results in
// undefined behavior.
// Remove updates all distribution statistics in the expected way; however,
// it leaves min and max unchanged.
// To have min and max reflect the current min and max
// instead of the all-time min and max, see UpdateMinMax().
func (d *NonCumulativeDistribution) Remove(valueToBeRemoved interface{}) {
	(*distribution)(d).Remove(valueToBeRemoved)
}

// Sum returns the sum of the values in this distribution.
func (d *NonCumulativeDistribution) Sum() float64 {
	return (*distribution)(d).Sum()
}

// Count returns the number of values in this distribution
func (d *NonCumulativeDistribution) Count() uint64 {
	return (*distribution)(d).Count()
}

// Snapshot fetches a snapshot of this instance
func (d *NonCumulativeDistribution) Snapshot() *Snapshot {
	return nil
}

// Snapshot represents a snapshot of a cumulative or non-cumulative distribution
// Snapshot instances are immutable.
type Snapshot struct {
	s snapshot
}

// Sum returns the sum of the values
func (s *Snapshot) Sum() float64 {
	return 0.0
}

// Count returns the number of values
func (s *Snapshot) Count() uint64 {
	return 0
}

// Median returns the estimated median of the values
func (s *Snapshot) Median() float64 {
	return 0.0
}

// Average returns the average of the values
func (s *Snapshot) Average() float64 {
	return 0.0
}

// Min returns the minimum value.
// Min is accurate only if the distribution is cumulative.
func (s *Snapshot) Min() float64 {
	return 0.0
}

// Max returns the maximum value.
// Man is accurate only if the distribution is cumulative.
func (s *Snapshot) Max() float64 {
	return 0.0
}

// DirectorySpec represents a specific directory in the heirarchy of
// metrics.
type DirectorySpec directory

// RegisterDirectory returns the the DirectorySpec registered with path.
// If nothing is registered with path, RegisterDirectory registers a
// new DirectorySpec with path and returns it.
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

// UnregisterPath works just like the package level UnregisterPath
// except that path is relative to this DirectorySpec.
func (d *DirectorySpec) UnregisterPath(path string) {
	(*directory)(d).unregisterPath(newPathSpec(path))
}

// UnregisterDirectory unregisters this DirectorySpec instance along with
// all metrics and directories within it. The caller can unregister any
// DirectorySpec instance except the one representing the top level directory.
// That DirectorySpec instance simply ignores calls to UnregisterDirectory.
// Using an unregistered DirectorySpec instance to register new metrics may
// cause a panic.
func (d *DirectorySpec) UnregisterDirectory() {
	(*directory)(d).unregisterDirectory()
}

// RegisterFlags registers each application flag as a metric under /proc/flags.
func RegisterFlags() {
	registerFlags()
}

// SetFlagUnit sets the unit for a specific flag. If the flag is a
// time.Duration, the default unit is units.Second; otherwise the default
// unit is units.None. If the client wishes to override the default unit for
// a flag, they call this after registering a flag but before calling
// ParseFlags.
func SetFlagUnit(name string, unit units.Unit) {
	setFlagUnit(name, unit)
}
