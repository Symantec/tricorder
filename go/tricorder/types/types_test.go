package types_test

import (
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"github.com/Symantec/tricorder/go/tricorder/types"
	"testing"
	"time"
)

func TestZeroValue(t *testing.T) {
	assertValueEquals(t, false, types.Bool.ZeroValue())
	assertValueEquals(t, int8(0), types.Int8.ZeroValue())
	assertValueEquals(t, int16(0), types.Int16.ZeroValue())
	assertValueEquals(t, int32(0), types.Int32.ZeroValue())
	assertValueEquals(t, int64(0), types.Int64.ZeroValue())
	assertValueEquals(t, uint8(0), types.Uint8.ZeroValue())
	assertValueEquals(t, uint16(0), types.Uint16.ZeroValue())
	assertValueEquals(t, uint32(0), types.Uint32.ZeroValue())
	assertValueEquals(t, uint64(0), types.Uint64.ZeroValue())
	assertValueEquals(t, float32(0), types.Float32.ZeroValue())
	assertValueEquals(t, 0.0, types.Float64.ZeroValue())
	assertValueEquals(t, "", types.String.ZeroValue())
}

func TestCanToFromFloat(t *testing.T) {
	assertValueEquals(t, false, types.Bool.CanToFromFloat())
	assertValueEquals(t, true, types.Int8.CanToFromFloat())
	assertValueEquals(t, true, types.Int16.CanToFromFloat())
	assertValueEquals(t, true, types.Int32.CanToFromFloat())
	assertValueEquals(t, true, types.Int64.CanToFromFloat())
	assertValueEquals(t, true, types.Uint8.CanToFromFloat())
	assertValueEquals(t, true, types.Uint16.CanToFromFloat())
	assertValueEquals(t, true, types.Uint32.CanToFromFloat())
	assertValueEquals(t, true, types.Uint64.CanToFromFloat())
	assertValueEquals(t, true, types.Float32.CanToFromFloat())
	assertValueEquals(t, true, types.Float64.CanToFromFloat())
	assertValueEquals(t, false, types.String.CanToFromFloat())
	assertValueEquals(t, false, types.Dist.CanToFromFloat())
}

func TestToFloat(t *testing.T) {
	assertValueEquals(t, -128.0, types.Int8.ToFloat(int8(-128)))
	assertValueEquals(t, 127.0, types.Int8.ToFloat(int8(127)))
	assertValueEquals(t, 55.0, types.Int8.ToFloat(int8(55)))
	assertValueEquals(t, -32768.0, types.Int16.ToFloat(int16(-32768)))
	assertValueEquals(t, 32767.0, types.Int16.ToFloat(int16(32767)))
	assertValueEquals(t, 55.0, types.Int16.ToFloat(int16(55)))
	assertValueEquals(t, -2147483648.0, types.Int32.ToFloat(int32(-2147483648)))
	assertValueEquals(t, 2147483647.0, types.Int32.ToFloat(int32(2147483647)))
	assertValueEquals(t, 55.0, types.Int32.ToFloat(int32(55)))
	assertValueEquals(t, 12345678901.0, types.Int64.ToFloat(int64(12345678901)))
	assertValueEquals(t, -12345678901.0, types.Int64.ToFloat(int64(-12345678901)))
	assertValueEquals(t, 0.0, types.Uint8.ToFloat(uint8(0)))
	assertValueEquals(t, 255.0, types.Uint8.ToFloat(uint8(255)))
	assertValueEquals(t, 55.0, types.Uint8.ToFloat(uint8(55)))
	assertValueEquals(t, 0.0, types.Uint16.ToFloat(uint16(0)))
	assertValueEquals(t, 65535.0, types.Uint16.ToFloat(uint16(65535)))
	assertValueEquals(t, 55.0, types.Uint16.ToFloat(uint16(55)))
	assertValueEquals(t, 0.0, types.Uint32.ToFloat(uint32(0)))
	assertValueEquals(t, 4294967295.0, types.Uint32.ToFloat(uint32(4294967295)))
	assertValueEquals(t, 55.0, types.Uint32.ToFloat(uint32(55)))
	assertValueEquals(t, 12345678901.0, types.Uint64.ToFloat(uint64(12345678901)))
	assertValueEquals(t, 0.0, types.Uint64.ToFloat(uint64(0)))

	assertValueEquals(t, 69.25, types.Float32.ToFloat(float32(69.25)))
	assertValueEquals(t, -38.375, types.Float32.ToFloat(float32(-38.375)))
	assertValueEquals(t, 69.25, types.Float64.ToFloat(69.25))
	assertValueEquals(t, -38.375, types.Float64.ToFloat(-38.375))
}

func TestFromGoValue(t *testing.T) {
	assertValueEquals(t, types.Bool, types.FromGoValue(true))
	assertValueEquals(t, types.Int8, types.FromGoValue(int8(0)))
	assertValueEquals(t, types.Int16, types.FromGoValue(int16(32)))
	assertValueEquals(t, types.Int32, types.FromGoValue(int32(47)))
	assertValueEquals(t, types.Int64, types.FromGoValue(int64(50)))
	assertValueEquals(t, types.Uint8, types.FromGoValue(uint8(0)))
	assertValueEquals(t, types.Uint16, types.FromGoValue(uint16(32)))
	assertValueEquals(t, types.Uint32, types.FromGoValue(uint32(47)))
	assertValueEquals(t, types.Uint64, types.FromGoValue(uint64(50)))
	assertValueEquals(t, types.Float32, types.FromGoValue(float32(9.24)))
	assertValueEquals(t, types.Float64, types.FromGoValue(9.24))
	assertValueEquals(t, types.String, types.FromGoValue("boo"))
	assertValueEquals(t, types.GoDuration, types.FromGoValue(time.Minute))
	assertValueEquals(t, types.GoTime, types.FromGoValue(time.Now()))
	var dist *messages.Distribution
	assertValueEquals(t, types.Dist, types.FromGoValue(dist))
}

func TestFromFloat(t *testing.T) {
	assertValueEquals(t, int8(-128), types.Int8.FromFloat(-128.0))
	assertValueEquals(t, int8(-128), types.Int8.FromFloat(-128.49))
	assertValueEquals(t, int8(-128), types.Int8.FromFloat(-127.5))
	assertValueEquals(t, int8(-127), types.Int8.FromFloat(-127.49))
	assertValueEquals(t, int8(0), types.Int8.FromFloat(0.0))
	assertValueEquals(t, int8(1), types.Int8.FromFloat(0.5))
	assertValueEquals(t, int8(0), types.Int8.FromFloat(0.49))
	assertValueEquals(t, int8(127), types.Int8.FromFloat(127.49))

	assertValueEquals(t, int16(-32768), types.Int16.FromFloat(-32768.0))
	assertValueEquals(t, int16(-32768), types.Int16.FromFloat(-32768.49))
	assertValueEquals(t, int16(-32768), types.Int16.FromFloat(-32767.5))
	assertValueEquals(t, int16(-32767), types.Int16.FromFloat(-32767.49))
	assertValueEquals(t, int16(0), types.Int16.FromFloat(0.0))
	assertValueEquals(t, int16(1), types.Int16.FromFloat(0.5))
	assertValueEquals(t, int16(0), types.Int16.FromFloat(0.49))
	assertValueEquals(t, int16(32767), types.Int16.FromFloat(32767.49))

	assertValueEquals(t, int32(-2147483648), types.Int32.FromFloat(-2147483648.0))
	assertValueEquals(t, int32(-2147483648), types.Int32.FromFloat(-2147483648.49))
	assertValueEquals(t, int32(-2147483648), types.Int32.FromFloat(-2147483647.5))
	assertValueEquals(t, int32(-2147483647), types.Int32.FromFloat(-2147483647.49))
	assertValueEquals(t, int32(0), types.Int32.FromFloat(0.0))
	assertValueEquals(t, int32(1), types.Int32.FromFloat(0.5))
	assertValueEquals(t, int32(0), types.Int32.FromFloat(0.49))
	assertValueEquals(t, int32(2147483647), types.Int32.FromFloat(2147483647.49))

	assertValueEquals(t, int64(-2147483648), types.Int64.FromFloat(-2147483648.0))
	assertValueEquals(t, int64(-2147483648), types.Int64.FromFloat(-2147483648.49))
	assertValueEquals(t, int64(-2147483648), types.Int64.FromFloat(-2147483647.5))
	assertValueEquals(t, int64(-2147483647), types.Int64.FromFloat(-2147483647.49))
	assertValueEquals(t, int64(0), types.Int64.FromFloat(0.0))
	assertValueEquals(t, int64(1), types.Int64.FromFloat(0.5))
	assertValueEquals(t, int64(0), types.Int64.FromFloat(0.49))
	assertValueEquals(t, int64(2147483647), types.Int64.FromFloat(2147483647.49))

	assertValueEquals(t, uint8(0), types.Uint8.FromFloat(0.0))
	assertValueEquals(t, uint8(1), types.Uint8.FromFloat(0.5))
	assertValueEquals(t, uint8(0), types.Uint8.FromFloat(0.49))
	assertValueEquals(t, uint8(255), types.Uint8.FromFloat(255.49))

	assertValueEquals(t, uint16(0), types.Uint16.FromFloat(0.0))
	assertValueEquals(t, uint16(1), types.Uint16.FromFloat(0.5))
	assertValueEquals(t, uint16(0), types.Uint16.FromFloat(0.49))
	assertValueEquals(t, uint16(65535), types.Uint16.FromFloat(65535.49))

	assertValueEquals(t, uint32(0), types.Uint32.FromFloat(0.0))
	assertValueEquals(t, uint32(1), types.Uint32.FromFloat(0.5))
	assertValueEquals(t, uint32(0), types.Uint32.FromFloat(0.49))
	assertValueEquals(t, uint32(4294967295), types.Uint32.FromFloat(4294967295.49))

	assertValueEquals(t, uint64(0), types.Uint64.FromFloat(0.0))
	assertValueEquals(t, uint64(1), types.Uint64.FromFloat(0.5))
	assertValueEquals(t, uint64(0), types.Uint64.FromFloat(0.49))
	assertValueEquals(t, uint64(4294967295), types.Uint64.FromFloat(4294967295.49))

	assertValueEquals(t, float32(69.25), types.Float32.FromFloat(69.25))
	assertValueEquals(t, float32(-38.375), types.Float32.FromFloat(-38.375))
	assertValueEquals(t, 69.25, types.Float64.FromFloat(69.25))
	assertValueEquals(t, -38.375, types.Float64.FromFloat(-38.375))
}

func assertValueEquals(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
