package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/shacl"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
	"github.com/go-chi/chi/v5"
)

// Triples handles SHACL-validated triple insertion scoped to a dataset.
//
//	POST /beamlines/{beamline}/datasets/{did}/triples
//
// The {did} path parameter is URL-encoded because dataset DIDs contain slashes,
// e.g. %2Fbeamline%3Did3a%2Fbtr%3Dval123%2Fcycle%3D2024-3%2Fsample_name%3Dbla
//
// Body: JSON array of store.Triple objects.  The Graph field of each triple is
// overwritten by the canonical named-graph IRI derived from the DID, so callers
// do not need to set it.
func Triples(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ref, ok := datasetRefFromPath(w, req)
		if !ok {
			return
		}

		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		// SHACL validation scoped to beamline (checks sensor ownership)
		result := shacl.ValidateForDataset(ref, triples)
		if !result.Conforms {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"conforms": false,
				"errors":   result.Errors,
			})
			return
		}

		if err := db.InsertForDataset(ref, triples); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"inserted":  len(triples),
			"conforms":  true,
			"beamline":  ref.Beamline,
			"did":       ref.DID,
			"graphIRI":  ref.GraphIRI(),
		})
	}
}

// Validate handles dry-run SHACL validation without inserting triples.
//
//	POST /beamlines/{beamline}/datasets/{did}/validate
func Validate(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ref, ok := datasetRefFromPath(w, req)
		if !ok {
			return
		}

		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}

		result := shacl.ValidateForDataset(ref, triples)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Shared path helper
// ──────────────────────────────────────────────────────────────────────────────

// datasetRefFromPath extracts and validates the {beamline} and {did} URL
// parameters.  The DID is URL-decoded before parsing.
func datasetRefFromPath(w http.ResponseWriter, r *http.Request) (model.DatasetRef, bool) {
	bl := model.BeamlineID(chi.URLParam(r, "beamline"))
	didRaw := chi.URLParam(r, "did")
	didDecoded, err := url.PathUnescape(didRaw)
	if err != nil {
		http.Error(w, "malformed DID in path", http.StatusBadRequest)
		return model.DatasetRef{}, false
	}
	ref := model.DatasetRef{
		Beamline: bl,
		DID:      model.DatasetDID(didDecoded),
	}
	if err := ref.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return model.DatasetRef{}, false
	}
	return ref, true
}
