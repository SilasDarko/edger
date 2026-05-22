// claims-service is a mock backend that simulates an insurance claims service.
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
		port = "4002"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "claims-service",
		})
	})

	mux.HandleFunc("/claims", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"claim_id": "C-001", "status": "approved", "amount": 1200.00},
			{"claim_id": "C-002", "status": "pending", "amount": 450.00},
		})
	})

	mux.HandleFunc("/claims/status", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"claim_id":    "C-001",
			"status":      "approved",
			"filed_date":  "2026-03-10",
			"resolved_at": "2026-03-22",
			"amount":      1200.00,
		})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("claims-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
