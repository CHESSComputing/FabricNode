package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/store"
)

func main() {
	cfg := &handlers.Config{
		Inbox:  store.New(),
		NodeID: getEnv("NODE_ID", "chess-node"),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// ── W3C LDN inbox ────────────────────────────────────────────────────────
	r.Get("/inbox",          handlers.InboxList(cfg))
	r.Post("/inbox",         handlers.InboxReceive(cfg))
	r.Get("/inbox/{id}",     handlers.InboxGet(cfg))
	r.Post("/inbox/{id}/ack", handlers.InboxAck(cfg))

	// ── Stats + Health ───────────────────────────────────────────────────────
	r.Get("/inbox/stats", handlers.InboxStats(cfg))
	r.Get("/health",      handlers.Health(cfg))
	r.Get("/",            handlers.Index(cfg))

	port := getEnv("PORT", "8084")
	log.Printf("notification-service listening on :%s (node: %s)", port, cfg.NodeID)
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
