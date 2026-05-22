// Package ratelimit implements a simple in-memory fixed-window rate limiter.
//
// Each unique key (API key or client IP) gets its own counter that resets
// once per minute. This is intentionally simple: no Redis, no sliding window.
// Swapping in a distributed backend would require only this package to change.
package ratelimit

import (
	"sync"
	"time"
)

// window tracks the request count within a single one-minute bucket.
type window struct {
	count   int
	resetAt time.Time
}

// Limiter holds per-key rate limit windows. It is safe for concurrent use.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
}

// NewLimiter returns an initialised Limiter.
func NewLimiter() *Limiter {
	return &Limiter{
		windows: make(map[string]*window),
	}
}

// Allow returns true if the key is within its allowance for the current minute
// window and increments its counter. Returns false when the limit is exceeded.
func (l *Limiter) Allow(key string, limitPerMinute int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.windows[key]
	if !ok || now.After(w.resetAt) {
		l.windows[key] = &window{
			count:   1,
			resetAt: now.Add(time.Minute),
		}
		return true
	}

	if w.count >= limitPerMinute {
		return false
	}
	w.count++
	return true
}
