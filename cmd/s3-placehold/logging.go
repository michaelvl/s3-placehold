package main

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder wraps an http.ResponseWriter to capture the status code and
// response body size ultimately written, for access logging. Handlers that
// never call WriteHeader explicitly (relying on the implicit 200 on first
// Write) still report the correct status.
type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

// logRequests wraps next with a basic per-request access log line: method,
// host, request URI (path + query), status code, response size, and
// latency. Written to the standard logger, so it shares main's log.Fatal*
// output stream.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r)

		log.Printf("%s %s%s %d %dB %s", r.Method, r.Host, r.URL.RequestURI(), rec.status, rec.size, time.Since(start))
	})
}
