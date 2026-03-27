package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/foxden"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func main() {
	db := store.New()

	foxdenCfg := handlers.FoxdenConfig{
		Client: foxden.NewClient(getEnv("FOXDEN_URL", "http://localhost:8300")),
		Store:  db,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// ── Global SPARQL + graphs ────────────────────────────────────────────────
	r.Get("/sparql", handlers.SPARQL(db))
	r.Post("/sparql", handlers.SPARQL(db))
	r.Get("/graphs", handlers.Graphs(db))

	// ── Beamline-scoped routes ────────────────────────────────────────────────
	r.Route("/beamlines/{beamline}", func(r chi.Router) {
		r.Get("/sparql", handlers.BeamlineSPARQL(db))
		r.Get("/graphs", handlers.BeamlineGraphs(db))

		// FOXDEN metadata listing for this beamline (?cycle=<val> optional)
		r.Get("/foxden/datasets", handlers.FoxdenDatasets(foxdenCfg))

		// ── Dataset-scoped routes ─────────────────────────────────────────────
		// {did} is URL-encoded because DIDs contain slashes and equals signs.
		// Example: %2Fbeamline%3D3a%2Fbtr%3Dtest-123-a%2Fcycle%3D2026-1%2Fsample_name%3Dbla
		r.Route("/datasets/{did}", func(r chi.Router) {
			r.Get("/sparql", handlers.DatasetSPARQL(db))
			r.Post("/triples", handlers.Triples(db))
			r.Post("/validate", handlers.Validate(db))

			// FOXDEN metadata for this specific dataset
			r.Get("/foxden", handlers.FoxdenDataset(foxdenCfg))

			// Fetch from FOXDEN and store as RDF triples
			r.Post("/foxden/ingest", handlers.FoxdenIngest(foxdenCfg))
		})
	})

	// ── Health + index ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(db))
	r.Get("/", handlers.Index())

	port := getEnv("PORT", "8082")
	log.Printf("data-service listening on :%s (foxden: %s)", port, getEnv("FOXDEN_URL", "http://localhost:8300"))
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
