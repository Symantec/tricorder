package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"log"
	"net/rpc"
	"os"
	"time"
)

func printAsJson(desc string, value interface{}) {
	fmt.Println(desc)
	var buffer bytes.Buffer
	content, err := json.Marshal(value)
	if err != nil {
		log.Fatal("Marshalling:", err)
	}
	json.Indent(&buffer, content, "", "\t")
	buffer.WriteTo(os.Stdin)
	fmt.Println()
}

func main() {
	client, err := rpc.DialHTTP("tcp", ":8080")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer client.Close()
	var metrics messages.Metrics
	err = client.Call("MetricsServer.ListMetrics", "", &metrics)
	if err != nil {
		log.Fatal("Calling:", err)
	}
	printAsJson("All metrics", metrics)

	err = client.Call("MetricsServer.ListMetrics", "/aaa/bbb", &metrics)
	if err != nil {
		log.Fatal("Calling:", err)
	}
	printAsJson("aaa/bbb metrics", metrics)

	var single messages.Metric
	err = client.Call("MetricsServer.GetMetric", "/proc/foo/bar/baz", &single)
	if err != nil {
		log.Fatal("Calling:", err)
	}
	printAsJson("/proc/foo/bar/baz metric", single)

	err = client.Call("MetricsServer.GetMetric", "/proc/foo/ddd", &single)
	if err != nil {
		log.Println("Got error for /proc/foo/ddd:", err)
	} else {
		printAsJson("/proc/foo/ddd metric", single)
	}
	time.Sleep(5 * time.Second)

}
