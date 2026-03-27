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

	// ── Global SPARQL + graphs ────────────────────────────────────────────────
	r.Get("/sparql", handlers.SPARQL(db))
	r.Post("/sparql", handlers.SPARQL(db))
	r.Get("/graphs", handlers.Graphs(db))

	// ── Beamline-scoped routes ────────────────────────────────────────────────
	// Beamline IDs: lower-case letters + digits, e.g. id1, id3a, fast, qm2.
	r.Route("/beamlines/{beamline}", func(r chi.Router) {
		r.Get("/sparql", handlers.BeamlineSPARQL(db))
		r.Get("/graphs", handlers.BeamlineGraphs(db))

		// ── Dataset-scoped routes ─────────────────────────────────────────────
		// Dataset DIDs are URL-encoded in the path because they contain slashes.
		// Example DID:  /beamline=id3a/btr=val123/cycle=2024-3/sample_name=bla
		// URL-encoded:  %2Fbeamline%3Did3a%2Fbtr%3Dval123%2Fcycle%3D2024-3%2Fsample_name%3Dbla
		//
		// The {did} wildcard uses a Chi wildcard segment (*) to capture
		// the rest of the path; the handler URL-decodes it before use.
		r.Route("/datasets/{did}", func(r chi.Router) {
			r.Get("/sparql", handlers.DatasetSPARQL(db))
			r.Post("/triples", handlers.Triples(db))
			r.Post("/validate", handlers.Validate(db))
		})
	})

	// ── Health + index ────────────────────────────────────────────────────────
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
