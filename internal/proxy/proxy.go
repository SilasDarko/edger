// Package proxy wraps httputil.ReverseProxy with per-route retry logic and
// circuit breaker integration.
package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/edger/edger/internal/circuitbreaker"
	"github.com/edger/edger/internal/config"
)

// Handler proxies requests to a single upstream using httputil.ReverseProxy.
// Retries are performed by buffering the upstream response before writing to
// the client, so a failed attempt can be discarded and retried cleanly.
type Handler struct {
	upstream string
	timeout  time.Duration
	retries  int
	cb       *circuitbreaker.CircuitBreaker
	rp       *httputil.ReverseProxy
}

// NewHandler creates a Handler for the given route and circuit breaker.
func NewHandler(route config.Route, cb *circuitbreaker.CircuitBreaker) *Handler {
	upstreamURL, err := url.Parse(route.Upstream)
	if err != nil {
		panic(fmt.Sprintf("proxy: invalid upstream URL %q: %v", route.Upstream, err))
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = upstreamURL.Scheme
			req.URL.Host = upstreamURL.Host
			req.Host = upstreamURL.Host
			// Suppress the default Go user-agent so upstream logs stay clean.
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "edger-gateway/1.0")
			}
		},
		// ErrorHandler fires when the transport returns an error (e.g. connection refused).
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `{"error":"bad gateway","detail":%q}`, err.Error())
		},
	}

	return &Handler{
		upstream: route.Upstream,
		timeout:  time.Duration(route.TimeoutMs) * time.Millisecond,
		retries:  route.Retries,
		cb:       cb,
		rp:       rp,
	}
}

// ServeHTTP forwards the request to the upstream. For safe methods (GET, HEAD)
// it will retry up to route.Retries additional times on 502/503/504 responses
// or connection errors. A *bool pointed to by the RetriedKey context value is
// set to true if any retry was attempted.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	maxAttempts := 1
	if IsSafeMethod(r.Method) && h.retries > 0 {
		maxAttempts = 1 + h.retries
	}

	var lastBuf *responseBuffer
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Mark that at least one retry happened.
			if rf, ok := r.Context().Value(RetriedKey{}).(*bool); ok {
				*rf = true
			}
			// Brief pause before retrying to avoid hammering a struggling upstream.
			time.Sleep(200 * time.Millisecond)
		}

		// Wrap the request with a per-attempt timeout derived from the parent context.
		ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
		buf := newResponseBuffer()
		h.rp.ServeHTTP(buf, r.WithContext(ctx))
		cancel()

		lastBuf = buf

		// Stop retrying if the response looks healthy.
		if !IsRetryableStatus(buf.code) {
			break
		}
	}

	// Update the circuit breaker based on the final response status.
	if lastBuf.code >= 500 {
		h.cb.RecordFailure()
	} else {
		h.cb.RecordSuccess()
	}

	lastBuf.flush(w)
}

// responseBuffer is a minimal http.ResponseWriter that records the response
// in memory so that failed attempts can be discarded before the client sees them.
type responseBuffer struct {
	code    int
	headers http.Header
	body    bytes.Buffer
	written bool
}

func newResponseBuffer() *responseBuffer {
	return &responseBuffer{
		code:    http.StatusOK,
		headers: make(http.Header),
	}
}

func (rb *responseBuffer) Header() http.Header { return rb.headers }

func (rb *responseBuffer) WriteHeader(code int) {
	if !rb.written {
		rb.code = code
		rb.written = true
	}
}

func (rb *responseBuffer) Write(b []byte) (int, error) {
	if !rb.written {
		rb.written = true
	}
	return rb.body.Write(b)
}

// flush writes the buffered response to the real ResponseWriter.
func (rb *responseBuffer) flush(w http.ResponseWriter) {
	for k, vs := range rb.headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rb.code)
	io.Copy(w, &rb.body) //nolint:errcheck
}
