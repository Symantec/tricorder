package messages

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/duration"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"reflect"
	"time"
)

func valueAsString(value interface{}) (valueStr string, err error) {
	valueStr, ok := value.(string)
	if !ok {
		err = fmt.Errorf("Value '%v' not a string", value)
	}
	return
}

func valueAsStringSlice(value interface{}) (valueSlice []string, err error) {
	valueSlice, ok := value.([]string)
	if !ok {
		err = fmt.Errorf("Value '%v' not a string slice", value)
	}
	return
}

func asGoRPC(value interface{}, kind, subType types.Type, unit units.Unit) (
	goRPCValue interface{}, goRPCKind, goRPCSubType types.Type, err error) {
	switch kind {
	case types.Duration:
		var valueStr string
		if valueStr, err = valueAsString(value); err != nil {
			return
		}
		goRPCValue, err = stringAsDuration(valueStr, unit)
		if err != nil {
			return
		}
		goRPCKind = types.GoDuration
	case types.Time:
		var valueStr string
		if valueStr, err = valueAsString(value); err != nil {
			return
		}
		goRPCValue, err = stringAsTime(valueStr, unit)
		if err != nil {
			return
		}
		goRPCKind = types.GoTime
	case types.List:
		switch subType {
		case types.Duration:
			var valueSlice []string
			if valueSlice, err = valueAsStringSlice(value); err != nil {
				return
			}
			goRPCDurations := make([]time.Duration, len(valueSlice))
			for i := range goRPCDurations {
				goRPCDurations[i], err = stringAsDuration(valueSlice[i], unit)
				if err != nil {
					return
				}
			}
			goRPCSubType = types.GoDuration
			goRPCValue = goRPCDurations
		case types.Time:
			var valueSlice []string
			if valueSlice, err = valueAsStringSlice(value); err != nil {
				return
			}
			goRPCTimes := make([]time.Time, len(valueSlice))
			for i := range goRPCTimes {
				goRPCTimes[i], err = stringAsTime(valueSlice[i], unit)
				if err != nil {
					return
				}
			}
			goRPCSubType = types.GoTime
			goRPCValue = goRPCTimes
		default:
			goRPCSubType = subType
			goRPCValue = value
		}
		goRPCKind = types.List
	default:
		goRPCKind = kind
		goRPCValue = value
	}
	return
}

func asJson(value interface{}, kind, subType types.Type, unit units.Unit) (
	jsonValue interface{}, jsonKind, jsonSubType types.Type) {
	// asJson does not return an error like asGoRPC does. The reason is that
	// asGoRPC gets its input from incoming JSON which may have kind fields
	// and value fields that do not match. asJson on the other hand receives
	// its inputs from only our code for now, so it should panic if the
	// inputs aren't consistent. For instance, when scotty reads metrics
	// from GoRPC, it ignores the kind field and derives the kind and sub
	// type from what is stored in the value field.
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
	switch i := m.TimeStamp.(type) {
	case nil:
		m.TimeStamp = ""
	case time.Time:
		m.TimeStamp = duration.SinceEpoch(i).String()
	case string:
		// Do nothing we are already in json format
	default:
		m.TimeStamp = fmt.Sprintf("%v", i)
	}
}

func (m *Metric) convertToGoRPC() error {
	v, k, s, err := asGoRPC(m.Value, m.Kind, m.SubType, m.Unit)
	if err != nil {
		return err
	}
	var newTimeStamp interface{}
	switch i := m.TimeStamp.(type) {
	case nil, time.Time:
		// already in go rpc format
		newTimeStamp = i
	case string:
		if i == "" {
			newTimeStamp = nil
		} else {
			newTimeStamp, err = stringAsTime(i, units.Second)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("Unrecognised value in timestamp field: '%v'", i)
	}
	m.Value, m.Kind, m.SubType = v, k, s
	m.TimeStamp = newTimeStamp
	return nil
}

func timeAsString(gotime time.Time, unit units.Unit) string {
	var dur duration.Duration
	if !gotime.IsZero() {
		dur = duration.SinceEpoch(gotime)
	}
	return dur.StringUsingUnits(unit)
}

func stringAsTime(timeStr string, unit units.Unit) (
	result time.Time, err error) {
	dur, err := duration.ParseWithUnit(timeStr, unit)
	if err != nil {
		return
	}
	result = dur.AsGoTime()
	return
}

func durationAsString(godur time.Duration, unit units.Unit) string {
	return duration.New(godur).StringUsingUnits(unit)
}

func stringAsDuration(durationStr string, unit units.Unit) (
	result time.Duration, err error) {
	dur, err := duration.ParseWithUnit(durationStr, unit)
	if err != nil {
		return
	}
	result = dur.AsGoDuration()
	return
}
