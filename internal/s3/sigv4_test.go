package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/image"
	"github.com/michaelvl/s3-placehold/internal/synth"
)

const (
	testAccessKeyID     = "TESTKEYID1234567890A"
	testSecretAccessKey = "testsecretkey1234567890abcdefghijklmnop"
	testRegion          = "us-east-1"
	testService         = "s3"
	testAmzDate         = "20250101T120000Z"
	emptyPayloadHash    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func testPrivateBucketHandler() *Handler {
	cfg := config.Config{
		Port: 9000,
		Buckets: []config.BucketConfig{
			{Name: "images", Mode: config.ModePublic},
			{Name: "assets", Mode: config.ModePrivate},
		},
		AccessKeyID:     testAccessKeyID,
		SecretAccessKey: testSecretAccessKey,
	}
	router := synth.NewRouter(image.New())
	return NewHandler(cfg, router)
}

// sign is a from-scratch reference SigV4 signer (independent of
// internal/sigv4's implementation) used to build "correctly signed" HTTP
// requests for these handler-level acceptance tests.
func sign(req *http.Request, accessKeyID, secretKey string, signedHeaders []string) {
	req.Header.Set("X-Amz-Date", testAmzDate)

	names := append([]string(nil), signedHeaders...)
	sort.Strings(names)

	var headerBlock strings.Builder
	for _, name := range names {
		value := req.Host
		if name != "host" {
			value = req.Header.Get(name)
		}
		headerBlock.WriteString(name + ":" + value + "\n")
	}
	signedHeadersStr := strings.Join(names, ";")

	payloadHash := req.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = emptyPayloadHash
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		encodePath(req.URL.Path),
		req.URL.RawQuery,
		headerBlock.String(),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	dateStamp := testAmzDate[:8]
	scope := dateStamp + "/" + testRegion + "/" + testService + "/aws4_request"

	hashedCR := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		testAmzDate,
		scope,
		hex.EncodeToString(hashedCR[:]),
	}, "\n")

	kDate := hmacSum([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSum(kDate, testRegion)
	kService := hmacSum(kRegion, testService)
	kSigning := hmacSum(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSum(kSigning, stringToSign))

	req.Header.Set("Authorization",
		"AWS4-HMAC-SHA256 Credential="+accessKeyID+"/"+scope+
			", SignedHeaders="+signedHeadersStr+
			", Signature="+signature)
}

func hmacSum(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func encodePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		var b strings.Builder
		for j := 0; j < len(seg); j++ {
			c := seg[j]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
				c == '-' || c == '_' || c == '.' || c == '~' {
				b.WriteByte(c)
			} else {
				b.WriteByte('%')
				b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
			}
		}
		segments[i] = b.String()
	}
	return strings.Join(segments, "/")
}

func signedHeadersRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/assets/format=png", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Content-Sha256", emptyPayloadHash)
	sign(req, testAccessKeyID, testSecretAccessKey, []string{"host", "x-amz-content-sha256", "x-amz-date"})
	return req
}

func TestPrivateBucketValidSignatureSucceeds(t *testing.T) {
	h := testPrivateBucketHandler()
	req := signedHeadersRequest()
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
}

func TestPrivateBucketNoCredentialsReturnsAccessDenied(t *testing.T) {
	h := testPrivateBucketHandler()
	req := httptest.NewRequest(http.MethodGet, "/assets/format=png", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	var errResp s3Error
	if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
	}
	if errResp.Code != "AccessDenied" {
		t.Errorf("Code = %q, want %q", errResp.Code, "AccessDenied")
	}
}

func TestPrivateBucketTamperedSignatureReturnsSignatureDoesNotMatch(t *testing.T) {
	h := testPrivateBucketHandler()
	req := signedHeadersRequest()

	auth := req.Header.Get("Authorization")
	last := auth[len(auth)-1]
	repl := byte('0')
	if last == '0' {
		repl = '1'
	}
	req.Header.Set("Authorization", auth[:len(auth)-1]+string(repl))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	var errResp s3Error
	if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
	}
	if errResp.Code != "SignatureDoesNotMatch" {
		t.Errorf("Code = %q, want %q", errResp.Code, "SignatureDoesNotMatch")
	}
}

func TestPublicBucketSucceedsWithOrWithoutCredentials(t *testing.T) {
	cases := []struct {
		name     string
		withAuth bool
	}{
		{"no credentials", false},
		{"with credentials", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := testPrivateBucketHandler()
			req := httptest.NewRequest(http.MethodGet, "/images/format=png", nil)
			req.Host = "localhost"
			if tc.withAuth {
				req.Header.Set("X-Amz-Content-Sha256", emptyPayloadHash)
				sign(req, testAccessKeyID, testSecretAccessKey, []string{"host", "x-amz-content-sha256", "x-amz-date"})
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestPrivateBucketAuthCheckedBeforeMethodDispatch confirms auth is
// enforced for every request to a private bucket, not just GET/HEAD: an
// unsupported method with no credentials is rejected as AccessDenied
// rather than falling through to MethodNotAllowed.
func TestPrivateBucketAuthCheckedBeforeMethodDispatch(t *testing.T) {
	h := testPrivateBucketHandler()
	req := httptest.NewRequest(http.MethodPut, "/assets/format=png", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
	var errResp s3Error
	if err := xml.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error XML: %v; body: %s", err, rec.Body.String())
	}
	if errResp.Code != "AccessDenied" {
		t.Errorf("Code = %q, want %q", errResp.Code, "AccessDenied")
	}
}
