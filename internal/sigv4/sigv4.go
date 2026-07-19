// Package sigv4 validates AWS Signature Version 4 signatures carried in a
// request's Authorization header, against a single configured credential
// pair. It only verifies signatures; it never generates them.
package sigv4

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrMissingAuthorization is returned when the request carries no
// Authorization header at all.
var ErrMissingAuthorization = errors.New("sigv4: missing Authorization header")

// ErrSignatureMismatch is returned when an Authorization header is present
// but malformed, references an unknown access key, or its signature does
// not match the request.
var ErrSignatureMismatch = errors.New("sigv4: signature does not match")

// ErrExpired is returned by VerifyPresigned when the signature is valid but
// the X-Amz-Date + X-Amz-Expires window has already elapsed.
var ErrExpired = errors.New("sigv4: presigned URL has expired")

// amzDateLayout is the time.Parse layout for X-Amz-Date's ISO 8601 basic
// format, e.g. "20060102T150405Z".
const amzDateLayout = "20060102T150405Z"

// emptyPayloadHash is Hex(SHA256("")), the hashed payload for a request
// with no body (every operation this server signs is GET/HEAD).
const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// unsignedPayload is the reserved payload-hash sentinel used in the
// canonical request for query-string (presigned URL) signing, per SigV4.
const unsignedPayload = "UNSIGNED-PAYLOAD"

// Credentials is the single configured access/secret key pair signatures
// are validated against.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

// credentialScope is the parsed Credential= component of an Authorization
// header: <access-key>/<date>/<region>/<service>/aws4_request.
type credentialScope struct {
	accessKeyID string
	date        string
	region      string
	service     string
}

// Verify validates req's Authorization header against creds. It returns nil
// if the signature is valid, ErrMissingAuthorization if no Authorization
// header is present, or ErrSignatureMismatch for any other failure
// (malformed header, unknown access key, or a signature that does not
// match the request).
func Verify(req *http.Request, creds Credentials) error {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return ErrMissingAuthorization
	}

	scope, signedHeaders, signature, err := parseAuthorization(authHeader)
	if err != nil {
		return ErrSignatureMismatch
	}

	amzDate := req.Header.Get("X-Amz-Date")
	canonicalRequest := buildCanonicalRequest(req, signedHeaders)
	return checkSignature(creds, scope, amzDate, canonicalRequest, signature)
}

// checkSignature validates that scope's access key matches creds and that
// amzDate is consistent with scope's date, then computes the expected SigV4
// signature for canonicalRequest and compares it against signature. It is
// the common tail shared by header-based and query-string (presigned)
// verification, which differ only in where scope, amzDate, canonicalRequest,
// and signature are sourced from.
func checkSignature(creds Credentials, scope credentialScope, amzDate, canonicalRequest, signature string) error {
	if scope.accessKeyID != creds.AccessKeyID {
		return ErrSignatureMismatch
	}
	if len(amzDate) < 8 || amzDate[:8] != scope.date {
		return ErrSignatureMismatch
	}

	stringToSign := buildStringToSign(amzDate, scope, canonicalRequest)
	signingKey := deriveSigningKey(creds.SecretAccessKey, scope.date, scope.region, scope.service)
	expected := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return ErrSignatureMismatch
	}
	return nil
}

// parseAuthorization parses an "AWS4-HMAC-SHA256 Credential=...,
// SignedHeaders=..., Signature=..." Authorization header value.
func parseAuthorization(header string) (credentialScope, []string, string, error) {
	const prefix = "AWS4-HMAC-SHA256 "
	if !strings.HasPrefix(header, prefix) {
		return credentialScope{}, nil, "", errors.New("sigv4: unsupported algorithm")
	}

	var scope credentialScope
	var signedHeaders []string
	var signature string

	for _, part := range strings.Split(strings.TrimPrefix(header, prefix), ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return credentialScope{}, nil, "", errors.New("sigv4: malformed authorization component")
		}
		switch key {
		case "Credential":
			var err error
			scope, err = parseCredential(value)
			if err != nil {
				return credentialScope{}, nil, "", err
			}
		case "SignedHeaders":
			signedHeaders = strings.Split(value, ";")
		case "Signature":
			signature = value
		default:
			return credentialScope{}, nil, "", errors.New("sigv4: unknown authorization component")
		}
	}

	if scope.accessKeyID == "" || len(signedHeaders) == 0 || signature == "" {
		return credentialScope{}, nil, "", errors.New("sigv4: incomplete authorization header")
	}
	return scope, signedHeaders, signature, nil
}

// parseCredential parses a Credential value shared by both the Authorization
// header and the X-Amz-Credential query parameter:
// <access-key>/<date>/<region>/<service>/aws4_request.
func parseCredential(value string) (credentialScope, error) {
	fields := strings.Split(value, "/")
	if len(fields) != 5 || fields[4] != "aws4_request" {
		return credentialScope{}, errors.New("sigv4: malformed credential scope")
	}
	return credentialScope{accessKeyID: fields[0], date: fields[1], region: fields[2], service: fields[3]}, nil
}

// VerifyPresigned validates a presigned URL's query-string SigV4 signature
// (X-Amz-Signature, X-Amz-Credential, X-Amz-Date, and — if present —
// X-Amz-SignedHeaders) against creds, and that its X-Amz-Expires window
// (relative to X-Amz-Date) has not elapsed. It returns nil if the request is
// valid and unexpired, ErrExpired if the signature checks out but the
// expiry window has passed, or ErrSignatureMismatch for any other failure:
// no X-Amz-Signature parameter, a malformed or unknown credential, a
// malformed X-Amz-Expires, or a signature that does not match the request.
// Callers are expected to have already detected the request as presigned
// (X-Amz-Signature present) before calling this.
func VerifyPresigned(req *http.Request, creds Credentials) error {
	query := req.URL.Query()

	signature := query.Get("X-Amz-Signature")
	if signature == "" {
		return ErrSignatureMismatch
	}

	scope, err := parseCredential(query.Get("X-Amz-Credential"))
	if err != nil {
		return ErrSignatureMismatch
	}

	signedHeaders := []string{"host"}
	if sh := query.Get("X-Amz-SignedHeaders"); sh != "" {
		signedHeaders = strings.Split(sh, ";")
	}

	amzDate := query.Get("X-Amz-Date")
	canonicalRequest := buildPresignedCanonicalRequest(req, query, signedHeaders)
	if err := checkSignature(creds, scope, amzDate, canonicalRequest, signature); err != nil {
		return err
	}
	return checkExpiry(amzDate, query.Get("X-Amz-Expires"))
}

// checkExpiry parses amzDate and expiresParam (the X-Amz-Date and
// X-Amz-Expires query parameters of a presigned URL) and reports whether
// the expiry window they describe has elapsed relative to the current time.
// Both values are covered by the signature, so a malformed amzDate or
// expiresParam here reflects a bad request rather than tampering; lacking a
// more specific code, that case is also reported as ErrSignatureMismatch. An
// elapsed window is reported as ErrExpired.
func checkExpiry(amzDate, expiresParam string) error {
	signedAt, err := time.Parse(amzDateLayout, amzDate)
	if err != nil {
		return ErrSignatureMismatch
	}

	expiresSeconds, err := strconv.Atoi(expiresParam)
	if err != nil || expiresSeconds < 0 {
		return ErrSignatureMismatch
	}

	if time.Now().After(signedAt.Add(time.Duration(expiresSeconds) * time.Second)) {
		return ErrExpired
	}
	return nil
}

// buildPresignedCanonicalRequest assembles the SigV4 canonical request
// string for a presigned URL request: like buildCanonicalRequest, but the
// query string excludes X-Amz-Signature (added to the URL only after
// signing) and the payload hash is the "UNSIGNED-PAYLOAD" sentinel rather
// than a hash of the (nonexistent) body.
func buildPresignedCanonicalRequest(req *http.Request, query url.Values, signedHeaders []string) string {
	headerBlock, signedHeadersStr := canonicalHeaders(req, signedHeaders)

	toSign := url.Values{}
	for k, v := range query {
		if k == "X-Amz-Signature" {
			continue
		}
		toSign[k] = v
	}

	return strings.Join([]string{
		req.Method,
		canonicalURIPath(req.URL.Path),
		canonicalQueryString(toSign),
		headerBlock,
		signedHeadersStr,
		unsignedPayload,
	}, "\n")
}

// buildCanonicalRequest assembles the SigV4 canonical request string for
// req, restricted to signedHeaders.
func buildCanonicalRequest(req *http.Request, signedHeaders []string) string {
	headerBlock, signedHeadersStr := canonicalHeaders(req, signedHeaders)

	payloadHash := req.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = emptyPayloadHash
	}

	return strings.Join([]string{
		req.Method,
		canonicalURIPath(req.URL.Path),
		canonicalQueryString(req.URL.Query()),
		headerBlock,
		signedHeadersStr,
		payloadHash,
	}, "\n")
}

// canonicalURIPath URI-encodes each segment of path individually, leaving
// the separating slashes intact (S3's canonical URI is not double-encoded).
func canonicalURIPath(path string) string {
	if path == "" {
		return "/"
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = uriEncode(seg)
	}
	return strings.Join(segments, "/")
}

// canonicalQueryString URI-encodes and sorts query into SigV4's canonical
// query string form.
func canonicalQueryString(query url.Values) string {
	pairs := make([]string, 0, len(query))
	for k, values := range query {
		for _, v := range values {
			pairs = append(pairs, uriEncode(k)+"="+uriEncode(v))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

// canonicalHeaders builds the CanonicalHeaders block and the sorted
// SignedHeaders list for the given (lowercased, deduplicated) header names.
func canonicalHeaders(req *http.Request, signedHeaders []string) (block, signedHeadersStr string) {
	names := make([]string, len(signedHeaders))
	for i, n := range signedHeaders {
		names[i] = strings.ToLower(n)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		var value string
		if name == "host" {
			value = req.Host
		} else {
			values := req.Header.Values(http.CanonicalHeaderKey(name))
			for i, v := range values {
				values[i] = collapseSpaces(v)
			}
			value = strings.Join(values, ",")
		}
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(collapseSpaces(value))
		b.WriteByte('\n')
	}
	return b.String(), strings.Join(names, ";")
}

// collapseSpaces trims leading/trailing whitespace and collapses interior
// runs of whitespace to a single space, per the SigV4 Trim() rule.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// buildStringToSign assembles the SigV4 string to sign.
func buildStringToSign(amzDate string, scope credentialScope, canonicalRequest string) string {
	hash := sha256.Sum256([]byte(canonicalRequest))
	return strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope.date + "/" + scope.region + "/" + scope.service + "/aws4_request",
		hex.EncodeToString(hash[:]),
	}, "\n")
}

// deriveSigningKey computes the SigV4 signing key from secret, date,
// region, and service via the standard chain of HMAC-SHA256 derivations.
func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// uriEncode percent-encodes s per SigV4 rules: unreserved characters
// (A-Z a-z 0-9 - _ . ~) pass through as-is; everything else, including '/',
// is percent-encoded with uppercase hex. canonicalURIPath calls this
// per path segment and rejoins with literal '/' separators, so the
// separators themselves are never affected by this encoding.
func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			b.WriteString("%")
			b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '_' || c == '.' || c == '~'
}
