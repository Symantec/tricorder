package tricorder

import (
	"errors"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	kCallbackError = errors.New("callback error")
)

var (
	firstGlobal   = 100
	secondGlobal  = 200
	thirdGlobal   = 300
	fourthGlobal  = 400
	fifthGlobal   = 500
	sixthGlobal   = 600
	seventhGlobal = 700
	eighthGlobal  = 800
)

func incrementFirstAndSecondGlobal() {
	firstGlobal++
	secondGlobal++
}

func incrementThirdToSixthGlobal() {
	thirdGlobal++
	fourthGlobal++
	fifthGlobal++
	sixthGlobal++
}

func incrementSeventhAndEighthGlobal() {
	seventhGlobal++
	eighthGlobal++
}

func registerMetricsForGlobalsTest() {
	r1and2 := RegisterRegion(incrementFirstAndSecondGlobal)
	r3to6 := RegisterRegion(incrementThirdToSixthGlobal)
	r7and8 := RegisterRegion(incrementSeventhAndEighthGlobal)
	RegisterMetricInRegion(
		"/firstGroup/first", &firstGlobal, r1and2, units.None, "")
	RegisterMetricInRegion(
		"/firstGroup/second", &secondGlobal, r1and2, units.None, "")
	RegisterMetricInRegion(
		"/firstGroup/third", &thirdGlobal, r3to6, units.None, "")
	RegisterMetricInRegion(
		"/firstGroup/fourth", &fourthGlobal, r3to6, units.None, "")
	RegisterMetricInRegion(
		"/secondGroup/fifth", &fifthGlobal, r3to6, units.None, "")
	RegisterMetricInRegion(
		"/secondGroup/sixth", &sixthGlobal, r3to6, units.None, "")
	RegisterMetricInRegion(
		"/secondGroup/seventh", &seventhGlobal, r7and8, units.None, "")
	RegisterMetricInRegion(
		"/secondGroup/eighth", &eighthGlobal, r7and8, units.None, "")
}

type intMetricsCollectorForTesting struct {
	// Number of expected metrics
	Count int
	// Barrier to ensure that go routines collecting metrics run
	// concurrently
	Barrier *sync.WaitGroup
	// The values collected by path
	Values map[string]int64
}

func (c *intMetricsCollectorForTesting) Collect(m *metric, s *session) error {
	c.Values[m.AbsPath()] = m.AsInt(s)
	c.Count--
	if c.Count == 0 {
		c.Barrier.Done()
		c.Barrier.Wait()
	}
	return nil
}

func doGlobalsTest(t *testing.T) {
	var barrier sync.WaitGroup
	var wg sync.WaitGroup

	// First be sure that fetching values regions work even when
	// caller does not create a session. In this case, each fetch
	// will invoke the region's update function.
	assertValueEquals(
		t,
		int64(101),
		root.GetMetric("/firstGroup/first").AsInt(nil))

	assertValueEquals(
		t,
		int64(202),
		root.GetMetric("/firstGroup/second").AsInt(nil))

	assertValueEquals(
		t,
		int64(301),
		root.GetMetric("/firstGroup/third").AsInt(nil))

	assertValueEquals(
		t,
		int64(602),
		root.GetMetric("/secondGroup/sixth").AsInt(nil))

	assertValueEquals(
		t,
		int64(701),
		root.GetMetric("/secondGroup/seventh").AsInt(nil))

	assertValueEquals(
		t,
		int64(802),
		root.GetMetric("/secondGroup/eighth").AsInt(nil))

	// The test has invoked each region's update function twice.
	// So all values should end in 02 at this point.

	collectorFirstGroup := &intMetricsCollectorForTesting{
		Count: 4, Barrier: &barrier, Values: make(map[string]int64)}
	collectorSecondGroup := &intMetricsCollectorForTesting{
		Count: 4, Barrier: &barrier, Values: make(map[string]int64)}
	// Our barrier expects 2 goroutines
	barrier.Add(2)

	// Collect metric in two goroutines running concurrently.
	// Because the collections run concurrently, each region's update
	// function gets called only one time even though they both collect
	// from region r3to6.

	wg.Add(2)
	go func() {
		root.GetAllMetricsByPath(
			"/firstGroup", collectorFirstGroup, nil)
		wg.Done()
	}()
	go func() {
		root.GetAllMetricsByPath(
			"/secondGroup", collectorSecondGroup, nil)
		wg.Done()
	}()
	wg.Wait()
	expectedFirstGroup := map[string]int64{
		"/firstGroup/first":  103,
		"/firstGroup/second": 203,
		"/firstGroup/third":  303,
		"/firstGroup/fourth": 403,
	}
	expectedSecondGroup := map[string]int64{
		"/secondGroup/fifth":   503,
		"/secondGroup/sixth":   603,
		"/secondGroup/seventh": 703,
		"/secondGroup/eighth":  803,
	}
	assertValueDeepEquals(t, expectedFirstGroup, collectorFirstGroup.Values)
	assertValueDeepEquals(t, expectedSecondGroup, collectorSecondGroup.Values)

	// Now collect the same metrics only do it sequentially.
	collectorFirstGroupSeq := &intMetricsCollectorForTesting{
		Count: 4, Barrier: &barrier, Values: make(map[string]int64)}
	collectorSecondGroupSeq := &intMetricsCollectorForTesting{
		Count: 4, Barrier: &barrier, Values: make(map[string]int64)}

	// This time our barrier expects only one goroutine
	barrier.Add(1)

	// Collect metric in two goroutines running one after the other.
	// The session mechanism ensures that each region's update
	// function is called once per collection even if multiple variables
	// in that region get collected. However, since the collections
	// run one after the other, the update function of r3to6 region gets
	// called twice, once per collection because both collections
	// collect metrics from that region.
	wg.Add(1)
	go func() {
		root.GetAllMetricsByPath(
			"/firstGroup", collectorFirstGroupSeq, nil)
		wg.Done()
	}()
	wg.Wait()

	// Barrier expects one goroutine
	barrier.Add(1)

	wg.Add(1)
	go func() {
		root.GetAllMetricsByPath(
			"/secondGroup", collectorSecondGroupSeq, nil)
		wg.Done()
	}()
	wg.Wait()
	expectedFirstGroupSeq := map[string]int64{
		"/firstGroup/first":  104,
		"/firstGroup/second": 204,
		"/firstGroup/third":  304,
		"/firstGroup/fourth": 404,
	}
	expectedSecondGroupSeq := map[string]int64{
		"/secondGroup/fifth":   505,
		"/secondGroup/sixth":   605,
		"/secondGroup/seventh": 704,
		"/secondGroup/eighth":  804,
	}
	assertValueDeepEquals(t, expectedFirstGroupSeq, collectorFirstGroupSeq.Values)
	assertValueDeepEquals(t, expectedSecondGroupSeq, collectorSecondGroupSeq.Values)

}

func TestAPI(t *testing.T) {
	// Do concurrent globals test
	registerMetricsForGlobalsTest()
	doGlobalsTest(t)

	var startTime int64
	var temperature float64
	var unused int64
	var name, args string
	var someTime time.Time
	var someTimePtr *time.Time
	var someBool bool
	var inSeconds time.Duration
	var inMilliseconds time.Duration
	var sizeInBytes int32
	var speedInBytesPerSecond uint32

	rpcBucketer := NewExponentialBucketer(6, 10, 2.5)
	rpcDistribution := rpcBucketer.NewDistribution()

	if err := RegisterMetric(
		"/bytes/bytes",
		&sizeInBytes,
		units.Byte,
		"Size in Bytes"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric(
		"/bytes/bytesPerSecond",
		&speedInBytesPerSecond,
		units.BytePerSecond,
		"Speed in Bytes per Second"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric(
		"/times/milliseconds",
		&inMilliseconds,
		units.Millisecond,
		"In milliseconds"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric(
		"/times/seconds",
		&inSeconds,
		units.Second,
		"In seconds"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric(
		"/proc/rpc-latency",
		rpcDistribution,
		units.Millisecond,
		"RPC latency"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/rpc-count",
		rpcCountCallback,
		units.None,
		"RPC count"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/test-start-time",
		&startTime,
		units.Second,
		"Start Time"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/some-time",
		&someTime,
		units.None,
		"Some time"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/some-time-ptr",
		&someTimePtr,
		units.None,
		"Some time pointer"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/temperature",
		&temperature,
		units.Celsius,
		"Temperature"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/testname",
		&name,
		units.None,
		"Name of app"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	if err := RegisterMetric(
		"/testargs",
		&args,
		units.None,
		"Args passed to app"); err != nil {
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
	err = barDir.RegisterMetric(
		"abool",
		&someBool,
		units.None,
		"A boolean value")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	err = barDir.RegisterMetric(
		"anotherBool",
		boolCallback,
		units.None,
		"A boolean callback value")
	if err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

	// This is already a directory
	if err := RegisterMetric(
		"/proc/foo/bar",
		&unused,
		units.None,
		"Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already registered as a metric
	if err := RegisterMetric(
		"proc/foo/bar/baz",
		&unused,
		units.None,
		"Bad registration"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// This path is already a metric and can't be a directory
	if _, err := fooDir.RegisterDirectory("/bar/baz"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric on an empty path
	if err := RegisterMetric(
		"/",
		&unused,
		units.None,
		"Empty path"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse, got %v", err)
	}

	// Can't call RegisterMetric where parent dir already a metric
	if err := RegisterMetric(
		"/proc/args/illegal",
		&unused,
		units.None,
		"parent dir already a metric"); err != ErrPathInUse {
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
	inSeconds = -21*time.Second - 53*time.Millisecond
	inMilliseconds = 7*time.Second + 8*time.Millisecond
	sizeInBytes = 934912
	speedInBytesPerSecond = 3538944

	someTime = time.Date(
		2015, time.November, 15, 13, 26, 53, 7265341, time.UTC)

	// Add data points to the distribution
	// < 10: 10
	// 10 - 25: 15
	// 25 - 62.5: 38
	// 62.5 - 156.25: 94
	// 156.25 - 390.625: 234
	var dur time.Duration
	for i := 0; i < 500; i++ {
		rpcDistribution.Add(dur)
		dur += time.Millisecond
	}

	verifyChildren(
		t,
		root.List(),
		"bytes",
		"firstGroup",
		"proc",
		"secondGroup",
		"testargs",
		"testname",
		"times")
	verifyChildren(
		t,
		root.GetDirectory("proc").List(),
		"args",
		"cpu",
		"foo",
		"io",
		"ipc",
		"memory",
		"name",
		"rpc-count",
		"rpc-latency",
		"scheduler",
		"signals",
		"some-time",
		"some-time-ptr",
		"start-time",
		"temperature",
		"test-start-time")
	verifyGetAllMetricsByPath(
		t,
		"foo/bar/baz",
		root.GetDirectory("proc"),
		"/proc/foo/bar/baz")
	verifyGetAllMetricsByPath(
		t,
		"ddd",
		root.GetDirectory("proc"))
	verifyGetAllMetricsByPath(
		t,
		"proc/foo",
		root,
		"/proc/foo/bar/abool",
		"/proc/foo/bar/anotherBool",
		"/proc/foo/bar/baz")
	if err := root.GetAllMetricsByPath("/proc/foo", collectErrorType{E: kCallbackError}, nil); err != kCallbackError {
		t.Errorf("Expected kCallbackError, got %v", err)
	}
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
	verifyJsonValue(t, argsMetric, types.String, 0, "--help")
	verifyRpcValue(t, argsMetric, types.String, 0, "--help")
	assertValueEquals(t, "\"--help\"", argsMetric.AsHtmlString(nil))

	// Check /testname
	nameMetric := root.GetMetric("/testname")
	verifyMetric(t, "Name of app", units.None, nameMetric)
	verifyJsonValue(
		t,
		nameMetric, types.String, 0,
		"My application")

	verifyRpcValue(
		t,
		nameMetric, types.String, 0,
		"My application")

	assertValueEquals(t, "\"My application\"", nameMetric.AsHtmlString(nil))

	// Check /bytes/bytes
	sizeInBytesMetric := root.GetMetric("/bytes/bytes")
	verifyMetric(t, "Size in Bytes", units.Byte, sizeInBytesMetric)
	verifyJsonValue(
		t,
		sizeInBytesMetric, types.Int,
		32,
		int64(934912))

	verifyRpcValue(
		t,
		sizeInBytesMetric, types.Int,
		32,
		int64(934912))

	assertValueEquals(
		t, "913 KiB", sizeInBytesMetric.AsHtmlString(nil))
	assertValueEquals(
		t, "934912", sizeInBytesMetric.AsTextString(nil))

	// Check /bytes/bytesPerSecond
	speedInBytesPerSecondMetric := root.GetMetric(
		"/bytes/bytesPerSecond")
	verifyMetric(
		t,
		"Speed in Bytes per Second",
		units.BytePerSecond,
		speedInBytesPerSecondMetric)
	verifyJsonValue(
		t,
		speedInBytesPerSecondMetric, types.Uint,
		32,
		uint64(3538944))

	verifyRpcValue(
		t,
		speedInBytesPerSecondMetric, types.Uint,
		32,
		uint64(3538944))

	assertValueEquals(
		t,
		"3.38 MiB/s",
		speedInBytesPerSecondMetric.AsHtmlString(nil))
	assertValueEquals(
		t,
		"3538944",
		speedInBytesPerSecondMetric.AsTextString(nil))

	// Check /times/seconds
	inSecondMetric := root.GetMetric("/times/seconds")
	verifyMetric(t, "In seconds", units.Second, inSecondMetric)
	verifyJsonValue(
		t,
		inSecondMetric, types.Duration, 0,
		"-21.053000000")

	verifyRpcValue(
		t,
		inSecondMetric, types.GoDuration, 0,
		-21*time.Second-53*time.Millisecond)

	assertValueEquals(t, "-21.053000000", inSecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "-21.053000000", inSecondMetric.AsTextString(nil))

	// Check /times/milliseconds
	inMillisecondMetric := root.GetMetric("/times/milliseconds")
	verifyMetric(t, "In milliseconds", units.Millisecond, inMillisecondMetric)
	verifyJsonValue(
		t,
		inMillisecondMetric, types.Duration, 0,
		"7008.000000")

	verifyRpcValue(
		t,
		inMillisecondMetric, types.GoDuration, 0,
		7*time.Second+8*time.Millisecond)

	assertValueEquals(t, "7.008s", inMillisecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "7008.000000", inMillisecondMetric.AsTextString(nil))

	// Check /proc/temperature
	temperatureMetric := root.GetMetric("/proc/temperature")
	verifyMetric(t, "Temperature", units.Celsius, temperatureMetric)
	verifyJsonValue(
		t,
		temperatureMetric, types.Float,
		64,
		22.5)

	verifyRpcValue(
		t,
		temperatureMetric, types.Float,
		64,
		22.5)

	assertValueEquals(t, "22.5", temperatureMetric.AsHtmlString(nil))

	// Check /proc/start-time
	startTimeMetric := root.GetMetric("/proc/test-start-time")
	verifyMetric(t, "Start Time", units.Second, startTimeMetric)
	verifyJsonValue(
		t,
		startTimeMetric, types.Int,
		64,
		int64(-1234567))

	verifyRpcValue(
		t,
		startTimeMetric, types.Int,
		64,
		int64(-1234567))

	assertValueEquals(t, "-1234567", startTimeMetric.AsTextString(nil))
	assertValueEquals(t, "-1.23 million", startTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time
	someTimeMetric := root.GetMetric("/proc/some-time")
	verifyMetric(t, "Some time", units.None, someTimeMetric)
	verifyJsonValue(
		t,
		someTimeMetric, types.Time, 0,
		"1447594013.007265341")

	verifyRpcValue(
		t,
		someTimeMetric, types.GoTime, 0,
		someTime)

	assertValueEquals(t, "2015-11-15T13:26:53.007265341Z", someTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time-ptr
	someTimePtrMetric := root.GetMetric("/proc/some-time-ptr")
	verifyMetric(t, "Some time pointer", units.None, someTimePtrMetric)
	// a nil time pointer should result in 0 time.
	verifyJsonValue(
		t,
		someTimePtrMetric, types.Time, 0,
		"0.000000000")

	verifyRpcValue(
		t,
		someTimePtrMetric, types.GoTime, 0,
		time.Time{})

	assertValueEquals(t, "0001-01-01T00:00:00Z", someTimePtrMetric.AsHtmlString(nil))

	newTime := time.Date(2015, time.September, 6, 5, 26, 35, 0, time.UTC)
	someTimePtr = &newTime
	verifyJsonValue(
		t,
		someTimePtrMetric, types.Time, 0,
		"1441517195.000000000")

	verifyRpcValue(
		t,
		someTimePtrMetric, types.GoTime, 0,
		newTime)

	assertValueEquals(
		t,
		"2015-09-06T05:26:35Z",
		someTimePtrMetric.AsHtmlString(nil))

	// Check /proc/rpc-count
	rpcCountMetric := root.GetMetric("/proc/rpc-count")
	verifyMetric(t, "RPC count", units.None, rpcCountMetric)
	verifyJsonValue(
		t,
		rpcCountMetric, types.Uint,
		64,
		uint64(500))

	verifyRpcValue(
		t,
		rpcCountMetric, types.Uint,
		64,
		uint64(500))

	assertValueEquals(t, "500", rpcCountMetric.AsHtmlString(nil))

	// check /proc/foo/bar/baz
	bazMetric := root.GetMetric("proc/foo/bar/baz")
	verifyMetric(t, "An error", units.None, bazMetric)
	verifyJsonValue(
		t,
		bazMetric, types.Float,
		32,
		12.375)

	verifyRpcValue(
		t,
		bazMetric, types.Float,
		32,
		12.375)

	assertValueEquals(t, "12.375", bazMetric.AsHtmlString(nil))

	// check /proc/foo/bar/abool
	aboolMetric := root.GetMetric("proc/foo/bar/abool")
	verifyMetric(t, "A boolean value", units.None, aboolMetric)
	verifyJsonValue(
		t,
		aboolMetric, types.Bool, 0,
		true)

	verifyRpcValue(
		t,
		aboolMetric, types.Bool, 0,
		true)

	assertValueEquals(t, "true", aboolMetric.AsHtmlString(nil))

	// check /proc/foo/bar/anotherBool
	anotherBoolMetric := root.GetMetric("proc/foo/bar/anotherBool")
	verifyMetric(t, "A boolean callback value", units.None, anotherBoolMetric)
	verifyJsonValue(
		t,
		anotherBoolMetric, types.Bool, 0,
		false)

	verifyRpcValue(
		t,
		anotherBoolMetric, types.Bool, 0,
		false)

	assertValueEquals(t, "false", anotherBoolMetric.AsHtmlString(nil))

	// Check /proc/rpc-latency
	rpcLatency := root.GetMetric("/proc/rpc-latency")
	verifyMetric(t, "RPC latency", units.Millisecond, rpcLatency)

	var actual messages.Metric
	rpcLatency.UpdateJsonMetric(nil, &actual)

	if actual.Value.(*messages.Distribution).Median < 249 || actual.Value.(*messages.Distribution).Median >= 250 {
		t.Errorf("Median out of range: %f", actual.Value.(*messages.Distribution).Median)
	}

	expected := &messages.Metric{
		Kind: types.Dist,
		Value: &messages.Distribution{
			Min:     0.0,
			Max:     499.0,
			Average: 249.5,
			Sum:     124750.0,
			Median:  actual.Value.(*messages.Distribution).Median,
			Count:   500,
			Ranges: []*messages.RangeWithCount{
				{
					Upper: 10.0,
					Count: 10,
				},
				{
					Lower: 10.0,
					Upper: 25.0,
					Count: 15,
				},
				{
					Lower: 25.0,
					Upper: 62.5,
					Count: 38,
				},
				{
					Lower: 62.5,
					Upper: 156.25,
					Count: 94,
				},
				{
					Lower: 156.25,
					Upper: 390.625,
					Count: 234,
				},
				{
					Lower: 390.625,
					Count: 109,
				},
			},
		},
	}
	assertValueDeepEquals(t, expected, &actual)

	var actualRpc messages.Metric
	rpcLatency.UpdateRpcMetric(nil, &actualRpc)

	if actualRpc.Value.(*messages.Distribution).Median < 249 || actualRpc.Value.(*messages.Distribution).Median >= 250 {
		t.Errorf("Median out of range in rpc: %f", actualRpc.Value.(*messages.Distribution).Median)
	}

	expectedRpc := &messages.Metric{
		Kind: types.Dist,
		Value: &messages.Distribution{
			Min:     0.0,
			Max:     499.0,
			Average: 249.5,
			Sum:     124750.0,
			Median:  actualRpc.Value.(*messages.Distribution).Median,
			Count:   500,
			Ranges: []*messages.RangeWithCount{
				{
					Upper: 10.0,
					Count: 10,
				},
				{
					Lower: 10.0,
					Upper: 25.0,
					Count: 15,
				},
				{
					Lower: 25.0,
					Upper: 62.5,
					Count: 38,
				},
				{
					Lower: 62.5,
					Upper: 156.25,
					Count: 94,
				},
				{
					Lower: 156.25,
					Upper: 390.625,
					Count: 234,
				},
				{
					Lower: 390.625,
					Count: 109,
				},
			},
		},
	}
	assertValueDeepEquals(t, expectedRpc, &actualRpc)

	// test PathFrom
	assertValueEquals(
		t, "/proc/foo/bar/baz", bazMetric.AbsPath())
	assertValueEquals(
		t, "/proc/rpc-count", rpcCountMetric.AbsPath())
	assertValueEquals(t, "/proc/foo", fooDir.AbsPath())

}

func TestLinearDistribution(t *testing.T) {
	verifyBucketer(
		t, NewLinearBucketer(3, 12, 5), 12.0, 17.0)
}

func TestExponentialDistribution(t *testing.T) {
	verifyBucketer(
		t, NewExponentialBucketer(5, 12.0, 3.5),
		12.0, 42.0, 147.0, 514.5)
}

func TestGeometricDistribution(t *testing.T) {
	verifyBucketer(
		t, NewGeometricBucketer(0.5, 20),
		0.5, 1.0, 2.0, 5.0, 10.0, 20.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.51, 19.9),
		0.5, 1.0, 2.0, 5.0, 10.0, 20.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.49, 20.1),
		0.2, 0.5, 1.0, 2.0, 5.0, 10.0, 20.0, 50.0)
	verifyBucketer(
		t, NewGeometricBucketer(100.0, 1000.0),
		100.0, 200.0, 500.0, 1000.0)
	verifyBucketer(
		t, NewGeometricBucketer(99.99, 1000.0),
		50.0, 100.0, 200.0, 500.0, 1000.0)
	verifyBucketer(
		t, NewGeometricBucketer(99.99, 1000.01),
		50.0, 100.0, 200.0, 500.0, 1000.0, 2000.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.02, 5),
		0.02, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.0201, 5.01),
		0.02, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.0199, 4.99),
		0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0)
	verifyBucketer(
		t, NewGeometricBucketer(0.005, 0.005), 0.005)
	verifyBucketer(
		t, NewGeometricBucketer(0.007, 0.007), 0.005, 0.01)
}

func TestArbitraryDistribution(t *testing.T) {
	bucketer := NewArbitraryBucketer([]float64{10, 22, 50})
	verifyBucketer(t, bucketer, 10.0, 22.0, 50.0)
	dist := newDistribution(bucketer)
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
		Sum:    5050.0,
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

func TestCompactDecimalSigned(t *testing.T) {
	suffixes := []string{" thou", " mil", " bil"}
	assertValueEquals(t, "0", iCompactForm(0, 900, suffixes))
	assertValueEquals(t, "93", iCompactForm(93, 900, suffixes))
	assertValueEquals(t, "-93", iCompactForm(-93, 900, suffixes))
	assertValueEquals(t, "899", iCompactForm(899, 900, suffixes))
	assertValueEquals(t, "-899", iCompactForm(-899, 900, suffixes))
	assertValueEquals(t, "1.00 thou", iCompactForm(900, 900, suffixes))
	assertValueEquals(t, "-1.00 thou", iCompactForm(-900, 900, suffixes))
	assertValueEquals(t, "1.01 thou", iCompactForm(905, 900, suffixes))
	assertValueEquals(t, "-1.01 thou", iCompactForm(-905, 900, suffixes))
	assertValueEquals(t, "9.99 thou", iCompactForm(8995, 900, suffixes))
	assertValueEquals(t, "-9.99 thou", iCompactForm(-8995, 900, suffixes))
	assertValueEquals(t, "10.0 thou", iCompactForm(8996, 900, suffixes))
	assertValueEquals(t, "-10.0 thou", iCompactForm(-8996, 900, suffixes))
	assertValueEquals(t, "11.1 thou", iCompactForm(10000, 900, suffixes))
	assertValueEquals(t, "-11.1 thou", iCompactForm(-10000, 900, suffixes))
	assertValueEquals(t, "99.9 thou", iCompactForm(89954, 900, suffixes))
	assertValueEquals(t, "-99.9 thou", iCompactForm(-89954, 900, suffixes))
	assertValueEquals(t, "100 thou", iCompactForm(89955, 900, suffixes))
	assertValueEquals(t, "-100 thou", iCompactForm(-89955, 900, suffixes))
	assertValueEquals(t, "900 thou", iCompactForm(809999, 900, suffixes))
	assertValueEquals(t, "-900 thou", iCompactForm(-809999, 900, suffixes))
	assertValueEquals(t, "1.00 mil", iCompactForm(810000, 900, suffixes))
	assertValueEquals(t, "-1.00 mil", iCompactForm(-810000, 900, suffixes))
}

func TestCompactDecimalUnsigned(t *testing.T) {
	suffixes := []string{" thou", " mil", " bil"}
	assertValueEquals(t, "0", uCompactForm(0, 900, suffixes))
	assertValueEquals(t, "93", uCompactForm(93, 900, suffixes))
	assertValueEquals(t, "899", uCompactForm(899, 900, suffixes))
	assertValueEquals(t, "1.00 thou", uCompactForm(900, 900, suffixes))
	assertValueEquals(t, "1.01 thou", uCompactForm(905, 900, suffixes))
	assertValueEquals(t, "9.99 thou", uCompactForm(8995, 900, suffixes))
	assertValueEquals(t, "10.0 thou", uCompactForm(8996, 900, suffixes))
	assertValueEquals(t, "11.1 thou", uCompactForm(10000, 900, suffixes))
	assertValueEquals(t, "99.9 thou", uCompactForm(89954, 900, suffixes))
	assertValueEquals(t, "100 thou", uCompactForm(89955, 900, suffixes))
	assertValueEquals(t, "900 thou", uCompactForm(809999, 900, suffixes))
	assertValueEquals(t, "1.00 mil", uCompactForm(810000, 900, suffixes))
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
	if unit != m.Unit() {
		t.Errorf("Expected %v, got %v", unit, m.Unit)
	}
}

func assertValueEquals(
	t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func assertValueDeepEquals(
	t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func verifyJsonValue(
	t *testing.T, m *metric, tp types.Type, bits int, value interface{}) {
	var ametric messages.Metric
	ametric.Unit = m.Unit()
	m.UpdateJsonMetric(nil, &ametric)
	assertValueEquals(t, tp, ametric.Kind)
	assertValueEquals(t, bits, ametric.Bits)
	assertValueEquals(t, value, ametric.Value)
}

func verifyRpcValue(
	t *testing.T, m *metric, tp types.Type, bits int, value interface{}) {
	var ametric messages.Metric
	m.UpdateRpcMetric(nil, &ametric)
	assertValueEquals(t, tp, ametric.Kind)
	assertValueEquals(t, bits, ametric.Bits)
	assertValueEquals(t, value, ametric.Value)
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

func verifyBucketer(
	t *testing.T, bucketer *Bucketer, endpoints ...float64) {
	dist := newDistribution(bucketer)
	buckets := dist.Snapshot().Breakdown
	if len(endpoints)+1 != len(buckets) {
		t.Errorf("Expected %d buckets, got %d", len(endpoints)+1, len(buckets))
		return
	}
	for i := range endpoints {
		assertValueEquals(t, endpoints[i], buckets[i].End)
	}
}

type metricNamesListType []string

func (l *metricNamesListType) Collect(m *metric, s *session) error {
	*l = append(*l, m.AbsPath())
	return nil
}

type collectErrorType struct {
	E error
}

func (c collectErrorType) Collect(m *metric, s *session) error {
	return c.E
}

func verifyGetAllMetricsByPath(
	t *testing.T, path string, d *directory, expectedPaths ...string) {
	var actual metricNamesListType
	if err := d.GetAllMetricsByPath(path, &actual, nil); err != nil {
		t.Errorf("Expected GetAllMetricsByPath to return nil, got %v", err)
	}
	if !reflect.DeepEqual(expectedPaths, ([]string)(actual)) {
		t.Errorf("Expected %v, got %v", expectedPaths, actual)
	}
}
