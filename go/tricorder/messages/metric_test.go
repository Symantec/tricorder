package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"testing"
	"time"
)

var (
	kUsualTime    = time.Date(2016, 7, 13, 20, 35, 0, 0, time.UTC)
	kUsualTimeStr = "1468442100.000000000"
)

func TestPlainNoTs(t *testing.T) {
	// For now, the only fields that affect ConvertToJSON are
	// Value, Kind, SubType, and Unit
	metric := Metric{
		Value: int64(69),
		Kind:  types.Int64,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     int64(69),
		Kind:      types.Int64,
		TimeStamp: "",
	}
	assertValueEquals(t, expected, metric)
}

func TestPlainTs(t *testing.T) {
	metric := Metric{
		Value:     int64(69),
		Kind:      types.Int64,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     int64(69),
		Kind:      types.Int64,
		TimeStamp: kUsualTimeStr,
	}
	assertValueEquals(t, expected, metric)
}

func TestDurationMillis(t *testing.T) {
	metric := Metric{
		Value:     16*time.Minute + 357*time.Millisecond,
		Kind:      types.GoDuration,
		Unit:      units.Millisecond,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     "960357.000000",
		Kind:      types.Duration,
		Unit:      units.Millisecond,
		TimeStamp: kUsualTimeStr,
	}
	assertValueEquals(t, expected, metric)
}

func TestDuration(t *testing.T) {
	metric := Metric{
		Value:     16*time.Minute + 357*time.Millisecond,
		Kind:      types.GoDuration,
		Unit:      units.Second,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     "960.357000000",
		Kind:      types.Duration,
		Unit:      units.Second,
		TimeStamp: kUsualTimeStr,
	}
	assertValueEquals(t, expected, metric)
}

func TestTimeMillis(t *testing.T) {
	metric := Metric{
		Value: kUsualTime.Add(
			16*time.Second + 924*time.Millisecond),
		Kind:      types.GoTime,
		Unit:      units.Millisecond,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     "1468442116924.000000",
		Kind:      types.Time,
		Unit:      units.Millisecond,
		TimeStamp: kUsualTimeStr,
	}
	assertValueEquals(t, expected, metric)
}

func TestTimeSeconds(t *testing.T) {
	metric := Metric{
		Value: kUsualTime.Add(
			16*time.Second + 924*time.Millisecond),
		Kind:      types.GoTime,
		Unit:      units.Second,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     "1468442116.924000000",
		Kind:      types.Time,
		Unit:      units.Second,
		TimeStamp: kUsualTimeStr,
	}
	assertValueEquals(t, expected, metric)
}

func TestNilSlice(t *testing.T) {
	var nilSlice []int32
	metric := Metric{
		Value:     nilSlice,
		Kind:      types.List,
		SubType:   types.Int32,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     []int32{},
		Kind:      types.List,
		SubType:   types.Int32,
		TimeStamp: kUsualTimeStr,
	}
	assertDeepEquals(t, expected, metric)
}

func TestDurationSlice(t *testing.T) {
	metric := Metric{
		Value:     []time.Duration{631 * time.Millisecond},
		Kind:      types.List,
		Unit:      units.Second,
		SubType:   types.GoDuration,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     []string{"0.631000000"},
		Kind:      types.List,
		Unit:      units.Second,
		SubType:   types.Duration,
		TimeStamp: kUsualTimeStr,
	}
	assertDeepEquals(t, expected, metric)
}

func TestTimeSlice(t *testing.T) {
	metric := Metric{
		Value:     []time.Time{kUsualTime.Add(453 * time.Millisecond)},
		Kind:      types.List,
		Unit:      units.Second,
		SubType:   types.GoTime,
		TimeStamp: kUsualTime,
	}
	metric.ConvertToJson()
	expected := Metric{
		Value:     []string{"1468442100.453000000"},
		Kind:      types.List,
		Unit:      units.Second,
		SubType:   types.Time,
		TimeStamp: kUsualTimeStr,
	}
	assertDeepEquals(t, expected, metric)
}

func assertValueEquals(t *testing.T, expected, actual interface{}) bool {
	if expected != actual {
		t.Errorf("Expected %+v, got %+v", expected, actual)
		return false
	}
	return true
}

func assertDeepEquals(t *testing.T, expected, actual interface{}) bool {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v, got %+v", expected, actual)
		return false
	}
	return true
}
