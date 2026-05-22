package middleware

import "net/http"

// ValidateAPIKey checks that the request carries the expected API key in the
// X-API-Key header. It returns true when the key matches, false otherwise.
func ValidateAPIKey(r *http.Request, expectedKey string) bool {
	return r.Header.Get("X-API-Key") == expectedKey
}
