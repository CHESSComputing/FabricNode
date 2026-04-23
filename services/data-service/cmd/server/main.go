package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/pkg/server"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/foxden"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func main() {
	// ── Load & validate configuration ────────────────────────────────────────
	cfg, err := fabricconfig.Load(server.GetEnv("FABRIC_CONFIG", ""))
	if err != nil {
		log.Printf("data-service: config warning: %v — using defaults", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("data-service: %v", err)
	}

	// ── Load FOXDEN schema field maps ─────────────────────────────────────────
	schemaFiles := cfg.Foxden.SchemaFiles
	fm, err := foxden.LoadFieldMaps(schemaFiles, cfg.Node.ChessNS())
	if err != nil {
		log.Fatalf("data-service: schema load: %v", err)
	}

	// ── Initialise graph store ───────────────────────────────────────────────
	db, err := store.NewFromConfig(&cfg.DataService, cfg.Node.IRIBase)
	if err != nil {
		log.Fatalf("data-service: graph store init: %v", err)
	}
	log.Printf("data-service: graph store type=%q iri_base=%q", cfg.DataService.StoreType, cfg.Node.IRIBase)

	token := GetTokenFromFoxden(
		cfg.Foxden.AuthzURL,
		cfg.Foxden.ClientID,
		cfg.Foxden.ClientSecret,
		cfg.Foxden.TokenScope,
	)
	if token == "" {
		// fallback mechanism to get token from env or file
		token = GetToken(cfg.Foxden.Token)
	}
	if token == "" {
		log.Println("WARNING: unable to get FOXDEN token...")
	}

	foxdenCfg := handlers.FoxdenConfig{
		Client: foxden.NewClientWithToken(
			cfg.Foxden.MetadataURL,
			token,
			cfg.Foxden.Timeout,
		),
		Store:     db,
		IRIBase:   cfg.Node.IRIBase,
		FieldMaps: fm,
	}

	storeCfg := handlers.StoreConfig{
		Store:   db,
		IRIBase: cfg.Node.IRIBase,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(server.ReadWriteCORS())

	// ── Global SPARQL + graphs ────────────────────────────────────────────────
	r.Get("/sparql", handlers.SPARQL(db))
	r.Post("/sparql", handlers.SPARQL(db))
	r.Get("/graphs", handlers.Graphs(db))

	// ── Beamline-scoped routes ────────────────────────────────────────────────
	r.Route("/beamlines/{beamline}", func(r chi.Router) {
		r.Get("/sparql", handlers.BeamlineSPARQL(db))
		r.Get("/graphs", handlers.BeamlineGraphs(db))
		r.Get("/foxden/datasets", handlers.FoxdenDatasets(foxdenCfg))

		r.Route("/datasets/{did:.*}", func(r chi.Router) {
			r.Get("/sparql", handlers.DatasetSPARQL(db))
			r.Post("/triples", handlers.Triples(storeCfg))
			r.Post("/validate", handlers.Validate(storeCfg))
			r.Get("/foxden", handlers.FoxdenDataset(foxdenCfg))
			r.Post("/foxden/ingest", handlers.FoxdenIngest(foxdenCfg))
		})
	})

	// ── Health + index ────────────────────────────────────────────────────────
	r.Get("/health", handlers.Health(db))
	r.Get("/", handlers.Index())

	port := server.GetEnv("PORT", fmt.Sprintf("%d", cfg.DataService.Port))
	if cfg.TSLConfig.ServerKey == "" && cfg.TSLConfig.ServerCert == "" {
		log.Printf("HTTP data-service listening on :%s (foxden: %s)", port, cfg.Foxden.MetadataURL)
		log.Fatal(http.ListenAndServe(":"+port, r))
	} else {
		log.Printf("HTTPs data-service listening on :%s (foxden: %s)", port, cfg.Foxden.MetadataURL)
		log.Fatal(http.ListenAndServeTLS(":"+port, cfg.TSLConfig.ServerCert, cfg.TSLConfig.ServerKey, r))
	}
}
