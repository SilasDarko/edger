// payments-service is a mock backend that simulates a payment history service.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4003"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "payments-service",
		})
	})

	mux.HandleFunc("/payments", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "P-001", "amount": 99.99, "status": "completed", "date": "2026-05-01"},
			{"id": "P-002", "amount": 49.99, "status": "completed", "date": "2026-05-10"},
		})
	})

	mux.HandleFunc("/payments/history", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "P-001", "amount": 99.99, "status": "completed", "date": "2026-05-01"},
			{"id": "P-002", "amount": 49.99, "status": "completed", "date": "2026-05-10"},
			{"id": "P-003", "amount": 199.00, "status": "refunded", "date": "2026-04-28"},
		})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("payments-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
