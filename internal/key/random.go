package key

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image/color"
	"math"
	"strings"
)

// RandomColourKeyword is the `colour` list member that asks for a stable
// pseudo-random colour, optionally followed by ':' and a seed.
const RandomColourKeyword = "random"

// A random colour varies only in hue, at a fixed saturation and lightness.
// Mapping hash bits straight onto RGB instead would give muddy colours with
// unpredictable luminance; holding S and L fixed yields an evenly spaced,
// consistently vivid wheel.
//
// These constants, the hash below and the seed construction together define
// which colour a given seed produces. Callers depend on that mapping being
// stable, so treat all three as API: changing any of them repaints every
// existing `colour=random` URL.
const (
	randomSaturation = 0.65
	randomLightness  = 0.55
)

// resolveColours turns a parsed colour list into the colours to paint with,
// drawing a stable pseudo-random colour for each `random` member. It runs once
// the rest of the key has been applied, since a member with no seed of its own
// derives one from p.
//
// It always allocates, so the returned slice never aliases specs — which
// matters because specs may be the server-wide configured default, shared
// across concurrent requests.
func resolveColours(specs []ColourSpec, p Params) []color.RGBA {
	cs := make([]color.RGBA, len(specs))
	for i, s := range specs {
		switch {
		case !s.Random:
			cs[i] = s.Colour
		case s.Seed != "":
			cs[i] = randomColourFor(s.Seed, i)
		default:
			cs[i] = randomColourFor(derivedSeed(p), i)
		}
	}
	return cs
}

// derivedSeed is the seed for a `random` with none of its own: the parameters
// that define the picture, less the colours themselves. `format` and `delay`
// are excluded so that the same image requested in another encoding, or with
// a different simulated latency, keeps its colour. Building the seed from
// parsed values rather than the raw key also makes it independent of segment
// order and percent-encoding.
func derivedSeed(p Params) string {
	return fmt.Sprintf("%dx%d\n%s", p.Width, p.Height, p.Text)
}

// randomColourFor maps a seed and a list position to a colour. The position
// is mixed in so that `colour=random,random` yields two different hues rather
// than a degenerate one-colour gradient.
//
// SHA-256 is used because it is deterministic across processes and releases
// and avalanches well on short, similar inputs — the common case here, where
// seeds are names like "alice" and "bob". A cheaper non-cryptographic hash
// such as FNV-1a mixes its low-order bits too weakly for that, and taking a
// hue modulo those bits maps similar names onto near-identical colours.
// hash/maphash would be worse still: it is seeded randomly per process, so
// colours would change on every restart.
func randomColourFor(seed string, index int) color.RGBA {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", seed, index)))
	hue := binary.BigEndian.Uint64(sum[:8]) % 360
	return hslToRGB(float64(hue), randomSaturation, randomLightness)
}

// parseRandomColour reports whether v is a `random` list member, returning
// its explicit seed if it carries one. A trailing ':' with nothing after it
// is a malformed seed rather than a seedless `random`.
func parseRandomColour(v string) (seed string, isRandom, ok bool) {
	name, rest, hasSeed := strings.Cut(v, ":")
	if name != RandomColourKeyword {
		return "", false, false
	}
	if hasSeed && rest == "" {
		return "", true, false
	}
	return rest, true, true
}

// hslToRGB converts an HSL colour, with hue in degrees and saturation and
// lightness in [0,1], to 8-bit RGB.
func hslToRGB(hue, saturation, lightness float64) color.RGBA {
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	sector := hue / 60
	second := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))

	var r, g, b float64
	switch {
	case sector < 1:
		r, g, b = chroma, second, 0
	case sector < 2:
		r, g, b = second, chroma, 0
	case sector < 3:
		r, g, b = 0, chroma, second
	case sector < 4:
		r, g, b = 0, second, chroma
	case sector < 5:
		r, g, b = second, 0, chroma
	default:
		r, g, b = chroma, 0, second
	}

	base := lightness - chroma/2
	to8 := func(v float64) uint8 { return uint8(math.Round((v + base) * 255)) }
	return color.RGBA{R: to8(r), G: to8(g), B: to8(b), A: 0xff}
}
