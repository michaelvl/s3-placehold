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
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

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

func renderSVG(params key.Params) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, params.Width, params.Height)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`, colourHex(params.Colour))
	if params.Text != "" {
		fmt.Fprintf(&b, `<text x="50%%" y="50%%" fill="%s" text-anchor="middle" dominant-baseline="middle">%s</text>`,
			colourHex(contrastColour(params.Colour)), xmlEscapeText(params.Text))
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

func renderRaster(params key.Params) *stdimage.RGBA {
	rect := stdimage.Rect(0, 0, params.Width, params.Height)
	img := stdimage.NewRGBA(rect)
	draw.Draw(img, rect, &stdimage.Uniform{C: params.Colour}, stdimage.Point{}, draw.Src)
	if params.Text != "" {
		drawText(img, params)
	}
	return img
}

func drawText(img *stdimage.RGBA, params key.Params) {
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  img,
		Src:  stdimage.NewUniform(contrastColour(params.Colour)),
		Face: face,
	}
	textWidth := d.MeasureString(params.Text).Ceil()
	x := (params.Width - textWidth) / 2
	y := (params.Height + face.Metrics().Ascent.Ceil()) / 2
	d.Dot = fixed.P(x, y)
	d.DrawString(params.Text)
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
