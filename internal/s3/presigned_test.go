package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// presignedRequest builds a correctly-signed presigned-URL GET/HEAD request
// fixture, independent of internal/sigv4's implementation: it signs only the
// "host" header with the "UNSIGNED-PAYLOAD" sentinel payload hash, per
// SigV4's query-string signing variant.
func presignedRequest(t *testing.T, method, path, host string) *http.Request {
	t.Helper()
	dateStamp := testAmzDate[:8]
	scope := dateStamp + "/" + testRegion + "/" + testService + "/aws4_request"

	query := url.Values{}
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", testAccessKeyID+"/"+scope)
	query.Set("X-Amz-Date", testAmzDate)
	query.Set("X-Amz-Expires", "300")
	query.Set("X-Amz-SignedHeaders", "host")

	pairs := make([]string, 0, len(query))
	for k, values := range query {
		for _, v := range values {
			pairs = append(pairs, encodeComponent(k)+"="+encodeComponent(v))
		}
	}
	sort.Strings(pairs)
	canonicalQuery := strings.Join(pairs, "&")

	canonicalRequest := strings.Join([]string{
		method,
		encodePath(path),
		canonicalQuery,
		"host:" + host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	hashedCR := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		testAmzDate,
		scope,
		hex.EncodeToString(hashedCR[:]),
	}, "\n")

	kDate := hmacSum([]byte("AWS4"+testSecretAccessKey), dateStamp)
	kRegion := hmacSum(kDate, testRegion)
	kService := hmacSum(kRegion, testService)
	kSigning := hmacSum(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSum(kSigning, stringToSign))

	query.Set("X-Amz-Signature", signature)

	req := httptest.NewRequest(method, path+"?"+query.Encode(), nil)
	req.Host = host
	return req
}

func TestPresignedGetOnPrivateBucketSucceeds(t *testing.T) {
	h := testPrivateBucketHandler()
	req := presignedRequest(t, http.MethodGet, "/assets/format=png", "localhost")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
	if rec.Body.Len() == 0 {
		t.Errorf("body is empty, want synthesized image bytes")
	}
}

func TestPresignedHeadOnPrivateBucketReturnsHeadersOnly(t *testing.T) {
	h := testPrivateBucketHandler()
	req := presignedRequest(t, http.MethodHead, "/assets/format=png", "localhost")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want %q", got, "image/png")
	}
	if rec.Header().Get("Content-Length") == "" {
		t.Errorf("Content-Length header missing")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rec.Body.Len())
	}
}

func TestPresignedInvalidSignatureReturnsSignatureDoesNotMatch(t *testing.T) {
	h := testPrivateBucketHandler()
	req := presignedRequest(t, http.MethodGet, "/assets/format=png", "localhost")

	q := req.URL.Query()
	sig := q.Get("X-Amz-Signature")
	last := sig[len(sig)-1]
	repl := byte('0')
	if last == '0' {
		repl = '1'
	}
	q.Set("X-Amz-Signature", sig[:len(sig)-1]+string(repl))
	req.URL.RawQuery = q.Encode()

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

func TestPresignedGetOnPublicBucketSucceeds(t *testing.T) {
	h := testPrivateBucketHandler()
	req := presignedRequest(t, http.MethodGet, "/images/format=png", "localhost")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}
