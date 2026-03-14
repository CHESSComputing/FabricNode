// Package rdf provides content negotiation helpers for RDF serialisation formats.
// The catalog service supports Turtle, JSON-LD, and N-Triples as required by
// the Knowledge Fabric conformance specification.
package rdf

import (
	"net/http"
	"strings"
)

// Format represents an RDF serialisation format.
type Format string

const (
	FormatTurtle   Format = "text/turtle"
	FormatJSONLD   Format = "application/ld+json"
	FormatNTriples Format = "application/n-triples"
	FormatDefault  Format = FormatTurtle
)

// Negotiate inspects the Accept header and returns the best matching RDF format.
// Falls back to Turtle if no match is found.
func Negotiate(r *http.Request) Format {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return FormatDefault
	}
	// Simple priority: first match wins (no quality value parsing needed for PoC).
	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch mt {
		case "application/ld+json":
			return FormatJSONLD
		case "application/n-triples":
			return FormatNTriples
		case "text/turtle", "text/*", "*/*":
			return FormatTurtle
		}
	}
	return FormatDefault
}
