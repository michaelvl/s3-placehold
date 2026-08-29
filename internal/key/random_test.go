package key

import (
	"fmt"
	"image/color"
	"math"
	"testing"
)

func mustParse(t *testing.T, raw string) Params {
	t.Helper()
	p, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", raw, err)
	}
	return p
}

func TestRandomColourIsStable(t *testing.T) {
	// The whole point: the same key always yields the same colour, across
	// calls and across processes.
	first := mustParse(t, "/text=hello/colour=random").BaseColour()
	second := mustParse(t, "/text=hello/colour=random").BaseColour()
	if first != second {
		t.Errorf("same key gave %+v then %+v", first, second)
	}
	if first.A != 0xff {
		t.Errorf("colour = %+v, want opaque", first)
	}
}

func TestRandomColourVariesWithSeed(t *testing.T) {
	seen := map[color.RGBA]string{}
	for _, raw := range []string{
		"/text=alice/colour=random",
		"/text=bob/colour=random",
		"/text=carol/colour=random",
		"/size=200x200/text=alice/colour=random",
		"/colour=random:avatar42",
		"/colour=random:other",
	} {
		c := mustParse(t, raw).BaseColour()
		if prev, dup := seen[c]; dup {
			t.Errorf("%q and %q both gave %+v", prev, raw, c)
		}
		seen[c] = raw
	}
}

func TestRandomColourIgnoresFormatAndDelay(t *testing.T) {
	// format selects the encoding, not the picture, so it must not repaint
	// the image. delay does not affect the picture either.
	want := mustParse(t, "/text=hello/colour=random").BaseColour()
	for _, raw := range []string{
		"/format=png/text=hello/colour=random",
		"/format=jpeg/text=hello/colour=random",
		"/text=hello/delay=500/colour=random",
	} {
		if got := mustParse(t, raw).BaseColour(); got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestRandomColourIgnoresSegmentOrder(t *testing.T) {
	// The seed is built from parsed values, not the raw key, so equivalent
	// keys agree.
	a := mustParse(t, "/size=300x200/text=hi/colour=random").BaseColour()
	b := mustParse(t, "/colour=random/text=hi/size=300x200").BaseColour()
	if a != b {
		t.Errorf("reordered key gave %+v, want %+v", a, b)
	}
}

func TestRandomColourResolvesAgainstLaterSegments(t *testing.T) {
	// `colour` precedes `text` here, so a naive implementation would seed
	// from an empty text.
	withText := mustParse(t, "/colour=random/text=hi").BaseColour()
	withoutText := mustParse(t, "/colour=random").BaseColour()
	if withText == withoutText {
		t.Error("a text segment after colour did not affect the seed")
	}
}

func TestRandomColoursInAListDiffer(t *testing.T) {
	// Without the list position in the seed this would be a degenerate
	// one-colour gradient.
	p := mustParse(t, "/text=hi/colour=random,random,random")
	if len(p.Colours) != 3 {
		t.Fatalf("Colours = %+v, want 3", p.Colours)
	}
	if p.Colours[0] == p.Colours[1] || p.Colours[1] == p.Colours[2] || p.Colours[0] == p.Colours[2] {
		t.Errorf("random list members repeat: %+v", p.Colours)
	}
	// Two colours with no explicit gradient still get the default geometry.
	if p.Gradient.Kind != GradientLinear {
		t.Errorf("Gradient = %+v, want linear", p.Gradient)
	}
}

func TestRandomColourMixesWithLiterals(t *testing.T) {
	p := mustParse(t, "/text=hi/colour=ff0000,random")
	if want := (color.RGBA{R: 0xff, A: 0xff}); p.Colours[0] != want {
		t.Errorf("Colours[0] = %+v, want %+v", p.Colours[0], want)
	}
	if p.Colours[1] == p.Colours[0] {
		t.Errorf("random member matched the literal: %+v", p.Colours)
	}
}

func TestRandomColourExplicitSeedIgnoresTheRestOfTheKey(t *testing.T) {
	// An explicit seed pins the colour, so unrelated segments cannot move it.
	want := mustParse(t, "/colour=random:avatar42").BaseColour()
	for _, raw := range []string{
		"/size=800x600/colour=random:avatar42",
		"/text=anything/colour=random:avatar42",
	} {
		if got := mustParse(t, raw).BaseColour(); got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", raw, got, want)
		}
	}
}

func TestRandomColourRejectsEmptySeed(t *testing.T) {
	// A trailing ':' is a malformed seed, not a seedless random.
	if _, err := Parse("/colour=random:"); err == nil {
		t.Error("Parse(colour=random:) = nil error, want error")
	}
}

func TestRandomColourGradientRequiresTwo(t *testing.T) {
	// A single random is still a single colour.
	if _, err := Parse("/colour=random/gradient=radial"); err == nil {
		t.Error("Parse(one random + gradient) = nil error, want error")
	}
	if _, err := Parse("/colour=random,random/gradient=radial"); err != nil {
		t.Errorf("Parse(two randoms + gradient) returned error: %v", err)
	}
}

func TestDefaultColourRandomIsPerRequest(t *testing.T) {
	// A configured `random` is resolved against each request's key, not once
	// at startup, so every distinct key gets its own stable colour.
	opts := DefaultOptions()
	opts.DefaultColours = []ColourSpec{{Random: true}}

	parse := func(raw string) color.RGBA {
		t.Helper()
		p, err := ParseWithOptions(raw, opts)
		if err != nil {
			t.Fatalf("ParseWithOptions(%q) returned error: %v", raw, err)
		}
		return p.BaseColour()
	}

	alice, bob := parse("/text=alice"), parse("/text=bob")
	if alice == bob {
		t.Errorf("different keys both gave %+v", alice)
	}
	if again := parse("/text=alice"); again != alice {
		t.Errorf("same key gave %+v then %+v", alice, again)
	}
}

func TestDefaultColourRandomMatchesExplicitRandom(t *testing.T) {
	// The default is the same feature as the segment, applied earlier, so a
	// key with no `colour` under DEFAULT_COLOUR=random must render exactly
	// what `colour=random` would have.
	opts := DefaultOptions()
	opts.DefaultColours = []ColourSpec{{Random: true}}

	got, err := ParseWithOptions("/size=300x200/text=hi", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	want := mustParse(t, "/size=300x200/text=hi/colour=random").BaseColour()
	if got.BaseColour() != want {
		t.Errorf("configured random = %+v, want %+v", got.BaseColour(), want)
	}
}

func TestDefaultColourRandomSeededIsPinned(t *testing.T) {
	// An explicit seed pins the colour server-wide: every key renders it.
	opts := DefaultOptions()
	opts.DefaultColours = []ColourSpec{{Random: true, Seed: "brand"}}

	first, err := ParseWithOptions("/text=alice", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	second, err := ParseWithOptions("/size=800x600/text=bob", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if first.BaseColour() != second.BaseColour() {
		t.Errorf("seeded default gave %+v then %+v", first.BaseColour(), second.BaseColour())
	}
}

func TestDefaultColourRandomList(t *testing.T) {
	// DEFAULT_COLOUR=random,random gives every key its own gradient.
	opts := DefaultOptions()
	opts.DefaultColours = []ColourSpec{{Random: true}, {Random: true}}

	got, err := ParseWithOptions("/text=hi", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if len(got.Colours) != 2 || got.Colours[0] == got.Colours[1] {
		t.Errorf("Colours = %+v, want two distinct colours", got.Colours)
	}
	if got.Gradient.Kind != GradientLinear {
		t.Errorf("Gradient = %+v, want linear", got.Gradient)
	}
}

func TestColourSegmentOverridesRandomDefault(t *testing.T) {
	opts := DefaultOptions()
	opts.DefaultColours = []ColourSpec{{Random: true}}

	got, err := ParseWithOptions("/text=hi/colour=ff0000", opts)
	if err != nil {
		t.Fatalf("ParseWithOptions returned error: %v", err)
	}
	if want := (color.RGBA{R: 0xff, A: 0xff}); got.BaseColour() != want {
		t.Errorf("Colour = %+v, want %+v", got.BaseColour(), want)
	}
}

func TestRandomColoursAreVividAndWellSpread(t *testing.T) {
	// Fixed saturation and lightness are the reason for going through HSL:
	// every result should be a clean colour, never near-grey or near-black.
	seen := map[color.RGBA]bool{}
	for i := 0; i < 200; i++ {
		c := randomColourFor("seed", i)
		lo := min(c.R, min(c.G, c.B))
		hi := max(c.R, max(c.G, c.B))
		if hi-lo < 40 {
			t.Errorf("colour %d = %+v is too close to grey", i, c)
		}
		if hi < 0x40 {
			t.Errorf("colour %d = %+v is too dark", i, c)
		}
		seen[c] = true
	}
	// 200 draws from a 360-hue wheel collide by the birthday effect, but a
	// collapsed or banded mapping would show up far below this.
	if len(seen) < 150 {
		t.Errorf("only %d distinct colours in 200 draws, want a well spread wheel", len(seen))
	}
}

func TestHSLToRGB(t *testing.T) {
	cases := []struct {
		h, s, l float64
		want    color.RGBA
	}{
		{0, 1, 0.5, color.RGBA{R: 0xff, A: 0xff}},
		{120, 1, 0.5, color.RGBA{G: 0xff, A: 0xff}},
		{240, 1, 0.5, color.RGBA{B: 0xff, A: 0xff}},
		{0, 0, 0.5, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}},
		{0, 1, 1, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		{0, 1, 0, color.RGBA{A: 0xff}},
	}
	for _, tc := range cases {
		if got := hslToRGB(tc.h, tc.s, tc.l); got != tc.want {
			t.Errorf("hslToRGB(%v,%v,%v) = %+v, want %+v", tc.h, tc.s, tc.l, got, tc.want)
		}
	}
}

func TestRandomColoursCoverTheWheel(t *testing.T) {
	// Guards the hue mapping against collapsing to a handful of values — a
	// smaller modulus, a truncated hash, a constant fed in by mistake. It is
	// a bulk-distribution check, not a guarantee that any two particular
	// seeds land far apart; 240 draws from 360 hues collide by chance.
	// Deterministic, so it can never flake.
	const (
		draws   = 240
		buckets = 12
	)
	counts := make([]int, buckets)
	for i := 0; i < draws; i++ {
		counts[hueOf(randomColourFor(fmt.Sprintf("100x100\nuser%d", i), 0))*buckets/360]++
	}

	expected := draws / buckets
	for i, n := range counts {
		if n < expected/4 || n > expected*3 {
			t.Errorf("hue bucket %d of %d holds %d draws, want near %d: %v", i, buckets, n, expected, counts)
		}
	}
}

// hueOf recovers the hue of a colour, for asserting on spread.
func hueOf(c color.RGBA) int {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	maxc := math.Max(r, math.Max(g, b))
	minc := math.Min(r, math.Min(g, b))
	d := maxc - minc
	if d == 0 {
		return 0
	}
	var h float64
	switch maxc {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return int(math.Round(h))
}
