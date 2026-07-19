// Package image implements the synth.Synthesizer for type=image requests.
package image

import (
	"fmt"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// Synthesizer produces raster/vector image bytes for type=image parameters.
type Synthesizer struct{}

// New constructs an image Synthesizer.
func New() *Synthesizer {
	return &Synthesizer{}
}

// Synthesize renders the image described by params.
func (s *Synthesizer) Synthesize(params key.Params) (data []byte, mimeType string, err error) {
	switch params.Format {
	case "svg":
		return renderSVG(params), "image/svg+xml", nil
	default:
		return nil, "", fmt.Errorf("unsupported format: %q", params.Format)
	}
}

func renderSVG(params key.Params) []byte {
	fill := fmt.Sprintf("#%02x%02x%02x", params.Colour.R, params.Colour.G, params.Colour.B)
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><rect width="100%%" height="100%%" fill="%s"/></svg>`,
		params.Width, params.Height, fill,
	)
	return []byte(svg)
}
