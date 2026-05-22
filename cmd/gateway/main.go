package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/edger/edger/internal/circuitbreaker"
	"github.com/edger/edger/internal/config"
	"github.com/edger/edger/internal/logging"
	"github.com/edger/edger/internal/metrics"
	"github.com/edger/edger/internal/middleware"
	"github.com/edger/edger/internal/proxy"
	"github.com/edger/edger/internal/ratelimit"
)

// gateway wires together all of the reliability middleware and the reverse
// proxy for every configured route.
type gateway struct {
	cfg      *config.Config
	metrics  *metrics.Metrics
	limiter  *ratelimit.Limiter
	circuits map[string]*circuitbreaker.CircuitBreaker
	handlers map[string]*proxy.Handler
	apiKey   string
}

func newGateway(cfg *config.Config, apiKey string) *gateway {
	circuits := make(map[string]*circuitbreaker.CircuitBreaker, len(cfg.Routes))
	handlers := make(map[string]*proxy.Handler, len(cfg.Routes))
	for _, route := range cfg.Routes {
		cb := circuitbreaker.New()
		circuits[route.Upstream] = cb
		handlers[route.Path] = proxy.NewHandler(route, cb)
	}
	return &gateway{
		cfg:      cfg,
		metrics:  metrics.New(),
		limiter:  ratelimit.NewLimiter(),
		circuits: circuits,
		handlers: handlers,
		apiKey:   apiKey,
	}
}

// findRoute returns the route whose path prefix is the longest match for the
// given request path, or nil if no route matches.
func (gw *gateway) findRoute(path string) *config.Route {
	var best *config.Route
	for i := range gw.cfg.Routes {
		r := &gw.cfg.Routes[i]
		if strings.HasPrefix(path, r.Path) {
			if best == nil || len(r.Path) > len(best.Path) {
				best = r
			}
		}
	}
	return best
}

// rateLimitKey returns the key used to identify a caller for rate limiting.
// If the caller provided an API key we rate-limit by key; otherwise by IP.
func rateLimitKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return "key:" + key
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return "ip:" + ip
}

// writeJSON is a small helper that writes a JSON response with the given status.
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// statusRecorder wraps a ResponseWriter so we can read the status code after
// the handler has run.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	return sr.ResponseWriter.Write(b)
}

// handleRoute is the main catch-all handler. It applies auth, rate limiting,
// circuit breaker checks, and then delegates to the upstream proxy.
func (gw *gateway) handleRoute(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	reqID := middleware.GetOrCreateRequestID(r)
	w.Header().Set("X-Request-ID", reqID)

	route := gw.findRoute(r.URL.Path)
	if route == nil {
		gw.metrics.RecordRequest("", 404, time.Since(start).Milliseconds())
		logging.Log(logging.Entry{
			RequestID:  reqID,
			Method:     r.Method,
			Path:       r.URL.Path,
			StatusCode: 404,
			LatencyMs:  time.Since(start).Milliseconds(),
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no route matched"})
		return
	}

	// --- Auth ---
	authResult := "skipped"
	if route.AuthRequired {
		if !middleware.ValidateAPIKey(r, gw.apiKey) {
			authResult = "denied"
			latency := time.Since(start).Milliseconds()
			logging.Log(logging.Entry{
				RequestID:  reqID,
				Method:     r.Method,
				Path:       r.URL.Path,
				Upstream:   route.Upstream,
				StatusCode: http.StatusUnauthorized,
				LatencyMs:  latency,
				AuthResult: authResult,
			})
			gw.metrics.RecordRequest(route.Path, http.StatusUnauthorized, latency)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		authResult = "ok"
	}

	// --- Rate limit ---
	key := rateLimitKey(r)
	if !gw.limiter.Allow(key, route.RateLimitPerMinute) {
		gw.metrics.RecordRateLimited()
		latency := time.Since(start).Milliseconds()
		logging.Log(logging.Entry{
			RequestID:   reqID,
			Method:      r.Method,
			Path:        r.URL.Path,
			Upstream:    route.Upstream,
			StatusCode:  http.StatusTooManyRequests,
			LatencyMs:   latency,
			AuthResult:  authResult,
			RateLimited: true,
		})
		gw.metrics.RecordRequest(route.Path, http.StatusTooManyRequests, latency)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	// --- Circuit breaker ---
	cb := gw.circuits[route.Upstream]
	if !cb.Allow() {
		gw.metrics.RecordCircuitOpen(route.Upstream)
		latency := time.Since(start).Milliseconds()
		logging.Log(logging.Entry{
			RequestID:   reqID,
			Method:      r.Method,
			Path:        r.URL.Path,
			Upstream:    route.Upstream,
			StatusCode:  http.StatusServiceUnavailable,
			LatencyMs:   latency,
			AuthResult:  authResult,
			CircuitOpen: true,
		})
		gw.metrics.RecordRequest(route.Path, http.StatusServiceUnavailable, latency)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable (circuit open)"})
		return
	}

	// --- Proxy ---
	// Inject a retried flag into the context; proxy.Handler sets it to true
	// if any retry was attempted.
	retriedFlag := new(bool)
	ctx := context.WithValue(r.Context(), proxy.RetriedKey{}, retriedFlag)

	rec := newStatusRecorder(w)
	gw.handlers[route.Path].ServeHTTP(rec, r.WithContext(ctx))

	latency := time.Since(start).Milliseconds()

	if *retriedFlag {
		gw.metrics.RecordRetried()
	}
	if rec.status >= 500 {
		gw.metrics.RecordUpstreamFailure(route.Upstream)
	}

	logging.Log(logging.Entry{
		RequestID:  reqID,
		Method:     r.Method,
		Path:       r.URL.Path,
		Upstream:   route.Upstream,
		StatusCode: rec.status,
		LatencyMs:  latency,
		AuthResult: authResult,
		Retried:    *retriedFlag,
	})
	gw.metrics.RecordRequest(route.Path, rec.status, latency)
}

// handleGatewayHealth returns a simple liveness response for the gateway itself.
func (gw *gateway) handleGatewayHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "edger-gateway",
	})
}

// handleUpstreamsHealth checks each configured upstream's /health endpoint
// and returns a summary of which services are reachable.
func (gw *gateway) handleUpstreamsHealth(w http.ResponseWriter, r *http.Request) {
	type result struct {
		Route    string `json:"route"`
		Upstream string `json:"upstream"`
		Status   string `json:"status"`
		Code     int    `json:"status_code,omitempty"`
	}

	client := &http.Client{Timeout: 3 * time.Second}
	results := make([]result, 0, len(gw.cfg.Routes))

	for _, route := range gw.cfg.Routes {
		target := strings.TrimRight(route.Upstream, "/") + "/health"
		resp, err := client.Get(target)
		if err != nil {
			results = append(results, result{
				Route:    route.Path,
				Upstream: route.Upstream,
				Status:   "unreachable",
			})
			continue
		}
		resp.Body.Close()
		s := "healthy"
		if resp.StatusCode >= 400 {
			s = "unhealthy"
		}
		results = append(results, result{
			Route:    route.Path,
			Upstream: route.Upstream,
			Status:   s,
			Code:     resp.StatusCode,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"upstreams": results,
	})
}

// handleMetrics returns a JSON snapshot of in-memory gateway metrics.
func (gw *gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, gw.metrics.Snapshot())
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	configPath := os.Getenv("EDGER_CONFIG_PATH")
	if configPath == "" {
		configPath = "config/routes.yaml"
	}

	apiKey := os.Getenv("EDGER_API_KEY")
	if apiKey == "" {
		apiKey = "dev-api-key"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Printf("loaded %d route(s) from %s", len(cfg.Routes), configPath)

	gw := newGateway(cfg, apiKey)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", gw.handleGatewayHealth)
	mux.HandleFunc("/upstreams/health", gw.handleUpstreamsHealth)
	mux.HandleFunc("/gateway/metrics", gw.handleMetrics)
	mux.HandleFunc("/", gw.handleRoute)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("edger gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
