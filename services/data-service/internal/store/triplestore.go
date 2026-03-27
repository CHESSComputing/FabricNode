// Package store provides an in-memory RDF triple store with named-graph
// support scoped to CHESS beamlines and dataset DIDs.
//
// Named graph IRI scheme:
//
//	http://chess.cornell.edu/graph/<beamlineID>/<did-segments>
//
// For production, replace with Oxigraph or Apache Jena; the Store interface
// is designed for a one-line swap in main.go.
package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/google/uuid"
)

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

// New creates a Store pre-seeded with representative CHESS data.
func New() *Store {
	s := &Store{graphs: make(map[string]*NamedGraph)}
	s.seed()
	return s
}

// ──────────────────────────────────────────────────────────────────────────────
// Beamline / Dataset scoped API
// ──────────────────────────────────────────────────────────────────────────────

// InsertForDataset inserts triples into the named graph derived from ref.
// Every triple's Graph field is overwritten with the canonical graph IRI so
// callers do not need to set it manually.
func (s *Store) InsertForDataset(ref model.DatasetRef, triples []Triple) error {
	if err := ref.Validate(); err != nil {
		return fmt.Errorf("store: invalid dataset ref: %w", err)
	}
	graphIRI := ref.GraphIRI()
	s.mu.Lock()
	defer s.mu.Unlock()
	ng := s.ensureGraph(graphIRI)
	for _, t := range triples {
		t.Graph = graphIRI
		ng.Triples = append(ng.Triples, t)
	}
	return nil
}

// QueryDataset returns triples from the named graph for ref.
// subject/predicate/object act as wildcards when empty.
func (s *Store) QueryDataset(ref model.DatasetRef, subject, predicate, object string) ([]Triple, error) {
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("store: invalid dataset ref: %w", err)
	}
	return s.Query(subject, predicate, object, ref.GraphIRI()), nil
}

// QueryBeamline returns triples from all graphs whose IRI starts with the
// beamline prefix — i.e. every dataset belonging to that beamline.
func (s *Store) QueryBeamline(bl model.BeamlineID, subject, predicate, object string) ([]Triple, error) {
	if !bl.Valid() {
		return nil, fmt.Errorf("store: invalid beamline id %q", bl)
	}
	prefix := fmt.Sprintf("http://chess.cornell.edu/graph/%s/", strings.ToLower(string(bl)))
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Triple
	for graphIRI, ng := range s.graphs {
		if !strings.HasPrefix(graphIRI, prefix) {
			continue
		}
		for _, t := range ng.Triples {
			if matches(t.Subject, subject) &&
				matches(t.Predicate, predicate) &&
				matches(t.Object, object) {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// DatasetsForBeamline returns the graph IRIs (= dataset DIDs in IRI form)
// belonging to a beamline.
func (s *Store) DatasetsForBeamline(bl model.BeamlineID) []string {
	prefix := fmt.Sprintf("http://chess.cornell.edu/graph/%s/", strings.ToLower(string(bl)))
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for graphIRI := range s.graphs {
		if strings.HasPrefix(graphIRI, prefix) {
			out = append(out, graphIRI)
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// Generic (non-scoped) API — used by legacy SPARQL handler and seed
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
// Empty string is a wildcard for subject/predicate/object/graph.
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

// Insert adds a triple to its named graph (set via t.Graph), creating the
// graph if needed.
func (s *Store) Insert(t Triple) Triple {
	s.mu.Lock()
	defer s.mu.Unlock()
	ng := s.ensureGraph(t.Graph)
	ng.Triples = append(ng.Triples, t)
	return t
}

// Describe returns all triples where subject or object equals iri.
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

// KeywordSearch does case-insensitive substring search across object literals.
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

func (s *Store) ensureGraph(iri string) *NamedGraph {
	ng, ok := s.graphs[iri]
	if !ok {
		ng = &NamedGraph{IRI: iri}
		s.graphs[iri] = ng
	}
	return ng
}

func matches(value, pattern string) bool {
	return pattern == "" || value == pattern
}

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
	// Shared metadata graphs (beamline descriptors, sensors)
	const (
		graphBeamlines = nsCHESS + "graph/beamlines"
		graphSensors   = nsCHESS + "graph/sensors"
	)

	beamlines := []struct{ id, label, blType string }{
		{"id1", "Beamline ID1 — X-ray Diffraction", "x-ray-diffraction"},
		{"id3a", "Beamline ID3A — Protein Crystallography", "protein-crystallography"},
		{"fast", "Beamline FAST — Time-Resolved Scattering", "time-resolved-scattering"},
	}
	for _, bl := range beamlines {
		subj := nsCHESS + "beamline/" + bl.id
		s.addTriple(subj, nsRDF+"type", nsCHESS+"Beamline", "uri", graphBeamlines)
		s.addTriple(subj, nsRDFS+"label", bl.label, "literal", graphBeamlines)
		s.addTriple(subj, nsCHESS+"ns#beamlineType", bl.blType, "literal", graphBeamlines)
		s.addTriple(subj, nsDCT+"description",
			fmt.Sprintf("CHESS beamline %s (%s)", strings.ToUpper(bl.id), bl.blType),
			"literal", graphBeamlines)
	}

	sensors := []struct{ id, label, blID string }{
		{"id1-detector-01", "ID1 Primary Area Detector", "id1"},
		{"id3a-detector-01", "ID3A CCD Crystallography Detector", "id3a"},
		{"fast-detector-01", "FAST High-Speed Scintillator Detector", "fast"},
	}
	for _, sen := range sensors {
		subj := nsCHESS + "sensor/" + sen.id
		s.addTriple(subj, nsRDF+"type", nsSOSA+"Sensor", "uri", graphSensors)
		s.addTriple(subj, nsRDFS+"label", sen.label, "literal", graphSensors)
		s.addTriple(subj, nsSOSA+"isHostedBy", nsCHESS+"beamline/"+sen.blID, "uri", graphSensors)
	}

	// Observations stored in dataset-scoped named graphs.
	// DID format: /beamline=<id>/btr=<btr>/cycle=<cycle>/sample_name=<name>
	seedObs := []struct {
		beamline   string
		btr        string
		cycle      string
		sampleName string
		sensorID   string
		propLabel  string
		result     string
		unit       string
		runNum     int
		daysAgo    int
	}{
		{"id1", "btr001", "2024-3", "silicon-std", "id1-detector-01", "lattice-parameter-a", "5.431", "angstrom", 1024, 1},
		{"id1", "btr001", "2024-3", "silicon-std", "id1-detector-01", "peak-intensity", "14253.7", "counts", 1024, 1},
		{"id1", "btr002", "2024-3", "lysozyme-1", "id1-detector-01", "peak-intensity", "13997.2", "counts", 1025, 0},
		{"id3a", "btr101", "2024-3", "thaumatin-a", "id3a-detector-01", "diffraction-resolution", "1.85", "angstrom", 512, 2},
		{"id3a", "btr101", "2024-3", "thaumatin-a", "id3a-detector-01", "completeness", "97.3", "percent", 512, 2},
		{"fast", "btr201", "2024-3", "fe3o4-nanoparticles", "fast-detector-01", "scattering-intensity", "8342.1", "counts", 3001, 0},
		{"fast", "btr201", "2024-3", "fe3o4-nanoparticles", "fast-detector-01", "scattering-intensity", "8401.5", "counts", 3002, 0},
	}

	for _, od := range seedObs {
		did := model.DatasetDID(fmt.Sprintf("/beamline=%s/btr=%s/cycle=%s/sample_name=%s",
			od.beamline, od.btr, od.cycle, od.sampleName))
		graphIRI := did.GraphIRI()

		obsID := nsCHESS + "observation/" + uuid.NewString()
		t := time.Now().AddDate(0, 0, -od.daysAgo).UTC()

		s.addTriple(obsID, nsRDF+"type", nsSOSA+"Observation", "uri", graphIRI)
		s.addTriple(obsID, nsSOSA+"madeBySensor",
			nsCHESS+"sensor/"+od.sensorID, "uri", graphIRI)
		s.addTriple(obsID, nsSOSA+"observedProperty",
			nsCHESS+"property/"+od.propLabel, "uri", graphIRI)
		s.addTriple(obsID, nsSOSA+"resultTime",
			t.Format(time.RFC3339)+"^^"+nsXSD+"dateTime", "literal", graphIRI)
		s.addTriple(obsID, nsSOSA+"hasSimpleResult",
			od.result+"^^"+nsXSD+"decimal", "literal", graphIRI)
		s.addTriple(obsID, nsCHESS+"ns#runNumber",
			fmt.Sprintf("%d", od.runNum)+"^^"+nsXSD+"integer", "literal", graphIRI)

		attrID := nsCHESS + "attr/" + uuid.NewString()
		s.addTriple(obsID, nsSIO+"has-attribute", attrID, "uri", graphIRI)
		s.addTriple(attrID, nsRDF+"type", nsSIO+"MeasuredValue", "uri", graphIRI)
		s.addTriple(attrID, nsSIO+"has-value",
			od.result+"^^"+nsXSD+"decimal", "literal", graphIRI)
		s.addTriple(attrID, nsSIO+"has-unit",
			nsCHESS+"unit/"+od.unit, "uri", graphIRI)
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
	ng := s.ensureGraph(graph)
	ng.Triples = append(ng.Triples, t)
}
