package circuitbreaker_test

import (
	"testing"
	"time"

	"github.com/edger/edger/internal/circuitbreaker"
)

func TestCircuitBreaker_InitiallyClosed(t *testing.T) {
	cb := circuitbreaker.New()
	if cb.CurrentState() != circuitbreaker.StateClosed {
		t.Errorf("expected StateClosed initially, got %s", cb.CurrentState())
	}
	if !cb.Allow() {
		t.Error("Allow() should return true when circuit is closed")
	}
}

func TestCircuitBreaker_OpensAfterThresholdFailures(t *testing.T) {
	cb := circuitbreaker.New() // threshold = 3

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.CurrentState() != circuitbreaker.StateClosed {
		t.Errorf("circuit should still be closed after 2 failures")
	}

	cb.RecordFailure() // third failure — should open
	if cb.CurrentState() != circuitbreaker.StateOpen {
		t.Errorf("circuit should be open after 3 failures, got %s", cb.CurrentState())
	}
	if cb.Allow() {
		t.Error("Allow() should return false when circuit is open")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := circuitbreaker.New()

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	if cb.CurrentState() != circuitbreaker.StateClosed {
		t.Errorf("success should reset circuit to closed, got %s", cb.CurrentState())
	}

	// Two more failures should not open the circuit because the count was reset.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.CurrentState() != circuitbreaker.StateClosed {
		t.Errorf("circuit should still be closed after reset+2 failures")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterCooldown(t *testing.T) {
	cb := circuitbreaker.NewForTesting(3, 50*time.Millisecond)

	// Open the circuit.
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.CurrentState() != circuitbreaker.StateOpen {
		t.Fatalf("expected open, got %s", cb.CurrentState())
	}

	// Before cooldown: still blocked.
	if cb.Allow() {
		t.Error("circuit should still be blocked before cooldown")
	}

	// Wait for the cooldown to elapse.
	time.Sleep(60 * time.Millisecond)

	// Allow() should now transition to HalfOpen and return true.
	if !cb.Allow() {
		t.Error("expected Allow()=true after cooldown (half-open)")
	}
	if cb.CurrentState() != circuitbreaker.StateHalfOpen {
		t.Errorf("expected StateHalfOpen after cooldown, got %s", cb.CurrentState())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	cb := circuitbreaker.NewForTesting(3, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow() // triggers HalfOpen transition
	cb.RecordSuccess()

	if cb.CurrentState() != circuitbreaker.StateClosed {
		t.Errorf("expected StateClosed after success in half-open, got %s", cb.CurrentState())
	}
}

func TestCircuitBreaker_ReOpensAfterFailureInHalfOpen(t *testing.T) {
	cb := circuitbreaker.NewForTesting(3, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.Allow() // triggers HalfOpen
	cb.RecordFailure()

	if cb.CurrentState() != circuitbreaker.StateOpen {
		t.Errorf("expected StateOpen after failure in half-open, got %s", cb.CurrentState())
	}
}
