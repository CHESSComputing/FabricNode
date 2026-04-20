// Package shacl provides structural validation of incoming triples against
// the ObservationShape rules, with optional beamline/dataset scope
// enforcement.
package shacl

import (
	"fmt"
	"strings"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

const (
	nsRDF  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsSOSA = "http://www.w3.org/ns/sosa/"
	nsXSD  = "http://www.w3.org/2001/XMLSchema#"
)

// ValidationResult holds the outcome of a SHACL shape check.
type ValidationResult struct {
	Conforms bool     `json:"conforms"`
	Errors   []string `json:"errors,omitempty"`
}

// ValidateObservation checks that the supplied triples satisfy
// fabric:ObservationShape.  The triples slice must represent one or more
// sosa:Observation resources.
func ValidateObservation(triples []store.Triple) ValidationResult {
	r := ValidationResult{Conforms: true}
	subjectTriples := indexBySubject(triples)

	for subj, props := range subjectTriples {
		types := getValues(props, nsRDF+"type")
		if !contains(types, nsSOSA+"Observation") {
			continue
		}
		if len(getValues(props, nsSOSA+"resultTime")) == 0 {
			r.fail(fmt.Sprintf("<%s>: missing required sosa:resultTime", subj))
		}
		for _, v := range getValues(props, nsSOSA+"resultTime") {
			if !strings.Contains(v, "T") {
				r.fail(fmt.Sprintf("<%s>: sosa:resultTime %q is not xsd:dateTime", subj, v))
			}
		}
		if len(getValues(props, nsSOSA+"madeBySensor")) == 0 {
			r.fail(fmt.Sprintf("<%s>: missing required sosa:madeBySensor", subj))
		}
		if len(getValues(props, nsSOSA+"observedProperty")) == 0 {
			r.fail(fmt.Sprintf("<%s>: missing required sosa:observedProperty", subj))
		}
	}
	return r
}

// ValidateForDataset runs observation validation AND checks that every
// sosa:madeBySensor value is hosted by the declared beamline.
// iriBase is the node-wide IRI prefix from Config.Node.IRIBase
// (e.g. "http://chess.cornell.edu/"), used to construct expected sensor IRI prefixes.
// This is the preferred entry point when writing via a beamline-scoped route.
func ValidateForDataset(ref model.DatasetRef, triples []store.Triple, iriBase string) ValidationResult {
	r := ValidateObservation(triples)
	if !r.Conforms {
		return r
	}

	// Sensor IRIs are expected to start with: <iriBase>sensor/<beamline>
	blPrefix := strings.TrimSuffix(iriBase, "/") + "/sensor/" + strings.ToLower(string(ref.Beamline))
	subjectTriples := indexBySubject(triples)
	for subj, props := range subjectTriples {
		types := getValues(props, nsRDF+"type")
		if !contains(types, nsSOSA+"Observation") {
			continue
		}
		for _, sensor := range getValues(props, nsSOSA+"madeBySensor") {
			if !strings.HasPrefix(sensor, blPrefix) {
				r.fail(fmt.Sprintf(
					"<%s>: sensor <%s> does not belong to beamline %q (expected prefix %s)",
					subj, sensor, ref.Beamline, blPrefix))
			}
		}
	}
	return r
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func (r *ValidationResult) fail(msg string) {
	r.Conforms = false
	r.Errors = append(r.Errors, msg)
}

func indexBySubject(triples []store.Triple) map[string][]store.Triple {
	idx := make(map[string][]store.Triple)
	for _, t := range triples {
		idx[t.Subject] = append(idx[t.Subject], t)
	}
	return idx
}

func getValues(triples []store.Triple, predicate string) []string {
	var out []string
	for _, t := range triples {
		if t.Predicate == predicate {
			out = append(out, t.Object)
		}
	}
	return out
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
