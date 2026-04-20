package foxden_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/foxden"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// ── test-scoped IRI base ──────────────────────────────────────────────────────
// All tests use this value to keep expected strings consistent.
// In production this comes from Config.Node.IRIBase.

const (
	chessBase = "http://chess.cornell.edu/"
	nsCHESS   = chessBase + "ns#" // = chessBase + "ns#"
	nsDCT     = "http://purl.org/dc/terms/"
	nsRDF     = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS    = "http://www.w3.org/2000/01/rdf-schema#"
	nsXSD     = "http://www.w3.org/2001/XMLSchema#"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// minimalRecord returns the smallest valid FOXDEN record (has a DID).
func minimalRecord(did string) foxden.Record {
	return foxden.Record{"did": did}
}

// findTriples returns all triples in ts where predicate matches pred.
func findTriples(ts []store.Triple, pred string) []store.Triple {
	var out []store.Triple
	for _, t := range ts {
		if t.Predicate == pred {
			out = append(out, t)
		}
	}
	return out
}

// objectValues returns the Object values for every triple matching pred.
func objectValues(ts []store.Triple, pred string) []string {
	var vals []string
	for _, t := range findTriples(ts, pred) {
		vals = append(vals, t.Object)
	}
	return vals
}

// hasObject returns true if any triple with pred has object obj.
func hasObject(ts []store.Triple, pred, obj string) bool {
	for _, v := range objectValues(ts, pred) {
		if v == obj {
			return true
		}
	}
	return false
}

// allGraphs returns true when every triple has the expected graph IRI.
func allGraphs(ts []store.Triple, graphIRI string) bool {
	for _, t := range ts {
		if t.Graph != graphIRI {
			return false
		}
	}
	return true
}

// ── RecordToTriples ───────────────────────────────────────────────────────────

func TestRecordToTriples_MissingDID(t *testing.T) {
	_, err := foxden.RecordToTriples(foxden.Record{}, "http://example.org/graph/1", chessBase)
	if err == nil {
		t.Fatal("expected error for record without DID, got nil")
	}
	if !strings.Contains(err.Error(), "did") {
		t.Errorf("expected error mentioning 'did', got: %v", err)
	}
}

func TestRecordToTriples_MinimalRecord_AlwaysEmitsTypeAndIdentifier(t *testing.T) {
	did := "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"
	graphIRI := chessBase + "graph/id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"

	ts, err := foxden.RecordToTriples(minimalRecord(did), graphIRI, chessBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// rdf:type = chess:Dataset
	if !hasObject(ts, nsRDF+"type", chessBase+"Dataset") {
		t.Errorf("missing rdf:type chess:Dataset")
	}
	// dct:identifier = DID literal
	if !hasObject(ts, nsDCT+"identifier", did) {
		t.Errorf("missing dct:identifier with DID value")
	}
	// All triples must carry the supplied graphIRI.
	if !allGraphs(ts, graphIRI) {
		t.Errorf("some triples have wrong Graph IRI")
	}
}

func TestRecordToTriples_ScalarFields(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     any
		predicate string
		wantObj   string
	}{
		{"btr string", "btr", "btr001", nsCHESS + "btr", "btr001"},
		{"cycle string", "cycle", "2024-3", nsCHESS + "cycle", "2024-3"},
		{"pi mapped to dct:creator", "pi", "Jane Smith", nsDCT + "creator", "Jane Smith"},
		{"sample_name", "sample_name", "silicon-std", nsCHESS + "sampleName", "silicon-std"},
		{"beam_energy float", "beam_energy", float64(67.5), nsCHESS + "beamEnergy", fmt.Sprintf("%g^^%sdecimal", 67.5, nsXSD)},
		{"doi mapped to dct:relation", "doi", "10.1000/xyz123", nsDCT + "relation", "10.1000/xyz123"},
	}

	did := "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"
	graphIRI := "http://example.org/graph/test"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := minimalRecord(did)
			rec[tc.field] = tc.value
			ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !hasObject(ts, tc.predicate, tc.wantObj) {
				vals := objectValues(ts, tc.predicate)
				t.Errorf("predicate %s: want object %q, got %v", tc.predicate, tc.wantObj, vals)
			}
		})
	}
}

func TestRecordToTriples_EmptyStringFieldIsSkipped(t *testing.T) {
	did := "/beamline=id1/btr=x/cycle=2024-3/sample_name=y"
	rec := minimalRecord(did)
	rec["btr"] = "" // empty — should not emit a triple
	ts, err := foxden.RecordToTriples(rec, "http://example.org/g", chessBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasObject(ts, nsCHESS+"btr", "") {
		t.Error("empty string scalar should not produce a triple")
	}
}

func TestRecordToTriples_BooleanFlags(t *testing.T) {
	tests := []struct {
		field     string
		predicate string
	}{
		{"alignment", nsCHESS + "alignment"},
		{"calibration", nsCHESS + "calibration"},
		{"in_situ", nsCHESS + "inSitu"},
		{"doi_public", nsCHESS + "doiPublic"},
	}

	did := "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=s"
	graphIRI := "http://example.org/g"

	for _, tc := range tests {
		t.Run(tc.field+"=true", func(t *testing.T) {
			rec := minimalRecord(did)
			rec[tc.field] = true
			ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := fmt.Sprintf("true^^%sboolean", nsXSD)
			if !hasObject(ts, tc.predicate, want) {
				t.Errorf("bool true: want %q for %s, got %v", want, tc.predicate, objectValues(ts, tc.predicate))
			}
		})

		t.Run(tc.field+"=false", func(t *testing.T) {
			rec := minimalRecord(did)
			rec[tc.field] = false
			ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := fmt.Sprintf("false^^%sboolean", nsXSD)
			if !hasObject(ts, tc.predicate, want) {
				t.Errorf("bool false: want %q for %s, got %v", want, tc.predicate, objectValues(ts, tc.predicate))
			}
		})
	}
}

func TestRecordToTriples_ArrayFields(t *testing.T) {
	tests := []struct {
		field     string
		predicate string
		values    []string
	}{
		{"beamline", nsCHESS + "beamline", []string{"id1", "id3a"}},
		{"detectors", nsCHESS + "detector", []string{"Pilatus 6M", "Eiger 16M"}},
		{"technique", nsCHESS + "technique", []string{"SAXS", "WAXS"}},
	}

	did := "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=s"
	graphIRI := "http://example.org/g"

	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			rec := minimalRecord(did)
			arr := make([]any, len(tc.values))
			for i, v := range tc.values {
				arr[i] = v
			}
			rec[tc.field] = arr
			ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := objectValues(ts, tc.predicate)
			for _, want := range tc.values {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("array field %s: missing value %q, got %v", tc.field, want, got)
				}
			}
			if len(got) != len(tc.values) {
				t.Errorf("array field %s: want %d triples, got %d", tc.field, len(tc.values), len(got))
			}
		})
	}
}

func TestRecordToTriples_ArrayField_EmptyStringsSkipped(t *testing.T) {
	did := "/beamline=id1/btr=x/cycle=2024-3/sample_name=y"
	rec := minimalRecord(did)
	rec["technique"] = []any{"SAXS", "", "WAXS"} // empty element should be skipped
	ts, err := foxden.RecordToTriples(rec, "http://example.org/g", chessBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vals := objectValues(ts, nsCHESS+"technique")
	if len(vals) != 2 {
		t.Errorf("expected 2 technique triples (empty skipped), got %d: %v", len(vals), vals)
	}
}

func TestRecordToTriples_CrystallographicPhases(t *testing.T) {
	did := "/beamline=id3a/btr=btr101/cycle=2024-3/sample_name=thaumatin"
	graphIRI := chessBase + "graph/id3a/btr=btr101/cycle=2024-3/sample_name=thaumatin"
	rec := minimalRecord(did)
	rec["sample_crystallographic_phases"] = []any{
		map[string]any{"name": "Lysozyme", "space_group": float64(96)},
		map[string]any{"name": "Buffer"},
	}

	ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should emit chess:crystallographicPhase triples
	phaseTriples := findTriples(ts, nsCHESS+"crystallographicPhase")
	if len(phaseTriples) != 2 {
		t.Errorf("expected 2 crystallographicPhase triples, got %d", len(phaseTriples))
	}

	// Phase 0 should have rdfs:label "Lysozyme"
	phase0IRI := fmt.Sprintf("%sdataset%s/phase/0", chessBase, did)
	labelTriples := findTriples(ts, nsRDFS+"label")
	foundLabel := false
	for _, lt := range labelTriples {
		if lt.Subject == phase0IRI && lt.Object == "Lysozyme" {
			foundLabel = true
			break
		}
	}
	if !foundLabel {
		t.Errorf("expected rdfs:label 'Lysozyme' for phase0 IRI %s", phase0IRI)
	}

	// Phase 0 should have chess:spaceGroup with value containing "96"
	sgTriples := findTriples(ts, nsCHESS+"spaceGroup")
	foundSG := false
	for _, sgt := range sgTriples {
		if sgt.Subject == phase0IRI && strings.Contains(sgt.Object, "96") {
			foundSG = true
			break
		}
	}
	if !foundSG {
		t.Errorf("expected chess:spaceGroup 96 for phase0, got %v", sgTriples)
	}
}

func TestRecordToTriples_AllTriplesHaveCorrectSubject(t *testing.T) {
	did := "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"
	graphIRI := "http://example.org/g"
	rec := minimalRecord(did)
	rec["btr"] = "btr001"
	rec["cycle"] = "2024-3"
	rec["technique"] = []any{"SAXS"}

	ts, err := foxden.RecordToTriples(rec, graphIRI, chessBase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSubj := chessBase + "dataset" + did
	for _, v := range ts {
		// Phase triples have their own subject — only check dataset-level triples.
		if v.Predicate == nsRDF+"type" || v.Predicate == nsDCT+"identifier" ||
			v.Predicate == nsCHESS+"btr" || v.Predicate == nsCHESS+"cycle" {
			if v.Subject != expectedSubj {
				t.Errorf("expected subject %q, got %q", expectedSubj, v.Subject)
			}
		}
	}
}

// ── GraphIRIFromDID ───────────────────────────────────────────────────────────

func TestGraphIRIFromDID(t *testing.T) {
	tests := []struct {
		did  string
		want string
	}{
		{
			did:  "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT",
			want: chessBase + "graph/3a/btr=test-123-a/cycle=2026-1/sample_name=PAT",
		},
		{
			did:  "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std",
			want: chessBase + "graph/id1/btr=btr001/cycle=2024-3/sample_name=silicon-std",
		},
		{
			did:  "/beamline=FAST/btr=btr201/cycle=2024-3/sample_name=fe3o4",
			want: chessBase + "graph/fast/btr=btr201/cycle=2024-3/sample_name=fe3o4",
		},
	}
	for _, tc := range tests {
		t.Run(tc.did, func(t *testing.T) {
			got := foxden.GraphIRIFromDID(tc.did, chessBase)
			if got != tc.want {
				t.Errorf("GraphIRIFromDID(%q)\n  got  %q\n  want %q", tc.did, got, tc.want)
			}
		})
	}
}

func TestGraphIRIFromDID_MalformedFallsBack(t *testing.T) {
	got := foxden.GraphIRIFromDID("not-a-valid-did", chessBase)
	if !strings.HasPrefix(got, chessBase+"graph/unknown/") {
		t.Errorf("malformed DID should produce unknown graph, got %q", got)
	}
}
