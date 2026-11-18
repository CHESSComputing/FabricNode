package tpf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SPARQLResult matches standard SPARQL JSON results format
type SPARQLResult struct {
	Head struct {
		Vars []string `json:"vars"`
	} `json:"head"`
	Results struct {
		Bindings []map[string]map[string]string `json:"bindings"`
	} `json:"results"`
}

// fetchTriplesFromStore translates TPF parameters into SPARQL SELECT and COUNT queries
func fetchTriplesFromStore(subject, predicate, object string, limit, offset int) (int64, []TPFTriple, error) {
	// 1. Format SPARQL graph pattern variables or values
	sPattern := formatTerm(subject, true)
	pPattern := formatTerm(predicate, true)
	oPattern := formatTerm(object, false)

	whereClause := fmt.Sprintf("%s %s %s .", sPattern, pPattern, oPattern)

	// 2. Execute COUNT query for metadata (hydra:totalItems)
	countSPARQL := fmt.Sprintf("SELECT (COUNT(*) as ?count) WHERE { %s }", whereClause)
	totalItems := executeSPARQLCount(countSPARQL)

	// 3. Execute SELECT query for paginated triples (hydra:member)
	dataSPARQL := fmt.Sprintf("SELECT ?s ?p ?o WHERE { %s } LIMIT %d OFFSET %d", whereClause, limit, offset)
	triples := executeSPARQLSelect(dataSPARQL)

	return totalItems, triples, nil
}

// Helper to format subjects, predicates, and objects into valid SPARQL terms
func formatTerm(val string, isIRI bool) string {
	if val == "" {
		if isIRI {
			return "?s" // Fallback variable name depending on position, handled generically below
		}
		return "?o"
	}
	// If it's an explicit IRI or variable, angle bracket it or keep it as-is
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") || strings.HasPrefix(val, "urn:") {
		return "<" + val + ">"
	}
	// If it's a literal or already formatted
	if strings.HasPrefix(val, "\"") {
		return val
	}
	// Default to literal string representation
	return fmt.Sprintf("\"%s\"", val)
}

// executeSPARQLCount sends a query to your internal /sparql endpoint and parses the count
func executeSPARQLCount(query string) int64 {
	bindings := queryInternalSPARQL(query)
	if len(bindings) == 0 {
		return 0
	}
	// Extract count value from bindings result
	if countVal, ok := bindings[0]["count"]; ok {
		var c int64
		fmt.Sscanf(countVal["value"], "%d", &c)
		return c
	}
	return 0
}

// executeSPARQLSelect executes a SELECT query and maps results to TPFTriple structs
func executeSPARQLSelect(query string) []TPFTriple {
	bindings := queryInternalSPARQL(query)
	var triples []TPFTriple

	for _, b := range bindings {
		s := b["s"]
		p := b["p"]
		o := b["o"]

		if s != nil && p != nil && o != nil {
			triples = append(triples, TPFTriple{
				Subject:   s["value"],
				Predicate: p["value"],
				Object:    o["value"],
			})
		}
	}
	return triples
}

// queryInternalSPARQL handles HTTP POST requests to your internal SPARQL endpoint
func queryInternalSPARQL(query string) []map[string]map[string]string {
	endpoint := "http://localhost:8782/sparql" // Adjust to your internal data service route/port

	data := url.Values{}
	data.Set("query", query)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var result SPARQLResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	return result.Results.Bindings
}
