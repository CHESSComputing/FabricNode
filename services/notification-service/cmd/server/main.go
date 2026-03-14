package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/store"
)

func main() {
	inbox := store.New()
	nodeID := getEnv("NODE_ID", "chess-node")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// ── W3C LDN inbox ────────────────────────────────────────────────────────
	//
	// GET  /inbox            — list notifications (application/ld+json)
	// POST /inbox            — receive a notification (JSON-LD body)
	// GET  /inbox/{id}       — retrieve a specific notification
	// POST /inbox/{id}/ack   — acknowledge (mark handled)

	r.Get("/inbox", func(w http.ResponseWriter, req *http.Request) {
		typeFilter := req.URL.Query().Get("type")
		notifications := inbox.List(typeFilter)
		w.Header().Set("Content-Type", "application/ld+json")
		w.Header().Set("Link", `<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`)
		// Wrap in LDN container response
		json.NewEncoder(w).Encode(map[string]interface{}{
			"@context":    "https://www.w3.org/ns/activitystreams",
			"@type":       "OrderedCollection",
			"totalItems":  len(notifications),
			"orderedItems": notifications,
		})
	})

	r.Post("/inbox", func(w http.ResponseWriter, req *http.Request) {
		// LDN spec: content-type must be JSON-LD (or Turtle, etc.)
		// We accept application/ld+json and application/json for convenience.
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
		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		n := inbox.Add(raw)
		log.Printf("[inbox] received notification id=%s types=%v actor=%s",
			n.ID, n.Type, n.Actor)

		// LDN spec: 201 Created with Location header
		w.Header().Set("Location", "/inbox/"+n.ID)
		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": n.ID})
	})

	r.Get("/inbox/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		n := inbox.Get(id)
		if n == nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/ld+json")
		json.NewEncoder(w).Encode(n)
	})

	r.Post("/inbox/{id}/ack", func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		if !inbox.Acknowledge(id) {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "acknowledged"})
	})

	// ── Stats + Health ───────────────────────────────────────────────────────
	r.Get("/inbox/stats", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inbox.Stats())
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := inbox.Stats()
		fmt.Fprintf(w, `{"status":"ok","service":"notification-service","node":%q,"pending":%d}`,
			nodeID, stats["pending"])
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Advertise LDN inbox in Link header (W3C LDN discovery mechanism)
		w.Header().Set("Link", `</inbox>; rel="http://www.w3.org/ns/ldp#inbox"`)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "notification-service",
			"node":    nodeID,
			"endpoints": map[string]string{
				"inbox":    "GET|POST /inbox",
				"message":  "GET /inbox/{id}",
				"ack":      "POST /inbox/{id}/ack",
				"stats":    "/inbox/stats",
				"health":   "/health",
			},
			"specs": []string{
				"https://www.w3.org/TR/ldn/",
				"https://www.w3.org/TR/activitystreams-core/",
			},
		})
	})

	port := getEnv("PORT", "8084")
	log.Printf("notification-service listening on :%s (node: %s)", port, nodeID)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
