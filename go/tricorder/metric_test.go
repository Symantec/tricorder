package tricorder

import (
	"errors"
	"flag"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type messageType int

const (
	jsonMessage messageType = iota
	rpcMessage
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

	int64List := newListWithTimeStamp(
		[]int64{2, 3, 5, 7},
		ImmutableSlice,
		kUsualTimeStamp)
	durationList := newListWithTimeStamp(
		[]time.Duration{time.Second, time.Minute},
		ImmutableSlice,
		kUsualTimeStamp.Add(2*time.Hour))
	var aNilSlice []uint32
	nilList := newListWithTimeStamp(
		aNilSlice,
		ImmutableSlice,
		kUsualTimeStamp.Add(6*time.Hour))
	emptyList := newListWithTimeStamp(
		aNilSlice,
		MutableSlice,
		kUsualTimeStamp.Add(9*time.Hour))

	firstListGroupId := int64List.GroupId()

	if err := RegisterMetric(
		"/nan/nan32",
		func() float32 {
			return float32(math.NaN())
		},
		units.None,
		"NaN as float32"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	nan64 := math.NaN()
	if err := RegisterMetric(
		"/nan/nan64",
		&nan64,
		units.None,
		"NaN as float64"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/nan/inf",
		func() float64 {
			return math.Inf(0)
		},
		units.None,
		"Inf"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	neginf := math.Inf(-1)
	if err := RegisterMetric(
		"/nan/neginf",
		&neginf,
		units.None,
		"-Inf"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
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
		"/proc/rpc-latency-again",
		rpcDistribution,
		units.Millisecond,
		"RPC latency again"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/proc/rpc-latency-wrong",
		rpcDistribution,
		units.Second,
		"RPC latency wrong units"); err != ErrWrongUnit {
		t.Error("Expected to get ErrWrongUnit registering same distribution with different units.")
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

	if err := RegisterMetric(
		"/list/int64",
		(*List)(int64List),
		units.None,
		"list of int 64s"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/list/duration",
		(*List)(durationList),
		units.None,
		"list of durations"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/list/nil",
		(*List)(nilList),
		units.None,
		"nil list"); err != nil {
		t.Fatalf("Got error %v registering metric", err)
	}
	if err := RegisterMetric(
		"/list/empty",
		(*List)(emptyList),
		units.None,
		"empty list"); err != nil {
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
		"list",
		"nan",
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
		"go",
		"io",
		"ipc",
		"memory",
		"name",
		"rpc-count",
		"rpc-latency",
		"rpc-latency-again",
		"scheduler",
		"signals",
		"some-time",
		"some-time-ptr",
		"start-time",
		"temperature",
		"test-start-time")
	verifyGetAllMetricsByPath(
		t,
		"/nan",
		root)
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

	// Test ReadMyMetrics
	nonExistentResult := ReadMyMetrics("/some-non-existent-metric")
	if len(nonExistentResult) != 0 {
		t.Error("Expected empty result")
	}

	procTemperatureResult := ReadMyMetrics("/proc/temperature")
	procTemperatureResult = zeroOutGroupId(procTemperatureResult)
	expectedList := messages.MetricList{
		{
			Path:        "/proc/temperature",
			Description: "Temperature",
			Unit:        units.Celsius,
			Kind:        types.Float64,
			Bits:        64,
			Value:       22.5,
			TimeStamp:   kUsualTimeStamp,
		},
	}
	if !reflect.DeepEqual(expectedList, procTemperatureResult) {
		t.Errorf("Expected %v, got %v", expectedList, procTemperatureResult)
	}
	listResult := ReadMyMetrics("/list")
	listResult = zeroOutGroupId(listResult)
	expectedList = messages.MetricList{
		{
			Path:        "/list/int64",
			Description: "list of int 64s",
			Unit:        units.None,
			Kind:        types.List,
			SubType:     types.Int64,
			Value:       []int64{2, 3, 5, 7},
			TimeStamp:   kUsualTimeStamp,
		},
		{
			Path:        "/list/duration",
			Description: "list of durations",
			Unit:        units.None,
			Kind:        types.List,
			SubType:     types.Duration,
			Value:       []time.Duration{time.Second, time.Minute},
			TimeStamp:   kUsualTimeStamp.Add(2 * time.Hour),
		},
		{
			Path:        "/list/nil",
			Description: "nil list",
			Unit:        units.None,
			Kind:        types.List,
			SubType:     types.Uint32,
			Value:       ([]uint32)(nil),
			TimeStamp:   kUsualTimeStamp.Add(6 * time.Hour),
		},
		{
			Path:        "/list/empty",
			Description: "empty list",
			Unit:        units.None,
			Kind:        types.List,
			SubType:     types.Uint32,
			Value:       []uint32{},
			TimeStamp:   kUsualTimeStamp.Add(9 * time.Hour),
		},
	}

	// check /proc/flags/int_flag
	anIntFlag = 923
	anIntFlagMetric := root.GetMetric("/proc/flags/int_flag")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		anIntFlagMetric, "/proc/flags/int_flag",
		"An integer flag", units.None, types.Int64, 64,
		int64(923))
	assertValueEquals(t, "923", anIntFlagMetric.AsHtmlString(nil))

	// check /proc/flags/duration_flag
	aDurationFlag = 25 * time.Second
	aDurationFlagMetric := root.GetMetric("/proc/flags/duration_flag")
	verifyJsonDefaultTsGroupId(
		t,
		aDurationFlagMetric, "/proc/flags/duration_flag",
		"A duration flag", units.Second, types.Duration, 0,
		"25.000000000")
	verifyRpcDefaultTsGroupId(
		t,
		aDurationFlagMetric, "/proc/flags/duration_flag",
		"A duration flag", units.Second, types.GoDuration, 0,
		25*time.Second)
	assertValueEquals(t, "25.000s", aDurationFlagMetric.AsHtmlString(nil))

	// check /proc/flags/slice_flag
	aSliceFlag.Set("one,two,three")
	aSliceFlagMetric := root.GetMetric("/proc/flags/slice_flag")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		aSliceFlagMetric, "/proc/flags/slice_flag",
		"A slice flag", units.None, types.String, 0,
		"one,two,three")
	assertValueEquals(
		t, "\"one,two,three\"", aSliceFlagMetric.AsHtmlString(nil))

	// check /proc/flags/no_getter_slice_flag
	aNoGetterSliceFlag.Set("four,five")
	aNoGetterSliceFlagMetric := root.GetMetric("/proc/flags/no_getter_slice_flag")
	verifyJsonAndRpcDefaultTsGroupId(
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
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		aUnitFlagMetric, "/proc/flags/unit_flag",
		"A unit flag", units.Celsius, types.Float64, 64,
		23.5)
	assertValueEquals(
		t, "23.5", aUnitFlagMetric.AsHtmlString(nil))

	// Check /testargs
	argsMetric := root.GetMetric("/testargs")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		argsMetric, "/testargs",
		"Args passed to app", units.None, types.String, 0,
		"--help")
	assertValueEquals(t, "\"--help\"", argsMetric.AsHtmlString(nil))

	// Check /testname
	nameMetric := root.GetMetric("/testname")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		nameMetric, "/testname",
		"Name of app", units.None, types.String, 0,
		"My application")

	assertValueEquals(t, "\"My application\"", nameMetric.AsHtmlString(nil))

	// Check /bytes/bytes
	sizeInBytesMetric := root.GetMetric("/bytes/bytes")
	verifyJsonAndRpcDefaultTsGroupId(
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
	verifyJsonAndRpcDefaultTsGroupId(
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
	verifyJsonDefaultTsGroupId(
		t,
		inSecondMetric, "/times/seconds",
		"In seconds", units.Second, types.Duration, 0,
		"-21.053000000")
	verifyRpcDefaultTsGroupId(
		t,
		inSecondMetric, "/times/seconds",
		"In seconds", units.Second, types.GoDuration, 0,
		-21*time.Second-53*time.Millisecond)

	assertValueEquals(t, "-21.053000000", inSecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "-21.053000000", inSecondMetric.AsTextString(nil))

	// Check /times/milliseconds
	inMillisecondMetric := root.GetMetric("/times/milliseconds")
	verifyJsonDefaultTsGroupId(
		t,
		inMillisecondMetric, "/times/milliseconds",
		"In milliseconds", units.Millisecond, types.Duration, 0,
		"7008.000000000")
	verifyRpcDefaultTsGroupId(
		t,
		inMillisecondMetric, "/times/milliseconds",
		"In milliseconds", units.Millisecond, types.GoDuration, 0,
		7*time.Second+8*time.Millisecond)

	assertValueEquals(t, "7.008s", inMillisecondMetric.AsHtmlString(nil))
	assertValueEquals(t, "7008.000000000", inMillisecondMetric.AsTextString(nil))

	// Check /proc/temperature
	temperatureMetric := root.GetMetric("/proc/temperature")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		temperatureMetric, "/proc/temperature",
		"Temperature", units.Celsius, types.Float64, 64,
		22.5)

	assertValueEquals(t, "22.5", temperatureMetric.AsHtmlString(nil))

	// Check /proc/start-time
	startTimeMetric := root.GetMetric("/proc/test-start-time")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		startTimeMetric, "/proc/test-start-time",
		"Start Time", units.Second, types.Int64, 64,
		int64(-1234567))

	assertValueEquals(t, "-1234567", startTimeMetric.AsTextString(nil))
	assertValueEquals(t, "-1.23 million", startTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time
	someTimeMetric := root.GetMetric("/proc/some-time")
	verifyJsonDefaultTsGroupId(
		t,
		someTimeMetric, "/proc/some-time",
		"Some time", units.None, types.Time, 0,
		"1447594013.007265341")
	verifyRpcDefaultTsGroupId(
		t,
		someTimeMetric, "/proc/some-time",
		"Some time", units.None, types.GoTime, 0,
		someTime)

	assertValueEquals(t, "2015-11-15T13:26:53.007265341Z", someTimeMetric.AsHtmlString(nil))

	// Check /proc/some-time-ptr
	someTimePtrMetric := root.GetMetric("/proc/some-time-ptr")
	// a nil time pointer should result in 0 time.
	verifyJsonDefaultTsGroupId(
		t,
		someTimePtrMetric, "/proc/some-time-ptr",
		"Some time pointer", units.None, types.Time, 0,
		"0.000000000")
	verifyRpcDefaultTsGroupId(
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
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		rpcCountMetric, "/proc/rpc-count",
		"RPC count", units.None, types.Uint64, 64,
		uint64(500))

	assertValueEquals(t, "500", rpcCountMetric.AsHtmlString(nil))

	// check /proc/foo/bar/baz
	bazMetric := root.GetMetric("proc/foo/bar/baz")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		bazMetric, "/proc/foo/bar/baz",
		"An error", units.None, types.Float32, 32,
		float32(12.375))

	assertValueEquals(t, "12.375", bazMetric.AsHtmlString(nil))

	// check /proc/foo/bar/abool
	aboolMetric := root.GetMetric("proc/foo/bar/abool")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		aboolMetric, "/proc/foo/bar/abool",
		"A boolean value", units.None, types.Bool, 0,
		true)

	assertValueEquals(t, "true", aboolMetric.AsHtmlString(nil))

	// check /proc/foo/bar/anotherBool
	anotherBoolMetric := root.GetMetric("proc/foo/bar/anotherBool")
	verifyJsonAndRpcDefaultTsGroupId(
		t,
		anotherBoolMetric, "/proc/foo/bar/anotherBool",
		"A boolean callback value", units.None, types.Bool, 0,
		false)
	assertValueEquals(t, "false", anotherBoolMetric.AsHtmlString(nil))

	// Check /list/int64
	int64ListMetric := root.GetMetric("/list/int64")
	verifyJsonAndRpc(
		t,
		int64ListMetric, "/list/int64",
		"list of int 64s", units.None, types.List, 64,
		[]int64{2, 3, 5, 7})
	var someJsonMetric messages.Metric
	var someRpcMetric messages.Metric
	int64ListMetric.InitJsonMetric(nil, &someJsonMetric)
	assertValueEquals(t, types.Int64, someJsonMetric.SubType)
	assertValueEquals(t, firstListGroupId, someJsonMetric.GroupId)
	assertValueEquals(t, "1463149655.000000000", someJsonMetric.TimeStamp)
	int64ListMetric.InitRpcMetric(nil, &someRpcMetric)
	assertValueEquals(t, types.Int64, someRpcMetric.SubType)
	assertValueEquals(t, firstListGroupId, someRpcMetric.GroupId)
	assertValueEquals(t, kUsualTimeStamp, someRpcMetric.TimeStamp)

	// Check /list/duration
	durationListMetric := root.GetMetric("/list/duration")
	verifyJson(
		t,
		durationListMetric, "/list/duration",
		"list of durations", units.None, types.List, 0,
		[]string{"1.000000000", "60.000000000"})
	verifyRpc(
		t,
		durationListMetric, "/list/duration",
		"list of durations", units.None, types.List, 0,
		[]time.Duration{time.Second, time.Minute})

	durationListMetric.InitJsonMetric(nil, &someJsonMetric)
	assertValueEquals(t, types.Duration, someJsonMetric.SubType)
	assertValueEquals(t, firstListGroupId+1, someJsonMetric.GroupId)
	assertValueEquals(
		t, "1463156855.000000000", someJsonMetric.TimeStamp)
	durationListMetric.InitRpcMetric(nil, &someRpcMetric)
	assertValueEquals(t, types.GoDuration, someRpcMetric.SubType)
	assertValueEquals(t, firstListGroupId+1, someRpcMetric.GroupId)
	assertValueEquals(
		t,
		kUsualTimeStamp.Add(2*time.Hour),
		someRpcMetric.TimeStamp)

	// Check /list/nil
	// The json shows an empty int slice instead of nil so that
	// the json gives [] instead of null. The stand-in slice doesn't
	// have to match the type of the metric since it gives [] in
	// json.
	nilListMetric := root.GetMetric("/list/nil")
	verifyJson(
		t,
		nilListMetric, "/list/nil",
		"nil list", units.None, types.List, 32,
		[]uint32{})
	var nilUint32Slice []uint32
	verifyRpc(
		t,
		nilListMetric, "/list/nil",
		"nil list", units.None, types.List, 32,
		nilUint32Slice)

	// Check /list/empty
	emptyListMetric := root.GetMetric("/list/empty")
	verifyJson(
		t,
		emptyListMetric, "/list/empty",
		"empty list", units.None, types.List, 32,
		[]uint32{})
	verifyRpc(
		t,
		emptyListMetric, "/list/empty",
		"empty list", units.None, types.List, 32,
		[]uint32{})

	// Check /proc/rpc-latency-again
	rpcLatencyAgain := root.GetMetric("/proc/rpc-latency-again")
	verifyMetric(
		t,
		rpcLatencyAgain, "/proc/rpc-latency-again",
		"RPC latency again", units.Millisecond)

	var actual messages.Metric
	rpcLatencyAgain.UpdateJsonMetric(nil, &actual)

	if actual.Value.(*messages.Distribution).Median < 249 || actual.Value.(*messages.Distribution).Median >= 250 {
		t.Errorf("Median out of range: %f", actual.Value.(*messages.Distribution).Median)
	}

	// Check /proc/rpc-latency
	rpcLatency := root.GetMetric("/proc/rpc-latency")
	verifyMetric(
		t,
		rpcLatency, "/proc/rpc-latency",
		"RPC latency", units.Millisecond)
	var rpcLatencyForRpc messages.Metric
	rpcLatency.InitRpcMetric(nil, &rpcLatencyForRpc)
	if rpcLatencyForRpc.TimeStamp == nil {
		t.Error("Expect a timestamp for distributions")
	}
	if rpcLatencyForRpc.GroupId == 0 {
		t.Error("Expect non-zero groupId for distributions")
	}

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
	// Make timestamps and group id equal for now to do compare
	actual.TimeStamp = expected.TimeStamp
	actual.GroupId = expected.GroupId
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
	// Make timestamps and group id equal for compare
	actualRpc.TimeStamp = expectedRpc.TimeStamp
	actualRpc.GroupId = expectedRpc.GroupId
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
		"go",
		"io",
		"ipc",
		"memory",
		"name",
		"rpc-count",
		"rpc-latency",
		"rpc-latency-again",
		"scheduler",
		"signals",
		"some-time",
		"some-time-ptr",
		"start-time",
		"temperature",
		"test-start-time")

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
		"list",
		"nan",
		"secondGroup",
		"testargs",
		"testname",
		"times")

	UnregisterPath("firstGroup")

	verifyChildren(
		t,
		root.List(),
		"bytes",
		"list",
		"nan",
		"secondGroup",
		"testargs",
		"testname",
		"times")

	topLevel := Root()
	rootDir, _ := RegisterDirectory("/")
	if rootDir != topLevel {
		t.Error("Expected Root to return top level directory")
	}

	// This should be a no-op
	rootDir.UnregisterDirectory()

	verifyChildren(
		t,
		root.List(),
		"bytes",
		"list",
		"nan",
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
	dist := newDistributionWithTimeStamp(bucketer, true, kUsualTimeStamp)
	dist.SetUnit(units.None)
	assertValueEquals(t, kUsualTimeStamp, dist.Snapshot().TimeStamp)
	for i := 100; i >= 1; i-- {
		dist.AddWithTs(100.0, kUsualTimeStamp.Add(
			time.Duration(i)*time.Minute))
	}
	actual := dist.Snapshot()
	assertValueEquals(
		t, kUsualTimeStamp.Add(time.Minute), actual.TimeStamp)
	if actual.Median != 100.0 {
		t.Errorf("Expected median to be 100: %f", actual.Median)
	}
	for i := 100; i >= 1; i-- {
		dist.UpdateWithTs(
			100.0,
			float64(i),
			kUsualTimeStamp.Add(time.Hour+time.Duration(i)*time.Minute))
	}
	for i := 0; i < 100; i++ {
		dist.AddWithTs(100.0, kUsualTimeStamp.Add(
			time.Duration(i)*time.Minute))
		dist.RemoveWithTs(100.0, kUsualTimeStamp.Add(
			time.Duration(i)*time.Minute))
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
		TimeStamp:       kUsualTimeStamp.Add(99 * time.Minute),
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
	dist.SetUnit(units.None)
	dist.Add(200.0)
	dist.Add(300.0)
	snapshot := dist.Snapshot()
	// 2 points between 200 and 300
	assertValueEquals(t, 250.0, snapshot.Median)
}

func TestMedianDataAllHigh(t *testing.T) {
	bucketer := NewArbitraryBucketer(1000.0)
	dist := newDistribution(bucketer, false)
	dist.SetUnit(units.None)
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
	dist.SetUnit(units.None)
	dist.Add(7000.0)
	assertValueEquals(t, 7000.0, dist.Snapshot().Median)

	dist1 := newDistribution(bucketer, false)
	dist1.SetUnit(units.None)
	dist1.Add(1700.0)
	assertValueEquals(t, 1700.0, dist1.Snapshot().Median)

	dist2 := newDistribution(bucketer, false)
	dist2.SetUnit(units.None)
	dist2.Add(350.0)
	assertValueEquals(t, 350.0, dist2.Snapshot().Median)
}

func TestMedianAllDataInBetween(t *testing.T) {
	bucketer := NewArbitraryBucketer(500.0, 700.0, 1000.0, 3000.0)
	dist := newDistribution(bucketer, false)
	dist.SetUnit(units.None)
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
	dist.SetUnit(units.None)
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
	dist.SetUnit(units.None)
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

func TestInt64List(t *testing.T) {
	alist := newListWithTimeStamp(
		[]int64{2, 3, 5, 7},
		ImmutableSlice,
		kUsualTimeStamp)
	aSlice, ts := alist.AsSlice()
	assertValueDeepEquals(t, []int64{2, 3, 5, 7}, aSlice)
	assertValueEquals(t, kUsualTimeStamp, ts)
	assertValueEquals(t, types.Int64, alist.SubType())

	mutableInt64Slice := []int64{
		-1234567,
		-123456,
		-12345,
		-1234,
		-123,
		-12,
		-1,
		0,
		1,
		12,
		123,
		1234,
		12345,
		123456,
		1234567,
		3000,
		5000,
		7000,
	}
	alist.ChangeWithTimeStamp(
		mutableInt64Slice,
		MutableSlice,
		kUsualTimeStamp.Add(5*time.Hour))
	// Change the slice
	mutableInt64Slice[0] = 19683
	mutableInt64Slice[1] = 12167
	mutableInt64Slice[2] = 8192

	aSlice, ts = alist.AsSlice()
	assertValueDeepEquals(
		t,
		[]int64{
			-1234567,
			-123456,
			-12345,
			-1234,
			-123,
			-12,
			-1,
			0,
			1,
			12,
			123,
			1234,
			12345,
			123456,
			1234567,
			3000,
			5000,
			7000,
		},
		aSlice)
	assertValueEquals(t, kUsualTimeStamp.Add(5*time.Hour), ts)
	textStrings := alist.TextStrings(units.None)
	if assertValueEquals(t, 18, len(textStrings)) {
		assertValueEquals(t, "-1234567", textStrings[0])
		assertValueEquals(t, "-123456", textStrings[1])
		assertValueEquals(t, "-12345", textStrings[2])
		assertValueEquals(t, "-1234", textStrings[3])
		assertValueEquals(t, "-123", textStrings[4])
		assertValueEquals(t, "-12", textStrings[5])
		assertValueEquals(t, "-1", textStrings[6])
		assertValueEquals(t, "0", textStrings[7])
		assertValueEquals(t, "1", textStrings[8])
		assertValueEquals(t, "12", textStrings[9])
		assertValueEquals(t, "123", textStrings[10])
		assertValueEquals(t, "1234", textStrings[11])
		assertValueEquals(t, "12345", textStrings[12])
		assertValueEquals(t, "123456", textStrings[13])
		assertValueEquals(t, "1234567", textStrings[14])
		assertValueEquals(t, "3000", textStrings[15])
		assertValueEquals(t, "5000", textStrings[16])
		assertValueEquals(t, "7000", textStrings[17])
	}
	htmlStrings := alist.HtmlStrings(units.Celsius)
	if assertValueEquals(t, 18, len(htmlStrings)) {
		assertValueEquals(t, "-1.23 million", htmlStrings[0])
		assertValueEquals(t, "-123 thousand", htmlStrings[1])
		assertValueEquals(t, "-12.3 thousand", htmlStrings[2])
		assertValueEquals(t, "-1.23 thousand", htmlStrings[3])
		assertValueEquals(t, "-123", htmlStrings[4])
		assertValueEquals(t, "-12", htmlStrings[5])
		assertValueEquals(t, "-1", htmlStrings[6])
		assertValueEquals(t, "0", htmlStrings[7])
		assertValueEquals(t, "1", htmlStrings[8])
		assertValueEquals(t, "12", htmlStrings[9])
		assertValueEquals(t, "123", htmlStrings[10])
		assertValueEquals(t, "1.23 thousand", htmlStrings[11])
		assertValueEquals(t, "12.3 thousand", htmlStrings[12])
		assertValueEquals(t, "123 thousand", htmlStrings[13])
		assertValueEquals(t, "1.23 million", htmlStrings[14])
		assertValueEquals(t, "3.00 thousand", htmlStrings[15])
		assertValueEquals(t, "5.00 thousand", htmlStrings[16])
		assertValueEquals(t, "7.00 thousand", htmlStrings[17])
	}
	htmlStrings = alist.HtmlStrings(units.None)
	if assertValueEquals(t, 18, len(htmlStrings)) {
		assertValueEquals(t, "-1,234,567", htmlStrings[0])
		assertValueEquals(t, "-123,456", htmlStrings[1])
		assertValueEquals(t, "-12,345", htmlStrings[2])
		assertValueEquals(t, "-1,234", htmlStrings[3])
		assertValueEquals(t, "-123", htmlStrings[4])
		assertValueEquals(t, "-12", htmlStrings[5])
		assertValueEquals(t, "-1", htmlStrings[6])
		assertValueEquals(t, "0", htmlStrings[7])
		assertValueEquals(t, "1", htmlStrings[8])
		assertValueEquals(t, "12", htmlStrings[9])
		assertValueEquals(t, "123", htmlStrings[10])
		assertValueEquals(t, "1,234", htmlStrings[11])
		assertValueEquals(t, "12,345", htmlStrings[12])
		assertValueEquals(t, "123,456", htmlStrings[13])
		assertValueEquals(t, "1,234,567", htmlStrings[14])
		assertValueEquals(t, "3,000", htmlStrings[15])
		assertValueEquals(t, "5,000", htmlStrings[16])
		assertValueEquals(t, "7,000", htmlStrings[17])
	}
	htmlStrings = alist.HtmlStrings(units.Byte)
	if assertValueEquals(t, 18, len(htmlStrings)) {
		assertValueEquals(t, "2.93 KiB", htmlStrings[15])
		assertValueEquals(t, "4.88 KiB", htmlStrings[16])
		assertValueEquals(t, "6.84 KiB", htmlStrings[17])
	}
}

func TestBoolList(t *testing.T) {
	alist := newListWithTimeStamp(
		[]bool{false, true, true},
		ImmutableSlice,
		kUsualTimeStamp)
	aSlice, ts := alist.AsSlice()
	assertValueDeepEquals(t, []bool{false, true, true}, aSlice)
	assertValueEquals(t, kUsualTimeStamp, ts)
	assertValueEquals(t, types.Bool, alist.SubType())
	assertValueDeepEquals(
		t,
		[]string{"false", "true", "true"},
		alist.TextStrings(units.None))
	assertValueDeepEquals(
		t,
		[]string{"false", "true", "true"},
		alist.HtmlStrings(units.None))
}

func TestDurationList(t *testing.T) {
	alist := newListWithTimeStamp(
		[]time.Duration{time.Second, time.Minute},
		ImmutableSlice,
		kUsualTimeStamp)
	aSlice, ts := alist.AsSlice()
	assertValueDeepEquals(
		t, []time.Duration{time.Second, time.Minute}, aSlice)
	assertValueEquals(t, kUsualTimeStamp, ts)
	assertValueEquals(t, types.GoDuration, alist.SubType())
	assertValueDeepEquals(
		t,
		[]string{"1000.000000000", "60000.000000000"},
		alist.TextStrings(units.Millisecond))
	assertValueDeepEquals(
		t,
		[]string{"1.000000000", "60.000000000"},
		alist.TextStrings(units.Second))
	assertValueDeepEquals(
		t,
		[]string{"1.000000000", "60.000000000"},
		alist.TextStrings(units.None))
	assertValueDeepEquals(
		t,
		[]string{"1.000s", "1m 0.000s"},
		alist.HtmlStrings(units.Millisecond))
	assertValueDeepEquals(
		t,
		[]string{"1.000s", "1m 0.000s"},
		alist.HtmlStrings(units.Second))
	assertValueDeepEquals(
		t,
		[]string{"1.000s", "1m 0.000s"},
		alist.HtmlStrings(units.None))
}

func TestTimeList(t *testing.T) {
	alist := newListWithTimeStamp(
		[]time.Time{
			kUsualTimeStamp.Add(time.Hour),
			kUsualTimeStamp.Add(3 * time.Hour)},
		ImmutableSlice,
		kUsualTimeStamp)
	aSlice, ts := alist.AsSlice()
	assertValueDeepEquals(
		t,
		[]time.Time{
			kUsualTimeStamp.Add(time.Hour),
			kUsualTimeStamp.Add(3 * time.Hour)},
		aSlice)
	assertValueEquals(t, kUsualTimeStamp, ts)
	assertValueEquals(t, types.GoTime, alist.SubType())
	assertValueDeepEquals(
		t,
		[]string{
			"1463153255.000000000",
			"1463160455.000000000"},
		alist.TextStrings(units.None))
	assertValueDeepEquals(
		t,
		[]string{
			"2016-05-13T15:27:35Z",
			"2016-05-13T17:27:35Z"},
		alist.HtmlStrings(units.None))
}

func TestIntList(t *testing.T) {
	alist := newListWithTimeStamp(
		[]int{-36, -49},
		ImmutableSlice,
		kUsualTimeStamp.Add(time.Hour))
	aSlice, ts := alist.AsSlice()
	switch aSlice.(type) {
	case []int32:
		assertValueDeepEquals(t, []int32{-36, -49}, aSlice)
		assertValueEquals(t, types.Int32, alist.SubType())
	case []int64:
		assertValueDeepEquals(t, []int64{-36, -49}, aSlice)
		assertValueEquals(t, types.Int64, alist.SubType())
	default:
		t.Fatal("Expected int32 or int64")
	}
	assertValueEquals(t, kUsualTimeStamp.Add(time.Hour), ts)
	alist.ChangeWithTimeStamp(
		[]int{-100, -121, -144},
		ImmutableSlice,
		kUsualTimeStamp.Add(4*time.Hour))
	aSlice, ts = alist.AsSlice()
	switch aSlice.(type) {
	case []int32:
		assertValueDeepEquals(t, []int32{-100, -121, -144}, aSlice)
		assertValueEquals(t, types.Int32, alist.SubType())
	case []int64:
		assertValueDeepEquals(t, []int64{-100, -121, -144}, aSlice)
		assertValueEquals(t, types.Int64, alist.SubType())
	default:
		t.Fatal("Expected int32 or int64")
	}
	assertValueEquals(t, kUsualTimeStamp.Add(4*time.Hour), ts)
	textStrings := alist.TextStrings(units.None)
	if assertValueEquals(t, 3, len(textStrings)) {
		assertValueEquals(t, "-100", textStrings[0])
		assertValueEquals(t, "-121", textStrings[1])
		assertValueEquals(t, "-144", textStrings[2])
	}
	htmlStrings := alist.HtmlStrings(units.None)
	if assertValueEquals(t, 3, len(htmlStrings)) {
		assertValueEquals(t, "-100", htmlStrings[0])
		assertValueEquals(t, "-121", htmlStrings[1])
		assertValueEquals(t, "-144", htmlStrings[2])
	}
}

func TestUintList(t *testing.T) {
	alist := newListWithTimeStamp(
		[]uint{89, 144, 233, 377, 610},
		ImmutableSlice,
		kUsualTimeStamp.Add(7*time.Hour))
	aSlice, ts := alist.AsSlice()
	switch aSlice.(type) {
	case []uint32:
		assertValueDeepEquals(t, []uint32{89, 144, 233, 377, 610}, aSlice)
		assertValueEquals(t, types.Uint32, alist.SubType())
	case []uint64:
		assertValueDeepEquals(t, []uint64{89, 144, 233, 377, 610}, aSlice)
		assertValueEquals(t, types.Uint64, alist.SubType())
	default:
		t.Fatal("Expected uint32 or uint64")
	}
	assertValueEquals(t, kUsualTimeStamp.Add(7*time.Hour), ts)
	textStrings := alist.TextStrings(units.None)
	if assertValueEquals(t, 5, len(textStrings)) {
		assertValueEquals(t, "89", textStrings[0])
		assertValueEquals(t, "144", textStrings[1])
		assertValueEquals(t, "233", textStrings[2])
		assertValueEquals(t, "377", textStrings[3])
		assertValueEquals(t, "610", textStrings[4])
	}
	htmlStrings := alist.HtmlStrings(units.None)
	if assertValueEquals(t, 5, len(htmlStrings)) {
		assertValueEquals(t, "89", htmlStrings[0])
		assertValueEquals(t, "144", htmlStrings[1])
		assertValueEquals(t, "233", htmlStrings[2])
		assertValueEquals(t, "377", textStrings[3])
		assertValueEquals(t, "610", textStrings[4])
	}
}

func TestListNoChangeSubType(t *testing.T) {
	alist := newListWithTimeStamp(
		[]int64{2, 3, 5, 7},
		ImmutableSlice,
		kUsualTimeStamp)
	defer func() {
		if recover() != panicListSubTypeChanging {
			t.Error("Expected panic trying to change list sub-type")
		}
	}()
	alist.ChangeWithTimeStamp(
		[]int32{2, 3, 5, 7},
		ImmutableSlice,
		kUsualTimeStamp)
}

func TestNoAssignedUnitNoAdd(t *testing.T) {
	bucketer := NewGeometricBucketer(1, 1000)
	dist := newDistribution(bucketer, false)
	defer func() {
		if recover() != panicNoAssignedUnit {
			t.Error("Expected panicNoAssignedUnit adding to distribution with no assigned unit")
		}
	}()
	dist.Add(37.0)
}

func TestErrWrongType(t *testing.T) {
	var c complex128
	if err := RegisterMetric(
		"/a/metric/with/a/bad/type",
		&c,
		units.None,
		"Should not get registered, unsupported"); err != ErrWrongType {
		t.Error("Expected ErrWrongType")
	}
}

func TestErrWrongTypeFunc(t *testing.T) {
	c := func() (result complex128) {
		return
	}
	if err := RegisterMetric(
		"/a/metric/with/a/bad/type/again",
		c,
		units.None,
		"Should not get registered, unsupported"); err != ErrWrongType {
		t.Error("Expected ErrWrongType")
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
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit) {
	assertValueEquals(t, path, m.AbsPath())
	assertValueEquals(t, description, m.Description)
	assertValueEquals(t, unit, m.Unit())
}

func assertValueEquals(
	t *testing.T, expected, actual interface{}) bool {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
		return false
	}
	return true
}

func assertValueDeepEquals(
	t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func verifyJsonAndRpcDefaultTsGroupId(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	verifyJsonDefaultTsGroupId(
		t, m, path, description, unit, tp, bits, value)
	verifyRpcDefaultTsGroupId(
		t, m, path, description, unit, tp, bits, value)
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

func verifyJsonDefaultTsGroupId(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	_verifyMessage(
		t,
		m,
		path,
		description,
		unit,
		tp,
		bits,
		value,
		jsonMessage,
		true)
}

func verifyJson(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	_verifyMessage(
		t,
		m,
		path,
		description,
		unit,
		tp,
		bits,
		value,
		jsonMessage,
		false)
}

func verifyRpcDefaultTsGroupId(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	_verifyMessage(
		t,
		m,
		path,
		description,
		unit,
		tp,
		bits,
		value,
		rpcMessage,
		true)
}

func verifyRpc(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{}) {
	_verifyMessage(
		t,
		m,
		path,
		description,
		unit,
		tp,
		bits,
		value,
		rpcMessage,
		false)
}

func _verifyMessage(
	t *testing.T,
	m *metric,
	path, description string,
	unit units.Unit,
	tp types.Type,
	bits int,
	value interface{},
	mt messageType,
	defaultTsGroupId bool) {
	var ametric messages.Metric
	switch mt {
	case jsonMessage:
		m.InitJsonMetric(nil, &ametric)
	case rpcMessage:
		m.InitRpcMetric(nil, &ametric)
	default:
		panic("Invalid messageType")
	}
	assertValueEquals(t, path, ametric.Path)
	assertValueEquals(t, description, ametric.Description)
	assertValueEquals(t, unit, ametric.Unit)
	assertValueEquals(t, tp, ametric.Kind)
	assertValueEquals(t, bits, ametric.Bits)
	if reflect.TypeOf(value).Kind() == reflect.Slice {
		assertValueDeepEquals(t, value, ametric.Value)
	} else {
		assertValueEquals(t, value, ametric.Value)
	}
	if defaultTsGroupId {
		switch mt {
		case jsonMessage:
			assertValueEquals(
				t,
				"1463149655.000000000",
				ametric.TimeStamp)
		case rpcMessage:
			assertValueEquals(
				t,
				kUsualTimeStamp,
				ametric.TimeStamp)
		default:
			panic("Invalid messageType")
		}
		assertValueEquals(
			t, 0, ametric.GroupId)
	}
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

func zeroOutGroupId(list messages.MetricList) messages.MetricList {
	result := make(messages.MetricList, len(list))
	for i, ptr := range list {
		metric := *ptr
		metric.GroupId = 0
		result[i] = &metric
	}
	return result
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
