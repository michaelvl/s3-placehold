package sigv4

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

const (
	testAccessKeyID     = "TESTKEYID1234567890A"
	testSecretAccessKey = "testsecretkey1234567890abcdefghijklmnop"
	testRegion          = "us-east-1"
	testService         = "s3"
	testDate            = "20250101T120000Z"
)

func testCreds() Credentials {
	return Credentials{AccessKeyID: testAccessKeyID, SecretAccessKey: testSecretAccessKey}
}

// sign is a from-scratch reference SigV4 signer, coded independently of the
// verifier under test, used to build "correctly signed" fixtures. It
// mutates req in place, adding X-Amz-Date and Authorization headers that
// sign exactly the headers named in signedHeaders.
func sign(req *http.Request, accessKeyID, secretKey, amzDate string, signedHeaders []string) {
	req.Header.Set("X-Amz-Date", amzDate)

	names := append([]string(nil), signedHeaders...)
	sort.Strings(names)

	var headerBlock strings.Builder
	for _, name := range names {
		var value string
		if name == "host" {
			value = req.Host
		} else {
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
		refEncodePath(req.URL.Path),
		refCanonicalQuery(req.URL.RawQuery),
		headerBlock.String(),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	dateStamp := amzDate[:8]
	scope := dateStamp + "/" + testRegion + "/" + testService + "/aws4_request"

	hashedCR := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
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

// refEncodeComponent, refEncodePath, and refCanonicalQuery reimplement
// SigV4's URI-encoding rules independently of sigv4.go's uriEncode /
// canonicalURIPath / canonicalQueryString, so this test fixture generator
// isn't just calling back into the code it's exercising.
func refEncodeComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if refUnreserved(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

func refEncodePath(path string) string {
	if path == "" {
		return "/"
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = refEncodeComponent(seg)
	}
	return strings.Join(segments, "/")
}

func refCanonicalQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	pairs := strings.Split(rawQuery, "&")
	encoded := make([]string, len(pairs))
	for i, p := range pairs {
		k, v, _ := strings.Cut(p, "=")
		encoded[i] = refEncodeComponent(k) + "=" + refEncodeComponent(v)
	}
	sort.Strings(encoded)
	return strings.Join(encoded, "&")
}

func refUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func signedRequest(t *testing.T, method, target, host string, signedHeaders []string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	req.Header.Set("X-Amz-Content-Sha256", emptyPayloadHash)
	sign(req, testAccessKeyID, testSecretAccessKey, testDate, signedHeaders)
	return req
}

func TestVerifyAcceptsCorrectlySignedRequest(t *testing.T) {
	req := signedRequest(t, http.MethodGet, "/assets/format=png/size=200x300", "localhost",
		[]string{"host", "x-amz-content-sha256", "x-amz-date"})

	if err := Verify(req, testCreds()); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
}

func TestVerifyAcceptsRequestWithQueryString(t *testing.T) {
	req := signedRequest(t, http.MethodGet, "/assets/?list-type=2&prefix=abc", "localhost",
		[]string{"host", "x-amz-content-sha256", "x-amz-date"})

	if err := Verify(req, testCreds()); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
}

func TestVerifyMissingAuthorizationHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	req.Host = "localhost"

	if err := Verify(req, testCreds()); err != ErrMissingAuthorization {
		t.Fatalf("Verify() = %v, want ErrMissingAuthorization", err)
	}
}

func TestVerifyTamperedSignatureRejected(t *testing.T) {
	req := signedRequest(t, http.MethodGet, "/assets/format=png", "localhost",
		[]string{"host", "x-amz-content-sha256", "x-amz-date"})

	tampered := req.Header.Get("Authorization")
	// Flip the last hex character of the signature.
	last := tampered[len(tampered)-1]
	repl := byte('0')
	if last == '0' {
		repl = '1'
	}
	tampered = tampered[:len(tampered)-1] + string(repl)
	req.Header.Set("Authorization", tampered)

	if err := Verify(req, testCreds()); err != ErrSignatureMismatch {
		t.Fatalf("Verify() = %v, want ErrSignatureMismatch", err)
	}
}

func TestVerifyTamperedPathRejected(t *testing.T) {
	req := signedRequest(t, http.MethodGet, "/assets/format=png", "localhost",
		[]string{"host", "x-amz-content-sha256", "x-amz-date"})

	req.URL.Path = "/assets/format=jpeg"

	if err := Verify(req, testCreds()); err != ErrSignatureMismatch {
		t.Fatalf("Verify() = %v, want ErrSignatureMismatch", err)
	}
}

func TestVerifyUnknownAccessKeyRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Content-Sha256", emptyPayloadHash)
	sign(req, "SOMEOTHERKEY0000000AA", testSecretAccessKey, testDate, []string{"host", "x-amz-content-sha256", "x-amz-date"})

	if err := Verify(req, testCreds()); err != ErrSignatureMismatch {
		t.Fatalf("Verify() = %v, want ErrSignatureMismatch", err)
	}
}

func TestVerifyWrongSecretRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
	req.Host = "localhost"
	req.Header.Set("X-Amz-Content-Sha256", emptyPayloadHash)
	sign(req, testAccessKeyID, "wrongsecretwrongsecretwrongsecretwrong1", testDate, []string{"host", "x-amz-content-sha256", "x-amz-date"})

	if err := Verify(req, testCreds()); err != ErrSignatureMismatch {
		t.Fatalf("Verify() = %v, want ErrSignatureMismatch", err)
	}
}

func TestVerifyMalformedAuthorizationHeaderRejected(t *testing.T) {
	cases := []string{
		"",
		"Basic dXNlcjpwYXNz",
		"AWS4-HMAC-SHA256 Credential=onlycredential",
		"AWS4-HMAC-SHA256 Credential=" + testAccessKeyID + "/20250101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=",
	}

	for _, authHeader := range cases {
		t.Run(authHeader, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
			req.Host = "localhost"
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}

			err := Verify(req, testCreds())
			if authHeader == "" {
				if err != ErrMissingAuthorization {
					t.Fatalf("Verify() = %v, want ErrMissingAuthorization", err)
				}
				return
			}
			if err != ErrSignatureMismatch {
				t.Fatalf("Verify() = %v, want ErrSignatureMismatch", err)
			}
		})
	}
}
