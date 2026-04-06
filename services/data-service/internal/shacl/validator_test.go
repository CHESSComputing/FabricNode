package shacl_test

import (
	"strings"
	"testing"

	"github.com/CHESSComputing/FabricNode/pkg/model"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/shacl"
	"github.com/CHESSComputing/FabricNode/services/data-service/internal/store"
)

// ── helpers ───────────────────────────────────────────────────────────────────

const (
	rdfType          = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	sosaObservation  = "http://www.w3.org/ns/sosa/Observation"
	sosaResultTime   = "http://www.w3.org/ns/sosa/resultTime"
	sosaMadeBySensor = "http://www.w3.org/ns/sosa/madeBySensor"
	sosaObsProp      = "http://www.w3.org/ns/sosa/observedProperty"
	chessBase        = "http://chess.cornell.edu/"
)

// validObservation returns a minimal set of triples that passes all shape checks
// for the given subject IRI and beamline sensor prefix.
func validObservation(subj, sensorIRI string) []store.Triple {
	return []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaResultTime, Object: "2024-03-01T10:00:00Z^^http://www.w3.org/2001/XMLSchema#dateTime", ObjectType: "literal"},
		{Subject: subj, Predicate: sosaMadeBySensor, Object: sensorIRI, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaObsProp, Object: chessBase + "property/peak-intensity", ObjectType: "uri"},
	}
}

// ── ValidateObservation ───────────────────────────────────────────────────────

func TestValidateObservation_ValidMinimal(t *testing.T) {
	triples := validObservation("http://example.org/obs/1", chessBase+"sensor/id1-detector-01")
	result := shacl.ValidateObservation(triples)
	if !result.Conforms {
		t.Errorf("expected Conforms=true, got errors: %v", result.Errors)
	}
}

func TestValidateObservation_NonObservationTriples(t *testing.T) {
	// Triples that are not sosa:Observation should be silently ignored.
	triples := []store.Triple{
		{Subject: "http://example.org/sensor/1", Predicate: rdfType, Object: chessBase + "Sensor", ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if !result.Conforms {
		t.Errorf("non-Observation triples should not trigger errors, got: %v", result.Errors)
	}
}

func TestValidateObservation_MissingResultTime(t *testing.T) {
	subj := "http://example.org/obs/missing-time"
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaMadeBySensor, Object: chessBase + "sensor/id1-det", ObjectType: "uri"},
		{Subject: subj, Predicate: sosaObsProp, Object: chessBase + "property/x", ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false for missing resultTime")
	}
	if !containsSubstring(result.Errors, "sosa:resultTime") {
		t.Errorf("expected error mentioning sosa:resultTime, got: %v", result.Errors)
	}
}

func TestValidateObservation_MissingMadeBySensor(t *testing.T) {
	subj := "http://example.org/obs/no-sensor"
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaResultTime, Object: "2024-01-01T00:00:00Z^^xsd:dateTime", ObjectType: "literal"},
		{Subject: subj, Predicate: sosaObsProp, Object: chessBase + "property/x", ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false for missing madeBySensor")
	}
	if !containsSubstring(result.Errors, "sosa:madeBySensor") {
		t.Errorf("expected error mentioning sosa:madeBySensor, got: %v", result.Errors)
	}
}

func TestValidateObservation_MissingObservedProperty(t *testing.T) {
	subj := "http://example.org/obs/no-prop"
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaResultTime, Object: "2024-01-01T00:00:00Z^^xsd:dateTime", ObjectType: "literal"},
		{Subject: subj, Predicate: sosaMadeBySensor, Object: chessBase + "sensor/id1-det", ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false for missing observedProperty")
	}
	if !containsSubstring(result.Errors, "sosa:observedProperty") {
		t.Errorf("expected error mentioning sosa:observedProperty, got: %v", result.Errors)
	}
}

func TestValidateObservation_InvalidResultTimeFormat(t *testing.T) {
	subj := "http://example.org/obs/bad-time"
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		// Value lacks "T" — not a valid xsd:dateTime
		{Subject: subj, Predicate: sosaResultTime, Object: "2024-03-01", ObjectType: "literal"},
		{Subject: subj, Predicate: sosaMadeBySensor, Object: chessBase + "sensor/id1-det", ObjectType: "uri"},
		{Subject: subj, Predicate: sosaObsProp, Object: chessBase + "property/x", ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false for non-dateTime resultTime")
	}
	if !containsSubstring(result.Errors, "xsd:dateTime") {
		t.Errorf("expected xsd:dateTime error, got: %v", result.Errors)
	}
}

func TestValidateObservation_MultipleErrors(t *testing.T) {
	subj := "http://example.org/obs/many-errors"
	// Only rdf:type — missing all three required predicates.
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
	}
	result := shacl.ValidateObservation(triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false")
	}
	if len(result.Errors) < 3 {
		t.Errorf("expected ≥3 errors for three missing fields, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestValidateObservation_MultipleSubjects(t *testing.T) {
	// Two observations in one batch — both valid.
	obs1 := validObservation("http://example.org/obs/a", chessBase+"sensor/id1-det")
	obs2 := validObservation("http://example.org/obs/b", chessBase+"sensor/id3a-det")
	result := shacl.ValidateObservation(append(obs1, obs2...))
	if !result.Conforms {
		t.Errorf("two valid observations failed: %v", result.Errors)
	}
}

func TestValidateObservation_OneValidOneInvalid(t *testing.T) {
	good := validObservation("http://example.org/obs/good", chessBase+"sensor/id1-det")
	// Bad: missing madeBySensor and observedProperty.
	badSubj := "http://example.org/obs/bad"
	bad := []store.Triple{
		{Subject: badSubj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: badSubj, Predicate: sosaResultTime, Object: "2024-01-01T00:00:00Z^^xsd:dateTime", ObjectType: "literal"},
	}
	result := shacl.ValidateObservation(append(good, bad...))
	if result.Conforms {
		t.Fatal("expected Conforms=false when one observation is invalid")
	}
}

// ── ValidateForDataset ────────────────────────────────────────────────────────

func TestValidateForDataset_ValidSensorForBeamline(t *testing.T) {
	ref := model.DatasetRef{
		Beamline: "id1",
		DID:      "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std",
	}
	// Sensor IRI correctly prefixed for id1.
	triples := validObservation("http://example.org/obs/1", chessBase+"sensor/id1-detector-01")
	result := shacl.ValidateForDataset(ref, triples)
	if !result.Conforms {
		t.Errorf("expected Conforms=true, got: %v", result.Errors)
	}
}

func TestValidateForDataset_SensorWrongBeamline(t *testing.T) {
	ref := model.DatasetRef{
		Beamline: "id1",
		DID:      "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std",
	}
	// Sensor belongs to id3a, not id1.
	triples := validObservation("http://example.org/obs/1", chessBase+"sensor/id3a-detector-01")
	result := shacl.ValidateForDataset(ref, triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false for sensor from wrong beamline")
	}
	if !containsSubstring(result.Errors, "id3a") {
		t.Errorf("expected error mentioning sensor IRI, got: %v", result.Errors)
	}
}

func TestValidateForDataset_ObservationShapeFailsPropagates(t *testing.T) {
	ref := model.DatasetRef{
		Beamline: "id1",
		DID:      "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std",
	}
	// Missing resultTime — shape check should fail before sensor scope check.
	subj := "http://example.org/obs/shape-fail"
	triples := []store.Triple{
		{Subject: subj, Predicate: rdfType, Object: sosaObservation, ObjectType: "uri"},
		{Subject: subj, Predicate: sosaMadeBySensor, Object: chessBase + "sensor/id1-det", ObjectType: "uri"},
		{Subject: subj, Predicate: sosaObsProp, Object: chessBase + "property/x", ObjectType: "uri"},
	}
	result := shacl.ValidateForDataset(ref, triples)
	if result.Conforms {
		t.Fatal("expected Conforms=false when shape check fails")
	}
}

func TestValidateForDataset_CaseNormalisation(t *testing.T) {
	// Beamline IDs are stored lower-case; sensor prefix check must be case-insensitive.
	ref := model.DatasetRef{
		Beamline: "fast",
		DID:      "/beamline=fast/btr=btr201/cycle=2024-3/sample_name=fe3o4",
	}
	triples := validObservation("http://example.org/obs/1", chessBase+"sensor/fast-detector-01")
	result := shacl.ValidateForDataset(ref, triples)
	if !result.Conforms {
		t.Errorf("expected Conforms=true, got: %v", result.Errors)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsSubstring(msgs []string, sub string) bool {
	for _, m := range msgs {
		if strings.Contains(m, sub) {
			return true
		}
	}
	return false
}
