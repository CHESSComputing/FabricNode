package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/shacl"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/sparql"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

func main() {
	db := store.New()
	sparqlHandler := sparql.New(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// ── SPARQL endpoint (GET + POST) ─────────────────────────────────────────
	r.Get("/sparql", sparqlHandler.ServeHTTP)
	r.Post("/sparql", sparqlHandler.ServeHTTP)

	// ── Named graphs listing ─────────────────────────────────────────────────
	r.Get("/graphs", sparql.GraphsHandler(db))

	// ── Write endpoint (SHACL-validated) ────────────────────────────────────
	// POST /triples — insert triples into the store after SHACL validation.
	// Body: JSON array of Triple objects.
	r.Post("/triples", func(w http.ResponseWriter, req *http.Request) {
		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}
		// SHACL validation gate (fabric:ObservationShape)
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
	})

	// ── SHACL validation only (dry-run) ─────────────────────────────────────
	r.Post("/validate", func(w http.ResponseWriter, req *http.Request) {
		var triples []store.Triple
		if err := json.NewDecoder(req.Body).Decode(&triples); err != nil {
			http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
			return
		}
		result := shacl.ValidateObservation(triples)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// ── Health ───────────────────────────────────────────────────────────────
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"data-service","graphs":%d}`, len(db.Graphs()))
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
  "service": "data-service",
  "endpoints": {
    "sparql":   "/sparql?s=&p=&o=&g= | ?describe=<iri> | ?search=<text>",
    "graphs":   "/graphs",
    "triples":  "POST /triples (SHACL-validated insert)",
    "validate": "POST /validate (dry-run SHACL check)",
    "health":   "/health"
  }
}`)
	})

	port := getEnv("PORT", "8082")
	log.Printf("data-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
