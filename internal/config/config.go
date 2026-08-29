// Package config parses environment variables into server configuration.
package config

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/michaelvl/s3-placehold/internal/key"
)

// BucketMode is the auth mode a bucket is configured with.
type BucketMode string

const (
	ModePublic  BucketMode = "public"
	ModePrivate BucketMode = "private"
)

// BucketConfig is a single configured bucket and its auth mode.
type BucketConfig struct {
	Name string
	Mode BucketMode
}

// Config is the fully parsed server configuration.
type Config struct {
	Port            int
	Buckets         []BucketConfig
	AccessKeyID     string
	SecretAccessKey string
	MaxWidth        int
	MaxHeight       int
	DefaultWidth    int
	DefaultHeight   int
	DefaultColours  []color.RGBA
	DefaultGradient key.Gradient
	DefaultDelayMin time.Duration
	DefaultDelayMax time.Duration
}

const (
	defaultPort    = 9000
	defaultBuckets = "placeholder:public"
)

// Load parses environment variables into a Config. Zero-config default is a
// single "placeholder" bucket in public mode on port 9000.
func Load() (Config, error) {
	cfg := Config{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}

	port := defaultPort
	if raw := os.Getenv("PORT"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PORT %q: %w", raw, err)
		}
		port = p
	}
	cfg.Port = port

	rawBuckets := os.Getenv("BUCKETS")
	if rawBuckets == "" {
		rawBuckets = defaultBuckets
	}
	buckets, err := parseBuckets(rawBuckets)
	if err != nil {
		return Config{}, err
	}
	cfg.Buckets = buckets

	maxWidth, err := parseMaxPixels("MAX_X_PIXELS", key.DefaultMaxWidth)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxWidth = maxWidth

	maxHeight, err := parseMaxPixels("MAX_Y_PIXELS", key.DefaultMaxHeight)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxHeight = maxHeight

	defWidth, defHeight, err := parseDefaultSize(cfg.MaxWidth, cfg.MaxHeight)
	if err != nil {
		return Config{}, err
	}
	cfg.DefaultWidth = defWidth
	cfg.DefaultHeight = defHeight

	defColours, err := parseDefaultColours()
	if err != nil {
		return Config{}, err
	}
	cfg.DefaultColours = defColours

	defGradient, err := parseDefaultGradient()
	if err != nil {
		return Config{}, err
	}
	cfg.DefaultGradient = defGradient

	delayMin, delayMax, err := parseDefaultDelay()
	if err != nil {
		return Config{}, err
	}
	cfg.DefaultDelayMin = delayMin
	cfg.DefaultDelayMax = delayMax

	return cfg, nil
}

// parseDefaultSize parses DEFAULT_SIZE, the image dimensions used for
// requests whose key carries no `size` segment. It accepts the same
// `{width}x{height}` syntax as that segment, capped by maxWidth/maxHeight,
// and falls back to the built-in 100x100 when unset.
func parseDefaultSize(maxWidth, maxHeight int) (width, height int, err error) {
	raw := os.Getenv("DEFAULT_SIZE")
	if raw == "" {
		return key.DefaultWidth, key.DefaultHeight, nil
	}
	w, h, ok := key.ParseSize(raw, maxWidth, maxHeight)
	if !ok {
		return 0, 0, fmt.Errorf("invalid DEFAULT_SIZE %q: must be {width}x{height} in pixels (\"100x100\"), within MAX_X_PIXELS/MAX_Y_PIXELS", raw)
	}
	return w, h, nil
}

// parseDefaultColours parses DEFAULT_COLOUR, the background fill used for
// requests whose key carries no `colour` segment. It accepts the same syntax
// as that segment — a comma-separated list of up to key.MaxColours lowercase
// hex values without '#', or CSS named colours — and falls back to the
// built-in cccccc when unset.
func parseDefaultColours() ([]color.RGBA, error) {
	raw := os.Getenv("DEFAULT_COLOUR")
	if raw == "" {
		return key.DefaultColours(), nil
	}
	cs, ok := key.ParseColours(strings.Split(raw, ","))
	if !ok {
		return nil, fmt.Errorf("invalid DEFAULT_COLOUR %q: must be up to %d comma-separated lowercase hex values without '#' (\"cccccc\") or CSS colour names (\"lightblue\")", raw, key.MaxColours)
	}
	return cs, nil
}

// parseDefaultGradient parses DEFAULT_GRADIENT, the background geometry used
// for requests whose key carries no `gradient` segment. It accepts the same
// syntax as that segment, and yields the zero Gradient when unset, which
// leaves the geometry to be chosen from the number of colours.
func parseDefaultGradient() (key.Gradient, error) {
	raw := os.Getenv("DEFAULT_GRADIENT")
	if raw == "" {
		return key.Gradient{}, nil
	}
	g, ok := key.ParseGradient(raw)
	if !ok {
		return key.Gradient{}, fmt.Errorf("invalid DEFAULT_GRADIENT %q: must be \"linear\" with an optional angle (\"linear:45\"), \"radial\", \"mesh\" or \"none\"", raw)
	}
	return g, nil
}

// parseDefaultDelay parses DEFAULT_DELAY_MS, the delay applied to requests
// whose key carries no `delay` segment. It accepts the same syntax as that
// segment — a fixed millisecond count or a `min,max` range — and yields no
// delay when unset.
func parseDefaultDelay() (lo, hi time.Duration, err error) {
	raw := os.Getenv("DEFAULT_DELAY_MS")
	if raw == "" {
		return 0, 0, nil
	}
	lo, hi, ok := key.ParseDelay(strings.Split(raw, ","))
	if !ok {
		return 0, 0, fmt.Errorf("invalid DEFAULT_DELAY_MS %q: must be milliseconds (\"200\") or a range (\"100,500\")", raw)
	}
	return lo, hi, nil
}

// parseMaxPixels parses an env var holding a positive pixel-dimension cap,
// returning def if the var is unset.
func parseMaxPixels(envVar string, def int) (int, error) {
	raw := os.Getenv(envVar)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid %s %q: must be a positive integer", envVar, raw)
	}
	return v, nil
}

// Lookup returns the BucketConfig for name and whether it is configured.
func (c Config) Lookup(name string) (BucketConfig, bool) {
	for _, b := range c.Buckets {
		if b.Name == name {
			return b, true
		}
	}
	return BucketConfig{}, false
}

func parseBuckets(raw string) ([]BucketConfig, error) {
	entries := strings.Split(raw, ",")
	buckets := make([]BucketConfig, 0, len(entries))
	for _, entry := range entries {
		name, mode, ok := strings.Cut(entry, ":")
		if !ok {
			return nil, fmt.Errorf("invalid BUCKETS entry %q: expected name:mode", entry)
		}
		switch BucketMode(mode) {
		case ModePublic, ModePrivate:
		default:
			return nil, fmt.Errorf("invalid BUCKETS entry %q: mode must be %q or %q", entry, ModePublic, ModePrivate)
		}
		buckets = append(buckets, BucketConfig{Name: name, Mode: BucketMode(mode)})
	}
	return buckets, nil
}
