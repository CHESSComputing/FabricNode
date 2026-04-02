package handlers

import (
	"fmt"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func Health(db store.GraphStore) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"data-service","graphs":%d}`, len(db.Graphs()))
	}
}

func Index() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "service": "data-service",
  "endpoints": {
    "sparql (global)":          "GET|POST /sparql",
    "sparql (beamline)":        "GET /beamlines/{beamline}/sparql",
    "sparql (dataset)":         "GET /beamlines/{beamline}/datasets/{did}/sparql",
    "graphs (global)":          "GET /graphs",
    "graphs (beamline)":        "GET /beamlines/{beamline}/graphs",
    "triples (insert)":         "POST /beamlines/{beamline}/datasets/{did}/triples",
    "triples (validate)":       "POST /beamlines/{beamline}/datasets/{did}/validate",
    "foxden (list datasets)":   "GET /beamlines/{beamline}/foxden/datasets[?cycle=<val>]",
    "foxden (one dataset)":     "GET /beamlines/{beamline}/datasets/{did}/foxden",
    "foxden (ingest to RDF)":   "POST /beamlines/{beamline}/datasets/{did}/foxden/ingest",
    "health":                   "GET /health"
  },
  "notes": [
    "Beamline IDs: lower-case letters + digits, e.g. id1 id3a fast 3a",
    "Dataset DIDs are URL-encoded in path: %2Fbeamline%3D3a%2Fbtr%3D..."
  ]
}`)
	}
}
