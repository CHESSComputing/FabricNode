package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/pkg/server"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func main() {
	// ── Load configuration ───────────────────────────────────────────────────
	// Searches for fabric.yaml in standard locations; falls back to safe
	// defaults if no file is found. FABRIC_CONFIG env var overrides the path.
	nodeCfg, beamlines := loadConfig()

	cfg := void.NodeConfig{
		BaseURL:        nodeCfg.Node.BaseURL,
		NodeID:         nodeCfg.Node.ID,
		NodeName:       nodeCfg.Node.Name,
		DataServiceURL: server.GetEnv("DATA_SERVICE_URL", "http://localhost:8082"),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(server.ReadOnlyCORS())

	// ── L1: VoID dataset description ────────────────────────────────────────
	r.Get("/.well-known/void", handlers.VoID(cfg))

	// ── L1: PROF capability profile ─────────────────────────────────────────
	r.Get("/.well-known/profile", handlers.Profile(cfg))

	// ── L3: SHACL shapes ────────────────────────────────────────────────────
	r.Get("/.well-known/shacl", handlers.SHACL(cfg))

	// ── L4: SPARQL examples catalog ─────────────────────────────────────────
	r.Get("/.well-known/sparql-examples", handlers.SPARQLExamples(cfg))

	// ── Catalog: beamline + dataset discovery ────────────────────────────────
	r.Get("/catalog/beamlines", handlers.Beamlines(cfg, beamlines))
	r.Get("/catalog/beamlines/{beamline}/datasets", handlers.Datasets(cfg, beamlines))

	// ── Health + info ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(cfg))
	r.Get("/", handlers.Index(cfg))

	port := server.GetEnv("PORT", fmt.Sprintf("%d", nodeCfg.Catalog.Port))
	log.Printf("catalog-service listening on :%s (node: %s, base: %s, beamlines: %d)",
		port, cfg.NodeID, cfg.BaseURL, len(beamlines))
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// loadConfig loads the node configuration and extracts the parts used by
// this service. Logs a warning (not a fatal) if no config file is found —
// the built-in defaults are sufficient for local development.
func loadConfig() (*fabricconfig.Config, []fabricconfig.BeamlineConfig) {
	cfg, err := fabricconfig.Load(server.GetEnv("FABRIC_CONFIG", ""))
	if err != nil {
		log.Printf("catalog-service: config warning: %v — using defaults", err)
		cfg, _ = fabricconfig.Load("") // re-call to get defaults (Load never returns nil)
		// If even that errors (shouldn't), fall back manually.
		if cfg == nil {
			return &fabricconfig.Config{
				Node: fabricconfig.NodeConfig{
					ID:      server.GetEnv("NODE_ID", "chess-node"),
					Name:    server.GetEnv("NODE_NAME", "CHESS Federated Knowledge Fabric Node"),
					BaseURL: server.GetEnv("NODE_BASE_URL", "http://localhost:8081"),
				},
			}, nil
		}
	}
	// Honour legacy env vars that may have been set without a config file.
	if v := server.GetEnv("NODE_BASE_URL", ""); v != "" {
		cfg.Node.BaseURL = v
	}
	return cfg, cfg.Catalog.Beamlines
}


