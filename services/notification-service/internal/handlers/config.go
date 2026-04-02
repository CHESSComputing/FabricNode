package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/store"
)

// Config holds the dependencies shared across all notification handlers.
type Config struct {
	Inbox  *store.Inbox
	NodeID string
}

// writeJSON writes a JSON-encoded value with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
