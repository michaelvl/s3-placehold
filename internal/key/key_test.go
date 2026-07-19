package key

import (
	"image/color"
	"testing"
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
