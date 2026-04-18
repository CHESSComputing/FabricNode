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
	nsCHESS = "http://chess.cornell.edu/ns#"
	nsDCT   = "http://purl.org/dc/terms/"
	nsRDF   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsRDFS  = "http://www.w3.org/2000/01/rdf-schema#"
	nsXSD   = "http://www.w3.org/2001/XMLSchema#"
	nsPROV  = "http://www.w3.org/ns/prov#"

	// defaultChessBase is the fallback when no DatasetIRIBase is configured.
	defaultChessBase = "http://chess.cornell.edu/"
)

// RecordToTriples converts one FOXDEN metadata record into a slice of
// store.Triple values, all tagged with graphIRI.
// datasetIRIBase is the configurable prefix for dataset subject IRIs
// (from DataServiceConfig.DatasetIRIBase); pass "" to use the default.
// Returns an error only if the record is missing its DID.
func RecordToTriples(rec Record, graphIRI, datasetIRIBase string) ([]store.Triple, error) {
	if datasetIRIBase == "" {
		datasetIRIBase = defaultChessBase
	}
	chessBase := strings.TrimSuffix(datasetIRIBase, "/") + "/"

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
		"btr":                             nsCHESS + "btr",
		"cycle":                           nsCHESS + "cycle",
		"schema":                          nsCHESS + "schema",
		"facility":                        nsCHESS + "facility",
		"user":                            nsCHESS + "user",
		"pi":                              nsDCT + "creator",
		"experimenters":                   nsCHESS + "experimenters",
		"sample_name":                     nsCHESS + "sampleName",
		"sample_common_name":              nsCHESS + "sampleCommonName",
		"sample_geometry":                 nsCHESS + "sampleGeometry",
		"sample_state":                    nsCHESS + "sampleState",
		"sample_mat_ped_heat_treatment":   nsCHESS + "heatTreatment",
		"sample_mat_ped_processing_route": nsCHESS + "processingRoute",
		"beam_energy":                     nsCHESS + "beamEnergy",
		"date":                            nsDCT + "date",
		"doi":                             nsDCT + "relation",
		"data_location_raw":               nsCHESS + "dataLocationRaw",
		"data_location_reduced":           nsCHESS + "dataLocationReduced",
		"data_location_meta":              nsCHESS + "dataLocationMeta",
		"data_location_scratch":           nsCHESS + "dataLocationScratch",
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
		"alignment":       nsCHESS + "alignment",
		"calibration":     nsCHESS + "calibration",
		"in_situ":         nsCHESS + "inSitu",
		"mechanical_test": nsCHESS + "mechanicalTest",
		"doi_public":      nsCHESS + "doiPublic",
	}
	for field, pred := range boolMap {
		if val, ok := rec[field].(bool); ok {
			addLit(pred, fmt.Sprintf("%t^^%sboolean", val, nsXSD))
		}
	}

	// Array fields — one triple per element
	arrayMap := map[string]string{
		"beamline":                 nsCHESS + "beamline",
		"beamline_funding_partner": nsCHESS + "fundingPartner",
		"cesr_conditions":          nsCHESS + "cesrConditions",
		"detectors":                nsCHESS + "detector",
		"experiment_type":          nsCHESS + "experimentType",
		"focusing":                 nsCHESS + "focusing",
		"furnace":                  nsCHESS + "furnace",
		"insertion_device":         nsCHESS + "insertionDevice",
		"mechanical_grips":         nsCHESS + "mechanicalGrips",
		"mechanical_load_frame":    nsCHESS + "mechanicalLoadFrame",
		"mechanical_test_type":     nsCHESS + "mechanicalTestType",
		"monochromator":            nsCHESS + "monochromator",
		"processing_environment":   nsCHESS + "processingEnvironment",
		"staff_scientist":          nsCHESS + "staffScientist",
		"supplementary_technique":  nsCHESS + "supplementaryTechnique",
		"technique":                nsCHESS + "technique",
		"sample_state":             nsCHESS + "sampleState",
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
			addURI(nsCHESS+"crystallographicPhase", phaseIRI)
			if name, ok := phase["name"].(string); ok {
				triples = append(triples, store.Triple{
					Subject: phaseIRI, Predicate: nsRDFS + "label",
					Object: name, ObjectType: "literal", Graph: graphIRI,
				})
			}
			if sg, ok := phase["space_group"].(float64); ok {
				triples = append(triples, store.Triple{
					Subject: phaseIRI, Predicate: nsCHESS + "spaceGroup",
					Object:     fmt.Sprintf("%g^^%sinteger", sg, nsXSD),
					ObjectType: "literal", Graph: graphIRI,
				})
			}
		}
	}

	return triples, nil
}

// GraphIRIFromDID derives the named-graph IRI from a raw FOXDEN DID string.
// graphIRIBase is the configurable prefix (from DataServiceConfig.GraphIRIBase);
// pass "" to use the default "http://chess.cornell.edu/".
//
// DID format: /beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-...
// IRI format:  http://chess.cornell.edu/graph/3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-...
func GraphIRIFromDID(did, graphIRIBase string) string {
	if graphIRIBase == "" {
		graphIRIBase = defaultChessBase
	}
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
