package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func main() {
	db := store.New()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// ── SPARQL endpoint (GET + POST) ─────────────────────────────────────────
	r.Get("/sparql", handlers.SPARQL(db))
	r.Post("/sparql", handlers.SPARQL(db))

	// ── Named graphs listing ─────────────────────────────────────────────────
	r.Get("/graphs", handlers.Graphs(db))

	// ── Write endpoint (SHACL-validated) ────────────────────────────────────
	r.Post("/triples", handlers.Triples(db))

	// ── SHACL validation only (dry-run) ─────────────────────────────────────
	r.Post("/validate", handlers.Validate(db))

	// ── Health + info ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(db))
	r.Get("/", handlers.Index())

	port := getEnv("PORT", "8082")
	log.Printf("data-service listening on :%s", port)
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
