package key

import (
	"fmt"
	"image/color"
	"testing"
	"time"
)

func TestParseEmptyKeyReturnsDefaults(t *testing.T) {
	cases := []string{"", "/"}
	for _, raw := range cases {
		got, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) returned error: %v", raw, err)
		}
		want := Default()
		if got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestDefaultParams(t *testing.T) {
	p := Default()

	if p.Type != "image" {
		t.Errorf("Type = %q, want %q", p.Type, "image")
	}
	if p.Format != "svg" {
		t.Errorf("Format = %q, want %q", p.Format, "svg")
	}
	if p.Width != 100 || p.Height != 100 {
		t.Errorf("Width/Height = %d/%d, want 100/100", p.Width, p.Height)
	}
	wantColour := color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	if p.Colour != wantColour {
		t.Errorf("Colour = %+v, want %+v", p.Colour, wantColour)
	}
	if p.Text != "" {
		t.Errorf("Text = %q, want empty", p.Text)
	}
	if p.DelayMin != 0 || p.DelayMax != 0 {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 0/0", p.DelayMin, p.DelayMax)
	}
}

func TestParseFormat(t *testing.T) {
	for _, format := range []string{"svg", "png", "jpeg"} {
		got, err := Parse("/format=" + format)
		if err != nil {
			t.Fatalf("Parse(format=%s) returned error: %v", format, err)
		}
		if got.Format != format {
			t.Errorf("Format = %q, want %q", got.Format, format)
		}
	}
}

func TestParseSize(t *testing.T) {
	got, err := Parse("/size=200x300")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Width != 200 || got.Height != 300 {
		t.Errorf("Width/Height = %d/%d, want 200/300", got.Width, got.Height)
	}
}

func TestParseColourHex(t *testing.T) {
	got, err := Parse("/colour=ff0000")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	want := color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	if got.Colour != want {
		t.Errorf("Colour = %+v, want %+v", got.Colour, want)
	}
}

func TestParseColourNamed(t *testing.T) {
	got, err := Parse("/colour=lightblue")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	want := color.RGBA{R: 0xad, G: 0xd8, B: 0xe6, A: 0xff}
	if got.Colour != want {
		t.Errorf("Colour = %+v, want %+v", got.Colour, want)
	}
}

func TestParseText(t *testing.T) {
	got, err := Parse("/text=hello+world")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Text != "hello world" {
		t.Errorf("Text = %q, want %q", got.Text, "hello world")
	}
}

func TestParseDelayFixed(t *testing.T) {
	got, err := Parse("/delay=200")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.DelayMin != 200*time.Millisecond || got.DelayMax != 200*time.Millisecond {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 200ms/200ms", got.DelayMin, got.DelayMax)
	}
}

func TestParseDelayRange(t *testing.T) {
	got, err := Parse("/delay=100,500")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.DelayMin != 100*time.Millisecond || got.DelayMax != 500*time.Millisecond {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 100ms/500ms", got.DelayMin, got.DelayMax)
	}
}

func TestParseMultipleSegmentsAnyOrder(t *testing.T) {
	got, err := Parse("/colour=ff0000/format=png/size=200x300/text=hi")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Format != "png" || got.Width != 200 || got.Height != 300 || got.Text != "hi" {
		t.Errorf("got = %+v", got)
	}
	want := color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}
	if got.Colour != want {
		t.Errorf("Colour = %+v, want %+v", got.Colour, want)
	}
}

func TestParseTypeImage(t *testing.T) {
	got, err := Parse("/type=image")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Type != "image" {
		t.Errorf("Type = %q, want %q", got.Type, "image")
	}
}

func TestParseInvalidSize(t *testing.T) {
	_, err := Parse("/size=abc")
	if err == nil {
		t.Fatalf("Parse(size=abc) = nil error, want error")
	}
}

func TestParseInvalidSizeNonPositive(t *testing.T) {
	for _, v := range []string{"0x100", "100x0", "-1x100"} {
		if _, err := Parse("/size=" + v); err == nil {
			t.Errorf("Parse(size=%s) = nil error, want error", v)
		}
	}
}

func TestParseRejectsSizeOverDefaultMax(t *testing.T) {
	_, err := Parse(fmt.Sprintf("/size=%dx100", DefaultMaxWidth+1))
	if err == nil {
		t.Fatalf("Parse(size over DefaultMaxWidth) = nil error, want error")
	}
}

func TestParseWithLimitsBoundary(t *testing.T) {
	if _, err := ParseWithLimits("/size=500x300", 500, 300); err != nil {
		t.Errorf("ParseWithLimits at exactly the limit returned error: %v", err)
	}
	if _, err := ParseWithLimits("/size=501x300", 500, 300); err == nil {
		t.Errorf("ParseWithLimits(width over max) = nil error, want error")
	}
	if _, err := ParseWithLimits("/size=500x301", 500, 300); err == nil {
		t.Errorf("ParseWithLimits(height over max) = nil error, want error")
	}
}

func TestParseInvalidFormat(t *testing.T) {
	_, err := Parse("/format=gif")
	if err == nil {
		t.Fatalf("Parse(format=gif) = nil error, want error")
	}
}

func TestParseInvalidColour(t *testing.T) {
	_, err := Parse("/colour=notacolour")
	if err == nil {
		t.Fatalf("Parse(colour=notacolour) = nil error, want error")
	}
}

func TestParseInvalidType(t *testing.T) {
	_, err := Parse("/type=pdf")
	if err == nil {
		t.Fatalf("Parse(type=pdf) = nil error, want error")
	}
}

func TestParseRejectsMultiValueForSingleValuedParams(t *testing.T) {
	for _, key := range []string{"/format=png,jpeg", "/type=image,image", "/size=200x300,100x100", "/colour=ff0000,00ff00"} {
		if _, err := Parse(key); err == nil {
			t.Errorf("Parse(%q) = nil error, want error", key)
		}
	}
}

func TestParseColourHexRejectsUppercase(t *testing.T) {
	if _, err := Parse("/colour=FF0000"); err == nil {
		t.Errorf("Parse(colour=FF0000) = nil error, want error")
	}
}

func TestParseInvalidDelay(t *testing.T) {
	for _, v := range []string{"abc", "-5", "500,100"} {
		if _, err := Parse("/delay=" + v); err == nil {
			t.Errorf("Parse(delay=%s) = nil error, want error", v)
		}
	}
}

func TestParseSegmentMissingEquals(t *testing.T) {
	_, err := Parse("/format")
	if err == nil {
		t.Fatalf("Parse(/format) = nil error, want error")
	}
}

func TestParseUnknownSegmentNameIgnored(t *testing.T) {
	got, err := Parse("/foo=bar/format=png")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Format != "png" {
		t.Errorf("Format = %q, want %q", got.Format, "png")
	}
}

func TestParsePercentEncodedCommaInText(t *testing.T) {
	got, err := Parse("/text=a%2Cb")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Text != "a,b" {
		t.Errorf("Text = %q, want %q", got.Text, "a,b")
	}
}

func TestParseWithOptionsDefaultDelayApplied(t *testing.T) {
	opts := Options{MaxWidth: DefaultMaxWidth, MaxHeight: DefaultMaxHeight, DefaultDelayMin: 100 * time.Millisecond, DefaultDelayMax: 500 * time.Millisecond}

	got, err := ParseWithOptions("/format=png", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.DelayMin != 100*time.Millisecond || got.DelayMax != 500*time.Millisecond {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 100ms/500ms", got.DelayMin, got.DelayMax)
	}
}

func TestParseWithOptionsDelaySegmentOverridesDefault(t *testing.T) {
	opts := Options{MaxWidth: DefaultMaxWidth, MaxHeight: DefaultMaxHeight, DefaultDelayMin: 100 * time.Millisecond, DefaultDelayMax: 500 * time.Millisecond}

	got, err := ParseWithOptions("/delay=50", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.DelayMin != 50*time.Millisecond || got.DelayMax != 50*time.Millisecond {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 50ms/50ms", got.DelayMin, got.DelayMax)
	}
}

func TestParseWithOptionsZeroDelaySegmentDisablesDefault(t *testing.T) {
	opts := Options{MaxWidth: DefaultMaxWidth, MaxHeight: DefaultMaxHeight, DefaultDelayMin: 100 * time.Millisecond, DefaultDelayMax: 500 * time.Millisecond}

	got, err := ParseWithOptions("/delay=0", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.DelayMin != 0 || got.DelayMax != 0 {
		t.Errorf("DelayMin/DelayMax = %v/%v, want 0/0", got.DelayMin, got.DelayMax)
	}
}

func TestParseDelayValues(t *testing.T) {
	lo, hi, ok := ParseDelay([]string{"200"})
	if !ok || lo != 200*time.Millisecond || hi != 200*time.Millisecond {
		t.Errorf("ParseDelay([200]) = %v/%v/%v, want 200ms/200ms/true", lo, hi, ok)
	}
	lo, hi, ok = ParseDelay([]string{"100", "500"})
	if !ok || lo != 100*time.Millisecond || hi != 500*time.Millisecond {
		t.Errorf("ParseDelay([100 500]) = %v/%v/%v, want 100ms/500ms/true", lo, hi, ok)
	}
	for _, values := range [][]string{{"abc"}, {"-1"}, {"500", "100"}, {"1", "2", "3"}, {}} {
		if _, _, ok := ParseDelay(values); ok {
			t.Errorf("ParseDelay(%v) = true, want false", values)
		}
	}
}

func TestParseWithOptionsDefaultSizeApplied(t *testing.T) {
	opts := DefaultOptions()
	opts.DefaultWidth, opts.DefaultHeight = 800, 600

	got, err := ParseWithOptions("/format=png", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.Width != 800 || got.Height != 600 {
		t.Errorf("Width/Height = %dx%d, want 800x600", got.Width, got.Height)
	}
}

func TestParseWithOptionsSizeSegmentOverridesDefault(t *testing.T) {
	opts := DefaultOptions()
	opts.DefaultWidth, opts.DefaultHeight = 800, 600

	got, err := ParseWithOptions("/size=200x300", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.Width != 200 || got.Height != 300 {
		t.Errorf("Width/Height = %dx%d, want 200x300", got.Width, got.Height)
	}
}

func TestParseWithOptionsZeroDefaultSizeFallsBack(t *testing.T) {
	got, err := ParseWithOptions("/format=png", Options{MaxWidth: DefaultMaxWidth, MaxHeight: DefaultMaxHeight})
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.Width != DefaultWidth || got.Height != DefaultHeight {
		t.Errorf("Width/Height = %dx%d, want %dx%d", got.Width, got.Height, DefaultWidth, DefaultHeight)
	}
}

func TestParseSizeValues(t *testing.T) {
	w, h, ok := ParseSize("200x300", DefaultMaxWidth, DefaultMaxHeight)
	if !ok || w != 200 || h != 300 {
		t.Errorf("ParseSize(200x300) = %d/%d/%v, want 200/300/true", w, h, ok)
	}
	for _, v := range []string{"200", "200x", "x300", "0x100", "100x0", "-1x100", "axb", "200x300x400"} {
		if _, _, ok := ParseSize(v, DefaultMaxWidth, DefaultMaxHeight); ok {
			t.Errorf("ParseSize(%q) = true, want false", v)
		}
	}
	if _, _, ok := ParseSize("501x300", 500, 300); ok {
		t.Errorf("ParseSize(over max width) = true, want false")
	}
	if _, _, ok := ParseSize("500x301", 500, 300); ok {
		t.Errorf("ParseSize(over max height) = true, want false")
	}
}

func TestParseWithOptionsDefaultColourApplied(t *testing.T) {
	opts := DefaultOptions()
	opts.DefaultColour = color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}

	got, err := ParseWithOptions("/format=png", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.Colour != opts.DefaultColour {
		t.Errorf("Colour = %+v, want %+v", got.Colour, opts.DefaultColour)
	}
}

func TestParseWithOptionsColourSegmentOverridesDefault(t *testing.T) {
	opts := DefaultOptions()
	opts.DefaultColour = color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}

	got, err := ParseWithOptions("/colour=ff0000", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	want := color.RGBA{R: 0xff, A: 0xff}
	if got.Colour != want {
		t.Errorf("Colour = %+v, want %+v", got.Colour, want)
	}
}

func TestParseWithOptionsZeroDefaultColourFallsBack(t *testing.T) {
	got, err := ParseWithOptions("/format=png", Options{MaxWidth: DefaultMaxWidth, MaxHeight: DefaultMaxHeight})
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if got.Colour != DefaultColour() {
		t.Errorf("Colour = %+v, want %+v", got.Colour, DefaultColour())
	}
}

func TestParseColourValues(t *testing.T) {
	c, ok := ParseColour("ff0000")
	if !ok || c != (color.RGBA{R: 0xff, A: 0xff}) {
		t.Errorf("ParseColour(ff0000) = %+v/%v, want red/true", c, ok)
	}
	if _, ok := ParseColour("lightblue"); !ok {
		t.Errorf("ParseColour(lightblue) = false, want true")
	}
	for _, v := range []string{"FF0000", "#ff0000", "ff00", "notacolour", ""} {
		if _, ok := ParseColour(v); ok {
			t.Errorf("ParseColour(%q) = true, want false", v)
		}
	}
}
