package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// InboxList returns all notifications, optionally filtered by type.
// GET /inbox
func InboxList(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		typeFilter := req.URL.Query().Get("type")
		notifications := cfg.Inbox.List(typeFilter)
		w.Header().Set("Content-Type", "application/ld+json")
		w.Header().Set("Link", `<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`)
		json.NewEncoder(w).Encode(map[string]any{
			"@context":     "https://www.w3.org/ns/activitystreams",
			"@type":        "OrderedCollection",
			"totalItems":   len(notifications),
			"orderedItems": notifications,
		})
	}
}

// InboxReceive accepts an incoming LDN notification.
// POST /inbox
func InboxReceive(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ct := req.Header.Get("Content-Type")
		if ct != "application/ld+json" && ct != "application/json" {
			http.Error(w, "Content-Type must be application/ld+json", http.StatusUnsupportedMediaType)
			return
		}

		body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20)) // 1 MB limit
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		n := cfg.Inbox.Add(raw)
		log.Printf("[inbox] received notification id=%s types=%v actor=%s", n.ID, n.Type, n.Actor)

		// LDN spec: 201 Created with Location header
		w.Header().Set("Location", "/inbox/"+n.ID)
		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": n.ID})
	}
}

// InboxGet retrieves a single notification by ID.
// GET /inbox/{id}
func InboxGet(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		n := cfg.Inbox.Get(id)
		if n == nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/ld+json")
		json.NewEncoder(w).Encode(n)
	}
}

// InboxAck marks a notification as acknowledged.
// POST /inbox/{id}/ack
func InboxAck(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		if !cfg.Inbox.Acknowledge(id) {
			http.NotFound(w, req)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": "acknowledged"})
	}
}

// InboxStats returns aggregate inbox statistics.
// GET /inbox/stats
func InboxStats(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, cfg.Inbox.Stats())
	}
}
