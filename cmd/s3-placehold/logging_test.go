package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestLogRequestsLogsMethodHostURIStatusAndSize(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	req := httptest.NewRequest(http.MethodGet, "/assets/format=png?list-type=2", nil)
	req.Host = "localhost:9000"
	rec := httptest.NewRecorder()

	logRequests(next).ServeHTTP(rec, req)

	line := buf.String()
	for _, want := range []string{"GET", "localhost:9000", "/assets/format=png?list-type=2", "404", "9B"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line = %q, want it to contain %q", line, want)
		}
	}
}

func TestLogRequestsDefaultsToStatus200WhenWriteHeaderNotCalled(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	req := httptest.NewRequest(http.MethodGet, "/images/", nil)
	rec := httptest.NewRecorder()

	logRequests(next).ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), "200") {
		t.Errorf("log line = %q, want it to contain %q", buf.String(), "200")
	}
}
