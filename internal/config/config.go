// Package config parses environment variables into server configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

	return cfg, nil
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
