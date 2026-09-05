package config

import (
	"image/color"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/michaelvl/s3-placehold/internal/key"
)

func TestLoadZeroConfigDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("BUCKETS", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("MAX_X_PIXELS", "")
	t.Setenv("MAX_Y_PIXELS", "")
	t.Setenv("DEFAULT_DELAY_MS", "")
	t.Setenv("DEFAULT_SIZE", "")
	t.Setenv("DEFAULT_COLOUR", "")
	t.Setenv("DEFAULT_GRADIENT", "")
	t.Setenv("DEFAULT_GUIDES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if len(cfg.Buckets) != 1 {
		t.Fatalf("len(Buckets) = %d, want 1", len(cfg.Buckets))
	}
	if cfg.Buckets[0].Name != "placeholder" {
		t.Errorf("Buckets[0].Name = %q, want %q", cfg.Buckets[0].Name, "placeholder")
	}
	if cfg.Buckets[0].Mode != ModePublic {
		t.Errorf("Buckets[0].Mode = %q, want %q", cfg.Buckets[0].Mode, ModePublic)
	}
	if cfg.MaxWidth != key.DefaultMaxWidth {
		t.Errorf("MaxWidth = %d, want %d", cfg.MaxWidth, key.DefaultMaxWidth)
	}
	if cfg.MaxHeight != key.DefaultMaxHeight {
		t.Errorf("MaxHeight = %d, want %d", cfg.MaxHeight, key.DefaultMaxHeight)
	}
	if cfg.DefaultDelayMin != 0 || cfg.DefaultDelayMax != 0 {
		t.Errorf("DefaultDelayMin/Max = %v/%v, want 0/0", cfg.DefaultDelayMin, cfg.DefaultDelayMax)
	}
	if cfg.DefaultWidth != key.DefaultWidth || cfg.DefaultHeight != key.DefaultHeight {
		t.Errorf("DefaultWidth/Height = %dx%d, want %dx%d", cfg.DefaultWidth, cfg.DefaultHeight, key.DefaultWidth, key.DefaultHeight)
	}
	if !reflect.DeepEqual(cfg.DefaultColours, key.DefaultColourSpecs()) {
		t.Errorf("DefaultColours = %+v, want %+v", cfg.DefaultColours, key.DefaultColourSpecs())
	}
	if cfg.DefaultGradient != (key.Gradient{}) {
		t.Errorf("DefaultGradient = %+v, want zero value", cfg.DefaultGradient)
	}
	if len(cfg.DefaultGuides) != 0 {
		t.Errorf("DefaultGuides = %v, want none", cfg.DefaultGuides)
	}
}

func TestLoadCustomMaxPixels(t *testing.T) {
	t.Setenv("MAX_X_PIXELS", "500")
	t.Setenv("MAX_Y_PIXELS", "300")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.MaxWidth != 500 {
		t.Errorf("MaxWidth = %d, want 500", cfg.MaxWidth)
	}
	if cfg.MaxHeight != 300 {
		t.Errorf("MaxHeight = %d, want 300", cfg.MaxHeight)
	}
}

func TestLoadInvalidMaxPixels(t *testing.T) {
	for _, envVar := range []string{"MAX_X_PIXELS", "MAX_Y_PIXELS"} {
		t.Run(envVar, func(t *testing.T) {
			t.Setenv(envVar, "abc")
			if _, err := Load(); err == nil {
				t.Fatalf("Load with invalid %s = nil error, want error", envVar)
			}
		})
	}
}

func TestLoadCustomPort(t *testing.T) {
	t.Setenv("PORT", "8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestLoadInvalidBucketMode(t *testing.T) {
	t.Setenv("BUCKETS", "images:readonly")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load with invalid bucket mode = nil error, want error")
	}
}

func TestLoadMultipleBuckets(t *testing.T) {
	t.Setenv("BUCKETS", "images:public,assets:private")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []BucketConfig{
		{Name: "images", Mode: ModePublic},
		{Name: "assets", Mode: ModePrivate},
	}
	if len(cfg.Buckets) != len(want) {
		t.Fatalf("len(Buckets) = %d, want %d", len(cfg.Buckets), len(want))
	}
	for i, b := range want {
		if cfg.Buckets[i] != b {
			t.Errorf("Buckets[%d] = %+v, want %+v", i, cfg.Buckets[i], b)
		}
	}
}

func TestLookup(t *testing.T) {
	cfg := Config{Buckets: []BucketConfig{
		{Name: "images", Mode: ModePublic},
		{Name: "assets", Mode: ModePrivate},
	}}

	b, ok := cfg.Lookup("assets")
	if !ok {
		t.Fatalf("Lookup(%q) ok = false, want true", "assets")
	}
	if b.Mode != ModePrivate {
		t.Errorf("Lookup(%q).Mode = %q, want %q", "assets", b.Mode, ModePrivate)
	}

	if _, ok := cfg.Lookup("missing"); ok {
		t.Errorf("Lookup(%q) ok = true, want false", "missing")
	}
}

func TestLoadDefaultDelayFixed(t *testing.T) {
	t.Setenv("DEFAULT_DELAY_MS", "200")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DefaultDelayMin != 200*time.Millisecond || cfg.DefaultDelayMax != 200*time.Millisecond {
		t.Errorf("DefaultDelayMin/Max = %v/%v, want 200ms/200ms", cfg.DefaultDelayMin, cfg.DefaultDelayMax)
	}
}

func TestLoadDefaultDelayRange(t *testing.T) {
	t.Setenv("DEFAULT_DELAY_MS", "100,500")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DefaultDelayMin != 100*time.Millisecond || cfg.DefaultDelayMax != 500*time.Millisecond {
		t.Errorf("DefaultDelayMin/Max = %v/%v, want 100ms/500ms", cfg.DefaultDelayMin, cfg.DefaultDelayMax)
	}
}

func TestLoadInvalidDefaultDelay(t *testing.T) {
	for _, v := range []string{"abc", "-1", "500,100", "1,2,3", "100,"} {
		t.Setenv("DEFAULT_DELAY_MS", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load(DEFAULT_DELAY_MS=%s) = nil error, want error", v)
		}
	}
}

func TestLoadDefaultSize(t *testing.T) {
	t.Setenv("DEFAULT_SIZE", "800x600")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DefaultWidth != 800 || cfg.DefaultHeight != 600 {
		t.Errorf("DefaultWidth/Height = %dx%d, want 800x600", cfg.DefaultWidth, cfg.DefaultHeight)
	}
}

func TestLoadInvalidDefaultSize(t *testing.T) {
	for _, v := range []string{"abc", "800", "800x", "0x100", "-1x100"} {
		t.Setenv("DEFAULT_SIZE", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load(DEFAULT_SIZE=%s) = nil error, want error", v)
		}
	}
}

func TestLoadDefaultSizeOverMaxPixels(t *testing.T) {
	t.Setenv("MAX_X_PIXELS", "500")
	t.Setenv("MAX_Y_PIXELS", "300")
	t.Setenv("DEFAULT_SIZE", "800x600")

	if _, err := Load(); err == nil {
		t.Errorf("Load(DEFAULT_SIZE over MAX_X_PIXELS) = nil error, want error")
	}
}

func TestLoadDefaultColour(t *testing.T) {
	t.Setenv("DEFAULT_COLOUR", "ff0000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := color.RGBA{R: 0xff, A: 0xff}
	if !reflect.DeepEqual(cfg.DefaultColours, []key.ColourSpec{key.FixedColour(want)}) {
		t.Errorf("DefaultColours = %+v, want [%+v]", cfg.DefaultColours, want)
	}
}

func TestLoadDefaultColourNamed(t *testing.T) {
	t.Setenv("DEFAULT_COLOUR", "lightblue")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := color.RGBA{R: 0xad, G: 0xd8, B: 0xe6, A: 0xff} // CSS lightblue
	if !reflect.DeepEqual(cfg.DefaultColours, []key.ColourSpec{key.FixedColour(want)}) {
		t.Errorf("DefaultColours = %+v, want [%+v]", cfg.DefaultColours, want)
	}
}

func TestLoadInvalidDefaultColour(t *testing.T) {
	for _, v := range []string{"FF0000", "#ff0000", "notacolour", "ff00", "random:", "randomm"} {
		t.Setenv("DEFAULT_COLOUR", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load(DEFAULT_COLOUR=%s) = nil error, want error", v)
		}
	}
}

func TestLoadDefaultColourList(t *testing.T) {
	t.Setenv("DEFAULT_COLOUR", "ff0000,lightblue")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []key.ColourSpec{
		key.FixedColour(color.RGBA{R: 0xff, A: 0xff}),
		key.FixedColour(color.RGBA{R: 0xad, G: 0xd8, B: 0xe6, A: 0xff}),
	}
	if !reflect.DeepEqual(cfg.DefaultColours, want) {
		t.Errorf("DefaultColours = %+v, want %+v", cfg.DefaultColours, want)
	}
}

func TestLoadInvalidDefaultColourList(t *testing.T) {
	for _, v := range []string{"ff0000,notacolour", "ff0000,", strings.Repeat("ff0000,", key.MaxColours) + "ff0000"} {
		t.Setenv("DEFAULT_COLOUR", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load with DEFAULT_COLOUR=%q = nil error, want error", v)
		}
	}
}

func TestLoadDefaultColourRandom(t *testing.T) {
	// DEFAULT_COLOUR=random is carried through unresolved: it names a colour
	// per request, so Load has nothing to resolve it against.
	t.Setenv("DEFAULT_COLOUR", "random")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []key.ColourSpec{{Random: true}}
	if !reflect.DeepEqual(cfg.DefaultColours, want) {
		t.Errorf("DefaultColours = %+v, want %+v", cfg.DefaultColours, want)
	}
}

func TestLoadDefaultColourRandomSeededAndMixed(t *testing.T) {
	t.Setenv("DEFAULT_COLOUR", "ff0000,random:brand")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []key.ColourSpec{
		key.FixedColour(color.RGBA{R: 0xff, A: 0xff}),
		{Random: true, Seed: "brand"},
	}
	if !reflect.DeepEqual(cfg.DefaultColours, want) {
		t.Errorf("DefaultColours = %+v, want %+v", cfg.DefaultColours, want)
	}
}

func TestLoadDefaultGradient(t *testing.T) {
	t.Setenv("DEFAULT_GRADIENT", "linear:45")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := key.Gradient{Kind: key.GradientLinear, Angle: 45}
	if cfg.DefaultGradient != want {
		t.Errorf("DefaultGradient = %+v, want %+v", cfg.DefaultGradient, want)
	}
}

func TestLoadInvalidDefaultGradient(t *testing.T) {
	for _, v := range []string{"spiral", "radial:45", "linear:abc"} {
		t.Setenv("DEFAULT_GRADIENT", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load with DEFAULT_GRADIENT=%q = nil error, want error", v)
		}
	}
}

func TestLoadDefaultGuides(t *testing.T) {
	t.Setenv("DEFAULT_GUIDES", "cross,frame")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []key.Guide{key.GuideFrame, key.GuideCross}
	if !slices.Equal(cfg.DefaultGuides, want) {
		t.Errorf("DefaultGuides = %v, want %v", cfg.DefaultGuides, want)
	}
}

func TestLoadInvalidDefaultGuides(t *testing.T) {
	for _, v := range []string{"grid", "cross,grid", "none,cross"} {
		t.Setenv("DEFAULT_GUIDES", v)
		if _, err := Load(); err == nil {
			t.Errorf("Load with DEFAULT_GUIDES=%q = nil error, want error", v)
		}
	}
}
