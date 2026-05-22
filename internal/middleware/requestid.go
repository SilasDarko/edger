// Package middleware provides HTTP middleware for the gateway.
package middleware

import (
	"fmt"
	"math/rand"
	"net/http"
)

// GetOrCreateRequestID returns the value of the X-Request-ID header if present,
// otherwise generates a short random hex ID. The returned ID is suitable for
// correlating logs across services.
func GetOrCreateRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return fmt.Sprintf("%08x", rand.Uint32())
}
