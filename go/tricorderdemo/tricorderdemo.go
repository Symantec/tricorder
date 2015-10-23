package main

import (
	"errors"
	"fmt"
	"github.com/gorilla/context"
	"github.com/keep94/tricorder/go/tricorder"
	"github.com/keep94/weblogs"
	"log"
	"net/http"
)

func registerMetrics() {
	// /proc/rpc-latency: Distribution millis 5 buckets, start: 10, scale 2.5
	// /proc/rpc-count: Callback to get RPC count, uint64
	// /proc/start-time: An int64 showing start time as seconds since epoch
	// /proc/temperature: A float64 showing tempurature in celsius
	// /proc/foo/bar/baz: Callback to get a float64 that returns an error
	// /name - name of app
	// /args - A string arguments to app

	var name, args string
	var startTime int64
	var temperature float64

	rpcBucketer := tricorder.NewBucketerWithScale(6, 10, 2.5)
	rpcDistribution := tricorder.NewDistribution(rpcBucketer)

	if err := tricorder.RegisterMetric("/proc/rpc-latency", rpcDistribution, tricorder.Millisecond, "RPC latency"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/rpc-count", rpcCountCallback, tricorder.None, "RPC count"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/start-time", &startTime, tricorder.Second, "Start Time"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/temperature", &temperature, tricorder.Celsius, "Temperature"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/name", &name, tricorder.None, "Name of app"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}

	if err := tricorder.RegisterMetric("/args", &args, tricorder.None, "Args passed to app"); err != nil {
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
	err = barDir.RegisterMetric("baz", errorCallback, tricorder.None, "An error")
	if err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}

	// Now set the variables to actual values.
	// No calls needed to update tricorder
	name = "My application"
	args = "--help"
	startTime = -1234567
	temperature = 22.5

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

func errorCallback() (float64, error) {
	return 0.0, errors.New("Some error happened")
}

func rpcCountCallback() uint64 {
	return 500
}

func main() {
	registerMetrics()
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
