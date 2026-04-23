package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/foxden"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
	"github.com/go-chi/chi/v5"
)

// FoxdenConfig holds the runtime dependencies for FOXDEN-backed handlers.
type FoxdenConfig struct {
	Client   *foxden.Client
	Store    store.GraphStore
	IRIBase  string            // node-wide IRI prefix from Config.Node.IRIBase, e.g. "http://chess.cornell.edu/"
	FieldMaps *foxden.FieldMaps // schema-derived field maps; nil → static maps only
}

// FoxdenDatasets lists FOXDEN metadata records for a beamline and returns
// them as JSON.  Optionally filters by ?cycle=<value>.
//
//	GET /beamlines/{beamline}/foxden/datasets
//	GET /beamlines/{beamline}/foxden/datasets?cycle=2026-1
func FoxdenDatasets(cfg FoxdenConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beamline := chi.URLParam(r, "beamline")
		cycle := r.URL.Query().Get("cycle")
		limitStr := r.URL.Query().Get("limit")

		limit := 100
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
				limit = n
			}
		}

		var (
			resp *foxden.ServiceResponse
			err  error
		)
		if cycle != "" {
			resp, err = cfg.Client.QueryByBeamlineAndCycle(beamline, cycle, limit)
		} else {
			resp, err = cfg.Client.QueryByBeamline(beamline, limit)
		}
		if err != nil {
			log.Printf("foxden: QueryByBeamline(%s): %v", beamline, err)
			http.Error(w, "foxden query failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if resp.Status != "ok" {
			http.Error(w, "foxden error: "+resp.Error, http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.Results)
	}
}

// FoxdenDataset returns the FOXDEN metadata record for one dataset DID.
//
//	GET /beamlines/{beamline}/datasets/{did}/foxden
func FoxdenDataset(cfg FoxdenConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		didRaw := chi.URLParam(r, "did")
		did, err := url.PathUnescape(didRaw)
		if err != nil {
			http.Error(w, "malformed DID in path", http.StatusBadRequest)
			return
		}

		resp, err := cfg.Client.QueryByDID(did)
		if err != nil {
			log.Printf("foxden: QueryByDID(%s): %v", did, err)
			http.Error(w, "foxden query failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if resp.Status != "ok" {
			http.Error(w, "foxden error: "+resp.Error, http.StatusBadGateway)
			return
		}
		if resp.Results.NRecords == 0 {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.Results.Records[0])
	}
}

// FoxdenIngest fetches FOXDEN metadata for a dataset DID and converts it to
// RDF triples stored in the named graph for that DID.  Safe to call multiple
// times — triples are appended (deduplication is a future concern).
//
//	POST /beamlines/{beamline}/datasets/{did}/foxden/ingest
func FoxdenIngest(cfg FoxdenConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		didRaw := chi.URLParam(r, "did")
		did, err := url.PathUnescape(didRaw)
		if err != nil {
			http.Error(w, "malformed DID in path", http.StatusBadRequest)
			return
		}

		// 1. Fetch from FOXDEN
		resp, err := cfg.Client.QueryByDID(did)
		if err != nil {
			log.Printf("foxden: ingest QueryByDID(%s): %v", did, err)
			http.Error(w, "foxden query failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if resp.Status != "ok" {
			http.Error(w, "foxden error: "+resp.Error, http.StatusBadGateway)
			return
		}
		if resp.Results.NRecords == 0 {
			http.Error(w, "no FOXDEN record found for DID: "+did, http.StatusNotFound)
			return
		}

		rec := foxden.Record(resp.Results.Records[0])
		graphIRI := foxden.GraphIRIFromDID(did, cfg.IRIBase)

		// 2. Convert to RDF triples
		triples, err := foxden.RecordToTriples(rec, graphIRI, cfg.IRIBase, cfg.FieldMaps)
		if err != nil {
			http.Error(w, "rdf conversion failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 3. Insert into store
		for _, t := range triples {
			cfg.Store.Insert(t)
		}

		log.Printf("foxden: ingested %d triples for DID %s → graph %s", len(triples), did, graphIRI)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"ingested": len(triples),
			"did":      did,
			"graphIRI": graphIRI,
		})
	}
}
