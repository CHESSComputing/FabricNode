package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/foxden"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// GetToken returns a token from either an environment variable
// or a file path (based on tokenSource value).
func GetToken(tokenSource string) string {
	if tokenSource == "" {
		panic("tokenSource is empty")
	}

	// 1. Try environment variable
	if val, ok := os.LookupEnv(tokenSource); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}

	// 2. Otherwise treat as file path
	data, err := os.ReadFile(tokenSource)
	if err == nil {
		token := strings.TrimSpace(string(data))
		return token
	}

	return tokenSource
}

func main() {
	// ── Load configuration ───────────────────────────────────────────────────
	cfg, err := fabricconfig.Load(getEnv("FABRIC_CONFIG", ""))
	if err != nil {
		log.Printf("data-service: config warning: %v — using defaults", err)
	}

	db := store.New()

	foxdenCfg := handlers.FoxdenConfig{
		Client: foxden.NewClientWithToken(
			cfg.Foxden.MetadataURL,
			GetToken(cfg.Foxden.Token),
			cfg.Foxden.Timeout,
		),
		Store: db,
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
		r.Get("/foxden/datasets", handlers.FoxdenDatasets(foxdenCfg))

		r.Route("/datasets/{did}", func(r chi.Router) {
			r.Get("/sparql", handlers.DatasetSPARQL(db))
			r.Post("/triples", handlers.Triples(db))
			r.Post("/validate", handlers.Validate(db))
			r.Get("/foxden", handlers.FoxdenDataset(foxdenCfg))
			r.Post("/foxden/ingest", handlers.FoxdenIngest(foxdenCfg))
		})
	})

	// ── Health + index ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(db))
	r.Get("/", handlers.Index())

	port := getEnv("PORT", "8082")
	log.Printf("data-service listening on :%s (foxden: %s)", port, cfg.Foxden.MetadataURL)
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
