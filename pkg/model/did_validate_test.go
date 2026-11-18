package model

import "testing"

func TestSegmentsParsesKeyValuePairs(t *testing.T) {
	segs := DatasetDID("/beamline=3a/btr=test-123-a/cycle=2024-1/sample_name=lup-20kev-1").Segments()
	if len(segs) != 4 || segs[0][0] != "beamline" || segs[0][1] != "3a" || segs[3][1] != "lup-20kev-1" {
		t.Fatalf("unexpected segments: %v", segs)
	}
}

func TestSegmentsSkipsFreeFormLabels(t *testing.T) {
	// Bare segments are legal at any position (DIDs are opaque paths);
	// Segments simply extracts the key=value pairs among them.
	segs := DatasetDID("/beamline=3a/btr=test-123-a/cycle=2024-1/sample_name=lup-20kev-1/test-analysis").Segments()
	if len(segs) != 4 || segs[3][1] != "lup-20kev-1" {
		t.Fatalf("trailing label must not break pair parsing: %v", segs)
	}
	segs = DatasetDID("/opaque/label/a=b/plain").Segments()
	if len(segs) != 1 || segs[0][0] != "a" || segs[0][1] != "b" {
		t.Fatalf("mixed plain path must parse its pairs: %v", segs)
	}
	if got := DatasetDID("/a/b/c").Segments(); len(got) != 0 {
		t.Fatalf("plain path has no pairs, got %v", got)
	}
}

func TestBeamlineSegmentPresence(t *testing.T) {
	if bl, err := DatasetDID("/beamline=3a/x=1").BeamlineSegment(); err != nil || bl != "3a" {
		t.Fatalf("beamline segment: %v, %v", bl, err)
	}
	if !DatasetDID("/beamline=3a/x=1").HasBeamlineSegment() {
		t.Fatal("beamline DID must report a beamline segment")
	}
	if DatasetDID("/a/b/c").HasBeamlineSegment() {
		t.Fatal("plain path must report no beamline segment")
	}
	if DatasetDID("/beamline=/x=1").HasBeamlineSegment() {
		t.Fatal("empty beamline value must not count as present")
	}
}

func TestGraphIRIPlainPath(t *testing.T) {
	// Plain-path DID maps directly under /graph/ with no beamline element.
	got := DatasetDID("/a/b/c").GraphIRIWithBase("http://example.org/")
	if got != "http://example.org/graph/a/b/c" {
		t.Fatalf("plain-path graph IRI = %q", got)
	}
	// Conventional DID keeps the lowercased beamline element.
	got = DatasetDID("/beamline=ID3a/btr=x").GraphIRIWithBase("http://example.org/")
	if got != "http://example.org/graph/id3a/btr=x" {
		t.Fatalf("beamline graph IRI = %q", got)
	}
}

func TestValidateAcceptsPlainPathsAndDemoDID(t *testing.T) {
	for _, did := range []DatasetDID{
		"/a/b/c",
		"/beamline=3a/btr=test-123-a/cycle=2024-1/sample_name=lup-20kev-1/test-analysis",
		"/beamline=3a/simple",
	} {
		if err := (DatasetRef{Beamline: "3a", DID: did}).Validate(); err != nil {
			t.Fatalf("DID %q must validate: %v", did, err)
		}
	}
}

func TestValidateRejectsBrokenDIDs(t *testing.T) {
	for _, tc := range []struct{ bl BeamlineID; did DatasetDID }{
		{"3a", ""},
		{"3a", "/"},
		{"3a", "//x"},          // empty segment
		{"3a", "/a//b"},        // interior empty segment
		{"3a", "/a/b/"},        // trailing slash
		{"3a", "/beamline=7a"}, // beamline segment disagrees with field
	} {
		if err := (DatasetRef{Beamline: tc.bl, DID: tc.did}).Validate(); err == nil {
			t.Fatalf("DID %q must fail validation", tc.did)
		}
	}
}
