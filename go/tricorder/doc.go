/*
Package tricorder provides health metrics of an application via http.

Using tricorder in code

Use the tricorder package in your application like so:

	import "github.com/Symantec/tricorder/go/tricorder"
	import "net/http"
	import "net/rpc"
	func main() {
		doYourInitialization();
		registerYourOwnHttpHandlers();
		// Needed to support Go RPC. Client must call.
		rpc.HandleHTTP()
		if err := http.ListenAndServe(":8080", http.DefaultServeMux); err != nil {
			fmt.Println(err)
		}
	}

Viewing Metrics with a Web Browser

Package tricorder uses the net/http package register its web UI at path "/metrics".
Package tricorder registers static content such as CSS files at "/metricsstatic".

URL formats to view metrics:

	http://yourhostname.com/metrics
		View all top level metrics in HTML
	http://yourhostname.com/metrics/single/metric/path
		View the metric with path single/metric/path in HTML.
	http://yourhostname.com/metrics/dirpath
		View all metrics with paths starting with 'dirpath/' in HTML.
		Does not expand metrics under subdirectories such as
		dirpath/asubdir but shows subdirectories such as
		dirpath/subdir as a hyper link instead.
	http://yourhostname.com/metrics/single/metric/path?format=text
		Value of metric single/metric/path in plain text.
	http://yourhostname.com/metrics/dirpath/?format=text
		Shows all metrics starting with 'dirpath/' in plain text.
		Unlike the HTML version, expands all subdirectories under
		/dirpath.
		Shows in this format:
			dirpath/subdir/ametric 21.3
			dirpath/first 12345
			dirpath/second 5.28

Fetching metrics using go RPC

Package tricorder registers the following go rpc methods. You can see
these methods by visiting http://yourhostname.com/debug/rpc

MetricsServer.ListMetrics:

Recursively lists all metrics under a particular path.
Request is the absolute path as a string.
Response is a messages.MetricList type.

MetricsServer.GetMetric

Gets a single metric with a particular path or returns
messages.ErrMetricNotFound if there is no such metric.
Request is the absolute path as a string.
Response is a messages.Metric type.

Example:

	import "github.com/Symantec/tricorder/go/tricorder/messages"
	client, _ := rpc.DialHTTP("tcp", ":8080")
	defer client.Close()
	var metrics messages.MetricList
	client.Call("MetricsServer.ListMetrics", "/a/directory", &metrics)
	var metric messages.Metric
	err := client.Call("MetricsServer.GetMetric", "/a/metric", &metric)
	if err == nil {
		// we found /a/metric
	}

Fetching metrics using REST API

Package tricorder registers its REST API at "/metricsapi"

REST urls:

	http://yourhostname.com/metricsapi/
		Returns a json array of every metric
	http://yourhostname.com/metricsapi/a/path
		Returns a possibly empty json array array of every metric
		anywhere under /a/path.
	http://yourhostname.com/metricsapi/path/to/metric?singleton=true
		Returns a metric json object with absolute path
		/path/to/metric or gives a 404 error if no such metric
		exists.

Sample metric json object:

	{
		"path": "/proc/foo/bar/baz",
		"description": "Another float value",
		"unit": "None",
		"kind": "float",
		"value": 12.375
	}

For more information on the json schema, see the messages.Metric type.

Register Custom Metrics

To add additional metrics to the default metrics tricorder provides,
Use tricorder.RegisterMetric() and tricorder.RegisterDirectory().
You must register all custom metrics at the beginning of the program
before starting additional goroutines or calling http.ListenAndServe.
Always pass the address of a variable to RegisterMetric() so that
tricorder can see changes in the variable's value.

To register a time.Time, you can pass a **time.Time to RegisterMetric().
To update the time value, change the pointer to point to a different time
value rather than changing the existing time in place. This makes the
update atomic.

Metric types:

	bool
		tricorder.RegisterMetric(
			"a/path/to/bool",
			&boolValue,
			tricorder.None,
			"bool value description")
	int, int8, int16, int32, int64
		tricorder.RegisterMetric(
			"a/path/to/int",
			&intValue,
			tricorder.None,
			"int value description")

	uint, uint8, uint16, uint32, uint64
		tricorder.RegisterMetric(
			"a/path/to/uint",
			&uintValue,
			tricorder.None,
			"uint value description")

	float32, float64
		tricorder.RegisterMetric(
			"a/path/to/float",
			&floatValue,
			tricorder.None,
			"float value description")

	string
		tricorder.RegisterMetric(
			"a/path/to/string",
			&stringValue,
			tricorder.None,
			"string value description")

	time.Time
		tricorder.RegisterMetric(
			"a/path/to/time",
			&timeValue,
			tricorder.None,
			"time value description")
		tricorder.RegisterMetric(
			"another/path/to/time",
			&pointerToAnotherTimeValue,
			tricorder.None,
			"another time description")
	time.Duration
		tricorder.RegisterMetric(
			"a/path/to/duration",
			&durationValue,
			tricorder.None,
			"duration value description")

If code generates a metric's value, register the callback function like so

	func generateAnInt() int {
		return 537;
	}
	tricorder.RegisterMetric(
		"path/to/intValue",
		generateAnInt,
		tricorder.None,
		"generated int description")

Tricorder can collect a distribution of values in a metric. In distributions, values are always floats. With distributions, the client program must manually add values. Unlike registering metrics, distribution instances are
safe to use from multiple goroutines.

	globalDist := tricorder.NewDistribution(tricorder.PowersOfTen)
	tricorder.RegisterMetric(
		"path/to/distribution",
		globalDist,
		tricorder.None,
		"A distribution description")

	func doSomethingDuringProgram() {
		globalDist.Add(getSomeFloatValue())
		globalDist.Add(getAnotherFloatValue())
	}
*/
package tricorder
