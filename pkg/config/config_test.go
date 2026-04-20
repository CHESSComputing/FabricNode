package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CHESSComputing/FabricNode/pkg/config"
)

const sampleYAML = `
node:
  id: test-node
  name: Test Fabric Node
  base_url: http://test.example.org
  iri_base: http://test.example.org/

catalog:
  port: 8781
  beamlines:
    - id: id1
      label: "Beamline ID1 — X-ray Diffraction"
      type: x-ray-diffraction
      location: Wilson Lab
    - id: 3a
      label: "Beamline 3A — Protein Crystallography"
      type: protein-crystallography

data_service:
  port: 8782
  sparql_result_limit: 200

foxden:
  metadata_url: http://foxden.example.org:8300
  provenance_url: http://foxden.example.org:8301
  token: ""
  timeout: 15
`

func TestLoadYAML(t *testing.T) {
	f := writeTmp(t, "fabric.yaml", sampleYAML)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Node.ID != "test-node" {
		t.Errorf("Node.ID: got %q want %q", cfg.Node.ID, "test-node")
	}
	if cfg.Node.IRIBase != "http://test.example.org/" {
		t.Errorf("Node.IRIBase: got %q want %q", cfg.Node.IRIBase, "http://test.example.org/")
	}
	if len(cfg.Catalog.Beamlines) != 2 {
		t.Errorf("Beamlines: got %d want 2", len(cfg.Catalog.Beamlines))
	}
	if cfg.Catalog.Beamlines[0].ID != "id1" {
		t.Errorf("Beamlines[0].ID: got %q want %q", cfg.Catalog.Beamlines[0].ID, "id1")
	}
	if cfg.DataService.SPARQLResultLimit != 200 {
		t.Errorf("SPARQLResultLimit: got %d want 200", cfg.DataService.SPARQLResultLimit)
	}
	if cfg.Foxden.MetadataURL != "http://foxden.example.org:8300" {
		t.Errorf("Foxden.MetadataURL: got %q", cfg.Foxden.MetadataURL)
	}
	if cfg.Foxden.Timeout != 15 {
		t.Errorf("Foxden.Timeout: got %d want 15", cfg.Foxden.Timeout)
	}
}

func TestLoadJSON(t *testing.T) {
	const sampleJSON = `{
		"node": {"id": "json-node", "name": "JSON Node", "base_url": "http://json.example.org", "iri_base": "http://json.example.org/"},
		"catalog": {"port": 8781, "beamlines": [{"id": "fast", "label": "FAST", "type": "time-resolved-scattering"}]}
	}`
	f := writeTmp(t, "fabric.json", sampleJSON)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load JSON: %v", err)
	}
	if cfg.Node.ID != "json-node" {
		t.Errorf("Node.ID: got %q", cfg.Node.ID)
	}
	if cfg.Node.IRIBase != "http://json.example.org/" {
		t.Errorf("Node.IRIBase: got %q want %q", cfg.Node.IRIBase, "http://json.example.org/")
	}
	if len(cfg.Catalog.Beamlines) != 1 || cfg.Catalog.Beamlines[0].ID != "fast" {
		t.Errorf("Beamlines: %+v", cfg.Catalog.Beamlines)
	}
}

func TestEnvOverride(t *testing.T) {
	f := writeTmp(t, "fabric.yaml", sampleYAML)
	t.Setenv("NODE_ID", "env-override-node")
	t.Setenv("FOXDEN_TOKEN", "secret-token")
	t.Setenv("NODE_IRI_BASE", "http://env-override.example.org/")

	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Node.ID != "env-override-node" {
		t.Errorf("Node.ID env override: got %q", cfg.Node.ID)
	}
	if cfg.Foxden.Token != "secret-token" {
		t.Errorf("Foxden.Token env override: got %q", cfg.Foxden.Token)
	}
	if cfg.Node.IRIBase != "http://env-override.example.org/" {
		t.Errorf("Node.IRIBase env override: got %q", cfg.Node.IRIBase)
	}
}

func TestDefaults(t *testing.T) {
	// Load a minimal file — everything not specified should use defaults.
	f := writeTmp(t, "fabric.yaml", "node:\n  id: minimal\n  iri_base: http://minimal.example.org/\n")
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataService.Port != 8782 {
		t.Errorf("DataService.Port default: got %d want 8782", cfg.DataService.Port)
	}
	if cfg.DataService.SPARQLResultLimit != 100 {
		t.Errorf("SPARQLResultLimit default: got %d want 100", cfg.DataService.SPARQLResultLimit)
	}
	if len(cfg.Catalog.Beamlines) == 0 {
		t.Error("default beamlines should be non-empty")
	}
}

// ── Validate ──────────────────────────────────────────────────────────────────

func TestValidate_MissingIRIBase(t *testing.T) {
	// A config with no iri_base must fail Validate.
	f := writeTmp(t, "fabric.yaml", "node:\n  id: no-iri-base\n")
	cfg, err := config.Load(f)
	if err != nil {
		// Load may return a non-fatal "using defaults" warning — that's OK.
		t.Logf("Load warning (expected): %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate: expected error for empty iri_base, got nil")
	}
}

func TestValidate_IRIBaseWithoutTrailingSlash(t *testing.T) {
	f := writeTmp(t, "fabric.yaml", "node:\n  id: bad-base\n  iri_base: http://example.org\n")
	cfg, _ := config.Load(f)
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate: expected error for iri_base without trailing slash, got nil")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	f := writeTmp(t, "fabric.yaml", sampleYAML)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: unexpected error for valid config: %v", err)
	}
}

func TestChessNS(t *testing.T) {
	cfg := &config.Config{}
	cfg.Node.IRIBase = "http://example.org/"
	want := "http://example.org/ns#"
	if got := cfg.Node.ChessNS(); got != want {
		t.Errorf("ChessNS: got %q want %q", got, want)
	}
}

// writeTmp creates a temp file with name and content, returning its path.
func writeTmp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTmp: %v", err)
	}
	return p
}
