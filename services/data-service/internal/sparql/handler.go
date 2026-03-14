// Package sparql implements a lightweight SPARQL-compatible HTTP endpoint.
// It supports SPARQL 1.1 SELECT-like queries via three mechanisms:
//
//  1. Structured query via query parameters (subject, predicate, object, graph)
//  2. Named-graph DESCRIBE via ?describe=<iri>
//  3. Keyword search via ?search=<text>
//
// In production, proxy to Oxigraph (dereferences /.../sparql to a real
// SPARQL 1.2 triplestore). The handler interface is unchanged.
package sparql

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// Handler provides the /sparql HTTP handler backed by a Store.
type Handler struct {
	store *store.Store
}

// New creates a Handler.
func New(s *store.Store) *Handler {
	return &Handler{store: s}
}

// ServeHTTP handles GET and POST /sparql requests.
//
// Query parameters:
//   - s         subject filter (IRI)
//   - p         predicate filter (IRI)
//   - o         object filter (literal or IRI)
//   - g         graph filter (IRI)
//   - describe  DESCRIBE a resource by IRI
//   - search    keyword search across object literals
//   - limit     max results (default: 100)
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	describe := q.Get("describe")
	search := q.Get("search")

	var results []store.Triple

	switch {
	case describe != "":
		results = h.store.Describe(describe)
	case search != "":
		results = h.store.KeywordSearch(search)
	default:
		results = h.store.Query(q.Get("s"), q.Get("p"), q.Get("o"), q.Get("g"))
	}

	// Apply limit
	limit := 100
	if len(results) > limit {
		results = results[:limit]
	}

	w.Header().Set("Content-Type", "application/sparql-results+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// SPARQL JSON Results format (subset)
	type binding map[string]interface{}
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

	resp := response{
		Head: map[string][]string{"vars": {"s", "p", "o", "g"}},
	}

	for _, t := range results {
		r := row{
			S: binding{"type": "uri", "value": t.Subject},
			P: binding{"type": "uri", "value": t.Predicate},
			G: binding{"type": "uri", "value": t.Graph},
		}
		if t.ObjectType == "uri" {
			r.O = binding{"type": "uri", "value": t.Object}
		} else {
			r.O = binding{"type": "literal", "value": t.Object}
		}
		resp.Results.Bindings = append(resp.Results.Bindings, r)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// GraphsHandler lists named graphs.
func GraphsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"graphs": s.Graphs(),
		})
	}
}
