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

// Default upper bounds on requested image dimensions, used by Parse. Callers
// that need different bounds (e.g. from configuration) should use
// ParseWithLimits or ParseWithOptions directly.
const (
	DefaultMaxWidth  = 10000
	DefaultMaxHeight = 10000
)

// Default image dimensions, used when neither the key nor the server
// configuration specifies a size.
const (
	DefaultWidth  = 100
	DefaultHeight = 100
)

// DefaultColour returns the background fill used when neither the key nor the
// server configuration specifies a colour.
func DefaultColour() color.RGBA {
	return color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
}

// Options carries server configuration into key parsing: the size caps a
// `size` segment is checked against, and the size, colour and delay applied
// to keys that carry no `size` / `colour` / `delay` segment. A zero
// DefaultWidth or DefaultHeight means the built-in DefaultWidth/DefaultHeight,
// a fully transparent DefaultColour means DefaultColour(), and a zero delay
// means no delay.
type Options struct {
	MaxWidth        int
	MaxHeight       int
	DefaultWidth    int
	DefaultHeight   int
	DefaultColour   color.RGBA
	DefaultDelayMin time.Duration
	DefaultDelayMax time.Duration
}

// DefaultOptions returns the options used by Parse: the default size bounds,
// dimensions and colour, and no delay.
func DefaultOptions() Options {
	return Options{
		MaxWidth:      DefaultMaxWidth,
		MaxHeight:     DefaultMaxHeight,
		DefaultWidth:  DefaultWidth,
		DefaultHeight: DefaultHeight,
		DefaultColour: DefaultColour(),
	}
}

// Default returns the parameter set used when a key carries no segments.
func Default() Params {
	return Params{
		Type:   "image",
		Format: "svg",
		Width:  DefaultWidth,
		Height: DefaultHeight,
		Colour: DefaultColour(),
	}
}

func invalidParam(name, value string) error {
	return fmt.Errorf("Invalid value for parameter '%s': '%s'", name, value) //nolint:staticcheck // ST1005: wording fixed by API contract, see docs/spec.md
}

func invalidSegment(seg string) error {
	return fmt.Errorf("Invalid key segment (missing '='): '%s'", seg) //nolint:staticcheck // ST1005: wording fixed by API contract, see docs/spec.md
}

// Parse parses an S3 key string into Params using the default size bounds
// (DefaultMaxWidth, DefaultMaxHeight). A key with no segments yields
// Default(). Segments are `/`-separated `name=value` pairs, in any order,
// with `,`-separated multi-values and percent-decoding applied to names and
// values.
func Parse(rawKey string) (Params, error) {
	return ParseWithOptions(rawKey, DefaultOptions())
}

// ParseWithLimits parses an S3 key string into Params, rejecting a `size`
// segment whose width or height exceeds maxWidth or maxHeight. See Parse for
// the key grammar.
func ParseWithLimits(rawKey string, maxWidth, maxHeight int) (Params, error) {
	return ParseWithOptions(rawKey, Options{MaxWidth: maxWidth, MaxHeight: maxHeight})
}

// ParseWithOptions parses an S3 key string into Params under opts: a `size`
// segment exceeding opts.MaxWidth/MaxHeight is rejected, and a key with no
// `size`, `colour` or `delay` segment gets the corresponding configured
// default. An explicit segment always overrides the configured default. See
// Parse for the key grammar.
func ParseWithOptions(rawKey string, opts Options) (Params, error) {
	p := Default()
	if opts.DefaultWidth > 0 && opts.DefaultHeight > 0 {
		p.Width, p.Height = opts.DefaultWidth, opts.DefaultHeight
	}
	if opts.DefaultColour.A != 0 {
		p.Colour = opts.DefaultColour
	}
	p.DelayMin, p.DelayMax = opts.DefaultDelayMin, opts.DefaultDelayMax

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

		if err := applySegment(&p, name, values, opts.MaxWidth, opts.MaxHeight); err != nil {
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
func applySegment(p *Params, name string, values []string, maxWidth, maxHeight int) error {
	switch name {
	case "type":
		return applySingleValue(values, name, func(v string) error { return applyType(p, v) })
	case "format":
		return applySingleValue(values, name, func(v string) error { return applyFormat(p, v) })
	case "size":
		return applySingleValue(values, name, func(v string) error { return applySize(p, v, maxWidth, maxHeight) })
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

func applySize(p *Params, v string, maxWidth, maxHeight int) error {
	w, h, ok := ParseSize(v, maxWidth, maxHeight)
	if !ok {
		return invalidParam("size", v)
	}
	p.Width = w
	p.Height = h
	return nil
}

// ParseSize parses the `size` value syntax — `{width}x{height}` in pixels —
// reporting whether the value is well-formed, positive, and within
// maxWidth/maxHeight. Callers outside key parsing (e.g. configuration) use it
// to accept the same syntax.
func ParseSize(v string, maxWidth, maxHeight int) (width, height int, ok bool) {
	wStr, hStr, cut := strings.Cut(v, "x")
	if !cut {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(wStr)
	h, errH := strconv.Atoi(hStr)
	if errW != nil || errH != nil || w <= 0 || h <= 0 || w > maxWidth || h > maxHeight {
		return 0, 0, false
	}
	return w, h, true
}

func applyColour(p *Params, v string) error {
	c, ok := ParseColour(v)
	if !ok {
		return invalidParam("colour", v)
	}
	p.Colour = c
	return nil
}

// ParseColour parses the `colour` value syntax — a lowercase 6-digit hex
// value without a leading '#', or a CSS named colour — reporting whether the
// value is recognised. Callers outside key parsing (e.g. configuration) use
// it to accept the same syntax.
func ParseColour(v string) (color.RGBA, bool) {
	if c, ok := parseHexColour(v); ok {
		return c, true
	}
	if c, ok := colornames.Map[strings.ToLower(v)]; ok {
		return c, true
	}
	return color.RGBA{}, false
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
	lo, hi, ok := ParseDelay(values)
	if !ok {
		return invalidParam("delay", strings.Join(values, ","))
	}
	p.DelayMin, p.DelayMax = lo, hi
	return nil
}

// ParseDelay parses the `delay` value syntax — a single non-negative
// millisecond count, or a `min,max` pair — into an inclusive duration range,
// reporting whether the values are valid. Callers outside key parsing (e.g.
// configuration) use it to accept the same syntax.
func ParseDelay(values []string) (lo, hi time.Duration, ok bool) {
	switch len(values) {
	case 1:
		ms, err := strconv.Atoi(values[0])
		if err != nil || ms < 0 {
			return 0, 0, false
		}
		d := time.Duration(ms) * time.Millisecond
		return d, d, true
	case 2:
		minMs, errMin := strconv.Atoi(values[0])
		maxMs, errMax := strconv.Atoi(values[1])
		if errMin != nil || errMax != nil || minMs < 0 || maxMs < minMs {
			return 0, 0, false
		}
		return time.Duration(minMs) * time.Millisecond, time.Duration(maxMs) * time.Millisecond, true
	default:
		return 0, 0, false
	}
}
