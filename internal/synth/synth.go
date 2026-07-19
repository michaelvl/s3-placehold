// Package synth defines the Synthesizer interface and dispatches by
// Params.Type to the appropriate backend.
package synth

import (
	"errors"
	"fmt"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// ErrUnknownType is wrapped in the error returned for an unrecognised
// Params.Type value.
var ErrUnknownType = errors.New("unknown type")

// Synthesizer produces bytes and a MIME type for the given parameters.
type Synthesizer interface {
	Synthesize(params key.Params) (data []byte, mimeType string, err error)
}

// Router dispatches Synthesize calls to a backend based on Params.Type.
type Router struct {
	Image Synthesizer
}

// NewRouter constructs a Router that dispatches type=image to image.
func NewRouter(image Synthesizer) *Router {
	return &Router{Image: image}
}

// Synthesize implements Synthesizer, dispatching on params.Type.
func (r *Router) Synthesize(params key.Params) (data []byte, mimeType string, err error) {
	switch params.Type {
	case "image":
		return r.Image.Synthesize(params)
	default:
		return nil, "", fmt.Errorf("%w: %q", ErrUnknownType, params.Type)
	}
}
