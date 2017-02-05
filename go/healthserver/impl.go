package healthserver

import (
	"net/http"
	"sync"
)

var (
	okSlice      []byte     = []byte("OK")
	mutex        sync.Mutex // Protect everything below.
	healthStatus string
	readyStatus  string = "not ready"
)

func init() {
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readiness", readinessHandler)
}

func commonHandler(w http.ResponseWriter, status string) {
	if status == "" {
		w.Write(okSlice)
	} else {
		http.Error(w, status, http.StatusServiceUnavailable)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	status := healthStatus
	mutex.Unlock()
	commonHandler(w, status)
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	status := readyStatus
	mutex.Unlock()
	commonHandler(w, status)
}

func setHealth(status string) {
	mutex.Lock()
	defer mutex.Unlock()
	healthStatus = status
}

func setReady(status string) {
	mutex.Lock()
	defer mutex.Unlock()
	readyStatus = status
}
