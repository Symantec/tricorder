package main

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"github.com/gorilla/context"
	"github.com/keep94/weblogs"
	"log"
	"net/http"
	"net/rpc"
)

func registerMetrics() {
	// /proc/rpc-latency: Distribution millis 5 buckets, start: 10, scale 2.5
	// /proc/rpc-count: Callback to get RPC count, uint64
	// /proc/start-time: An int64 showing start time as seconds since epoch
	// /proc/temperature: A float64 showing tempurature in celsius
	// /proc/foo/bar/baz: Callback to get a float64 that returns an error

	var startTime int64
	var temperature float64
	var someBool bool

	rpcDistribution := tricorder.NewDistribution(tricorder.PowersOfTen)

	if err := tricorder.RegisterMetric("/proc/rpc-latency", rpcDistribution, units.Millisecond, "RPC latency"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/rpc-count", rpcCountCallback, units.None, "RPC count"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/start-time", &startTime, units.Second, "Start Time"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/temperature", &temperature, units.Celsius, "Temperature"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	fooDir, err := tricorder.RegisterDirectory("proc/foo")
	if err != nil {
		log.Fatalf("Got error %v registering directory", err)
	}
	barDir, err := fooDir.RegisterDirectory("bar")
	if err != nil {
		log.Fatalf("Got error %v registering directory", err)
	}
	err = barDir.RegisterMetric("baz", bazCallback, units.None, "Another float value")
	if err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	err = barDir.RegisterMetric("abool", &someBool, units.None, "A boolean value")
	if err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}

	startTime = -1234567
	temperature = 22.5
	someBool = true

	// Add data points to the distribution
	// < 10: 10
	// 10 - 25: 15
	// 25 - 62.5: 38
	// 62.5 - 156.25: 94
	// 156.25 - 390.625: 234
	for i := 0; i < 500; i++ {
		rpcDistribution.Add(float64(i))
	}
}

func bazCallback() float64 {
	return 12.375
}

func rpcCountCallback() uint64 {
	return 500
}

func main() {
	registerMetrics()
	rpc.HandleHTTP()
	defaultHandler := context.ClearHandler(
		weblogs.HandlerWithOptions(
			http.DefaultServeMux,
			&weblogs.Options{
				Logger: weblogs.ApacheCommonLogger()}))
	if err := http.ListenAndServe(
		":8080", defaultHandler); err != nil {
		fmt.Println(err)
	}
}
