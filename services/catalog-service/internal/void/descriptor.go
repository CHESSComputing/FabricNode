// Package void generates the four-layer Knowledge Fabric self-description
// artifacts for the CHESS node:
//
//	L1  VoID dataset description   (/.well-known/void)
//	L1  PROF capability profile    (/.well-known/profile)
//	L3  SHACL shapes               (/.well-known/shacl)
//	L4  SPARQL examples catalog    (/.well-known/sparql-examples)
//
// Templates are embedded from the templates/ sub-directory at compile time.
// All returned strings are in Turtle (text/turtle) unless a JSON-LD variant
// is requested; the caller handles content negotiation.
package void

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// NodeConfig holds deployment-specific values injected at startup.
type NodeConfig struct {
	BaseURL  string // e.g. https://chess-node.example.org
	NodeID   string // e.g. chess-node
	NodeName string // human label
}

// DefaultConfig returns a config suitable for local development.
func DefaultConfig() NodeConfig {
	return NodeConfig{
		BaseURL:  "http://localhost:8081",
		NodeID:   "chess-node",
		NodeName: "CHESS Federated Knowledge Fabric Node",
	}
}

// templateData is the data model passed to every template.
// Fields are exported so text/template can access them.
type templateData struct {
	NodeConfig
	ModifiedDate string // "YYYY-MM-DD" — injected at render time
}

// render loads the named template file and executes it with the given data.
// name must match a file under the embedded templates/ directory,
// e.g. "templates/void.tmpl".
func render(name string, data any) (string, error) {
	src, err := templateFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("void: read template %q: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(src))
	if err != nil {
		return "", fmt.Errorf("void: parse template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("void: execute template %q: %w", name, err)
	}
	return buf.String(), nil
}

// newData builds the template data model, filling ModifiedDate with today's
// date unless the caller has set it already (zero value triggers auto-fill).
func newData(cfg NodeConfig) templateData {
	return templateData{
		NodeConfig:   cfg,
		ModifiedDate: time.Now().UTC().Format("2006-01-02"),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// L1 — VoID dataset description
// ──────────────────────────────────────────────────────────────────────────────

// VoIDTurtle returns a full VoID + DCAT service description in Turtle.
func VoIDTurtle(cfg NodeConfig) (string, error) {
	return render("templates/void.tmpl", newData(cfg))
}

// VoIDJSONLD returns a minimal JSON-LD representation of the VoID description.
func VoIDJSONLD(cfg NodeConfig) (string, error) {
	return render("templates/void-jsonld.tmpl", newData(cfg))
}

// ──────────────────────────────────────────────────────────────────────────────
// L1 — PROF capability profile
// ──────────────────────────────────────────────────────────────────────────────

// ProfileTurtle returns the PROF resource descriptor for this node.
func ProfileTurtle(cfg NodeConfig) (string, error) {
	return render("templates/profile.tmpl", newData(cfg))
}

// ──────────────────────────────────────────────────────────────────────────────
// L3 — SHACL shapes
// ──────────────────────────────────────────────────────────────────────────────

// SHACLTurtle returns SHACL NodeShapes for CHESS beamline observations.
// The SHACL content is static (no BaseURL references), so the template
// is rendered with config for consistency and future extensibility.
func SHACLTurtle(cfg NodeConfig) (string, error) {
	return render("templates/shacl.tmpl", newData(cfg))
}

// ──────────────────────────────────────────────────────────────────────────────
// L4 — SPARQL examples catalog
// ──────────────────────────────────────────────────────────────────────────────

// SPARQLExamplesTurtle returns a spex-pattern SPARQL examples catalog.
func SPARQLExamplesTurtle(cfg NodeConfig) (string, error) {
	return render("templates/sparql-examples.tmpl", newData(cfg))
}
