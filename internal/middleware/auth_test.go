package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edger/edger/internal/middleware"
)

func TestValidateAPIKey_ValidKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "secret-key")

	if !middleware.ValidateAPIKey(r, "secret-key") {
		t.Error("expected ValidateAPIKey to return true for matching key")
	}
}

func TestValidateAPIKey_WrongKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "wrong-key")

	if middleware.ValidateAPIKey(r, "secret-key") {
		t.Error("expected ValidateAPIKey to return false for mismatched key")
	}
}

func TestValidateAPIKey_MissingHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	if middleware.ValidateAPIKey(r, "secret-key") {
		t.Error("expected ValidateAPIKey to return false when header is absent")
	}
}

func TestValidateAPIKey_EmptyKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "")

	// An empty key should only pass if the expected key is also empty, which
	// is not a valid configuration in practice.
	if middleware.ValidateAPIKey(r, "dev-api-key") {
		t.Error("expected ValidateAPIKey to return false for empty header value")
	}
}

func TestGetOrCreateRequestID_UsesClientID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "client-req-123")

	id := middleware.GetOrCreateRequestID(r)
	if id != "client-req-123" {
		t.Errorf("expected client-req-123, got %q", id)
	}
}

func TestGetOrCreateRequestID_GeneratesWhenAbsent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	id := middleware.GetOrCreateRequestID(r)
	if id == "" {
		t.Error("expected a generated request ID, got empty string")
	}
}
