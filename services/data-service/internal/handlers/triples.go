package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/shacl"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// Triples handles SHACL-validated triple insertion.
// POST /triples — body: JSON array of Triple objects.
func Triples(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		result := shacl.ValidateObservation(triples)
		if !result.Conforms {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"conforms": false,
				"errors":   result.Errors,
			})
			return
		}

		for _, t := range triples {
			db.Insert(t)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"inserted": len(triples),
			"conforms": true,
		})
	}
}

// Validate handles dry-run SHACL validation without inserting triples.
// POST /validate — body: JSON array of Triple objects.
func Validate(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		result := shacl.ValidateObservation(triples)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
