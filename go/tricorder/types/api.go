// Package types contains the various types for metric values.
package types

import (
	"github.com/Symantec/tricorder/go/tricorder/duration"
	"time"
)

// Type represents the type of a metric value
type Type string

const (
	Unknown Type = ""
	Bool    Type = "bool"
	Int8    Type = "int8"
	Int16   Type = "int16"
	Int32   Type = "int32"
	Int64   Type = "int64"
	Uint8   Type = "uint8"
	Uint16  Type = "uint16"
	Uint32  Type = "uint32"
	Uint64  Type = "uint64"
	Float32 Type = "float32"
	Float64 Type = "float64"
	String  Type = "string"
	Dist    Type = "distribution"
	// for JSON RPC
	Time Type = "time"
	// for JSON RPC
	Duration Type = "duration"

	// for GoRPC
	GoTime Type = "goTime"
	// for GoRPC
	GoDuration Type = "goDuration"
)

// FromGoValue returns the type of a value found in the GoRPC API.
// FromGoValue returns unknown if it cannot determine the type.
func FromGoValue(value interface{}) Type {
	switch i := value.(type) {
	case bool:
		return Bool
	case int8:
		return Int8
	case int16:
		return Int16
	case int32:
		return Int32
	case int64:
		return Int64
	case uint8:
		return Uint8
	case uint16:
		return Uint16
	case uint32:
		return Uint32
	case uint64:
		return Uint64
	case float32:
		return Float32
	case float64:
		return Float64
	case string:
		return String
	case time.Time:
		return GoTime
	case time.Duration:
		return GoDuration
	case goValue:
		return i.Type()
	default:
		return Unknown
	}
}

// ZeroValue returns the zero value for this type.
// ZeroValue panics if this type is Dist.
func (t Type) ZeroValue() interface{} {
	switch t {
	case Bool:
		return false
	case Int8:
		return int8(0)
	case Int16:
		return int16(0)
	case Int32:
		return int32(0)
	case Int64:
		return int64(0)
	case Uint8:
		return uint8(0)
	case Uint16:
		return uint16(0)
	case Uint32:
		return uint32(0)
	case Uint64:
		return uint64(0)
	case Float32:
		return float32(0)
	case Float64:
		return float64(0)
	case String:
		return ""
	case Dist:
		panic("Dist type cannot create new value.")
	case Time, Duration:
		return "0.000000000"
	case GoTime:
		return time.Time{}
	case GoDuration:
		return time.Duration(0)
	default:
		panic("Unknown type")
	}
}

// CanToFromFloat returns true if this type supports conversion to/from float64
func (t Type) CanToFromFloat() bool {
	switch t {
	case Bool, String, Dist, Time, Duration:
		return false
	case Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64, Float32, Float64, GoTime, GoDuration:
		return true
	default:
		panic("Unknown type")
	}
}

// FromFloat converts a float64 to a value according to this type
// FromFloat panics if this type doesn't support conversion from float64
func (t Type) FromFloat(value float64) interface{} {
	switch t {
	case Int8:
		return int8(round(value))
	case Int16:
		return int16(round(value))
	case Int32:
		return int32(round(value))
	case Int64:
		return int64(round(value))
	case Uint8:
		return uint8(round(value))
	case Uint16:
		return uint16(round(value))
	case Uint32:
		return uint32(round(value))
	case Uint64:
		return uint64(round(value))
	case Float32:
		return float32(value)
	case Float64:
		return value
	case GoTime:
		return duration.FloatToTime(value)
	case GoDuration:
		return duration.FromFloat(value)
	default:
		panic("Type doesn't support converstion from a float")
	}
}

// ToFloat converts a value of this type to a float64
// ToFloat panics if this type doesn't support conversion to float64
func (t Type) ToFloat(x interface{}) float64 {
	switch t {
	case Int8:
		return float64(x.(int8))
	case Int16:
		return float64(x.(int16))
	case Int32:
		return float64(x.(int32))
	case Int64:
		return float64(x.(int64))
	case Uint8:
		return float64(x.(uint8))
	case Uint16:
		return float64(x.(uint16))
	case Uint32:
		return float64(x.(uint32))
	case Uint64:
		return float64(x.(uint64))
	case Float32:
		return float64(x.(float32))
	case Float64:
		return x.(float64)
	case GoTime:
		return duration.TimeToFloat(x.(time.Time))
	case GoDuration:
		return duration.ToFloat(x.(time.Duration))
	default:
		panic("Type doesn't support conversion to float.")
	}
}

func (t Type) IsInt() bool {
	return t == Int8 || t == Int16 || t == Int32 || t == Int64
}

func (t Type) IsUint() bool {
	return t == Uint8 || t == Uint16 || t == Uint32 || t == Uint64
}

func (t Type) IsFloat() bool {
	return t == Float32 || t == Float64
}

func (t Type) Bits() int {
	switch t {
	case Int8, Uint8:
		return 8
	case Int16, Uint16:
		return 16
	case Int32, Uint32, Float32:
		return 32
	case Int64, Uint64, Float64:
		return 64
	default:
		return 0
	}
}
