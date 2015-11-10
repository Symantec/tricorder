package tricorder

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	errLog       *log.Logger
	appStartTime time.Time
)

func handleError(w http.ResponseWriter, err error) {
	fmt.Fprintln(w, "Error in template.")
	errLog.Printf("Error in template: %v\n", err)
}

func httpError(w http.ResponseWriter, status int) {
	http.Error(
		w,
		fmt.Sprintf(
			"%d %s",
			status,
			http.StatusText(status)),
		status)
}

func addStatic(mux *http.ServeMux, path, content string) {
	addStaticBinary(mux, path, []byte(content))
}

func addStaticBinary(mux *http.ServeMux, path string, content []byte) {
	mux.Handle(
		path,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(
				w,
				r,
				path,
				appStartTime,
				bytes.NewReader(content))
		}))
}

type gzipResponseWriter struct {
	http.ResponseWriter
	W io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.W.Write(b)
}

type gzipHandler struct {
	H http.Handler
}

func (h gzipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.H.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	gzr := &gzipResponseWriter{ResponseWriter: w, W: gz}
	h.H.ServeHTTP(gzr, r)
}

func initHttpFramework() {
	appStartTime = time.Now()
	errLog = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}
