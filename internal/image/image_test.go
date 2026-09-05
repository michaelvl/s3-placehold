package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
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

func TestSynthesizeSVGWithText(t *testing.T) {
	s := New()
	params := key.Default()
	params.Text = "hello"

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("svg missing text overlay: %s", data)
	}
}

func TestSynthesizeUnsupportedFormat(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "gif"

	_, _, err := s.Synthesize(params)
	if err == nil {
		t.Fatalf("Synthesize with unsupported format = nil error, want error")
	}
}

func TestSynthesizePNG(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "png"
	params.Width = 200
	params.Height = 300
	params.Colours = []color.RGBA{{R: 0xff, A: 0xff}}

	data, mimeType, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if mimeType != "image/png" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/png")
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 300 {
		t.Errorf("dimensions = %dx%d, want 200x300", img.Bounds().Dx(), img.Bounds().Dy())
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if r>>8 != 0xff || g>>8 != 0x00 || b>>8 != 0x00 {
		t.Errorf("corner pixel = (%d,%d,%d), want (255,0,0)", r>>8, g>>8, b>>8)
	}
}

func TestSynthesizeJPEG(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "jpeg"
	params.Width = 400
	params.Height = 200

	data, mimeType, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("mimeType = %q, want %q", mimeType, "image/jpeg")
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode JPEG: %v", err)
	}
	if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 200 {
		t.Errorf("dimensions = %dx%d, want 400x200", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestSynthesizePNGWithTextOverlayDrawsContrastingPixels(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "png"
	params.Width = 200
	params.Height = 100
	params.Colours = []color.RGBA{{R: 0xff, A: 0xff}} // red background -> light text expected
	params.Text = "HELLO"

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}

	if !containsNonBackgroundPixel(img, params.BaseColour()) {
		t.Errorf("expected text overlay to draw at least one non-background pixel")
	}
}

func containsNonBackgroundPixel(img stdimage.Image, bg color.RGBA) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
				return true
			}
		}
	}
	return false
}

func TestFitFontSizeScalesWithWidth(t *testing.T) {
	// Tall enough that the height cap never binds, isolating width scaling.
	narrow := fitFontSize(100, 1000, "HELLO")
	wide := fitFontSize(1000, 1000, "HELLO")
	if !(narrow < wide) {
		t.Errorf("fitFontSize(100,1000) = %v, want < fitFontSize(1000,1000) = %v", narrow, wide)
	}
}

func TestFitFontSizeCappedByHeight(t *testing.T) {
	// Very wide, short image: width is never the binding constraint, so the
	// font size must not exceed heightFillRatio*height.
	size := fitFontSize(100000, 50, "HELLO")
	maxAllowed := 50.0 * heightFillRatio
	if size > maxAllowed {
		t.Errorf("fitFontSize(100000,50) = %v, want <= %v (heightFillRatio*height)", size, maxAllowed)
	}
}

func TestSynthesizePNGTextSpansImageWidth(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "png"
	params.Width = 200
	params.Height = 1000 // tall enough that width, not height, is the binding constraint
	params.Colours = []color.RGBA{{R: 0xff, A: 0xff}}
	params.Text = "HELLO"

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}

	minX, maxX := nonBackgroundColumnRange(img, params.BaseColour())
	if minX == -1 {
		t.Fatalf("no text pixels found")
	}
	span := maxX - minX
	target := float64(params.Width) * widthFillRatio
	if float64(span) < 0.5*target {
		t.Errorf("text span = %d px, want at least half of target width %.0f (text should fill the image width)", span, target)
	}
	if float64(span) > target+5 {
		t.Errorf("text span = %d px, want no more than target width %.0f (plus a small rendering margin)", span, target)
	}
}

func nonBackgroundColumnRange(img stdimage.Image, bg color.RGBA) (minX, maxX int) {
	minX, maxX = -1, -1
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
				if minX == -1 {
					minX = x
				}
				maxX = x
			}
		}
	}
	return minX, maxX
}

func TestSynthesizeSVGTextIncludesFontSize(t *testing.T) {
	s := New()
	params := key.Default()
	params.Text = "hello"

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if !strings.Contains(string(data), `font-size="`) {
		t.Errorf("svg missing font-size attribute: %s", data)
	}
}

func TestContrastColourAutoAdjusts(t *testing.T) {
	darkText := contrastColour(color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	if darkText != (color.RGBA{A: 0xff}) {
		t.Errorf("contrastColour(white) = %+v, want black", darkText)
	}
	lightText := contrastColour(color.RGBA{A: 0xff})
	if lightText != (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Errorf("contrastColour(black) = %+v, want white", lightText)
	}
}

func TestSynthesizeSVGGuides(t *testing.T) {
	s := New()
	params := key.Default()
	params.Width = 400
	params.Height = 300
	params.Guides = key.AllGuides()

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}

	svg := string(data)
	// Four arrowheads, one per end of the cross.
	if got := strings.Count(svg, "<polygon"); got != 4 {
		t.Errorf("polygon count = %d, want 4 (one arrowhead per cross end): %s", got, svg)
	}
	// The guide group is painted in the contrasting colour, like the text.
	if !strings.Contains(svg, `<g fill="#000000">`) {
		t.Errorf("svg missing guide group in the contrasting colour: %s", svg)
	}
	// An arrowhead tip touching each border is what makes a crop visible.
	for _, tip := range []string{`0,150`, `400,150`, `200,0`, `200,300`} {
		if !strings.Contains(svg, tip) {
			t.Errorf("svg missing arrowhead tip at %s: %s", tip, svg)
		}
	}
}

func TestSynthesizeSVGNoGuidesByDefault(t *testing.T) {
	data, _, err := New().Synthesize(key.Default())
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if strings.Contains(string(data), "<polygon") || strings.Contains(string(data), "<g fill=") {
		t.Errorf("svg has a guide overlay without a guides segment: %s", data)
	}
}

func TestSynthesizePNGGuidesDrawInsideTheImage(t *testing.T) {
	s := New()
	params := key.Default()
	params.Format = "png"
	params.Width = 400
	params.Height = 300
	params.Guides = []key.Guide{key.GuideCross, key.GuideFrame}

	data, _, err := s.Synthesize(params)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}

	bg := params.BaseColour()
	guide := contrastColour(bg)
	// Every guide pixel is inside the image, so cropping removes part of the
	// overlay: the frame's own corner, and each of the four arrow tips.
	for _, p := range []stdimage.Point{
		{X: 0, Y: 0},     // frame corner
		{X: 399, Y: 299}, // opposite frame corner
		{X: 0, Y: 150},   // left arrow tip
		{X: 399, Y: 150}, // right arrow tip
		{X: 200, Y: 0},   // top arrow tip
		{X: 200, Y: 299}, // bottom arrow tip
		{X: 200, Y: 150}, // centre, where the two lines meet
	} {
		r, g, b, _ := img.At(p.X, p.Y).RGBA()
		if uint8(r>>8) != guide.R || uint8(g>>8) != guide.G || uint8(b>>8) != guide.B {
			t.Errorf("pixel at %v = (%d,%d,%d), want guide colour (%d,%d,%d)",
				p, r>>8, g>>8, b>>8, guide.R, guide.G, guide.B)
		}
	}
	// A point well away from any guide keeps the background.
	r, g, b, _ := img.At(60, 60).RGBA()
	if uint8(r>>8) != bg.R || uint8(g>>8) != bg.G || uint8(b>>8) != bg.B {
		t.Errorf("pixel away from the guides = (%d,%d,%d), want background", r>>8, g>>8, b>>8)
	}
}

func TestGuideGeometryStaysInBounds(t *testing.T) {
	// Guides are only useful for judging a crop if none of them is already
	// outside the image. Includes sizes small enough for the clamped minimum
	// arm and arrowhead to be a large fraction of the image.
	for _, size := range [][2]int{{1, 1}, {3, 7}, {100, 100}, {400, 300}, {2000, 40}} {
		w, h := size[0], size[1]
		spec := guideGeometry(w, h, key.AllGuides())
		for _, r := range spec.rects {
			if r.x < 0 || r.y < 0 || r.x+r.w > w || r.y+r.h > h {
				t.Errorf("size %dx%d: rect %+v escapes the image", w, h, r)
			}
		}
		for _, tri := range spec.tris {
			for _, p := range tri.pts {
				if p[0] < 0 || p[1] < 0 || p[0] > float64(w) || p[1] > float64(h) {
					t.Errorf("size %dx%d: triangle point %v escapes the image", w, h, p)
				}
			}
		}
	}
}

func TestGuideGeometryEmptyWithoutGuides(t *testing.T) {
	if !guideGeometry(100, 100, nil).empty() {
		t.Errorf("guideGeometry with no guides = non-empty, want empty")
	}
}
