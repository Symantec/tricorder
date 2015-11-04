package main

import (
	"bytes"
	"encoding/json"
	"github.com/Symantec/tricorder/go/tricorder/messages"
	"log"
	"net/rpc"
	"os"
)

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
	var buffer bytes.Buffer
	content, err := json.Marshal(metrics)
	if err != nil {
		log.Fatal("Marshalling:", err)
	}
	json.Indent(&buffer, content, "", "\t")
	buffer.WriteTo(os.Stdin)
}
