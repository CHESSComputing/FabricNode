package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
	"github.com/go-chi/chi/v5"
)

func Datasets(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		beamline := chi.URLParam(req, "beamline")
		if beamline == "" {
			http.Error(w, "missing beamline", http.StatusBadRequest)
			return
		}

		valid := map[string]bool{
			"beamline-id1":  true,
			"beamline-id3":  true,
			"beamline-fast": true,
		}

		if !valid[beamline] {
			http.NotFound(w, req)
			return
		}

		datasets := mockDatasets(cfg, beamline)

		resp := map[string]any{
			"@context": map[string]string{
				"dcat": "http://www.w3.org/ns/dcat#",
				"dct":  "http://purl.org/dc/terms/",
			},
			"@id":          fmt.Sprintf("%s/catalog/%s", cfg.BaseURL, beamline),
			"@type":        "dcat:Catalog",
			"dcat:dataset": datasets,
		}

		w.Header().Set("Content-Type", "application/ld+json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
		//json.NewEncoder(w).Encode(resp)
	}
}

func mockDatasets(cfg void.NodeConfig, beamline string) []map[string]any {
	return []map[string]any{
		{
			"@id":         fmt.Sprintf("%s/dataset/%s/run-001", cfg.BaseURL, beamline),
			"@type":       "dcat:Dataset",
			"dct:title":   fmt.Sprintf("%s Run 001", strings.ToUpper(beamline)),
			"dct:created": "2025-01-01",
		},
		{
			"@id":         fmt.Sprintf("%s/dataset/%s/run-002", cfg.BaseURL, beamline),
			"@type":       "dcat:Dataset",
			"dct:title":   fmt.Sprintf("%s Run 002", strings.ToUpper(beamline)),
			"dct:created": "2025-01-02",
		},
	}
}
