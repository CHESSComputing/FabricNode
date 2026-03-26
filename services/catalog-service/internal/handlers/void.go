package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/rdf"
	"github.com/CHESSComputing/FabricNode/services/catalog-service/internal/void"
)

func VoID(cfg void.NodeConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		format := rdf.Negotiate(req)
		w.Header().Set("Content-Type", string(format))
		w.Header().Set("Link", `</.well-known/void>; rel="self"`)
		AddCacheHeaders(w, 300)

		switch format {
		case rdf.FormatJSONLD:
			body, err := void.VoIDJSONLD(cfg)
			if err != nil {
				log.Printf("VoID JSON-LD render error: %v", err)
				http.Error(w, "failed to render VoID JSON-LD", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, body)
		default:
			body, err := void.VoIDTurtle(cfg)
			if err != nil {
				log.Printf("VoID Turtle render error: %v", err)
				http.Error(w, "failed to render VoID Turtle", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, body)
		}
	}
}
