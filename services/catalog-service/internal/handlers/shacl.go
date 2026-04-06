package handlers

import (
	"fmt"
	"log"
	"net/http"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func SHACL(cfg *fabricconfig.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		AddCacheHeaders(w, 3600)

		body, err := void.SHACLTurtle(cfg)
		if err != nil {
			log.Printf("SHACL render error: %v", err)
			http.Error(w, "failed to render SHACL shapes", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, body)
	}
}
