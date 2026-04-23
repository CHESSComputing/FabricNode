// Package sparql implements a lightweight SPARQL-compatible HTTP endpoint
// with beamline/dataset-scoped routing.
//
// Endpoint hierarchy:
//
//	GET /sparql                                   — global query
//	GET /beamlines/{beamline}/sparql              — beamline-scoped query
//	GET /beamlines/{beamline}/datasets/{did}/sparql — dataset-scoped query
//
// Query parameters (all routes):
//
//	s, p, o, g   subject/predicate/object/graph filters
//	describe     DESCRIBE a resource by IRI
//	search       keyword search across object literals
package sparql

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
	"github.com/go-chi/chi/v5"
)

// Handler provides SPARQL HTTP handlers backed by a Store.
type Handler struct {
	store store.GraphStore
}

// New creates a Handler.
func New(s store.GraphStore) *Handler { return &Handler{store: s} }

// ServeHTTP handles global GET/POST /sparql.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	results := h.resolve(q, "")
	writeResults(w, results)
}

// BeamlineHandler handles GET /beamlines/{beamline}/sparql.
func (h *Handler) BeamlineHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bl := model.BeamlineID(chi.URLParam(r, "beamline"))
		if !bl.Valid() {
			http.Error(w, "invalid beamline id", http.StatusBadRequest)
			return
		}
		q := r.URL.Query()
		triples, err := h.store.QueryBeamline(bl, q.Get("s"), q.Get("p"), q.Get("o"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeResults(w, triples)
	}
}

// DatasetHandler handles GET /beamlines/{beamline}/datasets/{did}/sparql.
func (h *Handler) DatasetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref, ok := datasetRefFromPath(w, r)
		if !ok {
			return
		}
		q := r.URL.Query()
		triples, err := h.store.QueryDataset(ref, q.Get("s"), q.Get("p"), q.Get("o"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeResults(w, triples)
	}
}

// GraphsHandler lists all named graphs.
func GraphsHandler(s store.GraphStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"graphs": s.Graphs(),
		})
	}
}

// BeamlineGraphsHandler lists named graphs for one beamline.
func BeamlineGraphsHandler(s store.GraphStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bl := model.BeamlineID(chi.URLParam(r, "beamline"))
		if !bl.Valid() {
			http.Error(w, "invalid beamline id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"beamline": bl,
			"graphs":   s.DatasetsForBeamline(bl),
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// resolve dispatches between describe/search/filter based on query params.
// graphOverride locks the graph filter when coming from a scoped route.
func (h *Handler) resolve(q url.Values, graphOverride string) []store.Triple {
	describe := q.Get("describe")
	search := q.Get("search")
	graph := graphOverride
	if graph == "" {
		graph = q.Get("g")
	}

	var results []store.Triple
	switch {
	case describe != "":
		results = h.store.Describe(describe)
	case search != "":
		results = h.store.KeywordSearch(search)
	default:
		results = h.store.Query(q.Get("s"), q.Get("p"), q.Get("o"), graph)
	}
	const limit = 100
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// datasetRefFromPath extracts and validates beamline + DID from the URL.
// The {did} wildcard is URL-encoded because DIDs contain slashes.
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

// writeResults serialises triples as SPARQL JSON Results.
func writeResults(w http.ResponseWriter, results []store.Triple) {
	w.Header().Set("Content-Type", "application/sparql-results+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	type binding map[string]any
	type row struct {
		S binding `json:"s"`
		P binding `json:"p"`
		O binding `json:"o"`
		G binding `json:"g,omitempty"`
	}
	type response struct {
		Head    map[string][]string `json:"head"`
		Results struct {
			Bindings []row `json:"bindings"`
		} `json:"results"`
	}

	resp := response{Head: map[string][]string{"vars": {"s", "p", "o", "g"}}}
	for _, t := range results {
		r := row{
			S: binding{"type": "uri", "value": t.Subject},
			P: binding{"type": "uri", "value": t.Predicate},
			G: binding{"type": "uri", "value": t.Graph},
		}
		if t.ObjectType == "uri" {
			r.O = binding{"type": "uri", "value": t.Object}
		} else {
			if t.Datatype != "" {
				r.O = binding{"type": "literal", "value": t.Object, "datatype": t.Datatype}
			} else {
				r.O = binding{"type": "literal", "value": t.Object}
			}
		}
		resp.Results.Bindings = append(resp.Results.Bindings, r)
	}
	json.NewEncoder(w).Encode(resp)
}
