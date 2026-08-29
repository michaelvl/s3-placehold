package image

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/png"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/michaelvl/s3-placehold/internal/key"
)

var (
	red   = color.RGBA{R: 0xff, A: 0xff}
	green = color.RGBA{G: 0xff, A: 0xff}
	blue  = color.RGBA{B: 0xff, A: 0xff}
)

// gradientParams builds a 200x100 image with the given geometry and colours.
func gradientParams(g key.Gradient, colours ...color.RGBA) key.Params {
	p := key.Default()
	p.Width, p.Height = 200, 100
	p.Colours = colours
	p.Gradient = g
	return p
}

func linear(angle int) key.Gradient {
	return key.Gradient{Kind: key.GradientLinear, Angle: angle}
}

// --- SVG structure ---

func TestSVGSingleColourEmitsNoGradient(t *testing.T) {
	// The flat path must stay exactly as it was: a gradient with fewer than
	// two colours is a flat fill, whatever geometry was asked for.
	for _, g := range []key.Gradient{
		{Kind: key.GradientNone},
		linear(45),
		{Kind: key.GradientRadial},
		{Kind: key.GradientMesh},
	} {
		svg := string(renderSVG(gradientParams(g, red)))
		if strings.Contains(svg, "<defs") || strings.Contains(svg, "url(#") {
			t.Errorf("renderSVG(%v, one colour) emitted a gradient: %s", g, svg)
		}
		if !strings.Contains(svg, `fill="#ff0000"`) {
			t.Errorf("renderSVG(%v, one colour) = %s, want a flat #ff0000 fill", g, svg)
		}
	}
}

func TestSVGLinearGradientEndpointsForAngle(t *testing.T) {
	// Exact strings, which also pin the full-precision number formatting: a
	// rounded coordinate would quantise the emitted ramp away from the one
	// the raster sampler evaluates.
	cases := []struct {
		angle int
		want  string
	}{
		{90, `x1="0" y1="50" x2="200" y2="50"`},    // left to right
		{0, `x1="100" y1="100" x2="100" y2="0"`},   // bottom to top
		{180, `x1="100" y1="0" x2="100" y2="100"`}, // top to bottom
		{270, `x1="200" y1="50" x2="0" y2="50"`},   // right to left
	}
	for _, tc := range cases {
		svg := string(renderSVG(gradientParams(linear(tc.angle), red, blue)))
		if !strings.Contains(svg, tc.want) {
			t.Errorf("renderSVG(linear:%d) = %s, want %s", tc.angle, svg, tc.want)
		}
	}
}

func TestSVGLinearGradientStopOffsets(t *testing.T) {
	svg := string(renderSVG(gradientParams(linear(90), red, green, blue)))
	for _, want := range []string{`offset="0"`, `offset="0.5"`, `offset="1"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("renderSVG(3 colours) = %s, want %s", svg, want)
		}
	}

	// 1/6 must survive at full precision, not be rounded to 0.17.
	seven := []color.RGBA{red, green, blue, red, green, blue, red}
	svg = string(renderSVG(gradientParams(linear(90), seven...)))
	if !strings.Contains(svg, `offset="0.16666666666666666"`) {
		t.Errorf("renderSVG(7 colours) = %s, want a full-precision 1/6 offset", svg)
	}
}

func TestSVGRadialGradientCoversCorners(t *testing.T) {
	svg := string(renderSVG(gradientParams(key.Gradient{Kind: key.GradientRadial}, red, blue)))
	// Half the 200x100 diagonal, so all four corners sit at exactly offset 1.
	want := `cx="100" cy="50" r="111.80339887498948"`
	if !strings.Contains(svg, want) {
		t.Errorf("renderSVG(radial) = %s, want %s", svg, want)
	}
}

func TestSVGMeshBlobCountAndFade(t *testing.T) {
	colours := []color.RGBA{red, green, blue}
	svg := string(renderSVG(gradientParams(key.Gradient{Kind: key.GradientMesh}, colours...)))

	if got, want := strings.Count(svg, "<radialGradient"), len(colours)-1; got != want {
		t.Errorf("blob count = %d, want %d: %s", got, want, svg)
	}
	if !strings.Contains(svg, `stop-opacity="0"`) {
		t.Errorf("renderSVG(mesh) = %s, want a stop fading to zero opacity", svg)
	}

	// Both stops of a blob must carry the same stop-color. SVG leaves it open
	// whether stops interpolate in premultiplied alpha and renderers differ;
	// identical RGB makes the two provably equal, so this invariant is what
	// keeps mesh output renderer-independent.
	blobRe := regexp.MustCompile(`<radialGradient[^>]*>(.*?)</radialGradient>`)
	stopColourRe := regexp.MustCompile(`stop-color="(#[0-9a-f]{6})"`)
	blobs := blobRe.FindAllStringSubmatch(svg, -1)
	if len(blobs) == 0 {
		t.Fatalf("no blobs found in %s", svg)
	}
	for _, b := range blobs {
		cols := stopColourRe.FindAllStringSubmatch(b[1], -1)
		if len(cols) != 2 {
			t.Fatalf("blob has %d stops, want 2: %s", len(cols), b[1])
		}
		if cols[0][1] != cols[1][1] {
			t.Errorf("blob stops differ in colour (%s vs %s); only stop-opacity may vary", cols[0][1], cols[1][1])
		}
	}
}

func TestSVGGradientIDsResolve(t *testing.T) {
	for _, g := range []key.Gradient{linear(45), {Kind: key.GradientRadial}, {Kind: key.GradientMesh}} {
		svg := string(renderSVG(gradientParams(g, red, green, blue)))
		refs := regexp.MustCompile(`url\(#([^)]+)\)`).FindAllStringSubmatch(svg, -1)
		if len(refs) == 0 {
			t.Fatalf("renderSVG(%v) referenced no gradient: %s", g, svg)
		}
		for _, ref := range refs {
			if !strings.Contains(svg, `id="`+ref[1]+`"`) {
				t.Errorf("renderSVG(%v) references #%s with no matching id: %s", g, ref[1], svg)
			}
		}
	}
}

func TestSVGHasViewBox(t *testing.T) {
	// Without a viewBox, the userSpaceOnUse gradient coordinates detach from
	// the box when the image is displayed at a size other than its intrinsic one.
	svg := string(renderSVG(gradientParams(linear(45), red, blue)))
	if !strings.Contains(svg, `viewBox="0 0 200 100"`) {
		t.Errorf("renderSVG = %s, want a viewBox", svg)
	}
}

func TestSVGTextContrastUsesAverageColour(t *testing.T) {
	p := gradientParams(linear(90), color.RGBA{A: 0xff}, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	p.Text = "hi"
	svg := string(renderSVG(p))

	// Black and white average to mid grey, whose luminance sits just under
	// the threshold, so the text contrasts white.
	if !strings.Contains(svg, `fill="#ffffff"`) {
		t.Errorf("renderSVG(black+white) = %s, want white text", svg)
	}
}

// --- Raster geometry ---

// closeTo reports whether two colours match within a small per-channel
// tolerance, absorbing rounding differences.
func closeTo(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

func TestRasterLinearEndpointsAndMidpoint(t *testing.T) {
	img := renderRaster(gradientParams(linear(90), red, blue))
	cases := []struct {
		x, y int
		want color.RGBA
	}{
		{0, 50, color.RGBA{R: 0xfe, B: 0x01, A: 0xff}},   // half a pixel in from the start
		{199, 50, color.RGBA{R: 0x01, B: 0xfe, A: 0xff}}, // half a pixel in from the end
		{100, 50, color.RGBA{R: 0x7f, B: 0x80, A: 0xff}}, // midpoint
	}
	for _, tc := range cases {
		if got := img.RGBAAt(tc.x, tc.y); !closeTo(got, tc.want, 2) {
			t.Errorf("pixel (%d,%d) = %+v, want ~%+v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestRasterLinearMonotonicAlongAxis(t *testing.T) {
	// Endpoint probes alone would pass with a flipped sign somewhere in the
	// middle; scanning the whole row catches it.
	img := renderRaster(gradientParams(linear(90), red, blue))
	prev := img.RGBAAt(0, 50)
	for x := 1; x < 200; x++ {
		cur := img.RGBAAt(x, 50)
		if cur.R > prev.R || cur.B < prev.B {
			t.Fatalf("row not monotonic at x=%d: %+v then %+v", x, prev, cur)
		}
		prev = cur
	}
	if !closeTo(prev, blue, 2) {
		t.Errorf("row ends at %+v, want ~%+v", prev, blue)
	}
}

func TestRasterLinearAngleZeroIsVertical(t *testing.T) {
	// 0 degrees points towards the top, so the first colour is at the bottom.
	img := renderRaster(gradientParams(linear(0), red, blue))
	if got := img.RGBAAt(0, 99); !closeTo(got, red, 3) {
		t.Errorf("bottom pixel = %+v, want ~%+v", got, red)
	}
	if got := img.RGBAAt(0, 0); !closeTo(got, blue, 3) {
		t.Errorf("top pixel = %+v, want ~%+v", got, blue)
	}
	for x := 0; x < 200; x++ {
		if got, want := img.RGBAAt(x, 40), img.RGBAAt(0, 40); got != want {
			t.Fatalf("row 40 varies across x at %d: %+v vs %+v", x, got, want)
		}
	}
}

func TestRasterLinearAngle180ReversesAngle0(t *testing.T) {
	up := renderRaster(gradientParams(linear(0), red, blue))
	down := renderRaster(gradientParams(linear(180), red, blue))
	for y := 0; y < 100; y++ {
		if got, want := down.RGBAAt(100, y), up.RGBAAt(100, 99-y); got != want {
			t.Fatalf("linear:180 at y=%d = %+v, want linear:0 mirrored %+v", y, got, want)
		}
	}
}

func TestRasterRadialCentreAndAllFourCorners(t *testing.T) {
	img := renderRaster(gradientParams(key.Gradient{Kind: key.GradientRadial}, red, blue))
	if got := img.RGBAAt(100, 50); !closeTo(got, red, 3) {
		t.Errorf("centre = %+v, want ~%+v", got, red)
	}
	// The radius is half the diagonal, so every corner sits at offset 1.
	for _, c := range [][2]int{{0, 0}, {199, 0}, {0, 99}, {199, 99}} {
		if got := img.RGBAAt(c[0], c[1]); !closeTo(got, blue, 3) {
			t.Errorf("corner (%d,%d) = %+v, want ~%+v", c[0], c[1], got, blue)
		}
	}
}

func TestRasterMeshCompositesInOrder(t *testing.T) {
	img := renderRaster(gradientParams(key.Gradient{Kind: key.GradientMesh}, red, green, blue))

	// Blob 1 (blue) is painted over blob 0 (green), so its focal point reads
	// bluer than green.
	at := img.RGBAAt(int(meshFocalPoints[1][0]*200), int(meshFocalPoints[1][1]*100))
	if at.B <= at.G {
		t.Errorf("second blob's focal point = %+v, want blue to dominate green", at)
	}
	// The base colour survives farthest from every focal point.
	corner := img.RGBAAt(0, 99)
	if corner.R <= corner.G || corner.R <= corner.B {
		t.Errorf("far corner = %+v, want the red base to dominate", corner)
	}
}

func TestRasterMeshIsDeterministic(t *testing.T) {
	// Blob placement is a fixed table, not randomised, so a key always
	// synthesises the same bytes.
	p := gradientParams(key.Gradient{Kind: key.GradientMesh}, red, green, blue)
	if !bytes.Equal(renderRaster(p).Pix, renderRaster(p).Pix) {
		t.Error("mesh rendering is not deterministic")
	}
}

func TestRasterSingleColourStillUniform(t *testing.T) {
	img := renderRaster(gradientParams(linear(45), red))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			if got := img.RGBAAt(x, y); got != red {
				t.Fatalf("pixel (%d,%d) = %+v, want a uniform %+v", x, y, got, red)
			}
		}
	}
}

func TestPNGRoundTripPreservesGradient(t *testing.T) {
	p := gradientParams(linear(90), red, blue)
	p.Format = "png"
	data, _, err := New().Synthesize(p)
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	for _, tc := range []struct {
		x    int
		want color.RGBA
	}{{0, red}, {199, blue}} {
		r, g, b, _ := img.At(tc.x, 50).RGBA()
		got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
		if !closeTo(got, tc.want, 2) {
			t.Errorf("decoded pixel (%d,50) = %+v, want ~%+v", tc.x, got, tc.want)
		}
	}
}

// --- SVG/raster agreement ---

// TestRasterMatchesSVGGeometry checks the raster output against the geometry
// the SVG itself declares, rather than against our internal gradientSpec. We
// ship no SVG renderer, so this is the closest available approximation of
// parity: it catches sign flips, swapped axes, half-pixel sampling offsets
// and precision lost in number formatting. It cannot catch a misunderstanding
// of ramp interpolation shared by both paths — the endpoint and midpoint
// probes above cover that.
func TestRasterMatchesSVGGeometry(t *testing.T) {
	params := gradientParams(linear(30), red, green, blue)
	svg := string(renderSVG(params))

	m := regexp.MustCompile(`x1="([-0-9.e]+)" y1="([-0-9.e]+)" x2="([-0-9.e]+)" y2="([-0-9.e]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no linear gradient endpoints in %s", svg)
	}
	num := func(s string) float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("unparseable SVG coordinate %q: %v", s, err)
		}
		return v
	}
	x1, y1, x2, y2 := num(m[1]), num(m[2]), num(m[3]), num(m[4])

	var stops []color.RGBA
	for _, sm := range regexp.MustCompile(`stop-color="#([0-9a-f]{6})"`).FindAllStringSubmatch(svg, -1) {
		v, err := strconv.ParseUint(sm[1], 16, 32)
		if err != nil {
			t.Fatalf("unparseable SVG stop colour %q: %v", sm[1], err)
		}
		stops = append(stops, color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff})
	}
	if len(stops) != 3 {
		t.Fatalf("got %d stops, want 3: %s", len(stops), svg)
	}

	img := renderRaster(params)
	for _, pt := range []stdimage.Point{
		{X: 0, Y: 0}, {X: 199, Y: 0}, {X: 0, Y: 99}, {X: 199, Y: 99},
		{X: 100, Y: 50}, {X: 50, Y: 25}, {X: 150, Y: 75}, {X: 10, Y: 90},
		{X: 190, Y: 10}, {X: 100, Y: 0},
	} {
		want := sampleDeclaredRamp(x1, y1, x2, y2, stops, float64(pt.X)+0.5, float64(pt.Y)+0.5)
		if got := img.RGBAAt(pt.X, pt.Y); !closeTo(got, want, 2) {
			t.Errorf("pixel (%d,%d) = %+v, want ~%+v from the SVG's own geometry", pt.X, pt.Y, got, want)
		}
	}
}

// sampleDeclaredRamp evaluates an evenly spaced sRGB ramp along the gradient
// line the SVG declares. Deliberately written out longhand rather than
// calling the production sampler, so the two cannot agree by construction.
func sampleDeclaredRamp(x1, y1, x2, y2 float64, stops []color.RGBA, px, py float64) color.RGBA {
	dx, dy := x2-x1, y2-y1
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	t = math.Min(1, math.Max(0, t))

	scaled := t * float64(len(stops)-1)
	i := int(scaled)
	if i > len(stops)-2 {
		i = len(stops) - 2
	}
	u := scaled - float64(i)
	a, b := stops[i], stops[i+1]
	mix := func(p, q uint8) uint8 {
		return uint8(math.Round(float64(p) + (float64(q)-float64(p))*u))
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xff}
}

// BenchmarkRenderRasterMesh guards the one code path whose cost is
// superlinear in a user-controlled parameter: mesh work is O(width x height x
// colours).
func BenchmarkRenderRasterMesh(b *testing.B) {
	colours := make([]color.RGBA, key.MaxColours)
	for i := range colours {
		colours[i] = color.RGBA{R: uint8(i * 30), G: uint8(255 - i*30), B: uint8(i * 15), A: 0xff}
	}
	p := key.Default()
	p.Width, p.Height = 2000, 2000
	p.Colours = colours
	p.Gradient = key.Gradient{Kind: key.GradientMesh}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderRaster(p)
	}
}

func TestGradientGeometryClampsOversizedColourList(t *testing.T) {
	// Parsing caps the list, but the renderer must not panic if a caller
	// builds Params directly.
	colours := make([]color.RGBA, key.MaxColours+4)
	for i := range colours {
		colours[i] = color.RGBA{R: uint8(i * 10), A: 0xff}
	}
	p := gradientParams(key.Gradient{Kind: key.GradientMesh}, colours...)
	if got := len(gradientGeometry(p.Width, p.Height, p.Colours, p.Gradient).blobs); got != key.MaxColours-1 {
		t.Errorf("blobs = %d, want %d", got, key.MaxColours-1)
	}
	renderRaster(p) // must not panic
}
