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

// DatasetDID is a slash-separated path that uniquely identifies a dataset.
//
// The path grammar is opaque: any non-empty path without empty segments is
// acceptable, e.g. /a/b/c.  A conventional form prefixes a beamline=<id>
// segment:
//
//	/beamline=<id>/btr=<val>/cycle=<val>/sample_name=<val>[/<suffix>]
//
// Key=value segments are a convention, not a requirement; Segments parses
// pairs where they exist and ignores free-form labels.  When a leading
// beamline=<id> segment is present it must agree with the containing
// BeamlineID (enforced by DatasetRef.Validate).
type DatasetDID string

// Segments parses the DID into key/value pairs.  Segments of the form
// key=value contribute pairs; free-form label segments (any segment without
// a "=" prefix, legal at any position) are skipped.  The path grammar itself
// is opaque, so no error is produced for bare segments.
func (d DatasetDID) Segments() [][2]string {
	raw := strings.TrimPrefix(string(d), "/")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([][2]string, 0, len(parts))
	for _, p := range parts {
		idx := strings.IndexByte(p, '=')
		if idx < 1 {
			continue // free-form label segment, legal at any position
		}
		out = append(out, [2]string{p[:idx], p[idx+1:]})
	}
	return out
}

// BeamlineSegment returns the value of the leading beamline=<id> segment,
// or an error when the DID does not carry one (legal for plain-path DIDs
// such as /a/b/c).  Use HasBeamlineSegment to test for presence first.
func (d DatasetDID) BeamlineSegment() (BeamlineID, error) {
	segs := d.Segments()
	if len(segs) == 0 || segs[0][0] != "beamline" || segs[0][1] == "" {
		return "", fmt.Errorf("DID %q: no beamline=<id> segment", d)
	}
	return BeamlineID(segs[0][1]), nil
}

// HasBeamlineSegment reports whether the DID carries a leading
// beamline=<id> segment with a non-empty value.
func (d DatasetDID) HasBeamlineSegment() bool {
	_, err := d.BeamlineSegment()
	return err == nil
}

// GraphIRIWithBase converts a DatasetDID to a named-graph IRI using the
// provided base URL (e.g. from Config.Node.IRIBase).
// base must end with a trailing slash.
//
// Example:
//
//	base  = "http://example.org/"
//	DID   = /beamline=id3a/btr=val123/cycle=2024-3/sample_name=bla
//	→ http://example.org/graph/id3a/btr=val123/cycle=2024-3/sample_name=bla
func (d DatasetDID) GraphIRIWithBase(base string) string {
	trimmed := strings.TrimPrefix(string(d), "/")
	base = strings.TrimSuffix(base, "/")
	// Plain-path DIDs (no beamline= segment) map verbatim under /graph/;
	// conventional DIDs drop the redundant beamline=<id>/ prefix and keep
	// the lowercased beamline element in its place.
	if bl, err := d.BeamlineSegment(); err == nil {
		rest := trimmed
		if idx := strings.IndexByte(trimmed, '/'); idx >= 0 {
			rest = trimmed[idx+1:]
		}
		return fmt.Sprintf("%s/graph/%s/%s", base, strings.ToLower(string(bl)), rest)
	}
	return fmt.Sprintf("%s/graph/%s", base, trimmed)
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

// GraphIRIWithBase delegates to DID.GraphIRIWithBase.
func (r DatasetRef) GraphIRIWithBase(base string) string { return r.DID.GraphIRIWithBase(base) }

// Validate returns an error if either field is empty or malformed.
//
// A DID is an opaque '/'-separated path (e.g. /a/b/c); key=value segments
// are conventional but not required.  Enforced here: the DID must be a
// non-empty path without empty segments, and when it carries a leading
// beamline=<id> segment that segment must match the Beamline field.
func (r DatasetRef) Validate() error {
	if !r.Beamline.Valid() {
		return fmt.Errorf("invalid beamline id %q: must match [a-z][a-z0-9]{0,15}", r.Beamline)
	}
	did := string(r.DID)
	for _, seg := range strings.Split(strings.TrimPrefix(did, "/"), "/") {
		if seg == "" {
			return fmt.Errorf("invalid DID %q: must be a non-empty path without empty segments", did)
		}
	}
	if r.DID.HasBeamlineSegment() {
		bl, err := r.DID.BeamlineSegment()
		if err != nil {
			return fmt.Errorf("invalid DID: %w", err)
		}
		if bl != r.Beamline {
			return fmt.Errorf("DID beamline segment %q does not match Beamline field %q", bl, r.Beamline)
		}
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
