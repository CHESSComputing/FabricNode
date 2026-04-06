package handlers

import (
	"fmt"
	"log"
	"net/http"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func Profile(cfg *fabricconfig.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		AddCacheHeaders(w, 3600)

		body, err := void.ProfileTurtle(cfg)
		if err != nil {
			log.Printf("Profile render error: %v", err)
			http.Error(w, "failed to render profile", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, body)
	}
}
