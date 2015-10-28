package tricorder

import (
	"errors"
	"reflect"
	"testing"
)

var (
	kCallbackError = errors.New("callback error")
)

func TestUnit(t *testing.T) {
	verifyUnit(t, None)
	verifyUnit(t, Millisecond)
	verifyUnit(t, Second)
	verifyUnit(t, Celsius)
	unit, err := NewUnit("bad unit name")
	if unit != None || err == nil {
		t.Error("Expected to get error from bad unit name")
	}
	if out := Unit(99999).String(); out != "None" {
		t.Error("Expected None, got %s", out)
	}

}

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

	rpcBucketer := NewBucketerWithScale(6, 10, 2.5)
	rpcDistribution := NewDistribution(rpcBucketer)

	if err := RegisterMetric("/proc/rpc-latency", rpcDistribution, Millisecond, "RPC latency"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/rpc-count", rpcCountCallback, None, "RPC count"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/start-time", &startTime, Second, "Start Time"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/proc/temperature", &temperature, Celsius, "Temperature"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric("/testname", &name, None, "Name of app"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric("/testargs", &args, None, "Args passed to app"); err != nil {
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
	err = barDir.RegisterMetric("baz", bazCallback, None, "An error")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	// This is already a directory
	if err := RegisterMetric("/proc/foo/bar", &unused, None, "Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already registered as a metric
	if err := RegisterMetric("proc/foo/bar/baz", &unused, None, "Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already a metric and can't be a directory
	if _, err := fooDir.RegisterDirectory("/bar/baz"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric on an empty path
	if err := RegisterMetric("/", &unused, None, "Empty path"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric where parent dir already a metric
	if err := RegisterMetric("/args/illegal", &unused, None, "parent dir already a metric"); err != ErrPathInUse {
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

	// Add data points to the distribution
	// < 10: 10
	// 10 - 25: 15
	// 25 - 62.5: 38
	// 62.5 - 156.25: 94
	// 156.25 - 390.625: 234
	for i := 0; i < 500; i++ {
		rpcDistribution.Add(float64(i))
	}

	verifyChildren(t, root.List(), "args", "name", "proc", "testargs", "testname")
	verifyChildren(
		t,
		root.GetDirectory("proc").List(),
		"foo", "rpc-count", "rpc-latency", "start-time", "temperature")
	verifyChildren(
		t, root.GetDirectory("proc/foo/bar").List(), "baz")

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
	verifyMetric(t, "Args passed to app", None, argsMetric)
	assertValueEquals(t, String, argsMetric.Value.Type())
	assertValueEquals(t, "--help", argsMetric.Value.AsString())
	assertValueEquals(t, "\"--help\"", argsMetric.Value.AsHtmlString())

	// Check /testname
	nameMetric := root.GetMetric("/testname")
	verifyMetric(t, "Name of app", None, nameMetric)
	assertValueEquals(t, String, nameMetric.Value.Type())
	assertValueEquals(t, "My application", nameMetric.Value.AsString())
	assertValueEquals(t, "\"My application\"", nameMetric.Value.AsHtmlString())

	// Check /proc/temperature
	temperatureMetric := root.GetMetric("/proc/temperature")
	verifyMetric(t, "Temperature", Celsius, temperatureMetric)
	assertValueEquals(t, Float, temperatureMetric.Value.Type())
	assertValueEquals(t, 22.5, temperatureMetric.Value.AsFloat())
	assertValueEquals(t, "22.5", temperatureMetric.Value.AsHtmlString())

	// Check /proc/start-time
	startTimeMetric := root.GetMetric("/proc/start-time")
	verifyMetric(t, "Start Time", Second, startTimeMetric)
	assertValueEquals(t, Int, startTimeMetric.Value.Type())
	assertValueEquals(t, int64(-1234567), startTimeMetric.Value.AsInt())
	assertValueEquals(t, "-1234567", startTimeMetric.Value.AsHtmlString())

	// Check /proc/rpc-count
	rpcCountMetric := root.GetMetric("/proc/rpc-count")
	verifyMetric(t, "RPC count", None, rpcCountMetric)
	assertValueEquals(t, Uint, rpcCountMetric.Value.Type())
	assertValueEquals(t, uint64(500), rpcCountMetric.Value.AsUint())
	assertValueEquals(t, "500", rpcCountMetric.Value.AsHtmlString())

	// check /proc/foo/bar/baz
	bazMetric := root.GetMetric("proc/foo/bar/baz")
	verifyMetric(t, "An error", None, bazMetric)
	assertValueEquals(t, Float, bazMetric.Value.Type())
	assertValueEquals(t, 12.375, bazMetric.Value.AsFloat())
	assertValueEquals(t, "12.375", bazMetric.Value.AsHtmlString())

	// test PathFrom
	assertValueEquals(
		t, "/proc/foo/bar/baz", bazMetric.AbsPath())
	assertValueEquals(
		t, "/proc/rpc-count", rpcCountMetric.AbsPath())
	assertValueEquals(t, "/proc/foo", fooDir.AbsPath())

	// Check /proc/rpc-latency
	rpcLatency := root.GetMetric("/proc/rpc-latency")
	verifyMetric(t, "RPC latency", Millisecond, rpcLatency)
	assertValueEquals(t, Dist, rpcLatency.Value.Type())

	actual := rpcLatency.Value.AsDistribution().Snapshot()

	expected := &snapshot{
		Min:     0.0,
		Max:     499.0,
		Average: 249.5,
		// TODO: Have to figure out how to compute this.
		Median: 0.0,
		Count:  500,
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

func TestDistribution(t *testing.T) {
	bucketer := NewBucketerWithEndpoints([]float64{10, 22, 50})
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
	expected := &snapshot{
		Min:     1.0,
		Max:     100.0,
		Average: 50.5,
		// TODO: Have to figure out how to compute this.
		Median: 0.0,
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

func rpcCountCallback() uint {
	return 500
}

func bazCallback() float32 {
	return 12.375
}

func verifyMetric(
	t *testing.T, desc string, unit Unit, m *metric) {
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

func verifyUnit(t *testing.T, u Unit) {
	result, err := NewUnit(u.String())
	if result != u || err != nil {
		t.Error("Round trip for unit %d failed", u)
	}
}
