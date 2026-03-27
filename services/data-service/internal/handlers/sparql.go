package handlers

import (
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/sparql"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// SPARQL returns a handler for the global GET/POST /sparql endpoint.
func SPARQL(db *store.Store) http.HandlerFunc {
	h := sparql.New(db)
	return h.ServeHTTP
}

// BeamlineSPARQL returns a handler for GET /beamlines/{beamline}/sparql,
// scoping queries to the named graphs owned by that beamline.
func BeamlineSPARQL(db *store.Store) http.HandlerFunc {
	return sparql.New(db).BeamlineHandler()
}

// DatasetSPARQL returns a handler for GET /beamlines/{beamline}/datasets/{did}/sparql,
// scoping queries to the single named graph for that dataset DID.
func DatasetSPARQL(db *store.Store) http.HandlerFunc {
	return sparql.New(db).DatasetHandler()
}

// Graphs returns a handler that lists all named graphs in the store.
func Graphs(db *store.Store) http.HandlerFunc {
	return sparql.GraphsHandler(db)
}

// BeamlineGraphs returns a handler that lists named graphs for one beamline.
func BeamlineGraphs(db *store.Store) http.HandlerFunc {
	return sparql.BeamlineGraphsHandler(db)
}
