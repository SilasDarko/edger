// Package metrics tracks in-memory request counters for the gateway.
package metrics

import "sync"

// Metrics holds cumulative counters for the lifetime of the gateway process.
// All methods are safe for concurrent use.
type Metrics struct {
	mu                  sync.Mutex
	totalRequests       int64
	requestsByRoute     map[string]int64
	upstreamFailures    map[string]int64
	rateLimitedRequests int64
	retriedRequests     int64
	circuitOpenEvents   int64
	totalLatencyMs      int64
	latencyCount        int64
}

// New returns an initialised Metrics instance.
func New() *Metrics {
	return &Metrics{
		requestsByRoute:  make(map[string]int64),
		upstreamFailures: make(map[string]int64),
	}
}

// RecordRequest records a completed request with its route path, final HTTP
// status code, and round-trip latency.
func (m *Metrics) RecordRequest(routePath string, statusCode int, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRequests++
	m.requestsByRoute[routePath]++
	m.totalLatencyMs += latencyMs
	m.latencyCount++
}

// RecordUpstreamFailure increments the failure counter for the given upstream URL.
func (m *Metrics) RecordUpstreamFailure(upstream string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upstreamFailures[upstream]++
}

// RecordRateLimited increments the rate-limited request counter.
func (m *Metrics) RecordRateLimited() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimitedRequests++
}

// RecordRetried increments the retried request counter.
func (m *Metrics) RecordRetried() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retriedRequests++
}

// RecordCircuitOpen increments the circuit-open event counter for the given upstream.
func (m *Metrics) RecordCircuitOpen(upstream string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitOpenEvents++
	m.upstreamFailures[upstream]++
}

// Snapshot returns a point-in-time copy of all counters safe for JSON serialisation.
func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	byRoute := make(map[string]int64, len(m.requestsByRoute))
	for k, v := range m.requestsByRoute {
		byRoute[k] = v
	}
	upstreamFails := make(map[string]int64, len(m.upstreamFailures))
	for k, v := range m.upstreamFailures {
		upstreamFails[k] = v
	}

	var avgLatency float64
	if m.latencyCount > 0 {
		avgLatency = float64(m.totalLatencyMs) / float64(m.latencyCount)
	}

	return Snapshot{
		TotalRequests:       m.totalRequests,
		RequestsByRoute:     byRoute,
		UpstreamFailures:    upstreamFails,
		RateLimitedRequests: m.rateLimitedRequests,
		RetriedRequests:     m.retriedRequests,
		CircuitOpenEvents:   m.circuitOpenEvents,
		AverageLatencyMs:    avgLatency,
	}
}

// Snapshot is a read-only point-in-time view of the gateway metrics.
type Snapshot struct {
	TotalRequests       int64            `json:"total_requests"`
	RequestsByRoute     map[string]int64 `json:"requests_by_route"`
	UpstreamFailures    map[string]int64 `json:"upstream_failures"`
	RateLimitedRequests int64            `json:"rate_limited_requests"`
	RetriedRequests     int64            `json:"retried_requests"`
	CircuitOpenEvents   int64            `json:"circuit_open_events"`
	AverageLatencyMs    float64          `json:"average_latency_ms"`
}
