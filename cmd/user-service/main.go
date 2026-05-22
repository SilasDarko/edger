// user-service is a mock backend that simulates a user profile service.
// It is not a real service — it exists to give the gateway something realistic
// to route requests to during local development and testing.
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
		port = "4001"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "user-service",
		})
	})

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": 1, "name": "Alice Chen", "role": "engineer"},
			{"id": 2, "name": "Bob Smith", "role": "designer"},
		})
	})

	mux.HandleFunc("/users/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "simulated failure",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":         1,
			"name":       "Alice Chen",
			"email":      "alice@example.com",
			"role":       "engineer",
			"department": "platform",
		})
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("user-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
