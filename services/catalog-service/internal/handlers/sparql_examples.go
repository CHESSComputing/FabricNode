package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func SPARQLExamples(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		AddCacheHeaders(w, 3600)

		body, err := void.SPARQLExamplesTurtle(cfg)
		if err != nil {
			log.Printf("SPARQL examples render error: %v", err)
			http.Error(w, "failed to render SPARQL examples", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, body)
	}
}
