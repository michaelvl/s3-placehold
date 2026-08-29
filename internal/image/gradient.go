package image

import (
	"image/color"
	"math"
	"strconv"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// meshFocalPoints are the blob centres for a mesh gradient, as fractions of
// the image width and height. They are a fixed table rather than randomised
// so that a given key always synthesises byte-identical output. Colour i>=1
// takes entry i-1, so the table needs key.MaxColours-1 entries.
var meshFocalPoints = [key.MaxColours - 1][2]float64{
	{0.20, 0.20},
	{0.80, 0.25},
	{0.25, 0.80},
	{0.80, 0.80},
	{0.50, 0.35},
	{0.35, 0.55},
	{0.65, 0.60},
}

// meshBlobRadiusRatio scales a mesh blob's radius against the image diagonal.
// It matches the radial kind's radius, so the two geometries share one knob.
const meshBlobRadiusRatio = 0.5

type stop struct {
	offset float64
	colour color.RGBA
}

type blob struct {
	cx, cy, r float64
	colour    color.RGBA
}

// gradientSpec is the resolved, pixel-space description of a background fill.
// It is computed once by gradientGeometry and consumed twice: renderSVG
// formats it into markup, and renderRaster samples it. Having one source of
// geometry is what makes the SVG and raster outputs agree — we ship no SVG
// renderer, so there is nothing to check emitted markup against at runtime.
//
// All coordinates are in pixels, matching gradientUnits="userSpaceOnUse".
// objectBoundingBox would apply the gradient in a non-uniformly scaled space,
// so a 45-degree angle would not be 45 degrees on a non-square image.
type gradientSpec struct {
	kind   key.GradientKind
	base   color.RGBA // flat fill, and the mesh base layer
	stops  []stop     // linear and radial ramp
	x1, y1 float64    // linear gradient line start
	x2, y2 float64    // linear gradient line end
	cx, cy float64    // radial centre
	r      float64    // radial radius
	blobs  []blob     // mesh layers, composited in order over base
}

// gradientGeometry resolves colours and g into a pixel-space fill for a
// w-by-h image. Fewer than two colours always yields a flat fill, whatever
// geometry was requested.
func gradientGeometry(w, h int, colours []color.RGBA, g key.Gradient) gradientSpec {
	if len(colours) == 0 {
		colours = key.DefaultColours()
	}
	if len(colours) > key.MaxColours {
		// Key and config parsing both cap the list, so this only guards a
		// caller constructing Params directly; a panic here would take down
		// a request handler.
		colours = colours[:key.MaxColours]
	}
	spec := gradientSpec{kind: key.GradientNone, base: colours[0]}
	if len(colours) < 2 {
		return spec
	}

	fw, fh := float64(w), float64(h)
	cx, cy := fw/2, fh/2

	switch g.Kind {
	case key.GradientLinear:
		// CSS linear-gradient convention: 0 degrees points towards the top,
		// increasing clockwise, so 90 degrees is left-to-right. SVG's y axis
		// points down, hence the negated cosine.
		sin, cos := sinCosDeg(g.Angle)
		dx, dy := sin, -cos
		// Half the CSS gradient-line length: the projection of the box onto
		// the gradient direction, so the ramp spans exactly the image.
		l := (math.Abs(dx)*fw + math.Abs(dy)*fh) / 2
		spec.kind = key.GradientLinear
		spec.x1, spec.y1 = cx-dx*l, cy-dy*l
		spec.x2, spec.y2 = cx+dx*l, cy+dy*l
		spec.stops = evenStops(colours)
	case key.GradientRadial:
		spec.kind = key.GradientRadial
		spec.cx, spec.cy = cx, cy
		// Half the diagonal, so all four corners sit at exactly offset 1.
		// Note this is a circle covering the farthest corner, not the ellipse
		// CSS radial-gradient defaults to.
		spec.r = math.Hypot(fw, fh) / 2
		spec.stops = evenStops(colours)
	case key.GradientMesh:
		spec.kind = key.GradientMesh
		radius := math.Hypot(fw, fh) * meshBlobRadiusRatio
		spec.blobs = make([]blob, 0, len(colours)-1)
		for i, c := range colours[1:] {
			spec.blobs = append(spec.blobs, blob{
				cx:     meshFocalPoints[i][0] * fw,
				cy:     meshFocalPoints[i][1] * fh,
				r:      radius,
				colour: c,
			})
		}
	}
	return spec
}

// sinCosDeg returns the sine and cosine of an angle in whole degrees, exact
// at the four cardinal angles. math.Sin(math.Pi/2) is 1, but math.Cos(math.Pi/2)
// is 6.1e-17, and that residue leaks into gradient endpoints as coordinates
// like "50.00000000000001" — harmless to render, but it makes the emitted
// markup noisy and the geometry needlessly inexact.
func sinCosDeg(deg int) (sin, cos float64) {
	switch deg {
	case 0:
		return 0, 1
	case 90:
		return 1, 0
	case 180:
		return 0, -1
	case 270:
		return -1, 0
	}
	rad := float64(deg) * math.Pi / 180
	return math.Sin(rad), math.Cos(rad)
}

// evenStops spreads colours evenly across the ramp, from offset 0 to 1.
func evenStops(colours []color.RGBA) []stop {
	stops := make([]stop, len(colours))
	last := float64(len(colours) - 1)
	for i, c := range colours {
		stops[i] = stop{offset: float64(i) / last, colour: c}
	}
	return stops
}

// rampStepsPerSegment is how finely the ramp lookup table samples each
// colour-to-colour segment. At 256 steps the table's quantisation error stays
// under half a level, so it is invisible in the output while removing the
// per-pixel interpolation from the fill loop.
const rampStepsPerSegment = 256

// sampler is a gradientSpec prepared for the per-pixel fill loop: reciprocals
// and the ramp are computed once instead of per pixel. Every derived value
// comes from the same coordinates the markup declares, so the fast path
// cannot drift from the geometry the SVG describes.
type sampler struct {
	kind key.GradientKind
	base color.RGBA

	x1, y1, dx, dy, invLen2 float64 // linear
	cx, cy, invR            float64 // radial
	ramp                    []color.RGBA

	blobs    []blob // mesh
	blobInvR []float64
}

func (s gradientSpec) sampler() *sampler {
	sm := &sampler{kind: s.kind, base: s.base}
	switch s.kind {
	case key.GradientLinear:
		dx, dy := s.x2-s.x1, s.y2-s.y1
		sm.x1, sm.y1, sm.dx, sm.dy = s.x1, s.y1, dx, dy
		if length2 := dx*dx + dy*dy; length2 != 0 {
			sm.invLen2 = 1 / length2
		}
		sm.ramp = buildRamp(s.stops)
	case key.GradientRadial:
		sm.cx, sm.cy = s.cx, s.cy
		if s.r != 0 {
			sm.invR = 1 / s.r
		}
		sm.ramp = buildRamp(s.stops)
	case key.GradientMesh:
		sm.blobs = s.blobs
		sm.blobInvR = make([]float64, len(s.blobs))
		for i, bl := range s.blobs {
			if bl.r != 0 {
				sm.blobInvR[i] = 1 / bl.r
			}
		}
	}
	return sm
}

// buildRamp pre-evaluates the ramp into a lookup table.
func buildRamp(stops []stop) []color.RGBA {
	n := rampStepsPerSegment*(len(stops)-1) + 1
	ramp := make([]color.RGBA, n)
	for i := range ramp {
		ramp[i] = rampAt(stops, float64(i)/float64(n-1))
	}
	return ramp
}

// at returns the colour of the pixel whose top-left corner is (x,y). The
// gradient is evaluated at the pixel centre, which is where SVG renderers
// sample; using the corner would shift the raster half a pixel against the
// same geometry rendered from the emitted markup.
func (s *sampler) at(x, y int) color.RGBA {
	px, py := float64(x)+0.5, float64(y)+0.5
	switch s.kind {
	case key.GradientLinear:
		// Projection of the pixel onto the gradient line, in ramp units.
		return s.ramp[rampIndex(((px-s.x1)*s.dx+(py-s.y1)*s.dy)*s.invLen2, len(s.ramp))]
	case key.GradientRadial:
		return s.ramp[rampIndex(dist(px-s.cx, py-s.cy)*s.invR, len(s.ramp))]
	case key.GradientMesh:
		return s.meshAt(px, py)
	default:
		return s.base
	}
}

func rampIndex(t float64, n int) int {
	return int(clamp01(t)*float64(n-1) + 0.5)
}

// meshAt composites the blobs over the base colour in order, mirroring the
// stacked full-bleed rects the SVG emits. A blob's alpha falls linearly from
// 1 at its centre to 0 at its radius, which is what a two-stop radial
// gradient fading stop-opacity to 0 paints. The accumulation is in float and
// rounded once; renderers round per layer, but the drift stays under an LSB.
func (s *sampler) meshAt(px, py float64) color.RGBA {
	r, g, b := float64(s.base.R), float64(s.base.G), float64(s.base.B)
	for i, bl := range s.blobs {
		a := 1 - clamp01(dist(px-bl.cx, py-bl.cy)*s.blobInvR[i])
		r += (float64(bl.colour.R) - r) * a
		g += (float64(bl.colour.G) - g) * a
		b += (float64(bl.colour.B) - b) * a
	}
	return color.RGBA{R: round8(r), G: round8(g), B: round8(b), A: 0xff}
}

// rampAt interpolates evenly spaced stops at ramp position t.
func rampAt(stops []stop, t float64) color.RGBA {
	t = clamp01(t)
	segments := float64(len(stops) - 1)
	scaled := t * segments
	// int(scaled) reaches len(stops)-1 at t == 1, one past the last segment.
	i := int(scaled)
	if i > len(stops)-2 {
		i = len(stops) - 2
	}
	return lerpColour(stops[i].colour, stops[i+1].colour, scaled-float64(i))
}

// lerpColour blends a towards b, interpolating the raw 8-bit sRGB components.
// This looks wrong to anyone expecting linear-light blending, and is
// deliberate: SVG's color-interpolation property defaults to sRGB, so
// blending in linear light here would make the raster output disagree with
// the same gradient rendered from the emitted markup.
func lerpColour(a, b color.RGBA, u float64) color.RGBA {
	return color.RGBA{
		R: round8(float64(a.R) + (float64(b.R)-float64(a.R))*u),
		G: round8(float64(a.G) + (float64(b.G)-float64(a.G))*u),
		B: round8(float64(a.B) + (float64(b.B)-float64(a.B))*u),
		A: 0xff,
	}
}

// dist is the Euclidean length of (dx,dy). math.Hypot guards against
// intermediate overflow at a cost of several times the arithmetic; image
// coordinates are bounded by the pixel caps, so the plain form is safe and
// this runs once per pixel per layer.
func dist(dx, dy float64) float64 {
	return math.Sqrt(dx*dx + dy*dy)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round8(v float64) uint8 {
	return uint8(math.Round(clamp01(v/255) * 255))
}

// svgNum formats a coordinate or offset for an SVG attribute at full
// precision. Rounding here would quantise the emitted ramp away from the one
// the sampler evaluates; %g is unusable because its scientific notation
// ("1e+04") is not valid in SVG's attribute number grammar.
func svgNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
