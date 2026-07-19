package image

import (
	"strings"
	"testing"

	"github.com/michaelvl/s3-placehold/internal/key"
)

func TestSynthesizeDefaultParamsProducesSVG(t *testing.T) {
	s := New()
	params := key.Default()

	data, mimeType, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}

	if mimeType != "image/svg+xml" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/svg+xml")
	}

	svg := string(data)
	if !strings.Contains(svg, `width="100"`) {
		t.Errorf("svg missing width=100: %s", svg)
	}
	if !strings.Contains(svg, `height="100"`) {
		t.Errorf("svg missing height=100: %s", svg)
	}
	if !strings.Contains(svg, "#cccccc") {
		t.Errorf("svg missing fill colour #cccccc: %s", svg)
	}
}

func TestSynthesizeUnsupportedFormat(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "png"

	_, _, err := s.Synthesize(params)
	if err == nil {
		t.Fatalf("Synthesize with unsupported format = nil error, want error")
	}
}
