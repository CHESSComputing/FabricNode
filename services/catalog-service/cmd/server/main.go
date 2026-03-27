package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func main() {
	cfg := void.NodeConfig{
		BaseURL:  getEnv("NODE_BASE_URL", "http://localhost:8081"),
		NodeID:   getEnv("NODE_ID", "chess-node"),
		NodeName: getEnv("NODE_NAME", "CHESS Federated Knowledge Fabric Node"),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	// ── L1: VoID dataset description ────────────────────────────────────────
	r.Get("/.well-known/void", handlers.VoID(cfg))

	// ── L1: PROF capability profile ─────────────────────────────────────────
	r.Get("/.well-known/profile", handlers.Profile(cfg))

	// ── L3: SHACL shapes ────────────────────────────────────────────────────
	r.Get("/.well-known/shacl", handlers.SHACL(cfg))

	// ── L4: SPARQL examples catalog ─────────────────────────────────────────
	r.Get("/.well-known/sparql-examples", handlers.SPARQLExamples(cfg))

	// bealines end-point
	r.Get("/catalog/beamlines", handlers.Beamlines(cfg))

	// datasets end-point of specific beamline
	r.Get("/catalog/beamlines", handlers.Beamlines(cfg))
	r.Get("/catalog/beamlines/{beamline}/datasets", handlers.Datasets(cfg))

	// ── Health + info ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(cfg))
	r.Get("/", handlers.Index(cfg))

	port := getEnv("PORT", "8081")
	log.Printf("catalog-service listening on :%s (node: %s, base: %s)", port, cfg.NodeID, cfg.BaseURL)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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
