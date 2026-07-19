// Package s3 implements the S3-compatible HTTP surface: routing, request
// dispatch, and response serialization.
package s3

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/key"
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
	_, objectKey := splitPathStyle(r.URL.Path)

	noQuery := len(r.URL.Query()) == 0

	switch {
	case r.Method == http.MethodGet && noQuery:
		h.respondObject(w, objectKey, true)
	case r.Method == http.MethodHead && noQuery:
		h.respondObject(w, objectKey, false)
	default:
		writeS3Error(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "The specified method is not allowed against this resource.")
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

// writeInvalidArgument writes an InvalidArgument 400 whose message is the
// given error's text.
func writeInvalidArgument(w http.ResponseWriter, err error) {
	writeS3Error(w, http.StatusBadRequest, "InvalidArgument", err.Error())
}

// splitPathStyle extracts the bucket name and object key from a path-style
// request path: the first segment is the bucket, the remainder is the key.
func splitPathStyle(path string) (bucket, objectKey string) {
	trimmed := strings.TrimPrefix(path, "/")
	bucket, objectKey, _ = strings.Cut(trimmed, "/")
	return bucket, objectKey
}
