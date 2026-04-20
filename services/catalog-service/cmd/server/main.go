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
)

func main() {
	cfg, err := fabricconfig.Load(server.GetEnv("FABRIC_CONFIG", ""))
	if err != nil {
		log.Printf("catalog-service: config warning: %v — using defaults", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("catalog-service: %v", err)
	}
	// cfg.Node.DataServiceURL is populated from the config file or the
	// DATA_SERVICE_URL environment variable (applied by fabricconfig.Load).

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
	r.Get("/catalog/beamlines", handlers.Beamlines(cfg, cfg.Catalog.Beamlines))
	r.Get("/catalog/beamlines/{beamline}/datasets", handlers.Datasets(cfg, cfg.Catalog.Beamlines))

	// ── Health + info ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(cfg))
	r.Get("/", handlers.Index(cfg))

	port := server.GetEnv("PORT", fmt.Sprintf("%d", cfg.Catalog.Port))
	if cfg.TSLConfig.ServerKey == "" && cfg.TSLConfig.ServerCert == "" {
		log.Printf("HTTP catalog-service listening on :%s (node: %s, base: %s, beamlines: %d)",
			port, cfg.Node.ID, cfg.Node.BaseURL, len(cfg.Catalog.Beamlines))
		log.Fatal(http.ListenAndServe(":"+port, r))
	} else {
		log.Printf("HTTPs catalog-service listening on :%s (node: %s, base: %s, beamlines: %d)",
			port, cfg.Node.ID, cfg.Node.BaseURL, len(cfg.Catalog.Beamlines))
		log.Fatal(http.ListenAndServeTLS(":"+port, cfg.TSLConfig.ServerCert, cfg.TSLConfig.ServerKey, r))
	}
}
