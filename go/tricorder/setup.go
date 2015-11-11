package tricorder

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"os"
	"runtime"
	"strings"
	"syscall"
)

func getProgramArgs() string {
	return strings.Join(os.Args[1:], "|")
}

func timeValToString(val *syscall.Timeval) string {
	return fmt.Sprintf("%d.%06d", val.Sec, val.Usec)
}

func initDefaultMetrics() {
	programArgs := getProgramArgs()
	var memStats runtime.MemStats
	memStatsRegion := RegisterRegion(func() {
		runtime.ReadMemStats(&memStats)
	})
	RegisterMetricInRegion(
		"/proc/memory/total",
		&memStats.Sys,
		memStatsRegion,
		units.Byte,
		"Memory system has allocated to process")
	var resourceUsage syscall.Rusage
	var userTime string
	var sysTime string
	resourceUsageRegion := RegisterRegion(func() {
		syscall.Getrusage(syscall.RUSAGE_SELF, &resourceUsage)
		userTime = timeValToString(&resourceUsage.Utime)
		sysTime = timeValToString(&resourceUsage.Stime)
	})
	RegisterMetricInRegion(
		"/proc/cpu/user",
		&userTime,
		resourceUsageRegion,
		units.Second,
		"User CPU time used")
	RegisterMetricInRegion(
		"/proc/cpu/sys",
		&sysTime,
		resourceUsageRegion,
		units.Second,
		"User CPU time used")
	RegisterMetricInRegion(
		"/proc/memory/max-resident-set-size",
		&resourceUsage.Maxrss,
		resourceUsageRegion,
		units.Byte,
		"Maximum resident set size")
	RegisterMetricInRegion(
		"/proc/memory/shared",
		&resourceUsage.Ixrss,
		resourceUsageRegion,
		units.Byte,
		"Integral shared memory size")
	RegisterMetricInRegion(
		"/proc/memory/unshared-data",
		&resourceUsage.Idrss,
		resourceUsageRegion,
		units.Byte,
		"Integral unshared data size")
	RegisterMetricInRegion(
		"/proc/memory/unshared-stack",
		&resourceUsage.Isrss,
		resourceUsageRegion,
		units.Byte,
		"Integral unshared stack size")
	RegisterMetricInRegion(
		"/proc/memory/soft-page-fault",
		&resourceUsage.Minflt,
		resourceUsageRegion,
		units.None,
		"Soft page faults")
	RegisterMetricInRegion(
		"/proc/memory/hard-page-fault",
		&resourceUsage.Majflt,
		resourceUsageRegion,
		units.None,
		"Hard page faults")
	RegisterMetricInRegion(
		"/proc/memory/swaps",
		&resourceUsage.Nswap,
		resourceUsageRegion,
		units.None,
		"Swaps")
	RegisterMetricInRegion(
		"/proc/io/input",
		&resourceUsage.Inblock,
		resourceUsageRegion,
		units.None,
		"Block input operations")
	RegisterMetricInRegion(
		"/proc/io/output",
		&resourceUsage.Oublock,
		resourceUsageRegion,
		units.None,
		"Block output operations")
	RegisterMetricInRegion(
		"/proc/ipc/sent",
		&resourceUsage.Msgsnd,
		resourceUsageRegion,
		units.None,
		"IPC messages sent")
	RegisterMetricInRegion(
		"/proc/ipc/received",
		&resourceUsage.Msgrcv,
		resourceUsageRegion,
		units.None,
		"IPC messages received")
	RegisterMetricInRegion(
		"/proc/signals/received",
		&resourceUsage.Nsignals,
		resourceUsageRegion,
		units.None,
		"Signals received")
	RegisterMetricInRegion(
		"/proc/scheduler/voluntary-switches",
		&resourceUsage.Nvcsw,
		resourceUsageRegion,
		units.None,
		"Voluntary context switches")
	RegisterMetricInRegion(
		"/proc/scheduler/involuntary-switches",
		&resourceUsage.Nvcsw,
		resourceUsageRegion,
		units.None,
		"Involuntary context switches")
	RegisterMetric("/proc/name", &os.Args[0], units.None, "Program name")
	RegisterMetric("/proc/args", &programArgs, units.None, "Program args")
	RegisterMetric("/proc/start-time", &appStartTime, units.None, "Program start time")
}

func init() {
	initDefaultMetrics()
	initHttpFramework()
	initHtmlHandlers()
	initJsonHandlers()
	initRpcHandlers()
}
