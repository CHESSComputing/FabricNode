// Package config loads and validates the FabricNode configuration file.
//
// The configuration file is a single YAML (or JSON) document that covers
// every service in the node.  Each service reads the top-level fields it
// cares about; unrecognised fields are silently ignored.
//
// Lookup order for the config file path:
//  1. The path passed explicitly to Load(path)
//  2. The FABRIC_CONFIG environment variable
//  3. ./fabric.yaml  (working directory)
//  4. ./config/fabric.yaml
//  5. $HOME/.fabric/fabric.yaml
//
// Both YAML (.yaml / .yml) and JSON (.json) extensions are accepted.
// The format is detected from the file extension; default is YAML.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ──────────────────────────────────────────────────────────────────────────────
// Top-level config struct
// ──────────────────────────────────────────────────────────────────────────────

// Config is the root configuration document.
// Every section is optional; missing sections fall back to safe defaults.
type Config struct {
	Node         NodeConfig         `yaml:"node"         json:"node"`
	Catalog      CatalogConfig      `yaml:"catalog"      json:"catalog"`
	DataService  DataServiceConfig  `yaml:"data_service" json:"data_service"`
	Identity     IdentityConfig     `yaml:"identity"     json:"identity"`
	Notification NotificationConfig `yaml:"notification" json:"notification"`
	Foxden       FoxdenConfig       `yaml:"foxden"       json:"foxden"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: node — identity shared across all services
// ──────────────────────────────────────────────────────────────────────────────

// NodeConfig holds node-wide identity fields used by every service.
type NodeConfig struct {
	// ID is the short machine identifier, e.g. "chess-node".
	ID string `yaml:"id" json:"id"`

	// Name is the human-readable label, e.g. "CHESS Federated Knowledge Fabric Node".
	Name string `yaml:"name" json:"name"`

	// BaseURL is the externally reachable root URL of the node.
	// Each service appends its own port, e.g. "https://fabric.chess.cornell.edu".
	BaseURL string `yaml:"base_url" json:"base_url"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: catalog — catalog-service configuration
// ──────────────────────────────────────────────────────────────────────────────

// CatalogConfig configures the catalog-service.
type CatalogConfig struct {
	// Port the catalog-service listens on (default: 8081).
	Port int `yaml:"port" json:"port"`

	// Beamlines is the registry of beamlines served by this node.
	// These replace the hard-coded knownBeamlines slice in datasets.go.
	Beamlines []BeamlineConfig `yaml:"beamlines" json:"beamlines"`
}

// BeamlineConfig describes one CHESS beamline.
type BeamlineConfig struct {
	// ID is the canonical lower-case beamline identifier, e.g. "id1", "3a", "fast".
	// Must match the beamline= segment used in dataset DIDs.
	ID string `yaml:"id" json:"id"`

	// Label is the human-readable name shown in catalog responses.
	Label string `yaml:"label" json:"label"`

	// Type is the beamline technique class, e.g. "x-ray-diffraction".
	Type string `yaml:"type" json:"type"`

	// Location is an optional physical location string.
	Location string `yaml:"location,omitempty" json:"location,omitempty"`

	// Description is an optional long-form description.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: data_service — data-service configuration
// ──────────────────────────────────────────────────────────────────────────────

// DataServiceConfig configures the data-service.
type DataServiceConfig struct {
	// Port the data-service listens on (default: 8082).
	Port int `yaml:"port" json:"port"`

	// SPARQLResultLimit caps the number of triples returned by any SPARQL query.
	SPARQLResultLimit int `yaml:"sparql_result_limit" json:"sparql_result_limit"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: identity — identity-service configuration
// ──────────────────────────────────────────────────────────────────────────────

// IdentityConfig configures the identity-service.
type IdentityConfig struct {
	// Port the identity-service listens on (default: 8083).
	Port int `yaml:"port" json:"port"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: notification — notification-service configuration
// ──────────────────────────────────────────────────────────────────────────────

// NotificationConfig configures the notification-service.
type NotificationConfig struct {
	// Port the notification-service listens on (default: 8084).
	Port int `yaml:"port" json:"port"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Section: foxden — FOXDEN integration
// ──────────────────────────────────────────────────────────────────────────────

// FoxdenConfig holds connection details for every FOXDEN service the node
// needs to talk to.
type FoxdenConfig struct {
	// MetadataURL is the base URL of the FOXDEN metadata service.
	// Example: "http://foxden.chess.cornell.edu:8300"
	MetadataURL string `yaml:"metadata_url" json:"metadata_url"`

	// ProvenanceURL is the base URL of the FOXDEN provenance service.
	ProvenanceURL string `yaml:"provenance_url" json:"provenance_url"`

	// DOIURL is the base URL of the FOXDEN DOI/publication service.
	DOIURL string `yaml:"doi_url" json:"doi_url"`

	// AuthzURL is FOXDEN authentication/authrization service
	AuthzURL     string `yaml:"authz_url" json:"authz_url"`
	ClientID     string `yaml:"authz_client_id" json:"authz_client_id"`
	ClientSecret string `yaml:"authz_client_secret" json:"authz_client_secret"`
	TokenScope   string `yaml:"token_scope" json:"token_scope"`

	// Token is the bearer token sent in the Authorization header.
	// Leave empty if the FOXDEN instance does not require authentication.
	// In production, inject this via the FOXDEN_TOKEN environment variable
	// rather than storing it in the config file.
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// Timeout is the HTTP client timeout in seconds (default: 10).
	Timeout int `yaml:"timeout" json:"timeout"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Loading
// ──────────────────────────────────────────────────────────────────────────────

// Load reads a config file and returns the parsed Config.
// If path is empty the function searches the default locations.
// Environment variables override file values (see ApplyEnv).
func Load(path string) (*Config, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return defaults(), err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", resolved, err)
	}

	cfg := defaults()
	if isJSON(resolved) {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parse JSON %q: %w", resolved, err)
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config: parse YAML %q: %w", resolved, err)
		}
	}

	cfg.ApplyEnv()
	return cfg, nil
}

// ApplyEnv overrides config values with environment variables.
// This is called automatically by Load; you can also call it manually
// after constructing a Config in tests.
//
// Override map:
//
//	NODE_ID            → cfg.Node.ID
//	NODE_NAME          → cfg.Node.Name
//	NODE_BASE_URL      → cfg.Node.BaseURL
//	FOXDEN_URL         → cfg.Foxden.MetadataURL  (legacy single-URL override)
//	FOXDEN_TOKEN       → cfg.Foxden.Token
//	FOXDEN_METADATA_URL   → cfg.Foxden.MetadataURL
//	FOXDEN_PROVENANCE_URL → cfg.Foxden.ProvenanceURL
func (c *Config) ApplyEnv() {
	if v := os.Getenv("NODE_ID"); v != "" {
		c.Node.ID = v
	}
	if v := os.Getenv("NODE_NAME"); v != "" {
		c.Node.Name = v
	}
	if v := os.Getenv("NODE_BASE_URL"); v != "" {
		c.Node.BaseURL = v
	}
	if v := os.Getenv("FOXDEN_URL"); v != "" {
		c.Foxden.MetadataURL = v
	}
	if v := os.Getenv("FOXDEN_METADATA_URL"); v != "" {
		c.Foxden.MetadataURL = v
	}
	if v := os.Getenv("FOXDEN_PROVENANCE_URL"); v != "" {
		c.Foxden.ProvenanceURL = v
	}
	if v := os.Getenv("FOXDEN_TOKEN"); v != "" {
		c.Foxden.Token = v
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// defaults returns a Config pre-populated with safe fallback values so that
// services work out of the box without a config file (local development).
func defaults() *Config {
	return &Config{
		Node: NodeConfig{
			ID:      "chess-node",
			Name:    "CHESS Federated Knowledge Fabric Node",
			BaseURL: "http://localhost:8081",
		},
		Catalog: CatalogConfig{
			Port: 8081,
			Beamlines: []BeamlineConfig{
				{ID: "id1", Label: "Beamline ID1 — X-ray Diffraction", Type: "x-ray-diffraction", Location: "CHESS Wilson Laboratory"},
				{ID: "id3a", Label: "Beamline ID3A — Protein Crystallography", Type: "protein-crystallography"},
				{ID: "fast", Label: "Beamline FAST — Time-Resolved Scattering", Type: "time-resolved-scattering"},
			},
		},
		DataService: DataServiceConfig{
			Port:              8082,
			SPARQLResultLimit: 100,
		},
		Identity: IdentityConfig{
			Port: 8083,
		},
		Notification: NotificationConfig{
			Port: 8084,
		},
		Foxden: FoxdenConfig{
			MetadataURL: "http://localhost:8300",
			ProvenanceURL: "http://localhost:8310",
			DOIURL: "http://localhost:8377",
			AuthzURL: "http://localhost:8380",
			TokenScope: "read+write",
			Timeout:     10,
		},
	}
}

// resolvePath returns the concrete file path to load.
func resolvePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := os.Getenv("FABRIC_CONFIG"); v != "" {
		return v, nil
	}
	candidates := []string{
		"fabric.yaml",
		"fabric.yml",
		"fabric.json",
		"config/fabric.yaml",
		"config/fabric.yml",
		"config/fabric.json",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".fabric", "fabric.yaml"),
			filepath.Join(home, ".fabric", "fabric.yml"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// No file found — return empty path so caller can decide whether to error
	// or continue with defaults.
	return "", fmt.Errorf("config: no config file found; tried %s and $FABRIC_CONFIG; using defaults", strings.Join(candidates[:4], ", "))
}

func isJSON(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}
