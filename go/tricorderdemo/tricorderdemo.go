package main

import (
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder"
	"github.com/Symantec/tricorder/go/tricorder/units"
	"log"
	"net/http"
	"net/rpc"
	"time"
)

var kAnIntList *tricorder.List

func registerMetrics() {
	var temperature float64
	var someBool bool
	var someDuration time.Duration

	rpcDistribution := tricorder.PowersOfTen.NewCumulativeDistribution()
	var sample []int
	kAnIntList = tricorder.NewList(sample, tricorder.MutableSlice)

	if err := tricorder.RegisterMetric(
		"/list/squares",
		kAnIntList,
		units.None,
		"Squares"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
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
		units.Unknown,
		"RPC count"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/temperature", &temperature, units.Celsius, "Temperature"); err != nil {
		log.Fatalf("Got error %v registering metric", err)
	}
	if err := tricorder.RegisterMetric("/proc/duration", &someDuration, units.Second, "Duration"); err != nil {
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
	someDuration = 2*time.Minute + 5*time.Second

	// Add data points to the distribution
	// < 10: 10
	// 10 - 25: 15
	// 25 - 62.5: 38
	// 62.5 - 156.25: 94
	// 156.25 - 390.625: 234
	for i := 0; i < 500; i++ {
		rpcDistribution.Add(float64(i))
	}
	go updateList()
}

func updateList() {
	time.Sleep(30 * time.Second)
	for i := 1; i < 100; i++ {
		aslice := make([]int, i)
		for j := 1; j <= i; j++ {
			aslice[j-1] = j * j
		}
		kAnIntList.Change(aslice, tricorder.ImmutableSlice)
		time.Sleep(time.Second)
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
