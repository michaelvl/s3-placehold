// Package key parses S3 object keys into synthesis parameters.
package key

import (
	"image/color"
	"strings"
	"time"
)

// Params holds the parsed and validated parameters for a synthesis request.
type Params struct {
	Type     string
	Format   string
	Width    int
	Height   int
	Colour   color.RGBA
	Text     string
	DelayMin time.Duration
	DelayMax time.Duration
}

// Default returns the parameter set used when a key carries no segments.
func Default() Params {
	return Params{
		Type:   "image",
		Format: "svg",
		Width:  100,
		Height: 100,
		Colour: color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff},
	}
}

// Parse parses an S3 key string into Params. A key with no segments yields
// Default(). Full segment grammar and validation is implemented incrementally
// as the parameter vocabulary grows.
func Parse(rawKey string) (Params, error) {
	trimmed := strings.Trim(rawKey, "/")
	if trimmed == "" {
		return Default(), nil
	}
	return Default(), nil
}
