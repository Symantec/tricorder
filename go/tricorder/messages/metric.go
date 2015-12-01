package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
	"time"
)

func (m *Metric) isJson(modify bool) bool {
	switch m.Kind {
	case types.GoDuration:
		if modify {
			m.Kind = types.Duration
			m.Value = m.durationAsString(m.Value.(time.Duration))
		}
		return false
	case types.GoTime:
		if modify {
			m.Kind = types.Time
			m.Value = m.timeAsString(m.Value.(time.Time))
		}
		return false
	default:
		return true
	}
}

func (m *Metric) durationAsString(godur time.Duration) string {
	return NewDuration(godur).StringUsingUnits(m.Unit)
}

func (m *Metric) timeAsString(gotime time.Time) string {
	var dur Duration
	if !gotime.IsZero() {
		dur = SinceEpoch(gotime)
	}
	return dur.StringUsingUnits(m.Unit)
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
