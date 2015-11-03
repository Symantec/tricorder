package tricorder

import (
	"errors"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"testing"
	"time"
)

var (
	kCallbackError = errors.New("callback error")
)

func TestAPI(t *testing.T) {
	// /proc/rpc-latency: Distribution millis 5 buckets, start: 10, scale 2.5
	// /proc/rpc-count: Callback to get RPC count, uint64
	// /proc/start-time: An int64 showing start time as seconds since epoch
	// /proc/temperature: A float64 showing tempurature in celsius
	// /proc/foo/bar/baz: Callback to get a float64 that returns an error
	// /testname - name of app
	// /testargs - A string arguments to app

	var startTime int64
	var temperature float64
	var unused int64
	var name, args string
	var someTime time.Time
	var someTimePtr *time.Time
	var someBool bool

	rpcBucketer := NewExponentialBucketer(6, 10, 2.5)
	rpcDistribution := NewDistribution(rpcBucketer)

	if err := RegisterMetric("/proc/rpc-latency", rpcDistribution, units.Millisecond, "RPC latency"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/rpc-count", rpcCountCallback, units.None, "RPC count"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/start-time", &startTime, units.Second, "Start Time"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/some-time", &someTime, units.None, "Some time"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/some-time-ptr", &someTimePtr, units.None, "Some time pointer"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/temperature", &temperature, units.Celsius, "Temperature"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/testname", &name, units.None, "Name of app"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric("/testargs", &args, units.None, "Args passed to app"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	fooDir, err := RegisterDirectory("proc/foo")
	if err != nil {
		t.Fatalf("Got error %v registering directory", err)
	}
	barDir, err := fooDir.RegisterDirectory("bar")
	if err != nil {
		t.Fatalf("Got error %v registering directory", err)
	}
	err = barDir.RegisterMetric("baz", bazCallback, units.None, "An error")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	err = barDir.RegisterMetric("abool", &someBool, units.None, "A boolean value")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	err = barDir.RegisterMetric("anotherBool", boolCallback, units.None, "A boolean callback value")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	// This is already a directory
	if err := RegisterMetric("/proc/foo/bar", &unused, units.None, "Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already registered as a metric
	if err := RegisterMetric("proc/foo/bar/baz", &unused, units.None, "Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already a metric and can't be a directory
	if _, err := fooDir.RegisterDirectory("/bar/baz"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric on an empty path
	if err := RegisterMetric("/", &unused, units.None, "Empty path"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric where parent dir already a metric
	if err := RegisterMetric("/args/illegal", &unused, units.None, "parent dir already a metric"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This succeeds because /proc/foo is a directory
	if existing, err := RegisterDirectory("/proc/foo"); existing != fooDir || err != nil {
		t.Error("RegisterDirectory methods should be idempotent.")
	}

	// Now set the variables to actual values.
	// No calls needed to update tricorder
	name = "My application"
	args = "--help"
	startTime = -1234567
	temperature = 22.5
	someBool = true

	someTime = time.Date(2015, time.November, 15, 13, 26, 53, 7265341, time.UTC)

	// Add data points to the distribution
	// < 10: 10
	// 10 - 25: 15
	// 25 - 62.5: 38
	// 62.5 - 156.25: 94
	// 156.25 - 390.625: 234
	for i := 0; i < 500; i++ {
		rpcDistribution.Add(float64(i))
	}

	verifyChildren(
		t,
		root.List(),
		"args",
		"name",
		"proc",
		"start-time",
		"testargs",
		"testname")
	verifyChildren(
		t,
		root.GetDirectory("proc").List(),
		"foo",
		"rpc-count",
		"rpc-latency",
		"some-time",
		"some-time-ptr",
		"start-time",
		"temperature")
	verifyGetAllMetrics(
		t,
		"foo/bar/baz",
		root.GetDirectory("proc"),
		"/proc/foo/bar/baz")
	verifyGetAllMetrics(
		t,
		"ddd",
		root.GetDirectory("proc"))
	verifyGetAllMetrics(
		t,
		"proc",
		root,
		"/proc/foo/bar/abool",
		"/proc/foo/bar/anotherBool",
		"/proc/foo/bar/baz",
		"/proc/rpc-count",
		"/proc/rpc-latency",
		"/proc/some-time",
		"/proc/some-time-ptr",
		"/proc/start-time",
		"/proc/temperature")
	verifyChildren(
		t, root.GetDirectory("proc/foo/bar").List(), "abool", "anotherBool", "baz")

	// try path's that don't exist
	if root.GetDirectory("/testargs/foo") != nil {
		t.Error("/testargs/foo shouldn't exist")
	}
	if root.GetMetric("/testargs/foo") != nil {
		t.Error("/testargs/foo shouldn't exist")
	}
	if root.GetDirectory("/big/small/little") != nil {
		t.Error("/big/small/little shouldn't exist")
	}
	if root.GetMetric("/big/small/little") != nil {
		t.Error("/big/small/little shouldn't exist")
	}
	if root.GetDirectory("/proc/big/small") != nil {
		t.Error("/proc/big/small shouldn't exist")
	}
	if root.GetMetric("/proc/big/small") != nil {
		t.Error("/proc/big/small shouldn't exist")
	}
	if root.GetMetric("/") != nil {
		t.Error("/ metric shouldn't exist")
	}

	// Check /testargs
	argsMetric := root.GetMetric("/testargs")
	verifyMetric(t, "Args passed to app", units.None, argsMetric)
	assertValueEquals(t, types.String, argsMetric.Value.Type())
	assertValueEquals(t, "--help", argsMetric.Value.AsString())
	assertValueEquals(t, "\"--help\"", argsMetric.Value.AsHtmlString())

	// Check /testname
	nameMetric := root.GetMetric("/testname")
	verifyMetric(t, "Name of app", units.None, nameMetric)
	assertValueEquals(t, types.String, nameMetric.Value.Type())
	assertValueEquals(t, "My application", nameMetric.Value.AsString())
	assertValueEquals(t, "\"My application\"", nameMetric.Value.AsHtmlString())

	// Check /proc/temperature
	temperatureMetric := root.GetMetric("/proc/temperature")
	verifyMetric(t, "Temperature", units.Celsius, temperatureMetric)
	assertValueEquals(t, types.Float, temperatureMetric.Value.Type())
	assertValueEquals(t, 22.5, temperatureMetric.Value.AsFloat())
	assertValueEquals(t, "22.5", temperatureMetric.Value.AsHtmlString())

	// Check /proc/start-time
	startTimeMetric := root.GetMetric("/proc/start-time")
	verifyMetric(t, "Start Time", units.Second, startTimeMetric)
	assertValueEquals(t, types.Int, startTimeMetric.Value.Type())
	assertValueEquals(t, int64(-1234567), startTimeMetric.Value.AsInt())
	assertValueEquals(t, "-1234567", startTimeMetric.Value.AsHtmlString())

	// Check /proc/some-time
	someTimeMetric := root.GetMetric("/proc/some-time")
	verifyMetric(t, "Some time", units.None, someTimeMetric)
	assertValueEquals(t, types.Time, someTimeMetric.Value.Type())
	assertValueEquals(t, "1447594013.007265341", someTimeMetric.Value.AsTextString())
	someTime = time.Date(2015, time.November, 15, 13, 26, 53, 7265341, time.UTC)
	assertValueEquals(t, "2015-11-15T13:26:53.007265341Z", someTimeMetric.Value.AsHtmlString())

	// Check /proc/some-time-ptr
	someTimePtrMetric := root.GetMetric("/proc/some-time-ptr")
	verifyMetric(t, "Some time pointer", units.None, someTimePtrMetric)
	assertValueEquals(t, types.Time, someTimePtrMetric.Value.Type())
	var zeroTime time.Time
	assertValueEquals(t, zeroTime, someTimePtrMetric.Value.AsTime())

	newTime := time.Date(2015, time.September, 6, 5, 26, 35, 0, time.UTC)
	someTimePtr = &newTime
	assertValueEquals(
		t,
		"1441517195.000000000",
		someTimePtrMetric.Value.AsTextString())
	assertValueEquals(
		t,
		"2015-09-06T05:26:35Z",
		someTimePtrMetric.Value.AsHtmlString())

	// Check /proc/rpc-count
	rpcCountMetric := root.GetMetric("/proc/rpc-count")
	verifyMetric(t, "RPC count", units.None, rpcCountMetric)
	assertValueEquals(t, types.Uint, rpcCountMetric.Value.Type())
	assertValueEquals(t, uint64(500), rpcCountMetric.Value.AsUint())
	assertValueEquals(t, "500", rpcCountMetric.Value.AsHtmlString())

	// check /proc/foo/bar/baz
	bazMetric := root.GetMetric("proc/foo/bar/baz")
	verifyMetric(t, "An error", units.None, bazMetric)
	assertValueEquals(t, types.Float, bazMetric.Value.Type())
	assertValueEquals(t, 12.375, bazMetric.Value.AsFloat())
	assertValueEquals(t, "12.375", bazMetric.Value.AsHtmlString())

	// check /proc/foo/bar/abool
	aboolMetric := root.GetMetric("proc/foo/bar/abool")
	verifyMetric(t, "A boolean value", units.None, aboolMetric)
	assertValueEquals(t, types.Bool, aboolMetric.Value.Type())
	assertValueEquals(t, true, aboolMetric.Value.AsBool())
	assertValueEquals(t, "true", aboolMetric.Value.AsHtmlString())

	// check /proc/foo/bar/anotherBool
	anotherBoolMetric := root.GetMetric("proc/foo/bar/anotherBool")
	verifyMetric(t, "A boolean callback value", units.None, anotherBoolMetric)
	assertValueEquals(t, types.Bool, anotherBoolMetric.Value.Type())
	assertValueEquals(t, false, anotherBoolMetric.Value.AsBool())
	assertValueEquals(t, "false", anotherBoolMetric.Value.AsHtmlString())

	// test PathFrom
	assertValueEquals(
		t, "/proc/foo/bar/baz", bazMetric.AbsPath())
	assertValueEquals(
		t, "/proc/rpc-count", rpcCountMetric.AbsPath())
	assertValueEquals(t, "/proc/foo", fooDir.AbsPath())

	// Check /proc/rpc-latency
	rpcLatency := root.GetMetric("/proc/rpc-latency")
	verifyMetric(t, "RPC latency", units.Millisecond, rpcLatency)
	assertValueEquals(t, types.Dist, rpcLatency.Value.Type())

	actual := rpcLatency.Value.AsDistribution().Snapshot()

	if actual.Median < 249 || actual.Median >= 250 {
		t.Errorf("Median out of range: %f", actual.Median)
	}

	expected := &snapshot{
		Min:     0.0,
		Max:     499.0,
		Average: 249.5,
		Median:  actual.Median,
		Count:   500,
		Breakdown: breakdown{
			{
				bucketPiece: &bucketPiece{
					End:   10.0,
					First: true,
				},
				Count: 10,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 10.0,
					End:   25.0,
				},
				Count: 15,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 25.0,
					End:   62.5,
				},
				Count: 38,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 62.5,
					End:   156.25,
				},
				Count: 94,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 156.25,
					End:   390.625,
				},
				Count: 234,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 390.625,
					Last:  true,
				},
				Count: 109,
			},
		},
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestLinearDistribution(t *testing.T) {
	bucketer := NewLinearBucketer(3, 12, 5)
	dist := newDistribution(bucketer)
	actualEmpty := dist.Snapshot()
	expectedEmpty := &snapshot{
		Count: 0,
		Breakdown: breakdown{
			{
				bucketPiece: &bucketPiece{
					End:   12.0,
					First: true,
				},
				Count: 0,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 12.0,
					End:   17.0,
				},
				Count: 0,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 17.0,
					Last:  true,
				},
				Count: 0,
			},
		},
	}
	if !reflect.DeepEqual(expectedEmpty, actualEmpty) {
		t.Errorf("Expected %v, got %v", expectedEmpty, actualEmpty)
	}
}

func TestArbitraryDistribution(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{10, 22, 50})
	dist := newDistribution(bucketer)
	actualEmpty := dist.Snapshot()
	expectedEmpty := &snapshot{
		Count: 0,
		Breakdown: breakdown{
			{
				bucketPiece: &bucketPiece{
					End:   10.0,
					First: true,
				},
				Count: 0,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 10.0,
					End:   22.0,
				},
				Count: 0,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 22.0,
					End:   50.0,
				},
				Count: 0,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 50.0,
					Last:  true,
				},
				Count: 0,
			},
		},
	}
	if !reflect.DeepEqual(expectedEmpty, actualEmpty) {
		t.Errorf("Expected %v, got %v", expectedEmpty, actualEmpty)
	}

	for i := 100; i >= 1; i-- {
		dist.Add(float64(i))
	}
	actual := dist.Snapshot()
	if actual.Median < 50.0 || actual.Median >= 51 {
		t.Errorf("Median out of range: %f", actual.Median)
	}
	expected := &snapshot{
		Min:     1.0,
		Max:     100.0,
		Average: 50.5,
		// Let exact matching pass
		Median: actual.Median,
		Count:  100,
		Breakdown: breakdown{
			{
				bucketPiece: &bucketPiece{
					End:   10.0,
					First: true,
				},
				Count: 9,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 10.0,
					End:   22.0,
				},
				Count: 12,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 22.0,
					End:   50.0,
				},
				Count: 28,
			},
			{
				bucketPiece: &bucketPiece{
					Start: 50.0,
					Last:  true,
				},
				Count: 51,
			},
		},
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestMedianDataAllLow(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{1000.0})
	dist := newDistribution(bucketer)
	dist.Add(200.0)
	dist.Add(300.0)
	snapshot := dist.Snapshot()
	// 2 points between 200 and 300
	assertValueEquals(t, 250.0, snapshot.Median)
}

func TestMedianDataAllHigh(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{1000.0})
	dist := newDistribution(bucketer)
	dist.Add(3000.0)
	dist.Add(3000.0)
	dist.Add(7000.0)

	// Three points between 3000 and 7000
	snapshot := dist.Snapshot()
	assertValueEquals(t, 5000.0, snapshot.Median)
}

func TestMedianSingleData(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{1000.0, 3000.0})
	dist := newDistribution(bucketer)
	dist.Add(7000.0)
	assertValueEquals(t, 7000.0, dist.Snapshot().Median)

	dist1 := newDistribution(bucketer)
	dist1.Add(1700.0)
	assertValueEquals(t, 1700.0, dist1.Snapshot().Median)

	dist2 := newDistribution(bucketer)
	dist2.Add(350.0)
	assertValueEquals(t, 350.0, dist2.Snapshot().Median)
}

func TestMedianAllDataInBetween(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{500.0, 700.0, 1000.0, 3000.0})
	dist := newDistribution(bucketer)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(2900.0)
	// All ponits between 1000 and 2900
	assertValueEquals(t, 1950.0, dist.Snapshot().Median)
}

func TestMedianDataSkewedLow(t *testing.T) {
	dist := newDistribution(PowersOfTen)
	for i := 0; i < 500; i++ {
		dist.Add(float64(i))
	}
	median := dist.Snapshot().Median
	if median-250.0 > 1.0 || median-250.0 < -1.0 {
		t.Errorf("Median out of range: %f", median)
	}
}

func TestMedianDataSkewedHigh(t *testing.T) {
	dist := newDistribution(PowersOfTen)
	for i := 0; i < 500; i++ {
		dist.Add(float64(i + 500))
	}
	median := dist.Snapshot().Median
	if median-750.0 > 1.0 || median-750.0 < -1.0 {
		t.Errorf("Median out of range: %f", median)
	}
}

func rpcCountCallback() uint {
	return 500
}

func bazCallback() float32 {
	return 12.375
}

func boolCallback() bool {
	return false
}

func verifyMetric(
	t *testing.T, desc string, unit units.Unit, m *metric) {
	if desc != m.Description {
		t.Errorf("Expected %s, got %s", desc, m.Description)
	}
	if unit != m.Unit {
		t.Errorf("Expected %v, got %v", unit, m.Unit)
	}
}

func assertValueEquals(
	t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func verifyChildren(
	t *testing.T, listEntries []*listEntry, expected ...string) {
	if len(listEntries) != len(expected) {
		t.Errorf("Expected %d names, got %d", len(expected), len(listEntries))
		return
	}
	for i := range expected {
		if expected[i] != listEntries[i].Name {
			t.Errorf("Expected %s, but got %s, at index %d", expected[i], listEntries[i].Name, i)
		}
	}
}

type metricNamesListType []string

func (l *metricNamesListType) Collect(m *metric) {
	*l = append(*l, m.AbsPath())
}

func verifyGetAllMetrics(
	t *testing.T, path string, d *directory, expectedPaths ...string) {
	var actual metricNamesListType
	d.GetAllMetrics(path, &actual)
	if !reflect.DeepEqual(expectedPaths, ([]string)(actual)) {
		t.Errorf("Expected %v, got %v", expectedPaths, actual)
	}
}
