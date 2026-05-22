package proxy_test

import (
	"net/http"
	"testing"

	"github.com/edger/edger/internal/proxy"
)

func TestIsSafeMethod(t *testing.T) {
	cases := []struct {
		method string
		want   bool
	}{
		{http.MethodGet, true},
		{http.MethodHead, true},
		{http.MethodPost, false},
		{http.MethodPut, false},
		{http.MethodPatch, false},
		{http.MethodDelete, false},
		{http.MethodOptions, false},
	}

	for _, tc := range cases {
		got := proxy.IsSafeMethod(tc.method)
		if got != tc.want {
			t.Errorf("IsSafeMethod(%q) = %v, want %v", tc.method, got, tc.want)
		}
	}
}

func TestIsRetryableStatus(t *testing.T) {
	retryable := []int{
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
	for _, code := range retryable {
		if !proxy.IsRetryableStatus(code) {
			t.Errorf("expected status %d to be retryable", code)
		}
	}

	notRetryable := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}
	for _, code := range notRetryable {
		if proxy.IsRetryableStatus(code) {
			t.Errorf("expected status %d to NOT be retryable", code)
		}
	}
}
