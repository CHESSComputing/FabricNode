package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/rdf"
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
	r.Get("/.well-known/void", func(w http.ResponseWriter, req *http.Request) {
		format := rdf.Negotiate(req)
		w.Header().Set("Content-Type", string(format))
		w.Header().Set("Link", `</.well-known/void>; rel="self"`)
		addCacheHeaders(w, 300)
		switch format {
		case rdf.FormatJSONLD:
			fmt.Fprint(w, void.VoIDJSONLD(cfg))
		default:
			fmt.Fprint(w, void.VoIDTurtle(cfg))
		}
	})

	// ── L1: PROF capability profile ─────────────────────────────────────────
	r.Get("/.well-known/profile", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		addCacheHeaders(w, 3600)
		fmt.Fprint(w, void.ProfileTurtle(cfg))
	})

	// ── L3: SHACL shapes ────────────────────────────────────────────────────
	r.Get("/.well-known/shacl", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		addCacheHeaders(w, 3600)
		fmt.Fprint(w, void.SHACLTurtle(cfg))
	})

	// ── L4: SPARQL examples catalog ─────────────────────────────────────────
	r.Get("/.well-known/sparql-examples", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		addCacheHeaders(w, 3600)
		fmt.Fprint(w, void.SPARQLExamplesTurtle(cfg))
	})

	// ── Health + info ────────────────────────────────────────────────────────
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"catalog-service","nodeId":%q}`, cfg.NodeID)
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
  "service": "catalog-service",
  "description": "Knowledge Fabric self-description endpoints for CHESS node",
  "endpoints": {
    "void":           "/.well-known/void",
    "profile":        "/.well-known/profile",
    "shacl":          "/.well-known/shacl",
    "sparqlExamples": "/.well-known/sparql-examples",
    "health":         "/health"
  }
}`)
	})

	port := getEnv("PORT", "8081")
	log.Printf("catalog-service listening on :%s (node: %s, base: %s)", port, cfg.NodeID, cfg.BaseURL)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func addCacheHeaders(w http.ResponseWriter, maxAgeSeconds int) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAgeSeconds))
	w.Header().Set("Vary", "Accept")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
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
