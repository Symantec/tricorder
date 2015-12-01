package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
	"time"
)

func (m *Metric) toJson() {
	switch m.Kind {
	case types.GoDuration:
		m.Kind = types.Duration
		m.Value = m.durationAsString(m.Value.(time.Duration))
	case types.GoTime:
		m.Kind = types.Time
		m.Value = m.timeAsString(m.Value.(time.Time))
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
