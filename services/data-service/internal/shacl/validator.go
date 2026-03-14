// Package shacl provides structural validation of incoming triples against
// the CHESS ObservationShape and BeamlineShape rules.
//
// This is a hand-written subset of SHACL 1.2 validation. In production,
// delegate to a full SHACL validator (e.g. a Java/Python sidecar or
// Apache Jena Shapes).
package shacl

import (
	"fmt"
	"strings"

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

// ValidateObservation checks that the supplied triples satisfy fabric:ObservationShape.
// The triples slice must represent a single sosa:Observation resource.
func ValidateObservation(triples []store.Triple) ValidationResult {
	r := ValidationResult{Conforms: true}
	subjectTriples := indexBySubject(triples)

	for subj, props := range subjectTriples {
		// Must be typed sosa:Observation
		types := getValues(props, nsRDF+"type")
		if !contains(types, nsSOSA+"Observation") {
			continue // not an observation — skip
		}
		// sh:minCount 1 for sosa:resultTime
		if len(getValues(props, nsSOSA+"resultTime")) == 0 {
			r.Conforms = false
			r.Errors = append(r.Errors,
				fmt.Sprintf("<%s>: missing required sosa:resultTime", subj))
		}
		// sosa:resultTime must look like an xsd:dateTime (contains "T")
		for _, v := range getValues(props, nsSOSA+"resultTime") {
			if !strings.Contains(v, "T") {
				r.Conforms = false
				r.Errors = append(r.Errors,
					fmt.Sprintf("<%s>: sosa:resultTime value %q does not look like xsd:dateTime", subj, v))
			}
		}
		// sh:minCount 1 for sosa:madeBySensor
		if len(getValues(props, nsSOSA+"madeBySensor")) == 0 {
			r.Conforms = false
			r.Errors = append(r.Errors,
				fmt.Sprintf("<%s>: missing required sosa:madeBySensor", subj))
		}
		// sh:minCount 1 for sosa:observedProperty
		if len(getValues(props, nsSOSA+"observedProperty")) == 0 {
			r.Conforms = false
			r.Errors = append(r.Errors,
				fmt.Sprintf("<%s>: missing required sosa:observedProperty", subj))
		}
	}
	return r
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

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
