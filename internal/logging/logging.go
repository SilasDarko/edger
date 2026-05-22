// Package logging provides structured JSON request logging for the gateway.
package logging

import (
	"encoding/json"
	"log"
	"time"
)

// Entry holds all fields captured for a single proxied request.
type Entry struct {
	Timestamp   string `json:"timestamp"`
	RequestID   string `json:"request_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Upstream    string `json:"upstream,omitempty"`
	StatusCode  int    `json:"status_code"`
	LatencyMs   int64  `json:"latency_ms"`
	AuthResult  string `json:"auth_result,omitempty"`
	RateLimited bool   `json:"rate_limited,omitempty"`
	Retried     bool   `json:"retried,omitempty"`
	CircuitOpen bool   `json:"circuit_open,omitempty"`
}

// Log writes a single request entry as a JSON line to stderr via the standard logger.
func Log(e Entry) {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("log marshal error: %v", err)
		return
	}
	log.Printf("%s", b)
}
