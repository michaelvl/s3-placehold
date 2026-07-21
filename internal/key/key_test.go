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
