/*
Package tricorder provides health metrics of an application via http.

Using tricorder in code

Use the tricorder package in your application like so:

	import "github.com/Symantec/tricorder/go/tricorder"
	import "net/http"
	func main() {
		doYourInitialization();
		registerYourOwnHttpHandlers();
		if err := http.ListenAndServe(":8080", http.DefaultServeMux); err != nil {
			fmt.Println(err)
		}
	}

Viewing Metrics

Package tricorder uses the net/http package register its web UI at path "/metrics".
Package tricorder registers static content such as CSS files at "/metricsdemo".

URL formats to view metrics:

	http://yourhostname:8080/metrics
		View all top level metrics in HTML
	http://yourhostname:8080/metrics/single/metric/path
		View the metric with path single/metric/path in HTML.
	http://yourhostname:8080/metrics/dirpath
		View all metrics with paths starting with 'dirpath/' in HTML.
		Does not expand metrics under subdirectories such as
		dirpath/asubdir but shows subdirectories such as
		dirpath/subdir as a hyper link instead.
	http://yourhostname:8080/metrics/single/metric/path?format=text
		Value of metric single/metric/path in plain text.
	http://yourhostname:8080/metrics/dirpath/?format=text
		Shows all metrics starting with 'dirpath/' in plain text.
		Unlike the HTML version, expands all subdirectories under
		/dirpath.
		Shows in this format:
			dirpath/subdir/ametric 21.3
			dirpath/first 12345
			dirpath/second 5.28

Default Metrics

	/name - the application name
	/args - the application argument
	/start-time - the application start time

Register Custom Metrics

Use tricorder.RegisterMetric() and tricorder.RegisterDirectory()
to register custom metrics. You must register all custom metrics
at the beginning of the program before starting additional goroutines
or calling http.ListenAndServe. Always pass the address of a variable
to RegisterMetric() so that tricorder can see changes in the variable's
value.

To register a time.Time, you can pass a **time.Time to RegisterMetric().
To update the time value, change the pointer to point to a different time
value rather than changing the existing time in place. This makes the
update atomic.

Metric types:

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
