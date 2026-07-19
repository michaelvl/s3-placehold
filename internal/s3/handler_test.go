package s3

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/image"
	"github.com/michaelvl/s3-placehold/internal/synth"
)

func testHandler() *Handler {
	cfg := config.Config{
		Port:    9000,
		Buckets: []config.BucketConfig{{Name: "placeholder", Mode: config.ModePublic}},
	}
	router := synth.NewRouter(image.New())
	return NewHandler(cfg, router)
}

func TestGetObjectDefaultKey(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want %q", got, "image/svg+xml")
	}
	wantLen := strconv.Itoa(rec.Body.Len())
	if got := rec.Header().Get("Content-Length"); got != wantLen {
		t.Errorf("Content-Length = %q, want %q", got, wantLen)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `width="100"`) || !strings.Contains(body, "#cccccc") {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestHeadObjectDefaultKey(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodHead, "/placeholder/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want %q", got, "image/svg+xml")
	}
	if rec.Header().Get("Content-Length") == "" {
		t.Errorf("Content-Length header missing")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0 (HEAD must have no body)", rec.Body.Len())
	}
}

func TestUnsupportedMethodReturns405(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPut, "/placeholder/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/xml" {
		t.Errorf("Content-Type = %q, want %q", got, "application/xml")
	}

	var errResp s3Error
	if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
	}
	if errResp.Code != "MethodNotAllowed" {
		t.Errorf("Code = %q, want %q", errResp.Code, "MethodNotAllowed")
	}
	if errResp.RequestID == "" {
		t.Errorf("RequestId is empty")
	}
}
