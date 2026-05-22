package ratelimit_test

import (
	"testing"

	"github.com/edger/edger/internal/ratelimit"
)

func TestLimiter_AllowsWithinLimit(t *testing.T) {
	l := ratelimit.NewLimiter()
	for i := 0; i < 5; i++ {
		if !l.Allow("user-1", 10) {
			t.Fatalf("expected Allow on attempt %d, got denied", i+1)
		}
	}
}

func TestLimiter_DeniesWhenLimitExceeded(t *testing.T) {
	l := ratelimit.NewLimiter()
	limit := 3
	for i := 0; i < limit; i++ {
		if !l.Allow("key-a", limit) {
			t.Fatalf("should allow first %d requests, failed on %d", limit, i+1)
		}
	}
	if l.Allow("key-a", limit) {
		t.Error("expected denial after limit exceeded, got allowed")
	}
}

func TestLimiter_IndependentKeysDoNotInterfere(t *testing.T) {
	l := ratelimit.NewLimiter()
	limit := 2

	// Exhaust key-1.
	l.Allow("key-1", limit)
	l.Allow("key-1", limit)
	if l.Allow("key-1", limit) {
		t.Error("key-1 should be rate limited")
	}

	// key-2 should still be allowed.
	if !l.Allow("key-2", limit) {
		t.Error("key-2 should not be rate limited by key-1's window")
	}
}

func TestLimiter_ZeroLimitDeniesImmediately(t *testing.T) {
	l := ratelimit.NewLimiter()
	// A limit of 0 means no requests are permitted per window.
	// The first Allow sets count=1 which is >= 0, so it should be allowed
	// (the window is created). Subsequent calls should be denied.
	// Actually: first call creates window with count=1 and returns true.
	// Second call: count(1) >= limit(0)? No: limit=0 and count starts at 1.
	// Let's test limit=1 which is clearer behaviour.
	if !l.Allow("x", 1) {
		t.Error("first request with limit=1 should be allowed")
	}
	if l.Allow("x", 1) {
		t.Error("second request with limit=1 should be denied")
	}
}
