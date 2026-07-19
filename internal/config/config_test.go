package config

import "testing"

func TestLoadZeroConfigDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("BUCKETS", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

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
