package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
	"github.com/go-chi/chi/v5"
)

// Beamlines lists all registered beamlines as a DCAT JSON-LD catalog.
//
//	GET /catalog/beamlines
func Beamlines(cfg void.NodeConfig, beamlines []fabricconfig.BeamlineConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := map[string]any{
			"@context": map[string]string{
				"dcat":  "http://www.w3.org/ns/dcat#",
				"dct":   "http://purl.org/dc/terms/",
				"chess": "http://chess.cornell.edu/ns#",
			},
			"@id":          fmt.Sprintf("%s/catalog", cfg.BaseURL),
			"@type":        "dcat:Catalog",
			"dcat:dataset": beamlineEntries(cfg, beamlines),
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
func Datasets(cfg void.NodeConfig, beamlines []fabricconfig.BeamlineConfig) http.HandlerFunc {
	registry := make(map[string]fabricconfig.BeamlineConfig, len(beamlines))
	for _, bl := range beamlines {
		registry[bl.ID] = bl
	}

	return func(w http.ResponseWriter, req *http.Request) {
		blID := chi.URLParam(req, "beamline")
		if blID == "" {
			http.Error(w, "missing beamline", http.StatusBadRequest)
			return
		}
		bl, ok := registry[blID]
		if !ok {
			http.NotFound(w, req)
			return
		}
		resp := map[string]any{
			"@context": map[string]string{
				"dcat": "http://www.w3.org/ns/dcat#",
				"dct":  "http://purl.org/dc/terms/",
			},
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.BaseURL, bl.ID),
			"@type":        "dcat:Catalog",
			"dct:title":    bl.Label,
			"dcat:dataset": datasetsForBeamline(cfg, bl),
		}
		w.Header().Set("Content-Type", "application/ld+json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
	}
}

func beamlineEntries(cfg void.NodeConfig, beamlines []fabricconfig.BeamlineConfig) []map[string]any {
	out := make([]map[string]any, 0, len(beamlines))
	for _, bl := range beamlines {
		entry := map[string]any{
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.BaseURL, bl.ID),
			"@type":        "dcat:Catalog",
			"dct:title":    bl.Label,
			"chess:blType": bl.Type,
			"dcat:dataset": fmt.Sprintf("%s/catalog/beamlines/%s/datasets", cfg.BaseURL, bl.ID),
		}
		if bl.Location != "" {
			entry["chess:location"] = bl.Location
		}
		out = append(out, entry)
	}
	return out
}

// datasetsForBeamline returns stub dataset entries.
// Replace with a live call to data-service /beamlines/{beamline}/graphs in production.
func datasetsForBeamline(cfg void.NodeConfig, bl fabricconfig.BeamlineConfig) []map[string]any {
	dataURL := cfg.DataServiceURL
	dids := []string{
		fmt.Sprintf("/beamline=%s/btr=btr001/cycle=2024-3/sample_name=silicon-std", bl.ID),
		fmt.Sprintf("/beamline=%s/btr=btr002/cycle=2024-3/sample_name=lysozyme-1", bl.ID),
	}
	out := make([]map[string]any, 0, len(dids))
	for _, did := range dids {
		encodedDID := url.QueryEscape(did)
		out = append(out, map[string]any{
			"@id":       fmt.Sprintf("%s/catalog/beamlines/%s/datasets/%s", cfg.BaseURL, bl.ID, encodedDID),
			"@type":     "dcat:Dataset",
			"dct:title": did,
			"dcat:distribution": map[string]any{
				"@type":          "dcat:Distribution",
				"dcat:accessURL": fmt.Sprintf("%s/beamlines/%s/datasets/%s/sparql", dataURL, bl.ID, url.PathEscape(did)),
			},
		})
	}
	return out
}
