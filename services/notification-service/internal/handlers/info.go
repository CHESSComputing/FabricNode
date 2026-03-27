package handlers

import (
	"fmt"
	"net/http"
)

// Health returns service liveness and pending notification count.
// GET /health
func Health(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		stats := cfg.Inbox.Stats()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"notification-service","node":%q,"pending":%d}`,
			cfg.NodeID, stats["pending"])
	}
}

// Index returns the service directory with LDN inbox discovery Link header.
// GET /
func Index(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `</inbox>; rel="http://www.w3.org/ns/ldp#inbox"`)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"service": "notification-service",
			"node":    cfg.NodeID,
			"endpoints": map[string]string{
				"inbox":   "GET|POST /inbox",
				"message": "GET /inbox/{id}",
				"ack":     "POST /inbox/{id}/ack",
				"stats":   "/inbox/stats",
				"health":  "/health",
			},
			"specs": []string{
				"https://www.w3.org/TR/ldn/",
				"https://www.w3.org/TR/activitystreams-core/",
			},
		})
	}
}
