package main

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"log"
	"net/http"
	"net/rpc"
)

func registerMetrics() {
	var temperature float64
	var someBool bool

	rpcDistribution := tricorder.PowersOfTen.NewDistribution()

	if err := tricorder.RegisterMetric(
		"/proc/rpc-latency",
		rpcDistribution,
		units.Millisecond,
		"RPC latency"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric(
		"/proc/rpc-count",
		rpcCountCallback,
		units.None,
		"RPC count"); err != nil {
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
	err = barDir.RegisterMetric(
		"baz",
		bazCallback,
		units.None,
		"Another float value")
	if err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	err = barDir.RegisterMetric("abool", &someBool, units.None, "A boolean value")
	if err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}

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
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
	}
}
