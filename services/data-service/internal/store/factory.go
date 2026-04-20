// Package store — factory that constructs the configured GraphStore backend.
package store

import (
	"fmt"
	"time"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
)

// NewFromConfig creates the GraphStore backend described by cfg.
// iriBase is the node-wide IRI prefix from Config.Node.IRIBase; it is passed
// explicitly rather than read from DataServiceConfig so the factory remains
// decoupled from the section that owns the field.
//
// Supported store types (cfg.StoreType):
//
//	"memory"    — in-memory store, seeded with demo data (default)
//	"oxigraph"  — Oxigraph SPARQL server; cfg.OxigraphURL must be set
//
// An unknown store type returns an error; callers should treat this as fatal.
func NewFromConfig(cfg *fabricconfig.DataServiceConfig, iriBase string) (GraphStore, error) {
	switch cfg.StoreType {
	case "", "memory":
		return NewMemoryStoreWithBase(iriBase), nil

	case "oxigraph":
		if cfg.OxigraphURL == "" {
			return nil, fmt.Errorf("store: oxigraph selected but data_service.oxigraph_url is not set")
		}
		timeout := time.Duration(cfg.OxigraphTimeout) * time.Second
		return NewOxigraphStoreWithBase(cfg.OxigraphURL, iriBase, timeout), nil

	default:
		return nil, fmt.Errorf("store: unknown store_type %q (valid: memory, oxigraph)", cfg.StoreType)
	}
}
