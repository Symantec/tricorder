package tricorder

import (
	"github.com/Symantec/tricorder/go/tricorder/units"
	"github.com/Symantec/tricorder/go/tricorder/wrapper"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func getProgramArgs() string {
	return strings.Join(os.Args[1:], "|")
}

func timeValToDuration(val *wrapper.Timeval) time.Duration {
	return time.Duration(val.Sec)*time.Second + time.Duration(val.Usec)*time.Nanosecond
}

func initDefaultMetrics() {
	programArgs := getProgramArgs()
	var memStats runtime.MemStats
	memStatsGroup := NewGroup()
	memStatsGroup.RegisterUpdateFunc(func() time.Time {
		runtime.ReadMemStats(&memStats)
		return time.Now()
	})
	RegisterMetricInGroup(
		"/proc/memory/total",
		&memStats.Sys,
		memStatsGroup,
		units.Byte,
		"Memory system has allocated to process")
	var numGoroutines int
	var resourceUsage wrapper.Rusage
	var userTime time.Duration
	var sysTime time.Duration
	var maxResidentSetSize int64
	resourceUsageGroup := NewGroup()
	resourceUsageGroup.RegisterUpdateFunc(func() time.Time {
		numGoroutines = runtime.NumGoroutine()
		wrapper.Getrusage(syscall.RUSAGE_SELF, &resourceUsage)
		userTime = timeValToDuration(&resourceUsage.Utime)
		sysTime = timeValToDuration(&resourceUsage.Stime)
		maxResidentSetSize = resourceUsage.Maxrss
		return time.Now()
	})
	RegisterMetricInGroup(
		"/proc/cpu/user",
		&userTime,
		resourceUsageGroup,
		units.Second,
		"User CPU time used")
	RegisterMetricInGroup(
		"/proc/cpu/sys",
		&sysTime,
		resourceUsageGroup,
		units.Second,
		"User CPU time used")
	RegisterMetricInGroup(
		"/proc/memory/max-resident-set-size",
		&maxResidentSetSize,
		resourceUsageGroup,
		units.Byte,
		"Maximum resident set size")
	RegisterMetricInGroup(
		"/proc/memory/shared",
		&resourceUsage.Ixrss,
		resourceUsageGroup,
		units.Byte,
		"Integral shared memory size")
	RegisterMetricInGroup(
		"/proc/memory/unshared-data",
		&resourceUsage.Idrss,
		resourceUsageGroup,
		units.Byte,
		"Integral unshared data size")
	RegisterMetricInGroup(
		"/proc/memory/unshared-stack",
		&resourceUsage.Isrss,
		resourceUsageGroup,
		units.Byte,
		"Integral unshared stack size")
	RegisterMetricInGroup(
		"/proc/memory/soft-page-fault",
		&resourceUsage.Minflt,
		resourceUsageGroup,
		units.None,
		"Soft page faults")
	RegisterMetricInGroup(
		"/proc/memory/hard-page-fault",
		&resourceUsage.Majflt,
		resourceUsageGroup,
		units.None,
		"Hard page faults")
	RegisterMetricInGroup(
		"/proc/memory/swaps",
		&resourceUsage.Nswap,
		resourceUsageGroup,
		units.None,
		"Swaps")
	RegisterMetricInGroup(
		"/proc/io/input",
		&resourceUsage.Inblock,
		resourceUsageGroup,
		units.None,
		"Block input operations")
	RegisterMetricInGroup(
		"/proc/io/output",
		&resourceUsage.Oublock,
		resourceUsageGroup,
		units.None,
		"Block output operations")
	RegisterMetricInGroup(
		"/proc/ipc/sent",
		&resourceUsage.Msgsnd,
		resourceUsageGroup,
		units.None,
		"IPC messages sent")
	RegisterMetricInGroup(
		"/proc/ipc/received",
		&resourceUsage.Msgrcv,
		resourceUsageGroup,
		units.None,
		"IPC messages received")
	RegisterMetricInGroup(
		"/proc/signals/received",
		&resourceUsage.Nsignals,
		resourceUsageGroup,
		units.None,
		"Signals received")
	RegisterMetricInGroup(
		"/proc/scheduler/involuntary-switches",
		&resourceUsage.Nivcsw,
		resourceUsageGroup,
		units.None,
		"Involuntary context switches")
	RegisterMetricInGroup(
		"/proc/scheduler/num-goroutines",
		&numGoroutines,
		resourceUsageGroup,
		units.None,
		"Number of goroutines")
	RegisterMetricInGroup(
		"/proc/scheduler/voluntary-switches",
		&resourceUsage.Nvcsw,
		resourceUsageGroup,
		units.None,
		"Voluntary context switches")
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
