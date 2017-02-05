package healthserver

import "net/http"

var (
	healthStatus string
	readyStatus  string = "not ready"
)

func init() {
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readiness", readinessHandler)
}

func commonHandler(w http.ResponseWriter, status string) {
	if status == "" {
		w.Write([]byte("OK"))
	} else {
		http.Error(w, status, http.StatusServiceUnavailable)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	commonHandler(w, healthStatus)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	commonHandler(w, readyStatus)
}

func setHealth(status string) {
	healthStatus = status
}

func setReady(status string) {
	readyStatus = status
}
