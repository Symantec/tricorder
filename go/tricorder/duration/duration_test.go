package duration

import (
	"github.com/Symantec/tricorder/go/tricorder/units"
	"testing"
	"time"
)

func TestConversions(t *testing.T) {
	atime := time.Date(2016, 5, 25, 16, 9, 59, 0, time.Local)
	assertValueEquals(t, 1464217799.0, TimeToFloat(atime))
	assertValueEquals(t, atime, FloatToTime(1464217799.0))
	assertValueEquals(
		t, 3.625, ToFloat(3*time.Second+625*time.Millisecond))
	assertValueEquals(
		t, 3*time.Second+625*time.Millisecond, FromFloat(3.625))
}

func TestDuration(t *testing.T) {
	var expected Duration
	if expected.IsNegative() {
		t.Error("Expected duration to be positive.")
	}
	var duration time.Duration
	actual := New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: 0, Nanoseconds: 1}
	duration = time.Nanosecond
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: 1, Nanoseconds: 0}
	if expected.IsNegative() {
		t.Error("Expected duration to be positive.")
	}
	duration = time.Second
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: 1, Nanoseconds: 999999999}
	duration = 2*time.Second - time.Nanosecond
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: 0, Nanoseconds: -1}
	if !expected.IsNegative() {
		t.Error("Expected duration to be negative.")
	}
	duration = -time.Nanosecond
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: -1, Nanoseconds: 0}
	if !expected.IsNegative() {
		t.Error("Expected duration to be negative.")
	}
	duration = -time.Second
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = Duration{Seconds: -1, Nanoseconds: -999999999}
	duration = -2*time.Second + time.Nanosecond
	actual = New(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
}

func TestTime(t *testing.T) {
	epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	var expected Duration
	tm := epoch
	actual := SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: 0, Nanoseconds: 1}
	tm = epoch.Add(time.Nanosecond)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: 1, Nanoseconds: 0}
	tm = epoch.Add(time.Second)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: 1, Nanoseconds: 999999999}
	tm = epoch.Add(2*time.Second - time.Nanosecond)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: 0, Nanoseconds: -1}
	tm = epoch.Add(-time.Nanosecond)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: -1, Nanoseconds: 0}
	tm = epoch.Add(-time.Second)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = Duration{Seconds: -1, Nanoseconds: -999999999}
	tm = epoch.Add(-2*time.Second + time.Nanosecond)
	actual = SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
}

func TestString(t *testing.T) {
	dur := Duration{Seconds: 57}
	if out := dur.String(); out != "57.000000000" {
		t.Errorf("Expected 57.000000000, got %s", out)
	}
	dur = Duration{Seconds: -53, Nanoseconds: -200000000}
	if out := dur.String(); out != "-53.200000000" {
		t.Errorf("Expected -53.200000000, got %s", out)
	}
	if out := dur.StringUsingUnits(units.Millisecond); out != "-53200.000000000" {
		t.Errorf("Expected -53200.000000000, got %s", out)
	}
	dur = Duration{Seconds: 53, Nanoseconds: 123456789}
	if out := dur.StringUsingUnits(units.Millisecond); out != "53123.456789000" {
		t.Errorf("Expected 53123.456789000, got %s", out)
	}
}

func TestPrettyFormat(t *testing.T) {
	var dur Duration
	assertStringEquals(t, "0ns", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 7}
	assertStringEquals(t, "7ns", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 9999}
	assertStringEquals(t, "9999ns", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 10000}
	assertStringEquals(t, "10μs", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 13789}
	assertStringEquals(t, "13μs", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 9999999}
	assertStringEquals(t, "9999μs", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 10000000}
	assertStringEquals(t, "10ms", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 678000000}
	assertStringEquals(t, "678ms", dur.PrettyFormat())
	dur = Duration{Nanoseconds: 999000000}
	assertStringEquals(t, "999ms", dur.PrettyFormat())
	dur = Duration{Seconds: 1}
	assertStringEquals(t, "1.000s", dur.PrettyFormat())
	dur = Duration{Seconds: 35, Nanoseconds: 871000000}
	assertStringEquals(t, "35.871s", dur.PrettyFormat())
	dur = Duration{Seconds: 59, Nanoseconds: 999000000}
	assertStringEquals(t, "59.999s", dur.PrettyFormat())
	dur = Duration{Seconds: 60}
	assertStringEquals(t, "1m 0.000s", dur.PrettyFormat())
	dur = Duration{Seconds: 3541, Nanoseconds: 10000000}
	assertStringEquals(t, "59m 1.010s", dur.PrettyFormat())
	dur = Duration{Seconds: 3600}
	assertStringEquals(t, "1h 0m 0s", dur.PrettyFormat())
	dur = Duration{Seconds: 83000}
	assertStringEquals(t, "23h 3m 20s", dur.PrettyFormat())
	dur = Duration{Seconds: 86400}
	assertStringEquals(t, "1d 0h 0m 0s", dur.PrettyFormat())
	dur = Duration{Seconds: 200000}
	assertStringEquals(t, "2d 7h 33m 20s", dur.PrettyFormat())
}

func TestParseWithUnit(t *testing.T) {
	dur, err := ParseWithUnit("-4326.1601", units.Second)
	if err != nil {
		t.Fatal(err)
	}
	assertStringEquals(t, "-4326.160100000", dur.String())
	dur, err = ParseWithUnit("8078.211436", units.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	assertStringEquals(t, "8.078211436", dur.String())
	_, err = ParseWithUnit("abcde", units.Second)
	if err == nil {
		t.Error("Expected error")
	}
}

func assertStringEquals(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func assertValueEquals(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
