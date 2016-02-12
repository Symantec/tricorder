// Package types contains the various types for metric values.
package types

// Type represents the type of a metric value
type Type string

const (
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
