package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/units"
	"os"
	"strings"
)

func getProgramArgs() string {
	return strings.Join(os.Args[1:], "|")
}

func initDefaultMetrics() {
	programArgs := getProgramArgs()
	RegisterMetric("/name", &os.Args[0], units.None, "Program name")
	RegisterMetric("/args", &programArgs, units.None, "Program args")
	RegisterMetric("/start-time", &appStartTime, units.None, "Program start time")
}

func init() {
	initDefaultMetrics()
	initHttpFramework()
	initHtmlHandlers()
	initJsonHandlers()
	initRpcHandlers()
}
