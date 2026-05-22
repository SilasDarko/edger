package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edger/edger/internal/config"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "routes-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
routes:
  - path: /users
    upstream: http://localhost:4001
    auth_required: true
    timeout_ms: 2000
    retries: 2
    rate_limit_per_minute: 60
  - path: /payments
    upstream: http://localhost:4003
    auth_required: false
    timeout_ms: 1000
    retries: 1
    rate_limit_per_minute: 30
`
	path := writeTempConfig(t, yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(cfg.Routes))
	}

	r := cfg.Routes[0]
	if r.Path != "/users" {
		t.Errorf("expected path /users, got %q", r.Path)
	}
	if r.Upstream != "http://localhost:4001" {
		t.Errorf("unexpected upstream: %q", r.Upstream)
	}
	if !r.AuthRequired {
		t.Error("expected auth_required=true")
	}
	if r.TimeoutMs != 2000 {
		t.Errorf("expected timeout_ms=2000, got %d", r.TimeoutMs)
	}
	if r.Retries != 2 {
		t.Errorf("expected retries=2, got %d", r.Retries)
	}
	if r.RateLimitPerMinute != 60 {
		t.Errorf("expected rate_limit=60, got %d", r.RateLimitPerMinute)
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	// timeout_ms and rate_limit_per_minute are omitted — defaults should kick in.
	yaml := `
routes:
  - path: /health
    upstream: http://localhost:9999
`
	path := writeTempConfig(t, yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := cfg.Routes[0]
	if r.TimeoutMs != 5000 {
		t.Errorf("expected default timeout_ms=5000, got %d", r.TimeoutMs)
	}
	if r.RateLimitPerMinute != 60 {
		t.Errorf("expected default rate_limit=60, got %d", r.RateLimitPerMinute)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoad_EmptyRoutes(t *testing.T) {
	path := writeTempConfig(t, "routes: []\n")
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error for empty routes, got nil")
	}
}

func TestLoad_MissingUpstream(t *testing.T) {
	yaml := `
routes:
  - path: /users
`
	path := writeTempConfig(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Error("expected validation error for missing upstream, got nil")
	}
}
