// Package foxden — rdf.go converts FOXDEN metadata records into RDF triples
// suitable for insertion into the data-service triple store.
//
// Mapping decisions:
//   - Each FOXDEN record becomes a single named-graph keyed by its DID.
//   - Scalar string/numeric fields become plain literal triples.
//   - Array fields (beamline, detectors, technique, …) emit one triple per element.
//   - Nested objects (sample_crystallographic_phases) are flattened into
//     blank-node-style IRIs to avoid losing structure.
//   - The chess: namespace is used for CHESS-specific predicates that have no
//     standard vocabulary mapping.  Well-known fields (pi, cycle, schema) are
//     mapped to dct: / sosa: where appropriate.
package foxden

import (
	"fmt"
	"strings"

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

// RecordToTriples converts one FOXDEN metadata record into a slice of
// store.Triple values, all tagged with graphIRI.
// iriBase is the node-wide IRI prefix from Config.Node.IRIBase
// (e.g. "http://chess.cornell.edu/"). It must be non-empty and end with "/".
// Returns an error only if the record is missing its DID.
func RecordToTriples(rec Record, graphIRI, iriBase string) ([]store.Triple, error) {
	chessBase := strings.TrimSuffix(iriBase, "/") + "/"
	chessNS := strings.TrimSuffix(iriBase, "/") + "/ns#"

	did := rec.DID()
	if did == "" {
		return nil, fmt.Errorf("foxden: record missing 'did' field")
	}

	subj := chessBase + "dataset" + did // IRI for this dataset resource
	var triples []store.Triple

	add := func(pred, obj, objType string) {
		triples = append(triples, store.Triple{
			Subject:    subj,
			Predicate:  pred,
			Object:     obj,
			ObjectType: objType,
			Graph:      graphIRI,
		})
	}
	addLit := func(pred, val string) { add(pred, val, "literal") }
	addURI := func(pred, val string) { add(pred, val, "uri") }

	// rdf:type
	addURI(nsRDF+"type", chessBase+"Dataset")

	// dct:identifier → DID
	addLit(nsDCT+"identifier", did)

	// Well-known scalar fields
	scalarMap := map[string]string{
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
		"sample_state":                    chessNS + "sampleState",
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

	for field, pred := range scalarMap {
		val, ok := rec[field]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case string:
			if v != "" {
				addLit(pred, v)
			}
		case float64:
			addLit(pred, fmt.Sprintf("%g^^%sdecimal", v, nsXSD))
		case bool:
			addLit(pred, fmt.Sprintf("%t^^%sboolean", v, nsXSD))
		}
	}

	// Boolean flags
	boolMap := map[string]string{
		"alignment":       chessNS + "alignment",
		"calibration":     chessNS + "calibration",
		"in_situ":         chessNS + "inSitu",
		"mechanical_test": chessNS + "mechanicalTest",
		"doi_public":      chessNS + "doiPublic",
	}
	for field, pred := range boolMap {
		if val, ok := rec[field].(bool); ok {
			addLit(pred, fmt.Sprintf("%t^^%sboolean", val, nsXSD))
		}
	}

	// Array fields — one triple per element
	arrayMap := map[string]string{
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
	for field, pred := range arrayMap {
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

	// Nested: sample_crystallographic_phases
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
			if sg, ok := phase["space_group"].(float64); ok {
				triples = append(triples, store.Triple{
					Subject: phaseIRI, Predicate: chessNS + "spaceGroup",
					Object:     fmt.Sprintf("%g^^%sinteger", sg, nsXSD),
					ObjectType: "literal", Graph: graphIRI,
				})
			}
		}
	}

	return triples, nil
}

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
