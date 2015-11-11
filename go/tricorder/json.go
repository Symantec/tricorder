package tricorder

import (
	"bytes"
	"encoding/json"
	"net/http"
)

var (
	jsonUrl = "/metricsapi"
)

func jsonSetUpHeaders(h http.Header) {
	h.Set("Content-Type", "text/plain")
	h.Set("X-Tricorder-Media-Type", "tricorder.v1")
}

func jsonHandlerFunc(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	jsonSetUpHeaders(w.Header())
	path := r.URL.Path
	var content []byte
	var err error
	if r.Form.Get("singleton") != "" {
		m := root.GetMetric(path)
		if m == nil {
			httpError(w, http.StatusNotFound)
			return
		}
		content, err = json.Marshal(rpcAsMetric(m, nil))
	} else {
		var collector rpcMetricsCollector
		root.GetAllMetricsByPath(path, &collector, nil)
		content, err = json.Marshal(collector)
	}
	if err != nil {
		handleError(w, err)
		return
	}
	var buffer bytes.Buffer
	json.Indent(&buffer, content, "", "\t")
	buffer.WriteTo(w)
}

func initJsonHandlers() {
	http.Handle(jsonUrl+"/", http.StripPrefix(jsonUrl, gzipHandler{http.HandlerFunc(jsonHandlerFunc)}))
}
