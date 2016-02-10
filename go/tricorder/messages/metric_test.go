package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
	"testing"
)

func TestIsJson2(t *testing.T) {
	if IsJson(types.GoDuration) {
		t.Error("GoDuration is not json compatible")
	}
	if IsJson(types.GoTime) {
		t.Error("GoTime is not json compatible")
	}
	if !IsJson(types.Int32) {
		t.Error("Int is json compatible")
	}
}
