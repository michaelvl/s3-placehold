// Package key parses S3 object keys into synthesis parameters.
package key

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/colornames"
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

func invalidParam(name, value string) error {
	return fmt.Errorf("Invalid value for parameter '%s': '%s'", name, value)
}

func invalidSegment(seg string) error {
	return fmt.Errorf("Invalid key segment (missing '='): '%s'", seg)
}

// Parse parses an S3 key string into Params. A key with no segments yields
// Default(). Segments are `/`-separated `name=value` pairs, in any order,
// with `,`-separated multi-values and percent-decoding applied to names and
// values.
func Parse(rawKey string) (Params, error) {
	p := Default()

	trimmed := strings.Trim(rawKey, "/")
	if trimmed == "" {
		return p, nil
	}

	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "" {
			continue
		}

		rawName, rawValue, ok := strings.Cut(seg, "=")
		if !ok {
			return Params{}, invalidSegment(seg)
		}

		name, err := url.QueryUnescape(rawName)
		if err != nil {
			return Params{}, invalidSegment(seg)
		}

		values, err := decodeValues(rawValue)
		if err != nil {
			return Params{}, invalidParam(name, rawValue)
		}

		if err := applySegment(&p, name, values); err != nil {
			return Params{}, err
		}
	}

	return p, nil
}

func decodeValues(rawValue string) ([]string, error) {
	rawValues := strings.Split(rawValue, ",")
	values := make([]string, len(rawValues))
	for i, v := range rawValues {
		dv, err := url.QueryUnescape(v)
		if err != nil {
			return nil, err
		}
		values[i] = dv
	}
	return values, nil
}

// applySegment validates and applies a single decoded name/values pair to p.
// Unrecognised segment names are ignored for forward compatibility.
func applySegment(p *Params, name string, values []string) error {
	switch name {
	case "type":
		return applySingleValue(values, name, func(v string) error { return applyType(p, v) })
	case "format":
		return applySingleValue(values, name, func(v string) error { return applyFormat(p, v) })
	case "size":
		return applySingleValue(values, name, func(v string) error { return applySize(p, v) })
	case "colour":
		return applySingleValue(values, name, func(v string) error { return applyColour(p, v) })
	case "text":
		p.Text = strings.Join(values, ",")
	case "delay":
		return applyDelay(p, values)
	}
	return nil
}

// applySingleValue rejects segments carrying more than one comma-separated
// value for parameters that don't have documented multi-value (range)
// semantics.
func applySingleValue(values []string, name string, apply func(string) error) error {
	if len(values) != 1 {
		return invalidParam(name, strings.Join(values, ","))
	}
	return apply(values[0])
}

func applyType(p *Params, v string) error {
	if v != "image" {
		return invalidParam("type", v)
	}
	p.Type = v
	return nil
}

func applyFormat(p *Params, v string) error {
	switch v {
	case "svg", "png", "jpeg":
		p.Format = v
		return nil
	default:
		return invalidParam("format", v)
	}
}

func applySize(p *Params, v string) error {
	wStr, hStr, ok := strings.Cut(v, "x")
	if !ok {
		return invalidParam("size", v)
	}
	w, errW := strconv.Atoi(wStr)
	h, errH := strconv.Atoi(hStr)
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return invalidParam("size", v)
	}
	p.Width = w
	p.Height = h
	return nil
}

func applyColour(p *Params, v string) error {
	if c, ok := parseHexColour(v); ok {
		p.Colour = c
		return nil
	}
	if c, ok := colornames.Map[strings.ToLower(v)]; ok {
		p.Colour = c
		return nil
	}
	return invalidParam("colour", v)
}

// parseHexColour parses a lowercase 6-digit hex colour without a leading
// '#', per the documented value syntax. Uppercase hex digits are rejected.
func parseHexColour(v string) (color.RGBA, bool) {
	if len(v) != 6 || strings.ToLower(v) != v {
		return color.RGBA{}, false
	}
	b, err := hex.DecodeString(v)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: 0xff}, true
}

func applyDelay(p *Params, values []string) error {
	switch len(values) {
	case 1:
		ms, err := strconv.Atoi(values[0])
		if err != nil || ms < 0 {
			return invalidParam("delay", values[0])
		}
		d := time.Duration(ms) * time.Millisecond
		p.DelayMin, p.DelayMax = d, d
	case 2:
		minMs, errMin := strconv.Atoi(values[0])
		maxMs, errMax := strconv.Atoi(values[1])
		if errMin != nil || errMax != nil || minMs < 0 || maxMs < minMs {
			return invalidParam("delay", strings.Join(values, ","))
		}
		p.DelayMin = time.Duration(minMs) * time.Millisecond
		p.DelayMax = time.Duration(maxMs) * time.Millisecond
	default:
		return invalidParam("delay", strings.Join(values, ","))
	}
	return nil
}
