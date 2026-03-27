package handlers

import (
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/sparql"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func SPARQL(db *store.Store) http.HandlerFunc {
	h := sparql.New(db)
	return h.ServeHTTP
}

func Graphs(db *store.Store) http.HandlerFunc {
	return sparql.GraphsHandler(db)
}
