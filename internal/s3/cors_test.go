package s3

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	wantExposeHeaders = "ETag, Content-Type, Content-Length, x-amz-request-id"
	wantAllowMethods  = "GET, HEAD, DELETE, POST, OPTIONS"
	wantAllowHeaders  = "Authorization, Content-Type, x-amz-date, x-amz-content-sha256, x-amz-security-token, x-amz-user-agent"
	wantMaxAge        = "3600"
)

func assertBaseCORSHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != wantExposeHeaders {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, wantExposeHeaders)
	}
}

func TestOptionsPreflightUnconfiguredBucket(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodOptions, "/nope/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rec.Body.Len())
	}
	assertBaseCORSHeaders(t, rec)
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != wantAllowMethods {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, wantAllowMethods)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != wantAllowHeaders {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, wantAllowHeaders)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != wantMaxAge {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, wantMaxAge)
	}
}

func TestOptionsPreflightPrivateBucketNoCredentials(t *testing.T) {
	h := testMultiBucketHandler()
	req := httptest.NewRequest(http.MethodOptions, "/assets/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rec.Body.Len())
	}
	assertBaseCORSHeaders(t, rec)
}

func TestCORSHeadersOnSuccessResponse(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	assertBaseCORSHeaders(t, rec)
}

func TestCORSHeadersOnErrorResponse(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/nope/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
	assertBaseCORSHeaders(t, rec)
}
