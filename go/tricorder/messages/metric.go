package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/duration"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"time"
)

func isJson(kind, subType types.Type) bool {
	switch kind {
	case types.GoDuration, types.GoTime:
		return false
	case types.List:
		switch subType {
		case types.GoDuration, types.GoTime:
			return false
		default:
			return true
		}
	default:
		return true
	}
}

func asJson(value interface{}, kind, subType types.Type, unit units.Unit) (
	jsonValue interface{}, jsonKind, jsonSubType types.Type) {
	switch kind {
	case types.GoDuration:
		jsonKind = types.Duration
		if value != nil {
			jsonValue = durationAsString(value.(time.Duration), unit)
		}
	case types.GoTime:
		jsonKind = types.Time
		if value != nil {
			jsonValue = timeAsString(value.(time.Time), unit)
		}
	case types.List:
		jsonKind = types.List
		switch subType {
		case types.GoDuration:
			jsonSubType = types.Duration
			if value != nil {
				durations := value.([]time.Duration)
				jsonDurations := make([]string, len(durations))
				for i := range jsonDurations {
					jsonDurations[i] = durationAsString(durations[i], unit)
				}
				jsonValue = jsonDurations
			}
		case types.GoTime:
			jsonSubType = types.Time
			if value != nil {
				times := value.([]time.Time)
				jsonTimes := make([]string, len(times))
				for i := range jsonTimes {
					jsonTimes[i] = timeAsString(times[i], unit)
				}
				jsonValue = jsonTimes
			}
		default:
			jsonSubType = subType
			jsonValue = value
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

func (m MetricList) asJson() (result MetricList) {
	result = m
	resultSameAsM := true
	for i := range m {
		if !m[i].IsJson() {
			if resultSameAsM {
				result = make(MetricList, len(m))
				copy(result, m)
				resultSameAsM = false
			}
			metric := *m[i]
			metric.ConvertToJson()
			result[i] = &metric
		}
	}
	return
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
