package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/go-chi/chi/v5"
)

// Beamlines lists all registered beamlines as a DCAT JSON-LD catalog.
//
//	GET /catalog/beamlines
func Beamlines(cfg *fabricconfig.Config, beamlines []fabricconfig.BeamlineConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		resp := map[string]any{
			"@context": map[string]string{
				"dcat":  "http://www.w3.org/ns/dcat#",
				"dct":   "http://purl.org/dc/terms/",
				"chess": cfg.Node.ChessNS(),
			},
			"@id":          fmt.Sprintf("%s/catalog", cfg.Node.BaseURL),
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
func Datasets(cfg *fabricconfig.Config, beamlines []fabricconfig.BeamlineConfig) http.HandlerFunc {
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
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.Node.BaseURL, bl.ID),
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

func beamlineEntries(cfg *fabricconfig.Config, beamlines []fabricconfig.BeamlineConfig) []map[string]any {
	out := make([]map[string]any, 0, len(beamlines))
	for _, bl := range beamlines {
		entry := map[string]any{
			"@id":          fmt.Sprintf("%s/catalog/beamlines/%s", cfg.Node.BaseURL, bl.ID),
			"@type":        "dcat:Catalog",
			"dct:title":    bl.Label,
			"chess:blType": bl.Type,
			"dcat:dataset": fmt.Sprintf("%s/catalog/beamlines/%s/datasets", cfg.Node.BaseURL, bl.ID),
		}
		if bl.Location != "" {
			entry["chess:location"] = bl.Location
		}
		out = append(out, entry)
	}
	return out
}

// helper function to get token
func getToken(v string) string {
	if v == "" {
		log.Fatal("empty token source provided")
	}

	// Case 1: v is a file path
	if info, err := os.Stat(v); err == nil && !info.IsDir() {
		data, err := os.ReadFile(v)
		if err != nil {
			log.Fatalf("failed to read token file %s: %v", v, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			log.Fatalf("token file %s is empty", v)
		}
		return token
	}

	// Case 2: treat v as env variable name
	token := strings.TrimSpace(os.Getenv(v))
	if token == "" {
		log.Fatalf("environment variable %s is not set or empty", v)
	}

	return token
}

type DidResponse struct {
	Did string `json:"did"`
}

// helper function to get dids from FOXDEN
func getDids(cfg *fabricconfig.Config, bl string) ([]string, error) {
	var dids []string
	// FOXDEN stores beamline as an array field (e.g. ["7A"]), so we must send
	// the query value as an array too.  We include both the original case and
	// upper-case to handle any inconsistency in how beamline IDs are recorded.
	blLower := strings.ToLower(bl)
	//blUpper := strings.ToUpper(bl)
	spec := map[string]any{
		//"beamline": []string{blLower, blUpper},
		"beamline": blLower,
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return dids, err
	}
	// setup http client
	rurl := fmt.Sprintf("%s/records?projection=did", cfg.Foxden.MetadataURL)
	req, err := http.NewRequest("GET", rurl, bytes.NewBuffer(data))
	if err != nil {
		return dids, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", getToken(cfg.Foxden.Token)))
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return dids, err
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return dids, err
	}
	var records []DidResponse
	err = json.Unmarshal(data, &records)
	if err != nil {
		return dids, err
	}
	// Case-insensitive match: DID contains /beamline=<id>/ in any case
	pat := fmt.Sprintf("beamline=%s", blLower)
	for _, r := range records {
		if strings.Contains(strings.ToLower(r.Did), pat) {
			dids = append(dids, r.Did)
		}
	}
	return dids, nil
}

// datasetsForBeamline returns stub dataset entries.
// Replace with a live call to data-service /beamlines/{beamline}/graphs in production.
func datasetsForBeamline(cfg *fabricconfig.Config, bl fabricconfig.BeamlineConfig) []map[string]any {
	dataURL := cfg.Node.DataServiceURL
	// query FOXDEN for all dids with our beamline
	dids, err := getDids(cfg, bl.ID)
	if err != nil {
		log.Println("ERROR: unable to obtain dids from FOXDEN beamline %s, error %v", bl.ID, err)
		return make([]map[string]any, 0, 0)
	}
	out := make([]map[string]any, 0, len(dids))
	for _, did := range dids {
		encodedDID := url.PathEscape(did)
		out = append(out, map[string]any{
			"@id":       fmt.Sprintf("%s/catalog/beamlines/%s/datasets/%s", cfg.Node.BaseURL, bl.ID, encodedDID),
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
