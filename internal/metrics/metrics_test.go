package metrics_test

import (
	"testing"

	"github.com/edger/edger/internal/metrics"
)

func TestMetrics_RecordRequest(t *testing.T) {
	m := metrics.New()
	m.RecordRequest("/users", 200, 45)
	m.RecordRequest("/users", 200, 55)
	m.RecordRequest("/payments", 429, 5)

	snap := m.Snapshot()

	if snap.TotalRequests != 3 {
		t.Errorf("expected TotalRequests=3, got %d", snap.TotalRequests)
	}
	if snap.RequestsByRoute["/users"] != 2 {
		t.Errorf("expected 2 requests for /users, got %d", snap.RequestsByRoute["/users"])
	}
	if snap.RequestsByRoute["/payments"] != 1 {
		t.Errorf("expected 1 request for /payments, got %d", snap.RequestsByRoute["/payments"])
	}
}

func TestMetrics_AverageLatency(t *testing.T) {
	m := metrics.New()
	m.RecordRequest("/users", 200, 100)
	m.RecordRequest("/users", 200, 200)

	snap := m.Snapshot()
	if snap.AverageLatencyMs != 150.0 {
		t.Errorf("expected average latency 150ms, got %v", snap.AverageLatencyMs)
	}
}

func TestMetrics_Counters(t *testing.T) {
	m := metrics.New()
	m.RecordRateLimited()
	m.RecordRateLimited()
	m.RecordRetried()
	m.RecordCircuitOpen("http://localhost:4001")

	snap := m.Snapshot()
	if snap.RateLimitedRequests != 2 {
		t.Errorf("expected RateLimitedRequests=2, got %d", snap.RateLimitedRequests)
	}
	if snap.RetriedRequests != 1 {
		t.Errorf("expected RetriedRequests=1, got %d", snap.RetriedRequests)
	}
	if snap.CircuitOpenEvents != 1 {
		t.Errorf("expected CircuitOpenEvents=1, got %d", snap.CircuitOpenEvents)
	}
	if snap.UpstreamFailures["http://localhost:4001"] != 1 {
		t.Errorf("expected upstream failure count=1")
	}
}

func TestMetrics_SnapshotIsCopy(t *testing.T) {
	m := metrics.New()
	m.RecordRequest("/a", 200, 10)

	snap := m.Snapshot()
	snap.RequestsByRoute["/injected"] = 999

	snap2 := m.Snapshot()
	if _, ok := snap2.RequestsByRoute["/injected"]; ok {
		t.Error("modifying snapshot map should not affect internal state")
	}
}
