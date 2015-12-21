package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"time"
)

func isJson(kind types.Type) bool {
	switch kind {
	case types.GoDuration, types.GoTime:
		return false
	default:
		return true
	}
}

func asJson(value interface{}, kind types.Type, unit units.Unit) (
	jsonValue interface{}, jsonKind types.Type) {
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
	default:
		jsonKind = kind
		jsonValue = value
	}
	return
}

func (m *Metric) convertToJson() {
	m.Value, m.Kind = asJson(m.Value, m.Kind, m.Unit)
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
	var dur Duration
	if !gotime.IsZero() {
		dur = SinceEpoch(gotime)
	}
	return dur.StringUsingUnits(unit)
}

func durationAsString(godur time.Duration, unit units.Unit) string {
	return NewDuration(godur).StringUsingUnits(unit)
}
