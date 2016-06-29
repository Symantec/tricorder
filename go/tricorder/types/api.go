// Package types contains the various types for metric values.
package types

import (
	"github.com/Symantec/tricorder/go/tricorder/duration"
	"time"
)

// Type represents the type of a metric value
type Type string

const (
	Unknown    Type = ""
	Bool       Type = "bool"
	Int8       Type = "int8"
	Int16      Type = "int16"
	Int32      Type = "int32"
	Int64      Type = "int64"
	Uint8      Type = "uint8"
	Uint16     Type = "uint16"
	Uint32     Type = "uint32"
	Uint64     Type = "uint64"
	Float32    Type = "float32"
	Float64    Type = "float64"
	String     Type = "string"
	GoTime     Type = "goTime"
	GoDuration Type = "goDuration"
	Dist       Type = "distribution"
	List       Type = "list"
	// for JSON RPC only
	Time Type = "time"
	// for JSON RPC only
	Duration Type = "duration"
)

// FromGoValue returns the type of a value found in the GoRPC API.
// FromGoValue returns Unknown if it cannot determine the type.
// In case value is a slice of unknown type, FromGoValue returns Unknown
// rather than List.
func FromGoValue(value interface{}) Type {
	kind, _ := FromGoValueWithSubType(value)
	return kind
}

// FromGoValueWithSubType returns both the type and sub-type of the value
// found in the GoRPC API.
// FromGoValueWithSubType returns Unknown, Unknown if it cannot determine the
// type.
// In case value is a slice of an unknown type, FromGoValueWithSubType returns
// Unknown, Unknown rather than List, Unknown.
func FromGoValueWithSubType(value interface{}) (kind, subType Type) {
	switch i := value.(type) {
	case bool:
		kind = Bool
	case int8:
		kind = Int8
	case int16:
		kind = Int16
	case int32:
		kind = Int32
	case int64:
		kind = Int64
	case uint8:
		kind = Uint8
	case uint16:
		kind = Uint16
	case uint32:
		kind = Uint32
	case uint64:
		kind = Uint64
	case float32:
		kind = Float32
	case float64:
		kind = Float64
	case string:
		kind = String
	case time.Time:
		kind = GoTime
	case time.Duration:
		kind = GoDuration
	case goValue:
		kind = i.Type()
	case []bool:
		kind = List
		subType = Bool
	case []int8:
		kind = List
		subType = Int8
	case []int16:
		kind = List
		subType = Int16
	case []int32:
		kind = List
		subType = Int32
	case []int64:
		kind = List
		subType = Int64
	case []uint8:
		kind = List
		subType = Uint8
	case []uint16:
		kind = List
		subType = Uint16
	case []uint32:
		kind = List
		subType = Uint32
	case []uint64:
		kind = List
		subType = Uint64
	case []float32:
		kind = List
		subType = Float32
	case []float64:
		kind = List
		subType = Float64
	case []string:
		kind = List
		subType = String
	case []time.Time:
		kind = List
		subType = GoTime
	case []time.Duration:
		kind = List
		subType = GoDuration
	default:
	}
	return
}

// ZeroValue returns the zero value for this type.
// ZeroValue panics if this type is Dist or List.
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
	case List:
		panic("List type cannot create new value.")
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

// NilSlice returns the nil slice of this type.
// NilSlice panics if this type is Dist or List.
func (t Type) NilSlice() interface{} {
	switch t {
	case Bool:
		var result []bool
		return result
	case Int8:
		var result []int8
		return result
	case Int16:
		var result []int16
		return result
	case Int32:
		var result []int32
		return result
	case Int64:
		var result []int64
		return result
	case Uint8:
		var result []uint8
		return result
	case Uint16:
		var result []uint16
		return result
	case Uint32:
		var result []uint32
		return result
	case Uint64:
		var result []uint64
		return result
	case Float32:
		var result []float32
		return result
	case Float64:
		var result []float64
		return result
	case String:
		var result []string
		return result
	case Dist:
		panic("Dist type cannot create new value.")
	case List:
		panic("List type cannot create new value.")
	case Time, Duration:
		var result []string
		return result
	case GoTime:
		var result []time.Time
		return result
	case GoDuration:
		var result []time.Duration
		return result
	default:
		panic("Unknown type")

	}
}

// CanToFromFloat returns true if this type supports conversion to/from float64
func (t Type) CanToFromFloat() bool {
	switch t {
	case Bool, String, Dist, List, Time, Duration:
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

// UsesSubType returns true if this type uses a sub-type.
func (t Type) UsesSubType() bool {
	return t == List
}

// SupportsEquality returns true if this type supports equality.
func (t Type) SupportsEquality() bool {
	return t != List && t != Dist
}
