// Package foxden — rdf.go converts FOXDEN metadata records into RDF triples
// suitable for insertion into the data-service triple store.
//
// Field-to-predicate mapping is driven by FieldMaps, which is built at
// startup from FOXDEN beamline schema JSON files (see schema.go).  A set of
// built-in static maps is merged in first so that core fields (btr, cycle,
// pi, sample_name, data locations, …) always work even when no schema
// directory is configured.
//
// Mapping decisions:
//   - Each FOXDEN record becomes a single named-graph keyed by its DID.
//   - Scalar string/numeric fields become plain literal triples.
//   - Array fields emit one triple per element.
//   - Numeric array fields emit one xsd:decimal/integer triple per element.
//   - Nested objects (sample_crystallographic_phases) are flattened into
//     sub-resource IRIs.
//   - The chess: namespace is used for CHESS-specific predicates; well-known
//     fields (pi, cycle, doi) are mapped to dct: where appropriate.
package foxden

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
	"github.com/google/uuid"
)

const (
	nsDCT  = "http://purl.org/dc/terms/"
	nsRDF  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS = "http://www.w3.org/2000/01/rdf-schema#"
	nsXSD  = "http://www.w3.org/2001/XMLSchema#"
	nsPROV = "http://www.w3.org/ns/prov#"
)

// ──────────────────────────────────────────────────────────────────────────────
// Static (built-in) field maps
// ──────────────────────────────────────────────────────────────────────────────
// These are merged into every FieldMaps before the schema-file entries so that
// core fields are always covered, and so that standard vocabulary mappings
// (dct:creator for pi, dct:date for date, etc.) are preserved regardless of
// what the schema files say.

func staticScalarMap(chessNS string) map[string]string {
	return map[string]string{
		"btr":                             chessNS + "btr",
		"cycle":                           chessNS + "cycle",
		"schema":                          chessNS + "schema",
		"facility":                        chessNS + "facility",
		"user":                            chessNS + "user",
		"pi":                              nsDCT + "creator",
		"experimenters":                   chessNS + "experimenters",
		"sample_name":                     chessNS + "sampleName",
		"sample_common_name":              chessNS + "sampleCommonName",
		"sample_geometry":                 chessNS + "sampleGeometry",
		"sample_mat_ped_heat_treatment":   chessNS + "heatTreatment",
		"sample_mat_ped_processing_route": chessNS + "processingRoute",
		"beam_energy":                     chessNS + "beamEnergy",
		"date":                            nsDCT + "date",
		"doi":                             nsDCT + "relation",
		"data_location_raw":               chessNS + "dataLocationRaw",
		"data_location_reduced":           chessNS + "dataLocationReduced",
		"data_location_meta":              chessNS + "dataLocationMeta",
		"data_location_scratch":           chessNS + "dataLocationScratch",
	}
}

func staticBoolMap(chessNS string) map[string]string {
	return map[string]string{
		"alignment":       chessNS + "alignment",
		"calibration":     chessNS + "calibration",
		"in_situ":         chessNS + "inSitu",
		"mechanical_test": chessNS + "mechanicalTest",
		"doi_public":      chessNS + "doiPublic",
	}
}

func staticArrayMap(chessNS string) map[string]string {
	return map[string]string{
		"beamline":                 chessNS + "beamline",
		"beamline_funding_partner": chessNS + "fundingPartner",
		"cesr_conditions":          chessNS + "cesrConditions",
		"detectors":                chessNS + "detector",
		"experiment_type":          chessNS + "experimentType",
		"focusing":                 chessNS + "focusing",
		"furnace":                  chessNS + "furnace",
		"insertion_device":         chessNS + "insertionDevice",
		"mechanical_grips":         chessNS + "mechanicalGrips",
		"mechanical_load_frame":    chessNS + "mechanicalLoadFrame",
		"mechanical_test_type":     chessNS + "mechanicalTestType",
		"monochromator":            chessNS + "monochromator",
		"processing_environment":   chessNS + "processingEnvironment",
		"staff_scientist":          chessNS + "staffScientist",
		"supplementary_technique":  chessNS + "supplementaryTechnique",
		"technique":                chessNS + "technique",
		"sample_state":             chessNS + "sampleState",
	}
}

// mergeFieldMaps merges the schema-derived FieldMaps into the static maps,
// with static entries taking precedence (they must not be overridden by
// schema files because they carry deliberately chosen vocabulary mappings).
func mergeFieldMaps(fm *FieldMaps, chessNS string) *FieldMaps {
	merged := &FieldMaps{
		Scalar:       staticScalarMap(chessNS),
		Bool:         staticBoolMap(chessNS),
		Array:        staticArrayMap(chessNS),
		NumericArray: make(map[string]string),
		NumericType:  make(map[string]string),
	}

	// Add schema-derived entries that are not already covered by static maps.
	for k, v := range fm.Scalar {
		if _, exists := merged.Scalar[k]; !exists {
			merged.Scalar[k] = v
		}
	}
	for k, v := range fm.Bool {
		if _, exists := merged.Bool[k]; !exists {
			merged.Bool[k] = v
		}
	}
	for k, v := range fm.Array {
		if _, exists := merged.Array[k]; !exists {
			merged.Array[k] = v
		}
	}
	// NumericArray has no static entries; all come from schema files.
	for k, v := range fm.NumericArray {
		merged.NumericArray[k] = v
		merged.NumericType[k] = fm.NumericType[k]
	}
	// Propagate scalar numeric types from schema-derived maps.
	for k, t := range fm.NumericType {
		if _, inNumArr := fm.NumericArray[k]; !inNumArr {
			merged.NumericType[k] = t
		}
	}
	return merged
}

// ──────────────────────────────────────────────────────────────────────────────
// RecordToTriples
// ──────────────────────────────────────────────────────────────────────────────

// RecordToTriples converts one FOXDEN metadata record into a slice of
// store.Triple values, all tagged with graphIRI.
//
// iriBase is the node-wide IRI prefix from Config.Node.IRIBase
// (e.g. "http://chess.cornell.edu/"). It must be non-empty and end with "/".
//
// fm is the FieldMaps produced by LoadFieldMaps (may be nil or empty, in which
// case only the built-in static maps are used).
//
// Returns an error only if the record is missing its DID.
func RecordToTriples(rec Record, graphIRI, iriBase string, fm *FieldMaps) ([]store.Triple, error) {
	chessBase := strings.TrimSuffix(iriBase, "/") + "/"
	chessNS := strings.TrimSuffix(iriBase, "/") + "/ns#"

	// Merge schema-derived maps with static built-ins.
	if fm == nil {
		fm = &FieldMaps{
			Scalar: make(map[string]string), Bool: make(map[string]string),
			Array: make(map[string]string), NumericArray: make(map[string]string),
			NumericType: make(map[string]string),
		}
	}
	maps := mergeFieldMaps(fm, chessNS)

	did := rec.DID()
	if did == "" {
		return nil, fmt.Errorf("foxden: record missing 'did' field")
	}

	subj := chessBase + "dataset" + did // IRI for this dataset resource
	var triples []store.Triple

	keys := make(map[string]bool)

	add := func(pred, obj, objType string) {
		if _, ok := keys[pred]; ok {
			return
		}
		triples = append(triples, store.Triple{
			Subject:    subj,
			Predicate:  pred,
			Object:     obj,
			ObjectType: objType,
			Graph:      graphIRI,
		})
		keys[pred] = true
	}
	addLit := func(pred, val string) { add(pred, val, "literal") }
	addTypedLit := func(pred, val, dtype string) {
		if _, ok := keys[pred]; ok {
			return
		}
		triples = append(triples, store.Triple{
			Subject:    subj,
			Predicate:  pred,
			Object:     val,
			ObjectType: "literal",
			Datatype:   dtype,
			Graph:      graphIRI,
		})
		keys[pred] = true
	}
	addURI := func(pred, val string) { add(pred, val, "uri") }

	// rdf:type
	addURI(nsRDF+"type", chessBase+"Dataset")

	// dct:identifier → DID
	addLit(nsDCT+"identifier", did)

	// ── Scalar fields ─────────────────────────────────────────────────────────
	for field, pred := range maps.Scalar {
		val, ok := rec[field]
		if !ok {
			continue
		}
		if field == "date" {
			// convert value to ISO date format
			if vstr, err := UnixToISO8601(val); err == nil {
				addTypedLit(pred, vstr, nsXSD+"dateTime")
				continue
			}
		}
		switch v := val.(type) {
		case string:
			if v != "" {
				addLit(pred, v)
			}
		case float64:
			xsdType := nsXSD + "decimal"
			if maps.NumericType[field] == "int64" {
				xsdType = nsXSD + "integer"
			}
			//addLit(pred, fmt.Sprintf("%g^^%s", v, xsdType))
			addTypedLit(pred, fmt.Sprintf("%g", v), xsdType)
		case bool:
			//addLit(pred, fmt.Sprintf("%t^^%sboolean", v, nsXSD))
			addTypedLit(pred, fmt.Sprintf("%t", v), nsXSD+"boolean")
		}
	}

	// ── Boolean flags ─────────────────────────────────────────────────────────
	for field, pred := range maps.Bool {
		if val, ok := rec[field].(bool); ok {
			//addLit(pred, fmt.Sprintf("%t^^%sboolean", val, nsXSD))
			addTypedLit(pred, fmt.Sprintf("%t", val), nsXSD+"boolean")
		}
	}

	// ── Array fields (strings) ────────────────────────────────────────────────
	for field, pred := range maps.Array {
		arr, ok := rec[field].([]any)
		if !ok {
			continue
		}
		for _, elem := range arr {
			if s, ok := elem.(string); ok && s != "" {
				addLit(pred, s)
			}
		}
	}

	// ── Numeric array fields ──────────────────────────────────────────────────
	for field, pred := range maps.NumericArray {
		arr, ok := rec[field].([]any)
		if !ok {
			continue
		}
		xsdType := nsXSD + "decimal"
		if maps.NumericType[field] == "int64" {
			xsdType = nsXSD + "integer"
		}
		for _, elem := range arr {
			if v, ok := elem.(float64); ok {
				//addLit(pred, fmt.Sprintf("%g^^%s", v, xsdType))
				addTypedLit(pred, fmt.Sprintf("%g", v), xsdType)
			}
		}
	}

	// ── Nested: sample_crystallographic_phases ────────────────────────────────
	if phases, ok := rec["sample_crystallographic_phases"].([]any); ok {
		for i, p := range phases {
			phase, ok := p.(map[string]any)
			if !ok {
				continue
			}
			phaseIRI := fmt.Sprintf("%sdataset%s/phase/%d", chessBase, did, i)
			addURI(chessNS+"crystallographicPhase", phaseIRI)
			if name, ok := phase["name"].(string); ok {
				triples = append(triples, store.Triple{
					Subject: phaseIRI, Predicate: nsRDFS + "label",
					Object: name, ObjectType: "literal", Graph: graphIRI,
				})
			}
			if sg, ok := phase["space_group"].(int64); ok {
				triples = append(triples, store.Triple{
					Subject: phaseIRI, Predicate: chessNS + "spaceGroup",
					//Object:     fmt.Sprintf("%g^^%sinteger", sg, nsXSD),
					Object:     fmt.Sprintf("%g", sg),
					Datatype:   nsXSD + "integer",
					ObjectType: "literal", Graph: graphIRI,
				})
			}
		}
	}

	// ── Nested: sample_unit_cell (list of floats → one triple per dimension) ──
	if cell, ok := rec["sample_unit_cell"].([]any); ok && len(cell) > 0 {
		labels := []string{"a", "b", "c", "alpha", "beta", "gamma"}
		for i, v := range cell {
			if f, ok := v.(float64); ok {
				label := fmt.Sprintf("unitCell_%d", i)
				if i < len(labels) {
					label = "unitCell_" + labels[i]
				}
				//addLit(chessNS+label, fmt.Sprintf("%g^^%sdecimal", f, nsXSD))
				addTypedLit(chessNS+label, fmt.Sprintf("%g", f), nsXSD+"decimal")
			}
		}
	}

	// ── sample_space_group (int array → one triple per phase group) ───────────
	if sgs, ok := rec["sample_space_group"].([]any); ok {
		for _, v := range sgs {
			if f, ok := v.(float64); ok {
				//addLit(chessNS+"sampleSpaceGroup", fmt.Sprintf("%g^^%sinteger", f, nsXSD))
				addTypedLit(chessNS+"sampleSpaceGroup", fmt.Sprintf("%g", f), nsXSD+"integer")
			}
		}
	}

	return triples, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// GraphIRIFromDID
// ──────────────────────────────────────────────────────────────────────────────

// GraphIRIFromDID derives the named-graph IRI from a raw FOXDEN DID string.
// graphIRIBase is the node-wide IRI prefix from Config.Node.IRIBase — must be
// non-empty and end with a trailing slash.
//
// DID format: /beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-...
// IRI format: <graphIRIBase>graph/3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-...
func GraphIRIFromDID(did, graphIRIBase string) string {
	base := strings.TrimSuffix(graphIRIBase, "/")

	trimmed := strings.TrimPrefix(did, "/")
	parts := strings.SplitN(trimmed, "/", 2) // ["beamline=3a", "btr=.../.../..."]
	if len(parts) < 2 {
		// Malformed DID — fall back to a UUID-keyed graph
		return base + "/graph/unknown/" + uuid.NewString()
	}
	// Extract beamline value from first segment
	bl := ""
	if idx := strings.IndexByte(parts[0], '='); idx >= 0 {
		bl = strings.ToLower(parts[0][idx+1:])
	}
	return fmt.Sprintf("%s/graph/%s/%s", base, bl, parts[1])
}

// UnixToISO8601 converts values like "1.769456896e+09" into RFC3339 / ISO8601.
func UnixToISO8601(v any) (string, error) {
	var ts float64

	switch val := v.(type) {
	case float64:
		ts = val
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return "", fmt.Errorf("invalid unix timestamp string: %w", err)
		}
		ts = f
	default:
		return "", fmt.Errorf("unsupported type %T", v)
	}

	// assume seconds (not milliseconds)
	t := time.Unix(int64(ts), int64((ts-float64(int64(ts)))*1e9))

	return t.UTC().Format(time.RFC3339), nil
}
