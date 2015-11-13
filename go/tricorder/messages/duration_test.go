package messages_test

import (
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	var expected messages.Duration
	var duration time.Duration
	actual := messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: 0, Nanoseconds: 1}
	duration = time.Nanosecond
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: 1, Nanoseconds: 0}
	duration = time.Second
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: 1, Nanoseconds: 999999999}
	duration = 2*time.Second - time.Nanosecond
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: 0, Nanoseconds: -1}
	duration = -time.Nanosecond
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: -1, Nanoseconds: 0}
	duration = -time.Second
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
	expected = messages.Duration{Seconds: -1, Nanoseconds: -999999999}
	duration = -2*time.Second + time.Nanosecond
	actual = messages.NewDuration(duration)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoDuration(); out != duration {
		t.Errorf("Expected %d, got %d", duration, out)
	}
}

func TestTime(t *testing.T) {
	epoch := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	var expected messages.Duration
	tm := epoch
	actual := messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: 0, Nanoseconds: 1}
	tm = epoch.Add(time.Nanosecond)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: 1, Nanoseconds: 0}
	tm = epoch.Add(time.Second)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: 1, Nanoseconds: 999999999}
	tm = epoch.Add(2*time.Second - time.Nanosecond)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: 0, Nanoseconds: -1}
	tm = epoch.Add(-time.Nanosecond)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: -1, Nanoseconds: 0}
	tm = epoch.Add(-time.Second)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
	expected = messages.Duration{Seconds: -1, Nanoseconds: -999999999}
	tm = epoch.Add(-2*time.Second + time.Nanosecond)
	actual = messages.SinceEpoch(tm)
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	if out := expected.AsGoTime().UTC(); out != tm {
		t.Errorf("Expected %d, got %d", tm, out)
	}
}

func TestString(t *testing.T) {
	dur := messages.Duration{Seconds: 57}
	if out := dur.String(); out != "57.000000000" {
		t.Errorf("Expected 57.000000000, got %s", out)
	}
	dur = messages.Duration{Seconds: -53, Nanoseconds: -200000000}
	if out := dur.String(); out != "-53.200000000" {
		t.Errorf("Expected -53.200000000, got %s", out)
	}
	if out := dur.StringUsingUnits(units.Millisecond); out != "-53200.000000" {
		t.Errorf("Expected -53200.000000, got %s", out)
	}
	dur = messages.Duration{Seconds: 53, Nanoseconds: 123456789}
	if out := dur.StringUsingUnits(units.Millisecond); out != "53123.456789" {
		t.Errorf("Expected 53123.456789, got %s", out)
	}
}
