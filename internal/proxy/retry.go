package proxy

import "net/http"

// RetriedKey is the context key used to pass a *bool retried flag between
// the gateway handler and the RetryTransport. The gateway sets the pointer
// before calling ServeHTTP; the transport flips it to true on a retry.
type RetriedKey struct{}

// IsSafeMethod reports whether the HTTP method is idempotent and therefore
// safe to retry. Only GET and HEAD qualify; mutating methods must not be
// retried automatically.
func IsSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// IsRetryableStatus reports whether an HTTP status code warrants a retry.
// We retry on connection-level errors (502) and temporary upstream failures
// (503, 504). Client errors (4xx) and success responses are not retried.
func IsRetryableStatus(code int) bool {
	return code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable ||
		code == http.StatusGatewayTimeout
}
