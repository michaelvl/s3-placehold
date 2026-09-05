// Package image implements the synth.Synthesizer for type=image requests.
package image

import (
	"bytes"
	"fmt"
	stdimage "image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// textFont is the embedded outline font used for text overlays, parsed once
// at package init. goregular.TTF is a fixed asset, so parsing cannot fail in
// practice.
var textFont = mustParseFont(goregular.TTF)

func mustParseFont(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Sprintf("image: failed to parse embedded font: %v", err))
	}
	return f
}

// Text overlay auto-sizing: the font size is chosen so the text spans
// widthFillRatio of the image width, capped so it never exceeds
// heightFillRatio of the image height (for short text on tall/narrow
// images).
const (
	widthFillRatio  = 0.9
	heightFillRatio = 0.8
	minFontSize     = 1.0
	fitIterations   = 24
)

// newFace returns a font.Face for textFont at the given pixel size.
// opentype.NewFace never errors for a valid *opentype.Font, so the error is
// discarded.
func newFace(size float64) font.Face {
	face, _ := opentype.NewFace(textFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72, // at 72 DPI, Size is in pixels
		Hinting: font.HintingFull,
	})
	return face
}

// fitFontSize returns the largest font size, in pixels, at which text
// measures no wider than widthFillRatio*width, via binary search (glyph
// width is monotonic in font size). The result is capped so the font never
// exceeds heightFillRatio*height.
func fitFontSize(width, height int, text string) float64 {
	maxSize := float64(height) * heightFillRatio
	if maxSize < minFontSize {
		maxSize = minFontSize
	}
	targetWidth := float64(width) * widthFillRatio

	lo, hi := minFontSize, maxSize
	best := minFontSize
	for i := 0; i < fitIterations; i++ {
		mid := (lo + hi) / 2
		d := &font.Drawer{Face: newFace(mid)}
		if float64(d.MeasureString(text).Ceil()) <= targetWidth {
			best = mid
			lo = mid
		} else {
			hi = mid
		}
	}
	return best
}

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
	case "png":
		return encodeRaster(renderRaster(params), png.Encode, "image/png")
	case "jpeg":
		enc := func(w io.Writer, img stdimage.Image) error { return jpeg.Encode(w, img, nil) }
		return encodeRaster(renderRaster(params), enc, "image/jpeg")
	default:
		return nil, "", fmt.Errorf("unsupported format: %q", params.Format)
	}
}

func encodeRaster(img stdimage.Image, enc func(io.Writer, stdimage.Image) error, mimeType string) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := enc(&buf, img); err != nil {
		return nil, "", fmt.Errorf("encode %s: %w", mimeType, err)
	}
	return buf.Bytes(), mimeType, nil
}

// SVG element ids for generated gradients. They are namespaced because an
// SVG inlined into a host document shares its id space, and machine-generated
// because attribute values are emitted unescaped.
const (
	gradientID   = "sph-g"
	meshIDPrefix = "sph-b"
)

func renderSVG(params key.Params) []byte {
	cs := colours(params)
	spec := gradientGeometry(params.Width, params.Height, cs, params.Gradient)
	overlay := contrastColour(averageColour(cs))

	var b strings.Builder
	// The viewBox is what keeps a userSpaceOnUse gradient attached to the box
	// when the image is displayed at a size other than its intrinsic one.
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		params.Width, params.Height, params.Width, params.Height)
	writeBackground(&b, spec)
	writeGuides(&b, guideGeometry(params.Width, params.Height, params.Guides), overlay)
	// Text last, so the centre cross does not run through the label that names
	// the image.
	if params.Text != "" {
		size := fitFontSize(params.Width, params.Height, params.Text)
		fmt.Fprintf(&b, `<text x="50%%" y="50%%" fill="%s" font-size="%.1f" text-anchor="middle" dominant-baseline="middle">%s</text>`,
			colourHex(overlay), size, xmlEscapeText(params.Text))
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

// writeGuides emits the guide overlay as one filled group: rects for the
// bands, polygons for the arrowheads.
func writeGuides(b *strings.Builder, spec guideSpec, c color.RGBA) {
	if spec.empty() {
		return
	}
	fmt.Fprintf(b, `<g fill="%s">`, colourHex(c))
	for _, r := range spec.rects {
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d"/>`, r.x, r.y, r.w, r.h)
	}
	for _, t := range spec.tris {
		fmt.Fprintf(b, `<polygon points="%s,%s %s,%s %s,%s"/>`,
			svgNum(t.pts[0][0]), svgNum(t.pts[0][1]),
			svgNum(t.pts[1][0]), svgNum(t.pts[1][1]),
			svgNum(t.pts[2][0]), svgNum(t.pts[2][1]))
	}
	b.WriteString(`</g>`)
}

// writeBackground emits the fill described by spec: a plain rect for a flat
// colour, or gradient definitions plus the rects that reference them.
func writeBackground(b *strings.Builder, spec gradientSpec) {
	switch spec.kind {
	case key.GradientLinear:
		fmt.Fprintf(b, `<defs><linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%s" y1="%s" x2="%s" y2="%s">`,
			gradientID, svgNum(spec.x1), svgNum(spec.y1), svgNum(spec.x2), svgNum(spec.y2))
		writeStops(b, spec.stops)
		b.WriteString(`</linearGradient></defs>`)
		writeFullBleedRect(b, "url(#"+gradientID+")")
	case key.GradientRadial:
		fmt.Fprintf(b, `<defs><radialGradient id="%s" gradientUnits="userSpaceOnUse" cx="%s" cy="%s" r="%s">`,
			gradientID, svgNum(spec.cx), svgNum(spec.cy), svgNum(spec.r))
		writeStops(b, spec.stops)
		b.WriteString(`</radialGradient></defs>`)
		writeFullBleedRect(b, "url(#"+gradientID+")")
	case key.GradientMesh:
		b.WriteString(`<defs>`)
		for i, bl := range spec.blobs {
			hex := colourHex(bl.colour)
			fmt.Fprintf(b, `<radialGradient id="%s%d" gradientUnits="userSpaceOnUse" cx="%s" cy="%s" r="%s">`,
				meshIDPrefix, i, svgNum(bl.cx), svgNum(bl.cy), svgNum(bl.r))
			// Both stops carry the same stop-color, fading only stop-opacity.
			// SVG leaves it open whether stops interpolate in premultiplied
			// alpha and renderers differ; with identical RGB the two are
			// provably equal, so a blob paints the same everywhere. Fading
			// towards the base colour instead would be renderer-dependent.
			fmt.Fprintf(b, `<stop offset="0" stop-color="%s" stop-opacity="1"/><stop offset="1" stop-color="%s" stop-opacity="0"/></radialGradient>`,
				hex, hex)
		}
		b.WriteString(`</defs>`)
		writeFullBleedRect(b, colourHex(spec.base))
		for i := range spec.blobs {
			writeFullBleedRect(b, fmt.Sprintf("url(#%s%d)", meshIDPrefix, i))
		}
	default:
		writeFullBleedRect(b, colourHex(spec.base))
	}
}

func writeStops(b *strings.Builder, stops []stop) {
	for _, st := range stops {
		fmt.Fprintf(b, `<stop offset="%s" stop-color="%s"/>`, svgNum(st.offset), colourHex(st.colour))
	}
}

func writeFullBleedRect(b *strings.Builder, fill string) {
	fmt.Fprintf(b, `<rect width="100%%" height="100%%" fill="%s"/>`, fill)
}

func renderRaster(params key.Params) *stdimage.RGBA {
	cs := colours(params)
	rect := stdimage.Rect(0, 0, params.Width, params.Height)
	img := stdimage.NewRGBA(rect)

	spec := gradientGeometry(params.Width, params.Height, cs, params.Gradient)
	if spec.kind == key.GradientNone {
		draw.Draw(img, rect, &stdimage.Uniform{C: spec.base}, stdimage.Point{}, draw.Src)
	} else {
		fillGradient(img, spec)
	}

	overlay := contrastColour(averageColour(cs))
	drawGuides(img, guideGeometry(params.Width, params.Height, params.Guides), overlay)
	// Text last, matching renderSVG's paint order.
	if params.Text != "" {
		drawText(img, params, overlay)
	}
	return img
}

// fillGradient samples spec into every pixel, writing the buffer directly to
// avoid a bounds check per pixel.
func fillGradient(img *stdimage.RGBA, spec gradientSpec) {
	sampler := spec.sampler()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := img.Pix[img.PixOffset(bounds.Min.X, y):]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := sampler.at(x, y)
			i := (x - bounds.Min.X) * 4
			row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, c.A
		}
	}
}

func drawText(img *stdimage.RGBA, params key.Params, textColour color.RGBA) {
	face := newFace(fitFontSize(params.Width, params.Height, params.Text))
	d := &font.Drawer{
		Dst:  img,
		Src:  stdimage.NewUniform(textColour),
		Face: face,
	}
	textWidth := d.MeasureString(params.Text).Ceil()
	x := (params.Width - textWidth) / 2
	y := (params.Height + face.Metrics().Ascent.Ceil()) / 2
	d.Dot = fixed.P(x, y)
	d.DrawString(params.Text)
}

// colours returns the background colours of params, substituting the built-in
// default so the renderers never have to handle an empty list.
func colours(params key.Params) []color.RGBA {
	if len(params.Colours) == 0 {
		return key.DefaultColours()
	}
	return params.Colours
}

// averageColour returns the component-wise mean of cs — the stand-in for "the
// background" when picking a contrasting text colour. Averaging the
// components and then taking luminance is exactly equivalent to averaging the
// luminances, because contrastColour's luminance is linear in R, G and B.
// That equivalence would not survive a switch to sRGB-linearised luminance.
func averageColour(cs []color.RGBA) color.RGBA {
	if len(cs) == 0 {
		return key.DefaultColour()
	}
	var r, g, b int
	for _, c := range cs {
		r += int(c.R)
		g += int(c.G)
		b += int(c.B)
	}
	n := len(cs)
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 0xff}
}

// contrastColour returns black or white, whichever contrasts better against
// bg, using the perceptual luminance of bg.
func contrastColour(bg color.RGBA) color.RGBA {
	luminance := (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) / 255
	if luminance > 0.5 {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
}

func colourHex(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

var xmlTextReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func xmlEscapeText(s string) string {
	return xmlTextReplacer.Replace(s)
}
