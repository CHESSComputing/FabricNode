package handlers

import (
	"fmt"
	"net/http"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
)

func Health(cfg *fabricconfig.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"catalog-service","nodeId":%q}`, cfg.Node.ID)
	}
}

func Index(cfg *fabricconfig.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "service": "catalog-service",
  "description": "Knowledge Fabric self-description endpoints for CHESS node",
  "endpoints": {
    "void":           "/.well-known/void",
    "profile":        "/.well-known/profile",
    "shacl":          "/.well-known/shacl",
    "sparqlExamples": "/.well-known/sparql-examples",
    "health":         "/health"
  }
}`)
	}
}
