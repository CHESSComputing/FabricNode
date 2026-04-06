// OxigraphStore implements GraphStore by talking to a running Oxigraph server
// via its standard SPARQL 1.1 Query and Update HTTP endpoints.
//
// Oxigraph exposes:
//
//	POST /query   — SPARQL 1.1 SELECT / CONSTRUCT / ASK / DESCRIBE
//	POST /update  — SPARQL 1.1 Update (INSERT DATA, DELETE DATA, …)
//
// The store converts every GraphStore method call into one or more SPARQL
// statements and parses the JSON response.  Named-graph semantics follow the
// same IRI scheme used by MemoryStore so the two implementations are
// interchangeable.
//
// Thread safety: OxigraphStore is safe for concurrent use.  It carries no
// mutable state; all state lives in the Oxigraph process.
package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CHESSComputing/FabricNode/pkg/model"
)

// OxigraphStore is the Oxigraph-backed GraphStore implementation.
type OxigraphStore struct {
	queryURL  string      // e.g. "http://localhost:7878/query"
	updateURL string      // e.g. "http://localhost:7878/update"
	client    *http.Client
}

// Verify interface compliance at compile time.
var _ GraphStore = (*OxigraphStore)(nil)

// NewOxigraphStore creates an OxigraphStore that talks to the server at
// baseURL (e.g. "http://localhost:7878").  timeout is the HTTP client timeout;
// pass 0 to use the default (30 s).
func NewOxigraphStore(baseURL string, timeout time.Duration) *OxigraphStore {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	base := strings.TrimRight(baseURL, "/")
	return &OxigraphStore{
		queryURL:  base + "/query",
		updateURL: base + "/update",
		client:    &http.Client{Timeout: timeout},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Beamline / Dataset scoped writes
// ──────────────────────────────────────────────────────────────────────────────

// InsertForDataset inserts triples into the named graph derived from ref using
// a SPARQL INSERT DATA statement.
func (s *OxigraphStore) InsertForDataset(ref model.DatasetRef, triples []Triple) error {
	if err := ref.Validate(); err != nil {
		return fmt.Errorf("oxigraph: invalid dataset ref: %w", err)
	}
	graphIRI := ref.GraphIRI()
	for i := range triples {
		triples[i].Graph = graphIRI
	}
	return s.insertTriples(triples)
}

// ──────────────────────────────────────────────────────────────────────────────
// Beamline / Dataset scoped reads
// ──────────────────────────────────────────────────────────────────────────────

// QueryDataset returns triples from the named graph for ref.
func (s *OxigraphStore) QueryDataset(ref model.DatasetRef, subject, predicate, object string) ([]Triple, error) {
	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("oxigraph: invalid dataset ref: %w", err)
	}
	return s.runSelectTriples(subject, predicate, object, ref.GraphIRI())
}

// QueryBeamline returns triples from every named graph that belongs to bl.
func (s *OxigraphStore) QueryBeamline(bl model.BeamlineID, subject, predicate, object string) ([]Triple, error) {
	if !bl.Valid() {
		return nil, fmt.Errorf("oxigraph: invalid beamline id %q", bl)
	}
	prefix := fmt.Sprintf("http://chess.cornell.edu/graph/%s/", strings.ToLower(string(bl)))
	// Use SPARQL regex on graph IRI to match all beamline datasets.
	sparql := buildSelectWithGraphFilter(subject, predicate, object, prefix)
	return s.runQuery(sparql)
}

// DatasetsForBeamline returns the graph IRIs of every dataset that belongs to bl.
func (s *OxigraphStore) DatasetsForBeamline(bl model.BeamlineID) []string {
	prefix := fmt.Sprintf("http://chess.cornell.edu/graph/%s/", strings.ToLower(string(bl)))
	sparql := fmt.Sprintf(`SELECT DISTINCT ?g WHERE { GRAPH ?g { ?s ?p ?o } FILTER(STRSTARTS(STR(?g), %s)) }`,
		sparqlString(prefix))
	rows, err := s.execSelect(sparql)
	if err != nil {
		return nil
	}
	var out []string
	for _, row := range rows {
		if v, ok := row["g"]; ok {
			out = append(out, v.Value)
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// Generic (non-scoped) API
// ──────────────────────────────────────────────────────────────────────────────

// Graphs returns the IRIs of all named graphs.
func (s *OxigraphStore) Graphs() []string {
	sparql := `SELECT DISTINCT ?g WHERE { GRAPH ?g { ?s ?p ?o } }`
	rows, err := s.execSelect(sparql)
	if err != nil {
		return nil
	}
	var out []string
	for _, row := range rows {
		if v, ok := row["g"]; ok {
			out = append(out, v.Value)
		}
	}
	return out
}

// Query filters triples across all graphs (or within one graph).
func (s *OxigraphStore) Query(subject, predicate, object, graph string) []Triple {
	results, _ := s.runSelectTriples(subject, predicate, object, graph)
	return results
}

// Insert adds a triple to its named graph.
func (s *OxigraphStore) Insert(t Triple) Triple {
	_ = s.insertTriples([]Triple{t})
	return t
}

// Describe returns all triples where subject or object equals iri.
func (s *OxigraphStore) Describe(iri string) []Triple {
	escaped := sparqlIRI(iri)
	sparql := fmt.Sprintf(
		`SELECT ?s ?p ?o ?g WHERE { GRAPH ?g { ?s ?p ?o . FILTER(?s = %s || ?o = %s) } }`,
		escaped, escaped)
	results, _ := s.runQuery(sparql)
	return results
}

// KeywordSearch does case-insensitive substring search across object literals.
func (s *OxigraphStore) KeywordSearch(keyword string) []Triple {
	sparql := fmt.Sprintf(
		`SELECT ?s ?p ?o ?g WHERE { GRAPH ?g { ?s ?p ?o . FILTER(isLiteral(?o) && CONTAINS(LCASE(STR(?o)), LCASE(%s))) } }`,
		sparqlString(keyword))
	results, _ := s.runQuery(sparql)
	return results
}

// ──────────────────────────────────────────────────────────────────────────────
// SPARQL helpers
// ──────────────────────────────────────────────────────────────────────────────

// sparqlBinding is one cell in a SPARQL results row.
type sparqlBinding struct {
	Type     string `json:"type"`     // "uri", "literal", "bnode"
	Value    string `json:"value"`
	Datatype string `json:"datatype,omitempty"`
	Lang     string `json:"xml:lang,omitempty"`
}

// sparqlResults mirrors the SPARQL 1.1 JSON result format.
type sparqlResults struct {
	Head    struct{ Vars []string } `json:"head"`
	Results struct {
		Bindings []map[string]sparqlBinding `json:"bindings"`
	} `json:"results"`
}

// execSelect sends a SELECT query and returns raw binding rows.
func (s *OxigraphStore) execSelect(sparql string) ([]map[string]sparqlBinding, error) {
	body := url.Values{"query": {sparql}}.Encode()
	req, err := http.NewRequest(http.MethodPost, s.queryURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oxigraph query: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oxigraph query HTTP %d: %s", resp.StatusCode, raw)
	}

	var result sparqlResults
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("oxigraph parse: %w", err)
	}
	return result.Results.Bindings, nil
}

// runQuery executes a SELECT ?s ?p ?o ?g query and maps rows to Triples.
func (s *OxigraphStore) runQuery(sparql string) ([]Triple, error) {
	rows, err := s.execSelect(sparql)
	if err != nil {
		return nil, err
	}
	triples := make([]Triple, 0, len(rows))
	for _, row := range rows {
		t := Triple{}
		if b, ok := row["s"]; ok {
			t.Subject = b.Value
		}
		if b, ok := row["p"]; ok {
			t.Predicate = b.Value
		}
		if b, ok := row["o"]; ok {
			t.Object = b.Value
			t.ObjectType = b.Type
			t.Datatype = b.Datatype
			t.Lang = b.Lang
		}
		if b, ok := row["g"]; ok {
			t.Graph = b.Value
		}
		triples = append(triples, t)
	}
	return triples, nil
}

// runSelectTriples builds a standard ?s ?p ?o ?g SELECT and returns Triples.
func (s *OxigraphStore) runSelectTriples(subject, predicate, object, graph string) ([]Triple, error) {
	sparql := buildSelectQuery(subject, predicate, object, graph)
	return s.runQuery(sparql)
}

// buildSelectQuery constructs a SPARQL SELECT that respects wildcard ("") args.
func buildSelectQuery(subject, predicate, object, graph string) string {
	s := tripleVar("s", subject)
	p := tripleVar("p", predicate)
	o := tripleVar("o", object)

	var filters []string
	if subject != "" {
		filters = append(filters, fmt.Sprintf("FILTER(?s = %s)", sparqlIRI(subject)))
	}
	if predicate != "" {
		filters = append(filters, fmt.Sprintf("FILTER(?p = %s)", sparqlIRI(predicate)))
	}
	if object != "" {
		filters = append(filters, fmt.Sprintf("FILTER(STR(?o) = %s)", sparqlString(object)))
	}

	pattern := fmt.Sprintf("%s %s %s .", s, p, o)
	if len(filters) > 0 {
		pattern += " " + strings.Join(filters, " ")
	}

	if graph != "" {
		return fmt.Sprintf(
			`SELECT ?s ?p ?o ?g WHERE { BIND(%s AS ?g) GRAPH ?g { %s } }`,
			sparqlIRI(graph), pattern)
	}
	return fmt.Sprintf(`SELECT ?s ?p ?o ?g WHERE { GRAPH ?g { %s } }`, pattern)
}

// buildSelectWithGraphFilter builds a SELECT that filters graph by prefix.
func buildSelectWithGraphFilter(subject, predicate, object, graphPrefix string) string {
	s := tripleVar("s", subject)
	p := tripleVar("p", predicate)
	o := tripleVar("o", object)

	pattern := fmt.Sprintf("%s %s %s", s, p, o)

	var filters []string
	filters = append(filters, fmt.Sprintf("STRSTARTS(STR(?g), %s)", sparqlString(graphPrefix)))
	if subject != "" {
		filters = append(filters, fmt.Sprintf("?s = %s", sparqlIRI(subject)))
	}
	if predicate != "" {
		filters = append(filters, fmt.Sprintf("?p = %s", sparqlIRI(predicate)))
	}
	if object != "" {
		filters = append(filters, fmt.Sprintf("STR(?o) = %s", sparqlString(object)))
	}

	return fmt.Sprintf(
		`SELECT ?s ?p ?o ?g WHERE { GRAPH ?g { %s } FILTER(%s) }`,
		pattern, strings.Join(filters, " && "))
}

// insertTriples sends a SPARQL INSERT DATA update for one or more triples.
func (s *OxigraphStore) insertTriples(triples []Triple) error {
	if len(triples) == 0 {
		return nil
	}

	// Group triples by named graph for compact INSERT DATA blocks.
	byGraph := make(map[string][]Triple)
	for _, t := range triples {
		byGraph[t.Graph] = append(byGraph[t.Graph], t)
	}

	var buf bytes.Buffer
	buf.WriteString("INSERT DATA {\n")
	for graphIRI, ts := range byGraph {
		fmt.Fprintf(&buf, "  GRAPH %s {\n", sparqlIRI(graphIRI))
		for _, t := range ts {
			fmt.Fprintf(&buf, "    %s %s %s .\n",
				sparqlIRI(t.Subject),
				sparqlIRI(t.Predicate),
				formatObject(t),
			)
		}
		buf.WriteString("  }\n")
	}
	buf.WriteString("}")

	return s.execUpdate(buf.String())
}

// execUpdate sends a SPARQL 1.1 Update request.
func (s *OxigraphStore) execUpdate(sparql string) error {
	body := url.Values{"update": {sparql}}.Encode()
	req, err := http.NewRequest(http.MethodPost, s.updateURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("oxigraph update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("oxigraph update HTTP %d: %s", resp.StatusCode, raw)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// SPARQL serialisation helpers
// ──────────────────────────────────────────────────────────────────────────────

// sparqlIRI wraps an IRI in angle brackets.
func sparqlIRI(iri string) string {
	return "<" + iri + ">"
}

// sparqlString wraps a plain string in SPARQL double-quotes.
func sparqlString(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// tripleVar returns either "?name" (wildcard) or the SPARQL IRI of value.
func tripleVar(name, value string) string {
	if value == "" {
		return "?" + name
	}
	return sparqlIRI(value)
}

// formatObject serialises a Triple's object as a SPARQL term.
func formatObject(t Triple) string {
	switch t.ObjectType {
	case "uri":
		return sparqlIRI(t.Object)
	case "bnode":
		return "_:" + t.Object
	default: // literal
		// Detect datatype suffix embedded in Object ("value^^typeIRI")
		if idx := strings.Index(t.Object, "^^"); idx >= 0 {
			val := t.Object[:idx]
			dt := t.Object[idx+2:]
			return fmt.Sprintf("%s^^%s", sparqlString(val), sparqlIRI(dt))
		}
		if t.Lang != "" {
			return sparqlString(t.Object) + "@" + t.Lang
		}
		if t.Datatype != "" {
			return fmt.Sprintf("%s^^%s", sparqlString(t.Object), sparqlIRI(t.Datatype))
		}
		return sparqlString(t.Object)
	}
}
