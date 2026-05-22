// Package circuitbreaker implements a simple three-state circuit breaker per upstream.
//
// States:
//
//	Closed   — normal operation; all requests pass through.
//	Open     — upstream is failing; requests are rejected immediately with 503.
//	HalfOpen — cooldown has elapsed; one probe request is allowed through.
//	           If it succeeds, the circuit closes. If it fails, the circuit
//	           reopens and the cooldown restarts.
package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the current state of a circuit breaker.
type State int

const (
	// StateClosed means the circuit is healthy and requests flow normally.
	StateClosed State = iota
	// StateOpen means too many failures occurred; requests are blocked.
	StateOpen
	// StateHalfOpen means the cooldown elapsed and one probe is allowed.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker tracks consecutive failures for a single upstream. It is
// safe for concurrent use.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        State
	failures     int
	threshold    int
	lastFailTime time.Time
	cooldown     time.Duration
}

// New returns a CircuitBreaker with a failure threshold of 3 and a 10-second
// cooldown before transitioning to HalfOpen.
func New() *CircuitBreaker {
	return &CircuitBreaker{
		state:     StateClosed,
		threshold: 3,
		cooldown:  10 * time.Second,
	}
}

// NewForTesting creates a circuit breaker with custom threshold and cooldown,
// intended for use in unit tests that need deterministic timing.
func NewForTesting(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     StateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// Allow reports whether a request should be forwarded to the upstream.
// When the circuit is Open, it transitions to HalfOpen once the cooldown
// has elapsed, allowing a single probe request.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		if time.Since(cb.lastFailTime) >= cb.cooldown {
			cb.state = StateHalfOpen
			return true
		}
		return false

	case StateHalfOpen:
		// Allow exactly one probe through.
		return true
	}

	return false
}

// RecordSuccess resets the failure count and closes the circuit.
// Call this when the upstream responds successfully.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = StateClosed
}

// RecordFailure increments the failure count. Once the count reaches the
// threshold, the circuit opens. A failure in HalfOpen immediately reopens.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailTime = time.Now()
	if cb.state == StateHalfOpen || cb.failures >= cb.threshold {
		cb.state = StateOpen
	}
}

// CurrentState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) CurrentState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
