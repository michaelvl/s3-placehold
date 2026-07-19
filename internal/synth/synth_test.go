package synth

import (
	"errors"
	"testing"

	"github.com/michaelvl/s3-placehold/internal/key"
)

type stubSynthesizer struct {
	data     []byte
	mimeType string
	err      error
}

func (s *stubSynthesizer) Synthesize(params key.Params) ([]byte, string, error) {
	return s.data, s.mimeType, s.err
}

func TestRouterDispatchesImageType(t *testing.T) {
	image := &stubSynthesizer{data: []byte("svg-bytes"), mimeType: "image/svg+xml"}
	router := NewRouter(image)

	data, mimeType, err := router.Synthesize(key.Default())
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if string(data) != "svg-bytes" {
		t.Errorf("data = %q, want %q", data, "svg-bytes")
	}
	if mimeType != "image/svg+xml" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/svg+xml")
	}
}

func TestRouterUnknownType(t *testing.T) {
	router := NewRouter(&stubSynthesizer{})
	params := key.Default()
	params.Type = "pdf"

	_, _, err := router.Synthesize(params)
	if err == nil {
		t.Fatalf("Synthesize with unknown type = nil error, want error")
	}
	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("err = %v, want wrapping ErrUnknownType", err)
	}
}
