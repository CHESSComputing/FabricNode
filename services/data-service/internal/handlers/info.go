package handlers

import (
	"fmt"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// Health returns service liveness with graph count.
// GET /health
func Health(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"data-service","graphs":%d}`, len(db.Graphs()))
	}
}

// Index returns the service directory.
// GET /
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
    "health":                   "GET /health"
  },
  "notes": [
    "Beamline IDs: lower-case letters + digits, e.g. id1 id3a fast qm2",
    "Dataset DIDs are URL-encoded in path: /beamline=id3a/btr=val/cycle=2024-3/sample_name=bla"
  ]
}`)
	}
}
