package s3

import (
	"bytes"
	"encoding/xml"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/image"
	"github.com/michaelvl/s3-placehold/internal/key"
	"github.com/michaelvl/s3-placehold/internal/synth"
)

func testHandler() *Handler {
	cfg := config.Config{
		Port:      9000,
		Buckets:   []config.BucketConfig{{Name: "placeholder", Mode: config.ModePublic}},
		MaxWidth:  key.DefaultMaxWidth,
		MaxHeight: key.DefaultMaxHeight,
	}
	router := synth.NewRouter(image.New())
	return NewHandler(cfg, router)
}

func TestGetObjectDefaultKey(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/", nil)
	req.Host = "localhost"
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

func TestGetObjectSizeOverConfiguredMaxRejected(t *testing.T) {
	cfg := config.Config{
		Port:      9000,
		Buckets:   []config.BucketConfig{{Name: "placeholder", Mode: config.ModePublic}},
		MaxWidth:  500,
		MaxHeight: 500,
	}
	router := synth.NewRouter(image.New())
	h := NewHandler(cfg, router)

	req := httptest.NewRequest(http.MethodGet, "/placeholder/size=600x100", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}

	var errResp s3Error
	if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
	}
	if errResp.Code != "InvalidArgument" {
		t.Errorf("Code = %q, want %q", errResp.Code, "InvalidArgument")
	}
}

func TestHeadObjectDefaultKey(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodHead, "/placeholder/", nil)
	req.Host = "localhost"
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
	req.Host = "localhost"
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

func TestGetObjectPNGWithSizeColourAndText(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/format=png/size=200x300/colour=ff0000/text=hello+world", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 300 {
		t.Errorf("dimensions = %dx%d, want 200x300", img.Bounds().Dx(), img.Bounds().Dy())
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if r>>8 != 0xff || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("corner pixel = (%d,%d,%d), want (255,0,0)", r>>8, g>>8, b>>8)
	}
}

func TestGetObjectJPEGWithNamedColour(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/format=jpeg/size=400x200/colour=lightblue", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want %q", got, "image/jpeg")
	}
	img, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to decode JPEG: %v", err)
	}
	if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 200 {
		t.Errorf("dimensions = %dx%d, want 400x200", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestGetObjectFixedDelay(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/delay=50", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 50ms", elapsed)
	}
}

func TestGetObjectRangeDelay(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/delay=20,60", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	start := time.Now()
	h.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if elapsed < 20*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 20ms", elapsed)
	}
}

func TestListObjectsV2Empty(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/?list-type=2", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/xml" {
		t.Errorf("Content-Type = %q, want %q", got, "application/xml")
	}

	var result listBucketResult
	if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal ListBucketResult XML: %v; body: %s", err, rec.Body.String())
	}
	if result.Name != "placeholder" {
		t.Errorf("Name = %q, want %q", result.Name, "placeholder")
	}
	if result.KeyCount != 0 {
		t.Errorf("KeyCount = %d, want 0", result.KeyCount)
	}
	if result.IsTruncated {
		t.Errorf("IsTruncated = true, want false")
	}
}

func TestListObjectsDispatchOnListingParams(t *testing.T) {
	params := []string{"prefix", "delimiter", "marker", "continuation-token"}

	for _, p := range params {
		t.Run(p, func(t *testing.T) {
			h := testHandler()
			req := httptest.NewRequest(http.MethodGet, "/placeholder/?"+p+"=x", nil)
			req.Host = "localhost"
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			var result listBucketResult
			if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal ListBucketResult XML: %v; body: %s", err, rec.Body.String())
			}
		})
	}
}

func TestDeleteObjectNoop(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodDelete, "/placeholder/some/key", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rec.Body.Len())
	}
}

func TestDeleteObjectsBatchNoop(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/placeholder/?delete", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/xml" {
		t.Errorf("Content-Type = %q, want %q", got, "application/xml")
	}

	var result deleteResult
	if err := xml.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal DeleteResult XML: %v; body: %s", err, rec.Body.String())
	}
}

func TestGetObjectInvalidParametersReturn400(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"invalid size", "/placeholder/size=abc"},
		{"invalid format", "/placeholder/format=gif"},
		{"invalid colour", "/placeholder/colour=notacolour"},
		{"invalid type", "/placeholder/type=pdf"},
		{"segment without equals", "/placeholder/format"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := testHandler()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = "localhost"
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/xml" {
				t.Errorf("Content-Type = %q, want %q", got, "application/xml")
			}

			var errResp s3Error
			if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
			}
			if errResp.Code != "InvalidArgument" {
				t.Errorf("Code = %q, want %q", errResp.Code, "InvalidArgument")
			}
			if errResp.RequestID == "" {
				t.Errorf("RequestId is empty")
			}
		})
	}
}

func testMultiBucketHandler() *Handler {
	cfg := config.Config{
		Port: 9000,
		Buckets: []config.BucketConfig{
			{Name: "images", Mode: config.ModePublic},
			{Name: "assets", Mode: config.ModePrivate},
		},
		MaxWidth:  key.DefaultMaxWidth,
		MaxHeight: key.DefaultMaxHeight,
	}
	router := synth.NewRouter(image.New())
	return NewHandler(cfg, router)
}

func TestVirtualHostedStyleResolvesBucketAndKey(t *testing.T) {
	h := testMultiBucketHandler()
	req := httptest.NewRequest(http.MethodGet, "/format=png", nil)
	req.Host = "images.localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
}

func TestPathStyleResolvesEitherConfiguredBucket(t *testing.T) {
	// The public bucket resolves and dispatches normally. The private
	// bucket also resolves by path style (no NoSuchBucket), but this
	// unauthenticated request is rejected by auth (see internal/s3/sigv4_test.go
	// for the private-bucket auth acceptance criteria).
	cases := []struct {
		bucket   string
		wantCode int
	}{
		{"images", http.StatusOK},
		{"assets", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.bucket, func(t *testing.T) {
			h := testMultiBucketHandler()
			req := httptest.NewRequest(http.MethodGet, "/"+tc.bucket+"/", nil)
			req.Host = "localhost"
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestIPLiteralHostIsPathStyle(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/placeholder/", nil)
	req.Host = "127.0.0.1:9000"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUnconfiguredBucketReturnsNoSuchBucket(t *testing.T) {
	cases := []struct {
		name string
		host string
		path string
	}{
		{"path-style", "localhost", "/nope/"},
		{"virtual-hosted", "nope.localhost", "/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := testHandler()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
			}

			var errResp s3Error
			if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
			}
			if errResp.Code != "NoSuchBucket" {
				t.Errorf("Code = %q, want %q", errResp.Code, "NoSuchBucket")
			}
			if errResp.RequestID == "" {
				t.Errorf("RequestId is empty")
			}
		})
	}
}
