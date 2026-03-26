package handlers

import (
	"fmt"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func Health(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"catalog-service","nodeId":%q}`, cfg.NodeID)
	}
}

func Index(cfg void.NodeConfig) http.HandlerFunc {
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
