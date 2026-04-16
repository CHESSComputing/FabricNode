// Package void generates the four-layer Knowledge Fabric self-description
// artifacts for the CHESS node:
//
//	L1  VoID dataset description   (/.well-known/void)
//	L1  PROF capability profile    (/.well-known/profile)
//	L3  SHACL shapes               (/.well-known/shacl)
//	L4  SPARQL examples catalog    (/.well-known/sparql-examples)
//
// Templates are embedded from the templates/ sub-directory at compile time.
// Previously this package exposed its own NodeConfig struct that duplicated
// fabricconfig.NodeConfig.  It now accepts *fabricconfig.Config directly,
// eliminating the duplication.
package void

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
	"time"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templateData is the data model passed to every template.
type templateData struct {
	BaseURL        string
	NodeID         string
	NodeName       string
	DataServiceURL string
	ModifiedDate   string // "YYYY-MM-DD"
	Beamlines      []fabricconfig.BeamlineConfig
}

// newData builds the template data model from a canonical config.
func newData(cfg *fabricconfig.Config) templateData {
	return templateData{
		BaseURL:        cfg.Node.BaseURL,
		NodeID:         cfg.Node.ID,
		NodeName:       cfg.Node.Name,
		DataServiceURL: cfg.Node.DataServiceURL,
		ModifiedDate:   time.Now().UTC().Format("2006-01-02"),
		Beamlines:      cfg.Catalog.Beamlines,
	}
}

// render loads the named embedded template and executes it with data.
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

// ── L1 — VoID dataset description ────────────────────────────────────────────

// VoIDTurtle returns a full VoID + DCAT service description in Turtle.
func VoIDTurtle(cfg *fabricconfig.Config) (string, error) {
	return render("templates/void.tmpl", newData(cfg))
}

// VoIDJSONLD returns a minimal JSON-LD representation of the VoID description.
func VoIDJSONLD(cfg *fabricconfig.Config) (string, error) {
	return render("templates/void-jsonld.tmpl", newData(cfg))
}

// ── L1 — PROF capability profile ─────────────────────────────────────────────

// ProfileTurtle returns the PROF resource descriptor for this node.
func ProfileTurtle(cfg *fabricconfig.Config) (string, error) {
	return render("templates/profile.tmpl", newData(cfg))
}

// ── L3 — SHACL shapes ─────────────────────────────────────────────────────────

// SHACLTurtle returns SHACL NodeShapes for CHESS beamline observations.
func SHACLTurtle(cfg *fabricconfig.Config) (string, error) {
	return render("templates/shacl.tmpl", newData(cfg))
}

// ── L4 — SPARQL examples catalog ─────────────────────────────────────────────

// SPARQLExamplesTurtle returns a spex-pattern SPARQL examples catalog.
func SPARQLExamplesTurtle(cfg *fabricconfig.Config) (string, error) {
	return render("templates/sparql-examples.tmpl", newData(cfg))
}
