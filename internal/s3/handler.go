// Package s3 implements the S3-compatible HTTP surface: routing, request
// dispatch, and response serialization.
package s3

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/key"
	"github.com/michaelvl/s3-placehold/internal/sigv4"
	"github.com/michaelvl/s3-placehold/internal/synth"
)

// Handler serves the S3-compatible HTTP API.
type Handler struct {
	cfg   config.Config
	synth synth.Synthesizer
}

// NewHandler constructs a Handler.
func NewHandler(cfg config.Config, synthesizer synth.Synthesizer) *Handler {
	return &Handler{cfg: cfg, synth: synthesizer}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Type, Content-Length, x-amz-request-id")

	if r.Method == http.MethodOptions {
		respondPreflight(w)
		return
	}

	bucket, objectKey := splitRequestPath(r.Host, r.URL.Path)

	bucketCfg, ok := h.cfg.Lookup(bucket)
	if !ok {
		writeS3Error(w, http.StatusNotFound, "NoSuchBucket", "The specified bucket does not exist.")
		return
	}

	query := r.URL.Query()
	presigned := query.Has("X-Amz-Signature")

	if bucketCfg.Mode == config.ModePrivate {
		if !h.authorize(w, r, presigned) {
			return
		}
	}

	noQuery := len(query) == 0

	switch {
	case r.Method == http.MethodGet && (noQuery || presigned):
		h.respondObject(w, objectKey, true)
	case r.Method == http.MethodHead && (noQuery || presigned):
		h.respondObject(w, objectKey, false)
	case r.Method == http.MethodGet && isListRequest(query):
		h.respondListObjects(w, bucket)
	case r.Method == http.MethodDelete && noQuery:
		h.respondDeleteObject(w)
	case r.Method == http.MethodPost && query.Has("delete"):
		h.respondDeleteObjects(w)
	default:
		writeS3Error(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "The specified method is not allowed against this resource.")
	}
}

// authorize validates the SigV4 signature on a request to a private
// bucket, writing the appropriate S3 error and returning false on failure.
// presigned selects query-string (presigned URL) validation over the
// default Authorization-header validation.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request, presigned bool) bool {
	creds := sigv4.Credentials{AccessKeyID: h.cfg.AccessKeyID, SecretAccessKey: h.cfg.SecretAccessKey}

	if presigned {
		if err := sigv4.VerifyPresigned(r, creds); err != nil {
			writeS3Error(w, http.StatusForbidden, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided.")
			return false
		}
		return true
	}

	switch err := sigv4.Verify(r, creds); err {
	case nil:
		return true
	case sigv4.ErrMissingAuthorization:
		writeS3Error(w, http.StatusForbidden, "AccessDenied", "Access Denied")
		return false
	default:
		writeS3Error(w, http.StatusForbidden, "SignatureDoesNotMatch", "The request signature we calculated does not match the signature you provided.")
		return false
	}
}

// respondObject synthesizes objectKey and writes the response headers,
// including the body only when includeBody is set (GetObject vs HeadObject).
func (h *Handler) respondObject(w http.ResponseWriter, objectKey string, includeBody bool) {
	params, err := key.Parse(objectKey)
	if err != nil {
		writeInvalidArgument(w, err)
		return
	}

	data, mimeType, err := h.synth.Synthesize(params)
	if err != nil {
		writeInvalidArgument(w, err)
		return
	}

	time.Sleep(synth.DelayDuration(params.DelayMin, params.DelayMax))

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if includeBody {
		_, _ = w.Write(data)
	}
}

// respondPreflight writes the 200 OK, no-body response to a CORS preflight
// OPTIONS request, ahead of bucket lookup and auth so preflight succeeds
// even for an unconfigured or private bucket.
func respondPreflight(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, DELETE, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, x-amz-date, x-amz-content-sha256, x-amz-security-token, x-amz-user-agent")
	w.Header().Set("Access-Control-Max-Age", "3600")
	w.WriteHeader(http.StatusOK)
}

// writeInvalidArgument writes an InvalidArgument 400 whose message is the
// given error's text.
func writeInvalidArgument(w http.ResponseWriter, err error) {
	writeS3Error(w, http.StatusBadRequest, "InvalidArgument", err.Error())
}

// splitRequestPath extracts the bucket name and object key from a request,
// detecting virtual-hosted vs. path-style addressing from the Host header.
// A Host whose hostname carries more than one dot-separated label is
// virtual-hosted ({bucket}.{server-host}): the first label is the bucket
// and the full path is the key. Otherwise it's path-style: the first path
// segment is the bucket and the remainder is the key. An IP-literal
// hostname (e.g. a Docker Compose service reached by container IP) is
// always path-style, even though it contains dots.
func splitRequestPath(host, path string) (bucket, objectKey string) {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	if net.ParseIP(hostname) == nil {
		if label, _, ok := strings.Cut(hostname, "."); ok {
			return label, strings.TrimPrefix(path, "/")
		}
	}
	return splitPathStyle(path)
}

// splitPathStyle extracts the bucket name and object key from a path-style
// request path: the first segment is the bucket, the remainder is the key.
func splitPathStyle(path string) (bucket, objectKey string) {
	trimmed := strings.TrimPrefix(path, "/")
	bucket, objectKey, _ = strings.Cut(trimmed, "/")
	return bucket, objectKey
}
