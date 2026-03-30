package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
	"github.com/go-chi/chi/v5"
)

// TODO: should come from configuration of FabricNode

// knownBeamlines is the registry of beamlines served by this node.
// In production, load from config or discover via the data-service.
var knownBeamlines = []model.Beamline{
	{ID: "id1", Label: "Beamline ID1 — X-ray Diffraction",
		Type: "x-ray-diffraction", Location: "CHESS Wilson Laboratory"},
	{ID: "3a", Label: "Beamline ID3A — Protein Crystallography",
		Type: "protein-crystallography"},
	{ID: "fast", Label: "Beamline FAST — Time-Resolved Scattering",
		Type: "time-resolved-scattering"},
}

// Beamlines lists all registered beamlines.
//
//	GET /catalog/beamlines
func Beamlines(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := map[string]any{
			"@context": map[string]string{
				"dcat":  "http://www.w3.org/ns/dcat#",
				"dct":   "http://purl.org/dc/terms/",
				"chess": "http://chess.cornell.edu/ns#",
			},
			"@id":          fmt.Sprintf("%s/catalog", cfg.BaseURL),
			"@type":        "dcat:Catalog",
			"dcat:service": fmt.Sprintf("%s/catalog/beamlines", cfg.BaseURL),
			"dcat:dataset": beamlineEntries(cfg),
		}
		w.Header().Set("Content-Type", "application/ld+json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
	}
}

// Datasets lists datasets registered under a specific beamline.
//
//	GET /catalog/beamlines/{beamline}/datasets
//
// Beamline names follow CHESS convention: letters + digits, e.g. id1, id3a, fast.
func Datasets(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		blRaw := chi.URLParam(req, "beamline")
		bl := model.BeamlineID(blRaw)
		if !bl.Valid() {
			http.Error(w, fmt.Sprintf("invalid beamline id %q: must be lower-case letters and digits", blRaw),
				http.StatusBadRequest)
			return
		}
		if !beamlineRegistered(bl) {
			http.NotFound(w, req)
			return
		}

		datasets := datasetsForBeamline(cfg, bl)
		resp := map[string]any{
			"@context": map[string]string{
				"dcat": "http://www.w3.org/ns/dcat#",
				"dct":  "http://purl.org/dc/terms/",
			},
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.BaseURL, bl),
			"@type":        "dcat:Catalog",
			"dcat:dataset": datasets,
		}
		w.Header().Set("Content-Type", "application/ld+json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers — replace mock data with a real data-service client call
// ──────────────────────────────────────────────────────────────────────────────

func beamlineRegistered(bl model.BeamlineID) bool {
	for _, b := range knownBeamlines {
		if b.ID == bl {
			return true
		}
	}
	return false
}

func beamlineEntries(cfg void.NodeConfig) []map[string]any {
	out := make([]map[string]any, 0, len(knownBeamlines))
	for _, bl := range knownBeamlines {
		out = append(out, map[string]any{
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.BaseURL, bl.ID),
			"@type":        "dcat:Catalog",
			"dct:title":    bl.Label,
			"chess:blType": bl.Type,
			"dcat:dataset": fmt.Sprintf("%s/catalog/beamlines/%s/datasets", cfg.BaseURL, bl.ID),
		})
	}
	return out
}

// datasetsForBeamline returns mock dataset entries.
// Replace with a call to the data-service:
//
//	GET <DATA_SERVICE_URL>/beamlines/{beamline}/graphs
//
// and map each graph IRI back to a DCAT dataset entry.
func datasetsForBeamline(cfg void.NodeConfig, bl model.BeamlineID) []map[string]any {
	// Example DIDs for this beamline — in production, query data-service.
	// TODO: call data-service to obtain dids via FOXDEN look-up
	dids := []model.DatasetDID{
		model.DatasetDID(fmt.Sprintf("/beamline=%s/btr=btr001/cycle=2024-3/sample_name=silicon-std", bl)),
		model.DatasetDID(fmt.Sprintf("/beamline=%s/btr=btr002/cycle=2024-3/sample_name=lysozyme-1", bl)),
	}

	dataURL := cfg.DataServiceURL
	out := make([]map[string]any, 0, len(dids))
	for _, did := range dids {
		encodedDID := url.QueryEscape(string(did))
		entry := map[string]any{
			"@id":       fmt.Sprintf("%s/catalog/beamlines/%s/datasets/%s", cfg.BaseURL, bl, encodedDID),
			"@type":     "dcat:Dataset",
			"dct:title": string(did),
			"dcat:distribution": map[string]any{
				"@type": "dcat:Distribution",
				"dcat:accessURL": fmt.Sprintf(
					"%s/beamlines/%s/datasets/%s/sparql",
					dataURL, bl, encodedDID),
			},
		}
		out = append(out, entry)
	}
	return out
}
