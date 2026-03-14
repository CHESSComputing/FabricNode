// Package store provides an in-memory RDF triple store with named graph support.
// For production, replace with Oxigraph (SPARQL 1.2) or another triplestore;
// the interface is designed so the swap is a one-line change in main.go.
package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Triple represents one RDF statement (subject, predicate, object, graph).
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Graph     string `json:"graph,omitempty"`
	// ObjectType hints for serialisation: "uri", "literal", "bnode"
	ObjectType string `json:"objectType,omitempty"`
	// Datatype is the xsd datatype IRI for typed literals.
	Datatype string `json:"datatype,omitempty"`
	// Lang is the language tag for plain literals.
	Lang string `json:"lang,omitempty"`
}

// NamedGraph holds triples belonging to one named graph.
type NamedGraph struct {
	IRI     string
	Triples []Triple
}

// Store is the in-memory triple store.
type Store struct {
	mu     sync.RWMutex
	graphs map[string]*NamedGraph // key: graph IRI
}

// New creates a Store pre-seeded with representative CHESS beamline data.
func New() *Store {
	s := &Store{graphs: make(map[string]*NamedGraph)}
	s.seed()
	return s
}

// ──────────────────────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────────────────────

// Graphs returns the IRIs of all named graphs.
func (s *Store) Graphs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.graphs))
	for k := range s.graphs {
		out = append(out, k)
	}
	return out
}

// Query filters triples across all graphs (or a specific graph).
// Passing "" for subject/predicate/object acts as a wildcard.
// Passing "" for graph queries all graphs.
func (s *Store) Query(subject, predicate, object, graph string) []Triple {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []Triple
	for graphIRI, ng := range s.graphs {
		if graph != "" && graph != graphIRI {
			continue
		}
		for _, t := range ng.Triples {
			if matches(t.Subject, subject) &&
				matches(t.Predicate, predicate) &&
				matches(t.Object, object) {
				results = append(results, t)
			}
		}
	}
	return results
}

// Insert adds a triple to the given named graph, creating it if needed.
// Returns the stored triple with its graph IRI populated.
func (s *Store) Insert(t Triple) Triple {
	s.mu.Lock()
	defer s.mu.Unlock()
	ng, ok := s.graphs[t.Graph]
	if !ok {
		ng = &NamedGraph{IRI: t.Graph}
		s.graphs[t.Graph] = ng
	}
	ng.Triples = append(ng.Triples, t)
	return t
}

// Describe returns all triples where subject or object equals the given IRI.
func (s *Store) Describe(iri string) []Triple {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Triple
	for _, ng := range s.graphs {
		for _, t := range ng.Triples {
			if t.Subject == iri || t.Object == iri {
				out = append(out, t)
			}
		}
	}
	return out
}

// KeywordSearch does case-insensitive substring search across all object literals.
func (s *Store) KeywordSearch(keyword string) []Triple {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lower := strings.ToLower(keyword)
	var out []Triple
	for _, ng := range s.graphs {
		for _, t := range ng.Triples {
			if t.ObjectType == "literal" && strings.Contains(strings.ToLower(t.Object), lower) {
				out = append(out, t)
			}
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func matches(value, pattern string) bool {
	return pattern == "" || value == pattern
}

// uriT and litS are helper constructors kept for tests; var blank keeps compiler happy.
func uriT(v string) Triple     { return Triple{ObjectType: "uri", Object: v} }
func litS(v, dt string) string { if dt != "" { return v + "^^" + dt }; return v }

var _, _ = uriT, litS

// ──────────────────────────────────────────────────────────────────────────────
// Seed data — representative CHESS beamline observations
// ──────────────────────────────────────────────────────────────────────────────

const (
	nsRDF   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS  = "http://www.w3.org/2000/01/rdf-schema#"
	nsSOSA  = "http://www.w3.org/ns/sosa/"
	nsSIO   = "http://semanticscience.org/resource/"
	nsCHESS = "http://chess.cornell.edu/"
	nsXSD   = "http://www.w3.org/2001/XMLSchema#"
	nsDCT   = "http://purl.org/dc/terms/"
)

func (s *Store) seed() {
	// ── Named graphs ──────────────────────────────────────────────────────────
	const (
		graphBeamlines = nsCHESS + "graph/beamlines"
		graphObs       = nsCHESS + "graph/observations"
		graphSensors   = nsCHESS + "graph/sensors"
	)

	// ── Beamline descriptors ─────────────────────────────────────────────────
	beamlines := []struct{ id, label, blType string }{
		{"id1", "Beamline ID1 — X-ray Diffraction", "x-ray-diffraction"},
		{"id3", "Beamline ID3 — Protein Crystallography", "protein-crystallography"},
		{"fast", "Beamline FAST — Time-Resolved Scattering", "time-resolved-scattering"},
	}
	for _, bl := range beamlines {
		subj := nsCHESS + "beamline/" + bl.id
		g := graphBeamlines
		s.addTriple(subj, nsRDF+"type", nsCHESS+"Beamline", "uri", g)
		s.addTriple(subj, nsRDFS+"label", bl.label, "literal", g)
		s.addTriple(subj, nsCHESS+"ns#beamlineType", bl.blType, "literal", g)
		s.addTriple(subj, nsDCT+"description",
			fmt.Sprintf("CHESS beamline %s (%s)", strings.ToUpper(bl.id), bl.blType), "literal", g)
	}

	// ── Sensors ───────────────────────────────────────────────────────────────
	sensors := []struct{ id, label, blID string }{
		{"id1-detector-01", "ID1 Primary Area Detector", "id1"},
		{"id3-detector-01", "ID3 CCD Crystallography Detector", "id3"},
		{"fast-detector-01", "FAST High-Speed Scintillator Detector", "fast"},
	}
	for _, sen := range sensors {
		subj := nsCHESS + "sensor/" + sen.id
		g := graphSensors
		s.addTriple(subj, nsRDF+"type", nsSOSA+"Sensor", "uri", g)
		s.addTriple(subj, nsRDFS+"label", sen.label, "literal", g)
		s.addTriple(subj, nsSOSA+"isHostedBy", nsCHESS+"beamline/"+sen.blID, "uri", g)
	}

	// ── Observations ─────────────────────────────────────────────────────────
	obsData := []struct {
		sensorID  string
		propLabel string
		result    string
		unit      string
		runNum    int
		daysAgo   int
	}{
		{"id1-detector-01", "lattice-parameter-a", "5.431", "angstrom", 1024, 1},
		{"id1-detector-01", "peak-intensity", "14253.7", "counts", 1024, 1},
		{"id1-detector-01", "peak-intensity", "13997.2", "counts", 1025, 0},
		{"id3-detector-01", "diffraction-resolution", "1.85", "angstrom", 512, 2},
		{"id3-detector-01", "completeness", "97.3", "percent", 512, 2},
		{"fast-detector-01", "scattering-intensity", "8342.1", "counts", 3001, 0},
		{"fast-detector-01", "scattering-intensity", "8401.5", "counts", 3002, 0},
	}

	for _, od := range obsData {
		obsID := nsCHESS + "observation/" + uuid.NewString()
		g := graphObs
		t := time.Now().AddDate(0, 0, -od.daysAgo).UTC()

		s.addTriple(obsID, nsRDF+"type", nsSOSA+"Observation", "uri", g)
		s.addTriple(obsID, nsSOSA+"madeBySensor",
			nsCHESS+"sensor/"+od.sensorID, "uri", g)
		s.addTriple(obsID, nsSOSA+"observedProperty",
			nsCHESS+"property/"+od.propLabel, "uri", g)
		s.addTriple(obsID, nsSOSA+"resultTime",
			t.Format(time.RFC3339)+"^^"+nsXSD+"dateTime", "literal", g)
		s.addTriple(obsID, nsSOSA+"hasSimpleResult",
			od.result+"^^"+nsXSD+"decimal", "literal", g)
		s.addTriple(obsID, nsCHESS+"ns#runNumber",
			fmt.Sprintf("%d", od.runNum)+"^^"+nsXSD+"integer", "literal", g)

		// SIO measurement chain
		attrID := nsCHESS + "attr/" + uuid.NewString()
		s.addTriple(obsID, nsSIO+"has-attribute", attrID, "uri", g)
		s.addTriple(attrID, nsRDF+"type", nsSIO+"MeasuredValue", "uri", g)
		s.addTriple(attrID, nsSIO+"has-value",
			od.result+"^^"+nsXSD+"decimal", "literal", g)
		s.addTriple(attrID, nsSIO+"has-unit",
			nsCHESS+"unit/"+od.unit, "uri", g)
	}
}

func (s *Store) addTriple(subj, pred, obj, objType, graph string) {
	t := Triple{
		Subject:    subj,
		Predicate:  pred,
		Object:     obj,
		ObjectType: objType,
		Graph:      graph,
	}
	ng, ok := s.graphs[graph]
	if !ok {
		ng = &NamedGraph{IRI: graph}
		s.graphs[graph] = ng
	}
	ng.Triples = append(ng.Triples, t)
}
