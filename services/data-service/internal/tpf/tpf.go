package tpf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// TPFTriple represents a single RDF triple in the fragment response
type TPFTriple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// HydraView handles pagination metadata and navigation links
type HydraView struct {
	ID           string `json:"@id"`
	Type         string `json:"@type"`
	FirstPage    string `json:"hydra:firstPage,omitempty"`
	NextPage     string `json:"hydra:nextPage,omitempty"`
	PreviousPage string `json:"hydra:previousPage,omitempty"`
	ItemsPerPage int    `json:"hydra:itemsPerPage"`
	PageNumber   int    `json:"hydra:page,omitempty"`
}

// TPFResponse represents the Hydra-compliant JSON-LD response
type TPFResponse struct {
	Context      map[string]interface{} `json:"@context"`
	ID           string                 `json:"@id"`
	Type         []string               `json:"@type"`
	TotalItems   int64                  `json:"hydra:totalItems"`
	ItemsPerPage int                    `json:"hydra:itemsPerPage"`
	View         HydraView              `json:"hydra:view"`
	Members      []TPFTriple            `json:"hydra:member"`
}

// HandleTPF serves the Triple Pattern Fragments endpoint
func HandleTPF(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for OKN federated cross-domain queries
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/ld+json")

	queryVals := r.URL.Query()
	subject := queryVals.Get("subject")
	predicate := queryVals.Get("predicate")
	object := queryVals.Get("object")

	// Parse pagination parameters (default page 1, 100 items per page)
	page := 1
	if pStr := queryVals.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	limit := 100
	offset := (page - 1) * limit

	// 1. Execute backend query to fetch total count and paginated triples
	// (In your FabricNode architecture, you can wire this directly into your triple store or internal SPARQL engine)
	totalItems, triples, err := fetchTriplesFromStore(subject, predicate, object, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to evaluate triple pattern: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Build Hydra-compliant pagination links
	baseURL := "http://" + r.Host + r.URL.Path
	currentURL := fmt.Sprintf("%s?%s", baseURL, r.URL.RawQuery)
	
	viewID := currentURL
	if !hasParam(r.URL.RawQuery, "page") {
		if r.URL.RawQuery == "" {
			viewID = baseURL + "?page=1"
		} else {
			viewID = baseURL + "?" + r.URL.RawQuery + "&page=1"
		}
	}

	var nextPage, prevPage string
	if int64(offset+limit) < totalItems {
		nextPage = updateQueryParam(baseURL, r.URL.Query(), "page", strconv.Itoa(page+1))
	}
	if page > 1 {
		prevPage = updateQueryParam(baseURL, r.URL.Query(), "page", strconv.Itoa(page-1))
	}
	firstPage := updateQueryParam(baseURL, r.URL.Query(), "page", "1")

	// 3. Construct Hydra response structure
	response := TPFResponse{
		Context: map[string]interface{}{
			"hydra": "http://www.w3.org/ns/hydra/core#",
			"void":  "http://rdfs.org/ns/void#",
			"owl":   "http://www.w3.org/2002/07/owl#",
			"subject": map[string]string{
				"@id":   "hydra:subject",
				"@type": "@id",
			},
			"predicate": map[string]string{
				"@id":   "hydra:predicate",
				"@type": "@id",
			},
			"object": map[string]string{
				"@id":   "hydra:object",
				"@type": "@id",
			},
		},
		ID:           viewID,
		Type:         []string{"hydra:PagedCollection", "void:Dataset"},
		TotalItems:   totalItems,
		ItemsPerPage: limit,
		View: HydraView{
			ID:           viewID,
			Type:         "hydra:PartialCollectionView",
			FirstPage:    firstPage,
			NextPage:     nextPage,
			PreviousPage: prevPage,
			ItemsPerPage: limit,
			PageNumber:   page,
		},
		Members: triples,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Helper utility to safely update query parameters for links
func updateQueryParam(baseURL string, params url.Values, key, val string) string {
	p := cloneParams(params)
	p.Set(key, val)
	return baseURL + "?" + p.Encode()
}

func cloneParams(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}

func hasParam(rawQuery, key string) bool {
	q, _ := url.ParseQuery(rawQuery)
	_, exists := q[key]
	return exists
}

// Stub for your actual backend data-service triple store lookup
/*
func fetchTriplesFromStore(s, p, o string, limit, offset int) (int64, []TPFTriple, error) {
	// Implement your internal triple-store scan or SPARQL query translation here:
	// Example SPARQL COUNT: SELECT (COUNT(*) as ?count) WHERE { ... }
	// Example SPARQL SELECT: SELECT ?s ?p ?o WHERE { ... } LIMIT X OFFSET Y
	return 0, []TPFTriple{}, nil
}
*/
