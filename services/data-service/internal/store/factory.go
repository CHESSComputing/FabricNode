// Package store — factory that constructs the configured GraphStore backend.
package store

import (
	"fmt"
	"time"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
)

// NewFromConfig creates the GraphStore backend described by cfg.
//
// Supported store types (cfg.DataService.StoreType):
//
//	"memory"    — in-memory store, seeded with demo CHESS data (default)
//	"oxigraph"  — Oxigraph SPARQL server; cfg.DataService.OxigraphURL must be set
//
// An unknown store type returns an error; callers should treat this as fatal.
func NewFromConfig(cfg *fabricconfig.DataServiceConfig) (GraphStore, error) {
	switch cfg.StoreType {
	case "", "memory":
		return NewMemoryStore(), nil

	case "oxigraph":
		if cfg.OxigraphURL == "" {
			return nil, fmt.Errorf("store: oxigraph selected but data_service.oxigraph_url is not set")
		}
		timeout := time.Duration(cfg.OxigraphTimeout) * time.Second
		return NewOxigraphStore(cfg.OxigraphURL, timeout), nil

	default:
		return nil, fmt.Errorf("store: unknown store_type %q (valid: memory, oxigraph)", cfg.StoreType)
	}
}
