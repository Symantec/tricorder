package tricorder

import (
	"errors"
	"flag"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"strings"
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

var (
	kUsualTimeStamp = time.Date(2016, 5, 13, 14, 27, 35, 0, time.UTC)
)

var (
	firstAndSecondTimeGlobal   = time.Date(2016, 4, 20, 1, 0, 0, 0, time.UTC)
	thirdToSixthTimeGlobal     = time.Date(2016, 4, 20, 3, 0, 0, 0, time.UTC)
	seventhAndEighthTimeGlobal = time.Date(2016, 4, 20, 7, 0, 0, 0, time.UTC)
)

var (
	anIntFlag          int64
	aDurationFlag      time.Duration
	aSliceFlag         flagValue
	aNoGetterSliceFlag noGetterFlagValue
	aUnitFlag          float64
)

func incrementFirstAndSecondGlobal() time.Time {
	firstGlobal++
	secondGlobal++
	firstAndSecondTimeGlobal = firstAndSecondTimeGlobal.AddDate(0, 0, 1)
	return firstAndSecondTimeGlobal
}

func incrementThirdToSixthGlobal() time.Time {
	thirdGlobal++
	fourthGlobal++
	fifthGlobal++
	sixthGlobal++
	thirdToSixthTimeGlobal = thirdToSixthTimeGlobal.AddDate(0, 0, 1)
	return thirdToSixthTimeGlobal
}

func incrementSeventhAndEighthGlobal() time.Time {
	seventhGlobal++
	eighthGlobal++
	seventhAndEighthTimeGlobal = seventhAndEighthTimeGlobal.AddDate(0, 0, 1)
	return seventhAndEighthTimeGlobal
}

func registerGroup(f func() time.Time) *Group {
	result := NewGroup()
	result.RegisterUpdateFunc(f)
	return result
}

func registerMetricsForGlobalsTest() {
	r1and2 := registerGroup(incrementFirstAndSecondGlobal)
	r3to6 := registerGroup(incrementThirdToSixthGlobal)
	r7and8 := registerGroup(incrementSeventhAndEighthGlobal)
	RegisterMetricInGroup(
		"/firstGroup/first", &firstGlobal, r1and2, units.None, "")
	RegisterMetricInGroup(
		"/firstGroup/second", &secondGlobal, r1and2, units.None, "")
	RegisterMetricInGroup(
		"/firstGroup/firstTimesSecond",
		func() int {
			return firstGlobal * secondGlobal
		},
		r1and2,
		units.None,
		"")
	RegisterMetricInGroup(
		"/firstGroup/third", &thirdGlobal, r3to6, units.None, "")
	RegisterMetricInGroup(
		"/firstGroup/fourth", &fourthGlobal, r3to6, units.None, "")
	RegisterMetricInGroup(
		"/secondGroup/fifth", &fifthGlobal, r3to6, units.None, "")
	RegisterMetricInGroup(
		"/secondGroup/sixth", &sixthGlobal, r3to6, units.None, "")
	RegisterMetricInGroup(
		"/secondGroup/seventh", &seventhGlobal, r7and8, units.None, "")
	RegisterMetricInGroup(
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
	// TimeStamps
	TimeStamps map[string]time.Time
}

func (c *intMetricsCollectorForTesting) Collect(m *metric, s *session) error {
	c.Values[m.AbsPath()] = m.AsInt(s)
	c.TimeStamps[m.AbsPath()] = m.TimeStamp(s)
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
		int64(20604),
		root.GetMetric("/firstGroup/firstTimesSecond").AsInt(nil))
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
		Count:      4,
		Barrier:    &barrier,
		Values:     make(map[string]int64),
		TimeStamps: make(map[string]time.Time),
	}
	collectorSecondGroup := &intMetricsCollectorForTesting{
		Count:      4,
		Barrier:    &barrier,
		Values:     make(map[string]int64),
		TimeStamps: make(map[string]time.Time),
	}
	// Our barrier expects 2 goroutines
	barrier.Add(2)

	// Collect metric in two goroutines running concurrently.
	// Because the collections run concurrently, each region's,
	// including r3to6, update function gets called only one time even
	// though both goroutines collect from region r3to6.
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
		"/firstGroup/first":            103,
		"/firstGroup/second":           203,
		"/firstGroup/firstTimesSecond": 20909,
		"/firstGroup/third":            303,
		"/firstGroup/fourth":           403,
	}
	expectedSecondGroup := map[string]int64{
		"/secondGroup/fifth":   503,
		"/secondGroup/sixth":   603,
		"/secondGroup/seventh": 703,
		"/secondGroup/eighth":  803,
	}
	assertValueDeepEquals(t, expectedFirstGroup, collectorFirstGroup.Values)
	assertValueDeepEquals(t, expectedSecondGroup, collectorSecondGroup.Values)

	expectedFirstGroupTs := map[string]time.Time{
		"/firstGroup/first":            time.Date(2016, 4, 23, 1, 0, 0, 0, time.UTC),
		"/firstGroup/second":           time.Date(2016, 4, 23, 1, 0, 0, 0, time.UTC),
		"/firstGroup/firstTimesSecond": time.Date(2016, 4, 23, 1, 0, 0, 0, time.UTC),
		"/firstGroup/third":            time.Date(2016, 4, 23, 3, 0, 0, 0, time.UTC),
		"/firstGroup/fourth":           time.Date(2016, 4, 23, 3, 0, 0, 0, time.UTC),
	}
	expectedSecondGroupTs := map[string]time.Time{
		"/secondGroup/fifth":   time.Date(2016, 4, 23, 3, 0, 0, 0, time.UTC),
		"/secondGroup/sixth":   time.Date(2016, 4, 23, 3, 0, 0, 0, time.UTC),
		"/secondGroup/seventh": time.Date(2016, 4, 23, 7, 0, 0, 0, time.UTC),
		"/secondGroup/eighth":  time.Date(2016, 4, 23, 7, 0, 0, 0, time.UTC),
	}

	assertValueDeepEquals(t, expectedFirstGroupTs, collectorFirstGroup.TimeStamps)
	assertValueDeepEquals(t, expectedSecondGroupTs, collectorSecondGroup.TimeStamps)

	// Now collect the same metrics only do it sequentially.
	collectorFirstGroupSeq := &intMetricsCollectorForTesting{
		Count:      4,
		Barrier:    &barrier,
		Values:     make(map[string]int64),
		TimeStamps: make(map[string]time.Time),
	}
	collectorSecondGroupSeq := &intMetricsCollectorForTesting{
		Count:      4,
		Barrier:    &barrier,
		Values:     make(map[string]int64),
		TimeStamps: make(map[string]time.Time),
	}

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
		"/firstGroup/first":            104,
		"/firstGroup/second":           204,
		"/firstGroup/firstTimesSecond": 21216,
		"/firstGroup/third":            304,
		"/firstGroup/fourth":           404,
	}
	expectedSecondGroupSeq := map[string]int64{
		"/secondGroup/fifth":   505,
		"/secondGroup/sixth":   605,
		"/secondGroup/seventh": 704,
		"/secondGroup/eighth":  804,
	}
	assertValueDeepEquals(t, expectedFirstGroupSeq, collectorFirstGroupSeq.Values)
	assertValueDeepEquals(t, expectedSecondGroupSeq, collectorSecondGroupSeq.Values)

	expectedFirstGroupSeqTs := map[string]time.Time{
		"/firstGroup/first":            time.Date(2016, 4, 24, 1, 0, 0, 0, time.UTC),
		"/firstGroup/second":           time.Date(2016, 4, 24, 1, 0, 0, 0, time.UTC),
		"/firstGroup/firstTimesSecond": time.Date(2016, 4, 24, 1, 0, 0, 0, time.UTC),
		"/firstGroup/third":            time.Date(2016, 4, 24, 3, 0, 0, 0, time.UTC),
		"/firstGroup/fourth":           time.Date(2016, 4, 24, 3, 0, 0, 0, time.UTC),
	}
	expectedSecondGroupSeqTs := map[string]time.Time{
		"/secondGroup/fifth":   time.Date(2016, 4, 25, 3, 0, 0, 0, time.UTC),
		"/secondGroup/sixth":   time.Date(2016, 4, 25, 3, 0, 0, 0, time.UTC),
		"/secondGroup/seventh": time.Date(2016, 4, 24, 7, 0, 0, 0, time.UTC),
		"/secondGroup/eighth":  time.Date(2016, 4, 24, 7, 0, 0, 0, time.UTC),
	}

	assertValueDeepEquals(t, expectedFirstGroupSeqTs, collectorFirstGroupSeq.TimeStamps)
	assertValueDeepEquals(t, expectedSecondGroupSeqTs, collectorSecondGroupSeq.TimeStamps)

	firstmetric := root.GetMetric("/firstGroup/first")
	var ametric messages.Metric
	firstmetric.InitJsonMetric(nil, &ametric)
	firstMetricGroupId := ametric.GroupId

	seventhmetric := root.GetMetric("/secondGroup/seventh")
	seventhmetric.InitJsonMetric(nil, &ametric)
	// group for first and second metric created first.
	// Then group for third to sixth metric
	// Then group for this metric created
	assertValueEquals(t, firstMetricGroupId+2, ametric.GroupId)
}

func TestAPI(t *testing.T) {
	RegisterFlags()
	flag.Parse()
	DefaultGroup.RegisterUpdateFunc(func() time.Time {
		return kUsualTimeStamp
	})

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
	rpcDistribution := rpcBucketer.NewCumulativeDistribution()
	nonCumulativeDist := rpcBucketer.NewNonCumulativeDistribution()

	if err := RegisterMetric(
		"/times/asdf",
		nonCumulativeDist,
		units.None,
		"Test adding a non cumulative distribution"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}

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
		"flags",
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

	// check /proc/flags/int_flag
	anIntFlag = 923
	anIntFlagMetric := root.GetMetric("/proc/flags/int_flag")
	verifyJsonAndRpc(
		t,
		anIntFlagMetric, "/proc/flags/int_flag",
		"An integer flag", units.None, types.Int64, 64,
		int64(923))
	assertValueEquals(t, "923", anIntFlagMetric.AsHtmlString(nil))

	// check /proc/flags/duration_flag
	aDurationFlag = 25 * time.Second
	aDurationFlagMetric := root.GetMetric("/proc/flags/duration_flag")
	verifyJson(
		t,
		aDurationFlagMetric, "/proc/flags/duration_flag",
		"A duration flag", units.Second, types.Duration, 0,
		"25.000000000")
	verifyRpc(
		t,
		aDurationFlagMetric, "/proc/flags/duration_flag",
		"A duration flag", units.Second, types.GoDuration, 0,
		25*time.Second)
	assertValueEquals(t, "25.000s", aDurationFlagMetric.AsHtmlString(nil))

	// check /proc/flags/slice_flag
	aSliceFlag.Set("one,two,three")
	aSliceFlagMetric := root.GetMetric("/proc/flags/slice_flag")
	verifyJsonAndRpc(
		t,
		aSliceFlagMetric, "/proc/flags/slice_flag",
		"A slice flag", units.None, types.String, 0,
		"one,two,three")
	assertValueEquals(
		t, "\"one,two,three\"", aSliceFlagMetric.AsHtmlString(nil))

	// check /proc/flags/no_getter_slice_flag
	aNoGetterSliceFlag.Set("four,five")
	aNoGetterSliceFlagMetric := root.GetMetric("/proc/flags/no_getter_slice_flag")
	verifyJsonAndRpc(
		t,
		aNoGetterSliceFlagMetric,
		"/proc/flags/no_getter_slice_flag",
		"A no getter slice flag", units.None, types.String, 0,
		"four,five")
	assertValueEquals(
		t,
		"\"four,five\"",
		aNoGetterSliceFlagMetric.AsHtmlString(nil))

	// check /proc/flags/unit_flag
	aUnitFlag = 23.5
	aUnitFlagMetric := root.GetMetric("/proc/flags/unit_flag")
	verifyJsonAndRpc(
		t,
		aUnitFlagMetric, "/proc/flags/unit_flag",
		"A unit flag", units.Celsius, types.Float64, 64,
		23.5)
	assertValueEquals(
		t, "23.5", aUnitFlagMetric.AsHtmlString(nil))

	// Check /testargs
	argsMetric := root.GetMetric("/testargs")
	verifyJsonAndRpc(
		t,
		argsMetric, "/testargs",
		"Args passed to app", units.None, types.String, 0,
		"--help")
	assertValueEquals(t, "\"--help\"", argsMetric.AsHtmlString(nil))

	// Check /testname
	nameMetric := root.GetMetric("/testname")
	verifyJsonAndRpc(
		t,
		nameMetric, "/testname",
		"Name of app", units.None, types.String, 0,
		"My application")

	assertValueEquals(t, "\"My application\"", nameMetric.AsHtmlString(nil))

	// Check /bytes/bytes
	sizeInBytesMetric := root.GetMetric("/bytes/bytes")
	verifyJsonAndRpc(
		t,
		sizeInBytesMetric, "/bytes/bytes",
		"Size in Bytes", units.Byte, types.Int32, 32,
		int32(934912))

	assertValueEquals(
		t, "913 KiB", sizeInBytesMetric.AsHtmlString(nil))
	assertValueEquals(
		t, "934912", sizeInBytesMetric.AsTextString(nil))

	// Check /bytes/bytesPerSecond
	speedInBytesPerSecondMetric := root.GetMetric(
		"/bytes/bytesPerSecond")
	verifyJsonAndRpc(
		t,
		speedInBytesPerSecondMetric, "/bytes/bytesPerSecond",
		"Speed in Bytes per Second", units.BytePerSecond,
		types.Uint32, 32,
		uint32(3538944))

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
	verifyJson(
		t,
		inSecondMetric, "/times/seconds",
		"In seconds", units.Second, types.Duration, 0,
		"-21.053000000")
	verifyRpc(
		t,
		inSecondMetric, "/times/seconds",
		"In seconds", units.Second, types.GoDuration, 0,
		-21*time.Second-53*time.Millisecond)

	assertValueEquals(t, "-21.053000000", inSecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "-21.053000000", inSecondMetric.AsTextString(nil))

	// Check /times/milliseconds
	inMillisecondMetric := root.GetMetric("/times/milliseconds")
	verifyJson(
		t,
		inMillisecondMetric, "/times/milliseconds",
		"In milliseconds", units.Millisecond, types.Duration, 0,
		"7008.000000")
	verifyRpc(
		t,
		inMillisecondMetric, "/times/milliseconds",
		"In milliseconds", units.Millisecond, types.GoDuration, 0,
		7*time.Second+8*time.Millisecond)

	assertValueEquals(t, "7.008s", inMillisecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "7008.000000", inMillisecondMetric.AsTextString(nil))

	// Check /proc/temperature
	temperatureMetric := root.GetMetric("/proc/temperature")
	verifyJsonAndRpc(
		t,
		temperatureMetric, "/proc/temperature",
		"Temperature", units.Celsius, types.Float64, 64,
		22.5)

	assertValueEquals(t, "22.5", temperatureMetric.AsHtmlString(nil))

	// Check /proc/start-time
	startTimeMetric := root.GetMetric("/proc/test-start-time")
	verifyJsonAndRpc(
		t,
		startTimeMetric, "/proc/test-start-time",
		"Start Time", units.Second, types.Int64, 64,
		int64(-1234567))

	assertValueEquals(t, "-1234567", startTimeMetric.AsTextString(nil))
	assertValueEquals(t, "-1.23 million", startTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time
	someTimeMetric := root.GetMetric("/proc/some-time")
	verifyJson(
		t,
		someTimeMetric, "/proc/some-time",
		"Some time", units.None, types.Time, 0,
		"1447594013.007265341")
	verifyRpc(
		t,
		someTimeMetric, "/proc/some-time",
		"Some time", units.None, types.GoTime, 0,
		someTime)

	assertValueEquals(t, "2015-11-15T13:26:53.007265341Z", someTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time-ptr
	someTimePtrMetric := root.GetMetric("/proc/some-time-ptr")
	// a nil time pointer should result in 0 time.
	verifyJson(
		t,
		someTimePtrMetric, "/proc/some-time-ptr",
		"Some time pointer", units.None, types.Time, 0,
		"0.000000000")
	verifyRpc(
		t,
		someTimePtrMetric, "/proc/some-time-ptr",
		"Some time pointer", units.None, types.GoTime, 0,
		time.Time{})

	assertValueEquals(t, "0001-01-01T00:00:00Z", someTimePtrMetric.AsHtmlString(nil))

	newTime := time.Date(2015, time.September, 6, 5, 26, 35, 0, time.UTC)
	someTimePtr = &newTime
	verifyJsonValue(t, someTimePtrMetric, "1441517195.000000000")
	verifyRpcValue(t, someTimePtrMetric, newTime)

	assertValueEquals(
		t,
		"2015-09-06T05:26:35Z",
		someTimePtrMetric.AsHtmlString(nil))

	// Check /proc/rpc-count
	rpcCountMetric := root.GetMetric("/proc/rpc-count")
	verifyJsonAndRpc(
		t,
		rpcCountMetric, "/proc/rpc-count",
		"RPC count", units.None, types.Uint64, 64,
		uint64(500))

	assertValueEquals(t, "500", rpcCountMetric.AsHtmlString(nil))

	// check /proc/foo/bar/baz
	bazMetric := root.GetMetric("proc/foo/bar/baz")
	verifyJsonAndRpc(
		t,
		bazMetric, "/proc/foo/bar/baz",
		"An error", units.None, types.Float32, 32,
		float32(12.375))

	assertValueEquals(t, "12.375", bazMetric.AsHtmlString(nil))

	// check /proc/foo/bar/abool
	aboolMetric := root.GetMetric("proc/foo/bar/abool")
	verifyJsonAndRpc(
		t,
		aboolMetric, "/proc/foo/bar/abool",
		"A boolean value", units.None, types.Bool, 0,
		true)

	assertValueEquals(t, "true", aboolMetric.AsHtmlString(nil))

	// check /proc/foo/bar/anotherBool
	anotherBoolMetric := root.GetMetric("proc/foo/bar/anotherBool")
	verifyJsonAndRpc(
		t,
		anotherBoolMetric, "/proc/foo/bar/anotherBool",
		"A boolean callback value", units.None, types.Bool, 0,
		false)
	assertValueEquals(t, "false", anotherBoolMetric.AsHtmlString(nil))

	// Check /proc/rpc-latency
	rpcLatency := root.GetMetric("/proc/rpc-latency")
	verifyMetric(
		t,
		rpcLatency, "/proc/rpc-latency",
		"RPC latency", units.Millisecond)

	var actual messages.Metric
	rpcLatency.UpdateJsonMetric(nil, &actual)

	if actual.Value.(*messages.Distribution).Median < 249 || actual.Value.(*messages.Distribution).Median >= 250 {
		t.Errorf("Median out of range: %f", actual.Value.(*messages.Distribution).Median)
	}

	expected := &messages.Metric{
		Path:        "/proc/rpc-latency",
		Unit:        units.Millisecond,
		Description: "RPC latency",
		Kind:        types.Dist,
		Value: &messages.Distribution{
			Min:        0.0,
			Max:        499.0,
			Average:    249.5,
			Sum:        124750.0,
			Median:     actual.Value.(*messages.Distribution).Median,
			Count:      500,
			Generation: 500,
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
		TimeStamp: "",
	}
	assertValueDeepEquals(t, expected, &actual)

	var actualRpc messages.Metric
	rpcLatency.UpdateRpcMetric(nil, &actualRpc)

	if actualRpc.Value.(*messages.Distribution).Median < 249 || actualRpc.Value.(*messages.Distribution).Median >= 250 {
		t.Errorf("Median out of range in rpc: %f", actualRpc.Value.(*messages.Distribution).Median)
	}

	expectedRpc := &messages.Metric{
		Path:        "/proc/rpc-latency",
		Unit:        units.Millisecond,
		Description: "RPC latency",
		Kind:        types.Dist,
		Value: &messages.Distribution{
			Min:        0.0,
			Max:        499.0,
			Average:    249.5,
			Sum:        124750.0,
			Median:     actualRpc.Value.(*messages.Distribution).Median,
			Count:      500,
			Generation: 500,
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

	// UnregisterPath on non existent paths is a noop
	UnregisterPath("/proc/foo/pathDoesNotExist")
	UnregisterPath("/proc/pathDoesNotExist/pathDoesNotExist")
	UnregisterPath("/pathDoesNotExist/pathDoesNotExist/pathDoesNotExist")

	// UnregiserPath on root is a no-op
	UnregisterPath("/")

	var anIntValue int

	if err := fooDir.RegisterMetric(
		"/bar/baz",
		&anIntValue,
		units.None,
		"Metric already exists"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse for /bar/baz, got %v", err)
	}

	fooDir.UnregisterPath("/bar/baz")

	if err := fooDir.RegisterMetric(
		"/bar/baz",
		&anIntValue,
		units.None,
		"some metric"); err != nil {
		t.Errorf("Registration of /bar/baz should have succeded, got %v", err)
	}

	if err := RegisterMetric(
		"/proc/foo/bar/baz",
		&anIntValue,
		units.None,
		"Metric already exists"); err != ErrPathInUse {
		t.Errorf("Expected ErrPathInUse for /proc/foo/bar/baz, got %v", err)
	}

	fooDir.UnregisterDirectory()

	// No more fooDir
	verifyChildren(
		t,
		root.GetDirectory("proc").List(),
		"args",
		"cpu",
		"flags",
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

	// Regisering metrics using fooDir should cause panic
	func() {
		defer func() {
			if recover() == nil {
				t.Error("Expected registring a metric on unregistered directory to panic.")
			}
		}()
		fooDir.RegisterMetric("/should/not/work",
			&anIntValue,
			units.None,
			"some metric")

	}()

	func() {
		defer func() {
			if recover() == nil {
				t.Error("Expected registring a short metric on unregistered directory to panic.")
			}
		}()
		fooDir.RegisterMetric("/wontwork",
			&anIntValue,
			units.None,
			"some metric")

	}()

	if err := RegisterMetric(
		"/proc/foo/bar/baz",
		&anIntValue,
		units.None,
		"some metric"); err != nil {
		t.Errorf("registration of /proc/foo/bar/baz should have succeeded, got %v", err)
	}

	procDir, _ := RegisterDirectory("/proc")
	procDir.UnregisterDirectory()

	// No proc directory
	verifyChildren(
		t,
		root.List(),
		"bytes",
		"firstGroup",
		"secondGroup",
		"testargs",
		"testname",
		"times")

	UnregisterPath("firstGroup")

	verifyChildren(
		t,
		root.List(),
		"bytes",
		"secondGroup",
		"testargs",
		"testname",
		"times")

	rootDir, _ := RegisterDirectory("/")

	// This should be a no-op
	rootDir.UnregisterDirectory()

	verifyChildren(
		t,
		root.List(),
		"bytes",
		"secondGroup",
		"testargs",
		"testname",
		"times")
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
	bucketer := NewArbitraryBucketer(10, 22, 50)
	verifyBucketer(t, bucketer, 10.0, 22.0, 50.0)
	dist := newDistribution(bucketer, true)
	for i := 100; i >= 1; i-- {
		dist.Add(100.0)
	}
	actual := dist.Snapshot()
	if actual.Median != 100.0 {
		t.Errorf("Expected median to be 100: %f", actual.Median)
	}
	for i := 100; i >= 1; i-- {
		dist.Update(100.0, float64(i))
	}
	for i := 0; i < 100; i++ {
		dist.Add(100.0)
		dist.Remove(100.0)
	}
	actual = dist.Snapshot()
	if actual.Median < 49.5 || actual.Median >= 51.5 {
		t.Errorf("Median out of range: %f", actual.Median)
	}
	expected := &snapshot{
		Min:     1.0,
		Max:     100.0,
		Average: 50.5,
		// Let exact matching pass
		Median:          actual.Median,
		Sum:             5050.0,
		Count:           100,
		Generation:      400,
		IsNotCumulative: true,
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
	bucketer := NewArbitraryBucketer(1000.0)
	dist := newDistribution(bucketer, false)
	dist.Add(200.0)
	dist.Add(300.0)
	snapshot := dist.Snapshot()
	// 2 points between 200 and 300
	assertValueEquals(t, 250.0, snapshot.Median)
}

func TestMedianDataAllHigh(t *testing.T) {
	bucketer := NewArbitraryBucketer(1000.0)
	dist := newDistribution(bucketer, false)
	dist.Add(3000.0)
	dist.Add(3000.0)
	dist.Add(7000.0)

	// Three points between 3000 and 7000
	snapshot := dist.Snapshot()
	assertValueEquals(t, 5000.0, snapshot.Median)
}

func TestMedianSingleData(t *testing.T) {
	bucketer := NewArbitraryBucketer(1000.0, 3000.0)
	dist := newDistribution(bucketer, false)
	dist.Add(7000.0)
	assertValueEquals(t, 7000.0, dist.Snapshot().Median)

	dist1 := newDistribution(bucketer, false)
	dist1.Add(1700.0)
	assertValueEquals(t, 1700.0, dist1.Snapshot().Median)

	dist2 := newDistribution(bucketer, false)
	dist2.Add(350.0)
	assertValueEquals(t, 350.0, dist2.Snapshot().Median)
}

func TestMedianAllDataInBetween(t *testing.T) {
	bucketer := NewArbitraryBucketer(500.0, 700.0, 1000.0, 3000.0)
	dist := newDistribution(bucketer, false)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(1000.0)
	dist.Add(2900.0)
	// All ponits between 1000 and 2900
	assertValueEquals(t, 1950.0, dist.Snapshot().Median)
}

func TestMedianDataSkewedLow(t *testing.T) {
	dist := newDistribution(PowersOfTen, false)
	for i := 0; i < 500; i++ {
		dist.Add(float64(i))
	}
	median := dist.Snapshot().Median
	if median-250.0 > 1.0 || median-250.0 < -1.0 {
		t.Errorf("Median out of range: %f", median)
	}
}

func TestMedianDataSkewedHigh(t *testing.T) {
	dist := newDistribution(PowersOfTen, false)
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
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit) {
	assertValueEquals(t, path, m.AbsPath())
	assertValueEquals(t, description, m.Description)
	assertValueEquals(t, unit, m.Unit())
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

func verifyJsonAndRpc(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	verifyJson(
		t, m, path, description, unit, tp, bits, value)
	verifyRpc(
		t, m, path, description, unit, tp, bits, value)
}

func verifyJson(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	var ametric messages.Metric
	m.InitJsonMetric(nil, &ametric)
	assertValueEquals(t, path, ametric.Path)
	assertValueEquals(t, description, ametric.Description)
	assertValueEquals(t, unit, ametric.Unit)
	assertValueEquals(t, tp, ametric.Kind)
	assertValueEquals(t, bits, ametric.Bits)
	assertValueEquals(t, value, ametric.Value)
	assertValueEquals(t, "1463149655.000000000", ametric.TimeStamp)
	assertValueEquals(t, 0, ametric.GroupId)
}

func verifyRpc(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	var ametric messages.Metric
	m.InitRpcMetric(nil, &ametric)
	assertValueEquals(t, path, ametric.Path)
	assertValueEquals(t, description, ametric.Description)
	assertValueEquals(t, unit, ametric.Unit)
	assertValueEquals(t, tp, ametric.Kind)
	assertValueEquals(t, bits, ametric.Bits)
	assertValueEquals(t, value, ametric.Value)
	assertValueEquals(t, kUsualTimeStamp, ametric.TimeStamp)
	assertValueEquals(t, 0, ametric.GroupId)
}

func verifyJsonValue(
	t *testing.T,
	m *metric,
	value interface{}) {
	var ametric messages.Metric
	m.UpdateJsonMetric(nil, &ametric)
	assertValueEquals(t, value, ametric.Value)
}

func verifyRpcValue(
	t *testing.T,
	m *metric,
	value interface{}) {
	var ametric messages.Metric
	m.UpdateRpcMetric(nil, &ametric)
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
	dist := newDistribution(bucketer, false)
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

type flagValue []string

func (f flagValue) String() string {
	return strings.Join(f, ",")
}

func (f *flagValue) Set(s string) error {
	*f = strings.Split(s, ",")
	return nil
}

func (f flagValue) Get() interface{} {
	return ([]string)(f)
}

type noGetterFlagValue []string

func (f noGetterFlagValue) String() string {
	return strings.Join(f, ",")
}

func (f *noGetterFlagValue) Set(s string) error {
	*f = strings.Split(s, ",")
	return nil
}

func init() {
	flag.Int64Var(&anIntFlag, "int_flag", 0, "An integer flag")
	flag.DurationVar(&aDurationFlag, "duration_flag", time.Minute, "A duration flag")
	flag.Var(&aSliceFlag, "slice_flag", "A slice flag")
	flag.Var(
		&aNoGetterSliceFlag,
		"no_getter_slice_flag",
		"A no getter slice flag")
	flag.Float64Var(&aUnitFlag, "unit_flag", 0.0, "A unit flag")
	SetFlagUnit("unit_flag", units.Celsius)
}
