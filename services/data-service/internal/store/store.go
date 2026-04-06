// Package store defines the GraphStore interface for RDF named-graph storage
// and provides two implementations: MemoryStore (in-process, no persistence)
// and OxigraphStore (backed by an Oxigraph SPARQL server over HTTP).
//
// Callers depend only on the GraphStore interface.  The concrete type is
// chosen at startup via config and wired in main.go.
package store

import "github.com/CHESSComputing/FabricNode/pkg/model"

// Triple represents one RDF statement.
type Triple struct {
	Subject    string `json:"subject"`
	Predicate  string `json:"predicate"`
	Object     string `json:"object"`
	Graph      string `json:"graph,omitempty"`
	ObjectType string `json:"objectType,omitempty"` // "uri" | "literal" | "bnode"
	Datatype   string `json:"datatype,omitempty"`
	Lang       string `json:"lang,omitempty"`
}

// GraphStore is the common interface for graph database back-ends.
//
// Every method maps 1-to-1 to a capability used somewhere in the service
// (handlers, SPARQL layer, SHACL validator, seed data).  New back-ends
// implement this interface; callers never import a concrete type.
type GraphStore interface {
	// ── Beamline / Dataset scoped writes ────────────────────────────────────

	// InsertForDataset inserts triples into the named graph derived from ref.
	// Each triple's Graph field is overwritten with the canonical graph IRI.
	InsertForDataset(ref model.DatasetRef, triples []Triple) error

	// ── Beamline / Dataset scoped reads ─────────────────────────────────────

	// QueryDataset returns triples from the named graph for ref.
	// Empty subject/predicate/object act as wildcards.
	QueryDataset(ref model.DatasetRef, subject, predicate, object string) ([]Triple, error)

	// QueryBeamline returns triples from every named graph that belongs to bl.
	// Empty subject/predicate/object act as wildcards.
	QueryBeamline(bl model.BeamlineID, subject, predicate, object string) ([]Triple, error)

	// DatasetsForBeamline returns the graph IRIs of every dataset that
	// belongs to bl.
	DatasetsForBeamline(bl model.BeamlineID) []string

	// ── Generic (non-scoped) API ─────────────────────────────────────────────

	// Graphs returns the IRIs of all named graphs present in the store.
	Graphs() []string

	// Query filters triples across all graphs (or within one graph).
	// Empty string is a wildcard for every positional argument.
	Query(subject, predicate, object, graph string) []Triple

	// Insert adds a triple to its named graph (as given by t.Graph).
	Insert(t Triple) Triple

	// Describe returns all triples where subject or object equals iri.
	Describe(iri string) []Triple

	// KeywordSearch does case-insensitive substring search across object literals.
	KeywordSearch(keyword string) []Triple
}
