package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/units"
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	var expected duration
	if expected.IsNegative() {
		t.Error("Expected duration to be positive.")
	}
	var dur time.Duration
	actual := newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: 0, Nanoseconds: 1}
	dur = time.Nanosecond
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: 1, Nanoseconds: 0}
	if expected.IsNegative() {
		t.Error("Expected duration to be positive.")
	}
	dur = time.Second
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: 1, Nanoseconds: 999999999}
	dur = 2*time.Second - time.Nanosecond
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: 0, Nanoseconds: -1}
	if !expected.IsNegative() {
		t.Error("Expected duration to be negative.")
	}
	dur = -time.Nanosecond
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: -1, Nanoseconds: 0}
	if !expected.IsNegative() {
		t.Error("Expected duration to be negative.")
	}
	dur = -time.Second
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
	expected = duration{Seconds: -1, Nanoseconds: -999999999}
	dur = -2*time.Second + time.Nanosecond
	actual = newDuration(dur)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != dur {
		t.Errorf("Expected %d, got %d", dur, out)
	}
}

func TestTime(t *testing.T) {
	epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	var expected duration
	tm := epoch
	actual := durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: 0, Nanoseconds: 1}
	tm = epoch.Add(time.Nanosecond)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: 1, Nanoseconds: 0}
	tm = epoch.Add(time.Second)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: 1, Nanoseconds: 999999999}
	tm = epoch.Add(2*time.Second - time.Nanosecond)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: 0, Nanoseconds: -1}
	tm = epoch.Add(-time.Nanosecond)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: -1, Nanoseconds: 0}
	tm = epoch.Add(-time.Second)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = duration{Seconds: -1, Nanoseconds: -999999999}
	tm = epoch.Add(-2*time.Second + time.Nanosecond)
	actual = durationSinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
}

func TestString(t *testing.T) {
	dur := duration{Seconds: 57}
	if out := dur.String(); out != "57.000000000" {
		t.Errorf("Expected 57.000000000, got %s", out)
	}
	dur = duration{Seconds: -53, Nanoseconds: -200000000}
	if out := dur.String(); out != "-53.200000000" {
		t.Errorf("Expected -53.200000000, got %s", out)
	}
	if out := dur.StringUsingUnits(units.Millisecond); out != "-53200.000000" {
		t.Errorf("Expected -53200.000000, got %s", out)
	}
	dur = duration{Seconds: 53, Nanoseconds: 123456789}
	if out := dur.StringUsingUnits(units.Millisecond); out != "53123.456789" {
		t.Errorf("Expected 53123.456789, got %s", out)
	}
}

func TestPrettyFormat(t *testing.T) {
	var dur duration
	assertStringEquals(t, "0ns", dur.PrettyFormat())
	dur = duration{Nanoseconds: 7}
	assertStringEquals(t, "7ns", dur.PrettyFormat())
	dur = duration{Nanoseconds: 9999}
	assertStringEquals(t, "9999ns", dur.PrettyFormat())
	dur = duration{Nanoseconds: 10000}
	assertStringEquals(t, "10μs", dur.PrettyFormat())
	dur = duration{Nanoseconds: 13789}
	assertStringEquals(t, "13μs", dur.PrettyFormat())
	dur = duration{Nanoseconds: 9999999}
	assertStringEquals(t, "9999μs", dur.PrettyFormat())
	dur = duration{Nanoseconds: 10000000}
	assertStringEquals(t, "10ms", dur.PrettyFormat())
	dur = duration{Nanoseconds: 678000000}
	assertStringEquals(t, "678ms", dur.PrettyFormat())
	dur = duration{Nanoseconds: 999000000}
	assertStringEquals(t, "999ms", dur.PrettyFormat())
	dur = duration{Seconds: 1}
	assertStringEquals(t, "1.000s", dur.PrettyFormat())
	dur = duration{Seconds: 35, Nanoseconds: 871000000}
	assertStringEquals(t, "35.871s", dur.PrettyFormat())
	dur = duration{Seconds: 59, Nanoseconds: 999000000}
	assertStringEquals(t, "59.999s", dur.PrettyFormat())
	dur = duration{Seconds: 60}
	assertStringEquals(t, "1m 0.000s", dur.PrettyFormat())
	dur = duration{Seconds: 3541, Nanoseconds: 10000000}
	assertStringEquals(t, "59m 1.010s", dur.PrettyFormat())
	dur = duration{Seconds: 3600}
	assertStringEquals(t, "1h 0m 0s", dur.PrettyFormat())
	dur = duration{Seconds: 83000}
	assertStringEquals(t, "23h 3m 20s", dur.PrettyFormat())
	dur = duration{Seconds: 86400}
	assertStringEquals(t, "1d 0h 0m 0s", dur.PrettyFormat())
	dur = duration{Seconds: 200000}
	assertStringEquals(t, "2d 7h 33m 20s", dur.PrettyFormat())
}

func assertStringEquals(t *testing.T, expected, actual string) {
	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
