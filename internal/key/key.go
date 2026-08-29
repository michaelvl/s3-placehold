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

// GradientKind is the geometry a multi-colour background is painted with. The
// zero value means "not specified"; every valid `gradient` value maps to a
// non-empty kind, including GradientNone. Parsing relies on that to tell an
// explicit `gradient` segment apart from an absent one.
type GradientKind string

// The gradient geometries a `gradient` segment can select.
const (
	GradientNone   GradientKind = "none"
	GradientLinear GradientKind = "linear"
	GradientRadial GradientKind = "radial"
	GradientMesh   GradientKind = "mesh"
)

// Gradient is the background fill geometry. Angle applies only to
// GradientLinear: degrees clockwise from "towards the top", following the CSS
// linear-gradient convention, normalised to [0,360).
type Gradient struct {
	Kind  GradientKind
	Angle int
}

// IsSet reports whether the gradient geometry has been specified.
func (g Gradient) IsSet() bool { return g.Kind != "" }

// Params holds the parsed and validated parameters for a synthesis request.
type Params struct {
	Type     string
	Format   string
	Width    int
	Height   int
	Colours  []color.RGBA // at least one; Colours[0] is the base colour
	Gradient Gradient
	Text     string
	DelayMin time.Duration
	DelayMax time.Duration
}

// BaseColour returns the flat fill, which is also the gradient's first stop.
func (p Params) BaseColour() color.RGBA {
	if len(p.Colours) == 0 {
		return DefaultColour()
	}
	return p.Colours[0]
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

// MaxColours bounds the length of a `colour` list. It keeps generated SVG
// small and bounds the per-pixel work in the raster renderer, both of which
// grow linearly with the number of colours.
const MaxColours = 8

// DefaultGradientAngle is the linear gradient angle used when `gradient=linear`
// carries no angle, and by the multi-colour fallback geometry. 90 degrees is
// left-to-right.
const DefaultGradientAngle = 90

// DefaultColour returns the background fill used when neither the key nor the
// server configuration specifies a colour.
func DefaultColour() color.RGBA {
	return color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
}

// DefaultColours returns DefaultColour as a one-element list, the shape
// Params.Colours takes when no colour is specified.
func DefaultColours() []color.RGBA {
	return []color.RGBA{DefaultColour()}
}

// ColourSpec is a `colour` list member before it has been resolved to a
// colour: either a fixed colour, or a request for a stable pseudo-random one.
// Random members cannot be resolved where they are parsed, because a seedless
// `random` draws on the request's `size` and `text` — which may appear in a
// later segment of the key, and which a configured default does not know at
// all until a request arrives. Resolution happens once per request, at the end
// of parsing, in resolveColours.
type ColourSpec struct {
	Colour color.RGBA // the colour, when Random is false
	Random bool
	Seed   string // explicit seed for a random member; "" derives one from the request
}

// FixedColour returns a ColourSpec for an already-known colour.
func FixedColour(c color.RGBA) ColourSpec { return ColourSpec{Colour: c} }

// DefaultColourSpecs returns DefaultColour as a one-element spec list, the
// shape Options.DefaultColours takes when no colour is configured.
func DefaultColourSpecs() []ColourSpec {
	return []ColourSpec{FixedColour(DefaultColour())}
}

// Options carries server configuration into key parsing: the size caps a
// `size` segment is checked against, and the size, colours, gradient and delay
// applied to keys that carry no `size` / `colour` / `gradient` / `delay`
// segment. A zero DefaultWidth or DefaultHeight means the built-in
// DefaultWidth/DefaultHeight, an empty DefaultColours means
// DefaultColourSpecs(), an unset DefaultGradient means the geometry is chosen
// from the number of colours, and a zero delay means no delay.
//
// DefaultColours holds unresolved specs rather than colours so that a
// configured default can be `random`: it is resolved per request, against that
// request's key, not once at startup.
type Options struct {
	MaxWidth        int
	MaxHeight       int
	DefaultWidth    int
	DefaultHeight   int
	DefaultColours  []ColourSpec
	DefaultGradient Gradient
	DefaultDelayMin time.Duration
	DefaultDelayMax time.Duration
}

// DefaultOptions returns the options used by Parse: the default size bounds,
// dimensions and colours, no configured gradient, and no delay.
func DefaultOptions() Options {
	return Options{
		MaxWidth:       DefaultMaxWidth,
		MaxHeight:      DefaultMaxHeight,
		DefaultWidth:   DefaultWidth,
		DefaultHeight:  DefaultHeight,
		DefaultColours: DefaultColourSpecs(),
	}
}

// Default returns the parameter set used when a key carries no segments.
func Default() Params {
	return Params{
		Type:     "image",
		Format:   "svg",
		Width:    DefaultWidth,
		Height:   DefaultHeight,
		Colours:  DefaultColours(),
		Gradient: Gradient{Kind: GradientNone},
	}
}

func invalidParam(name, value string) error {
	return fmt.Errorf("Invalid value for parameter '%s': '%s'", name, value) //nolint:staticcheck // ST1005: wording fixed by API contract, see README
}

func invalidSegment(seg string) error {
	return fmt.Errorf("Invalid key segment (missing '='): '%s'", seg) //nolint:staticcheck // ST1005: wording fixed by API contract, see README
}

func gradientNeedsColours(kind GradientKind) error {
	return fmt.Errorf("Parameter 'gradient' value '%s' requires at least two 'colour' values", kind) //nolint:staticcheck // ST1005: wording fixed by API contract, see README
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
// `size`, `colour`, `gradient` or `delay` segment gets the corresponding
// configured default. An explicit segment always overrides the configured
// default. See Parse for the key grammar.
func ParseWithOptions(rawKey string, opts Options) (Params, error) {
	p := Default()
	// Cleared so resolveGradient below can tell an explicit `gradient` segment
	// apart from the built-in default: every valid segment value yields a
	// non-empty Kind, including `gradient=none`.
	p.Gradient = Gradient{}

	if opts.DefaultWidth > 0 && opts.DefaultHeight > 0 {
		p.Width, p.Height = opts.DefaultWidth, opts.DefaultHeight
	}
	p.DelayMin, p.DelayMax = opts.DefaultDelayMin, opts.DefaultDelayMax

	st := parseState{params: &p, colours: DefaultColourSpecs()}
	if len(opts.DefaultColours) > 0 {
		// Shared, not copied: opts is built once per server and read by
		// concurrent requests, but these specs are only ever read here —
		// resolveColours below allocates the slice that escapes into Params.
		st.colours = opts.DefaultColours
	}

	if trimmed := strings.Trim(rawKey, "/"); trimmed != "" {
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

			if err := applySegment(&st, name, values, opts.MaxWidth, opts.MaxHeight); err != nil {
				return Params{}, err
			}
		}
	}

	// Deferred until every segment has been seen: a seedless `random` draws
	// on `size` and `text`, which may appear after `colour` in the key, or
	// come from a configured default that never saw the key at all.
	p.Colours = resolveColours(st.colours, p)

	// A gradient needs two stops to interpolate between. A key that asked for
	// one explicitly is told so rather than being handed a flat fill it did
	// not ask for; a configured default degrades quietly, below, because such
	// a key requested no gradient at all.
	if p.Gradient.IsSet() && p.Gradient.Kind != GradientNone && len(p.Colours) < 2 {
		return Params{}, gradientNeedsColours(p.Gradient.Kind)
	}

	p.Gradient = resolveGradient(p.Gradient, opts.DefaultGradient, len(p.Colours))
	return p, nil
}

// resolveGradient picks the background geometry: an explicit `gradient`
// segment wins, then the configured default, then a default-angle linear
// gradient. A single colour always resolves to a flat fill, so the resolved
// geometry is never one the renderer would have to discard.
func resolveGradient(seen, configured Gradient, nColours int) Gradient {
	// One colour has nothing to interpolate towards. A key that asked for a
	// gradient explicitly has already been rejected, so this only flattens a
	// configured default, which requested nothing of this key.
	if nColours < 2 {
		return Gradient{Kind: GradientNone}
	}
	switch {
	case seen.IsSet():
		return seen
	case configured.IsSet():
		return configured
	default:
		return Gradient{Kind: GradientLinear, Angle: DefaultGradientAngle}
	}
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

// parseState is the parameter set being built plus the information needed to
// finish it once every segment has been seen. Segments may appear in any
// order, so anything depending on another segment's value has to be recorded
// here and resolved at the end.
type parseState struct {
	params  *Params
	colours []ColourSpec // the configured default until a `colour` segment replaces it
}

// applySegment validates and applies a single decoded name/values pair.
// Unrecognised segment names are ignored for forward compatibility.
func applySegment(st *parseState, name string, values []string, maxWidth, maxHeight int) error {
	p := st.params
	switch name {
	case "type":
		return applySingleValue(values, name, func(v string) error { return applyType(p, v) })
	case "format":
		return applySingleValue(values, name, func(v string) error { return applyFormat(p, v) })
	case "size":
		return applySingleValue(values, name, func(v string) error { return applySize(p, v, maxWidth, maxHeight) })
	case "colour":
		return applyColours(st, values)
	case "gradient":
		return applySingleValue(values, name, func(v string) error { return applyGradient(p, v) })
	case "text":
		p.Text = strings.Join(values, ",")
	case "delay":
		return applyDelay(p, values)
	}
	return nil
}

// applySingleValue rejects segments carrying more than one comma-separated
// value for parameters that don't have documented multi-value semantics.
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

// applyColours applies a `colour` list, replacing the configured default.
// Members are left unresolved until the rest of the key has been parsed.
func applyColours(st *parseState, values []string) error {
	specs, ok := ParseColourSpecs(values)
	if !ok {
		return invalidParam("colour", strings.Join(values, ","))
	}
	st.colours = specs
	return nil
}

// ParseColourSpecs parses the `colour` value syntax — one to MaxColours
// values, each a lowercase 6-digit hex value without a leading '#', a CSS
// named colour, or `random` with an optional `:{seed}` — reporting whether
// every value is recognised and the list is within bounds. Random members are
// returned unresolved; see ColourSpec. Callers outside key parsing (e.g.
// configuration) use it to accept the same syntax.
func ParseColourSpecs(values []string) ([]ColourSpec, bool) {
	if len(values) == 0 || len(values) > MaxColours {
		return nil, false
	}
	specs := make([]ColourSpec, len(values))
	for i, v := range values {
		s, ok := parseColourSpec(v)
		if !ok {
			return nil, false
		}
		specs[i] = s
	}
	return specs, true
}

// parseColourSpec parses a single `colour` list member.
func parseColourSpec(v string) (ColourSpec, bool) {
	if seed, isRandom, ok := parseRandomColour(v); isRandom {
		if !ok {
			return ColourSpec{}, false
		}
		return ColourSpec{Random: true, Seed: seed}, true
	}
	c, ok := ParseColour(v)
	if !ok {
		return ColourSpec{}, false
	}
	return FixedColour(c), true
}

// ParseColour parses a single `colour` list member — a lowercase 6-digit hex
// value without a leading '#', or a CSS named colour — reporting whether the
// value is recognised.
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

func applyGradient(p *Params, v string) error {
	g, ok := ParseGradient(v)
	if !ok {
		return invalidParam("gradient", v)
	}
	p.Gradient = g
	return nil
}

// ParseGradient parses the `gradient` value syntax — `linear` with an
// optional `:{degrees}` suffix, or `radial`, `mesh` or `none` — reporting
// whether the value is recognised. An angle is accepted only on `linear`, and
// is normalised into [0,360) so equal geometries compare equal. Callers
// outside key parsing (e.g. configuration) use it to accept the same syntax.
func ParseGradient(v string) (Gradient, bool) {
	name, angleStr, hasAngle := strings.Cut(v, ":")

	var kind GradientKind
	switch GradientKind(name) {
	case GradientNone, GradientLinear, GradientRadial, GradientMesh:
		kind = GradientKind(name)
	default:
		return Gradient{}, false
	}

	if !hasAngle {
		if kind == GradientLinear {
			return Gradient{Kind: kind, Angle: DefaultGradientAngle}, true
		}
		return Gradient{Kind: kind}, true
	}
	if kind != GradientLinear {
		return Gradient{}, false
	}
	a, err := strconv.Atoi(angleStr)
	if err != nil {
		return Gradient{}, false
	}
	return Gradient{Kind: kind, Angle: ((a % 360) + 360) % 360}, true
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
