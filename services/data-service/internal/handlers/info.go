package handlers

import (
	"fmt"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func Health(db *store.Store) http.HandlerFunc {
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
    "sparql":   "/sparql?s=&p=&o=&g= | ?describe=<iri> | ?search=<text>",
    "graphs":   "/graphs",
    "triples":  "POST /triples (SHACL-validated insert)",
    "validate": "POST /validate (dry-run SHACL check)",
    "health":   "/health"
  }
}`)
	}
}
