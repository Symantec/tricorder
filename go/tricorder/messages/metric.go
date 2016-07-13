package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/duration"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"time"
)

func asJson(value interface{}, kind, subType types.Type, unit units.Unit) (
	jsonValue interface{}, jsonKind, jsonSubType types.Type) {
	switch kind {
	case types.GoDuration:
		jsonKind = types.Duration
		jsonValue = durationAsString(value.(time.Duration), unit)
	case types.GoTime:
		jsonKind = types.Time
		jsonValue = timeAsString(value.(time.Time), unit)
	case types.List:
		jsonKind = types.List
		switch subType {
		case types.GoDuration:
			jsonSubType = types.Duration
			durations := value.([]time.Duration)
			jsonDurations := make([]string, len(durations))
			for i := range jsonDurations {
				jsonDurations[i] = durationAsString(
					durations[i], unit)
			}
			jsonValue = jsonDurations
		case types.GoTime:
			jsonSubType = types.Time
			times := value.([]time.Time)
			jsonTimes := make([]string, len(times))
			for i := range jsonTimes {
				jsonTimes[i] = timeAsString(times[i], unit)
			}
			jsonValue = jsonTimes
		default:
			jsonSubType = subType
			if reflect.ValueOf(value).IsNil() {
				jsonValue = reflect.MakeSlice(reflect.TypeOf(value), 0, 0).Interface()
			} else {
				jsonValue = value
			}
		}
	default:
		jsonKind = kind
		jsonValue = value
	}
	return
}

func (m *Metric) convertToJson() {
	m.Value, m.Kind, m.SubType = asJson(m.Value, m.Kind, m.SubType, m.Unit)
	if m.TimeStamp == nil {
		m.TimeStamp = ""
	} else {
		m.TimeStamp = duration.SinceEpoch(m.TimeStamp.(time.Time)).String()
	}
}

func timeAsString(gotime time.Time, unit units.Unit) string {
	var dur duration.Duration
	if !gotime.IsZero() {
		dur = duration.SinceEpoch(gotime)
	}
	return dur.StringUsingUnits(unit)
}

func durationAsString(godur time.Duration, unit units.Unit) string {
	return duration.New(godur).StringUsingUnits(unit)
}
