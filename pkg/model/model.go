// Package model defines the canonical domain types shared across all
// FabricNode services.
//
// Beamline names follow CHESS convention: letters and digits only in any order
// e.g. "id1", "id3a", "fast", "qm2", "3a", etc.
//
// Dataset identifiers (DIDs) are slash-separated key=value paths:
//
//	/beamline=id3a/btr=val123/cycle=2024-3/sample_name=bla
//
// A DatasetDID must begin with a /beamline= segment that matches the
// BeamlineID it belongs to.
package model

import (
	"fmt"
	"regexp"
	"strings"
)

// ──────────────────────────────────────────────────────────────────────────────
// Primitive identifiers
// ──────────────────────────────────────────────────────────────────────────────

// BeamlineID is a validated beamline name (letters + digits, lower-case).
// Examples: "id1", "id3a", "fast", "qm2".
type BeamlineID string

// beamlinePattern accepts lower-case letters and digits, length 1-16.
var beamlinePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9]{0,15}$`)

// Valid reports whether b is a well-formed beamline identifier.
func (b BeamlineID) Valid() bool {
	return beamlinePattern.MatchString(string(b))
}

// DatasetDID is a slash-separated key=value path that uniquely identifies
// a dataset within a beamline.
//
// Canonical form:
//
//	/beamline=<id>/btr=<val>/cycle=<val>/sample_name=<val>[/<key>=<val>…]
//
// The /beamline= segment is always first and must agree with the containing
// BeamlineID.
type DatasetDID string

// Segments parses the DID into an ordered slice of key/value pairs.
// Returns an error if any segment is not in "key=value" form.
func (d DatasetDID) Segments() ([][2]string, error) {
	raw := strings.TrimPrefix(string(d), "/")
	parts := strings.Split(raw, "/")
	out := make([][2]string, 0, len(parts))
	for _, p := range parts {
		idx := strings.IndexByte(p, '=')
		if idx < 1 {
			return nil, fmt.Errorf("invalid DID segment %q: expected key=value", p)
		}
		out = append(out, [2]string{p[:idx], p[idx+1:]})
	}
	return out, nil
}

// BeamlineSegment returns the value of the leading /beamline=<id> segment,
// or an error if the DID is malformed.
func (d DatasetDID) BeamlineSegment() (BeamlineID, error) {
	segs, err := d.Segments()
	if err != nil {
		return "", err
	}
	if len(segs) == 0 || segs[0][0] != "beamline" {
		return "", fmt.Errorf("DID %q: first segment must be 'beamline=<id>'", d)
	}
	return BeamlineID(segs[0][1]), nil
}

// GraphIRI converts a DatasetDID to the named-graph IRI used in the
// triple store.  The beamline name is normalised to lower-case.
//
// Example:
//
//	/beamline=id3a/btr=val123/cycle=2024-3/sample_name=bla
//	→ http://chess.cornell.edu/graph/id3a/btr=val123/cycle=2024-3/sample_name=bla
func (d DatasetDID) GraphIRI() string {
	trimmed := strings.TrimPrefix(string(d), "/")
	// strip leading "beamline=<id>/" prefix to avoid redundancy in the IRI
	rest := trimmed
	if idx := strings.IndexByte(trimmed, '/'); idx >= 0 {
		rest = trimmed[idx+1:]
	}
	bl, _ := d.BeamlineSegment()
	return fmt.Sprintf("http://chess.cornell.edu/graph/%s/%s",
		strings.ToLower(string(bl)), rest)
}

// ──────────────────────────────────────────────────────────────────────────────
// Composite types
// ──────────────────────────────────────────────────────────────────────────────

// DatasetRef is the minimal locator needed to address a dataset inside
// the data-service.  Both fields are required.
type DatasetRef struct {
	Beamline BeamlineID `json:"beamline"`
	DID      DatasetDID `json:"did"`
}

// GraphIRI delegates to DID.GraphIRI.
func (r DatasetRef) GraphIRI() string { return r.DID.GraphIRI() }

// Validate returns an error if either field is empty or malformed.
func (r DatasetRef) Validate() error {
	if !r.Beamline.Valid() {
		return fmt.Errorf("invalid beamline id %q: must match [a-z][a-z0-9]{0,15}", r.Beamline)
	}
	bl, err := r.DID.BeamlineSegment()
	if err != nil {
		return fmt.Errorf("invalid DID: %w", err)
	}
	if bl != r.Beamline {
		return fmt.Errorf("DID beamline segment %q does not match Beamline field %q", bl, r.Beamline)
	}
	return nil
}

// Beamline describes a CHESS beamline registered in the catalog.
type Beamline struct {
	ID          BeamlineID `json:"id"`
	Label       string     `json:"label"`
	Type        string     `json:"type"` // e.g. "x-ray-diffraction"
	Description string     `json:"description,omitempty"`
	Location    string     `json:"location,omitempty"`
}

// Dataset is a catalogue entry for one dataset within a beamline.
type Dataset struct {
	Ref         DatasetRef `json:"ref"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	CreatedAt   string     `json:"createdAt,omitempty"` // RFC3339
	DataURL     string     `json:"dataURL,omitempty"`   // link into data-service
}
