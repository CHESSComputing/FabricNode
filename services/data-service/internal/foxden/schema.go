// Package foxden — schema.go loads FOXDEN beamline schema JSON files and
// derives the RDF field maps used by RecordToTriples.
//
// Schema file format (array of field descriptors):
//
//	[
//	  { "key": "beam_energy", "type": "float64", "multiple": false, ... },
//	  { "key": "detectors",   "type": "list_str", "multiple": true,  ... },
//	  ...
//	]
//
// Type → RDF mapping rules:
//
//	"bool"                         → boolMap    (xsd:boolean literal)
//	"float64" / "int64", single    → scalarMap  (xsd:decimal / xsd:integer literal)
//	"float64" / "int64", multiple  → numericArrayMap (one literal triple per element)
//	"string",   single             → scalarMap  (plain literal)
//	"string",   multiple           → arrayMap   (one literal triple per element)
//	"list_str", single             → scalarMap  (single-select enumeration = scalar)
//	"list_str", multiple           → arrayMap   (multi-select = one triple per value)
//	"list_float", multiple         → numericArrayMap
//
// Special keys that are handled elsewhere and skipped here:
//
//	did, sample_crystallographic_phases, sample_unit_cell, sample_space_group
package foxden

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"unicode"
)

// FieldMaps holds the four RDF field-mapping tables derived from schema files.
// Every map key is a FOXDEN record field name; every value is the full RDF
// predicate IRI string (already constructed with the chess: namespace prefix).
type FieldMaps struct {
	// Scalar maps field → predicate for single-value fields.
	// Value type: string or numeric (formatted as xsd:decimal/integer literal).
	Scalar map[string]string

	// Bool maps field → predicate for boolean fields.
	Bool map[string]string

	// Array maps field → predicate for multi-value string/list_str fields.
	// One triple is emitted per array element.
	Array map[string]string

	// NumericArray maps field → predicate for multi-value float64/int64 fields.
	// One xsd:decimal or xsd:integer triple is emitted per array element.
	NumericArray map[string]string

	// NumericType records whether a numeric field uses "float64" or "int64",
	// keyed by field name.  Used when formatting literal values.
	NumericType map[string]string
}

// schemaField mirrors one entry in a FOXDEN schema JSON file.
type schemaField struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Multiple bool   `json:"multiple"`
}

// staticSkip lists fields that are handled by hand in RecordToTriples and must
// not be auto-generated into any map.
var staticSkip = map[string]bool{
	"did":                            true,
	"sample_crystallographic_phases": true,
	"sample_unit_cell":               true,
	"sample_space_group":             true,
}

// toCamelCase converts a snake_case field name to lowerCamelCase for use as
// an RDF local name, e.g. "beam_energy" → "beamEnergy".
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return s
	}
	var b strings.Builder
	for i, p := range parts {
		if i == 0 {
			b.WriteString(p)
			continue
		}
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		b.WriteRune(unicode.ToUpper(runes[0]))
		b.WriteString(string(runes[1:]))
	}
	return b.String()
}

// LoadFieldMaps reads all *.json files from schemasDir, merges their field
// descriptors, and returns a FieldMaps ready for use in RecordToTriples.
// chessNS is the RDF namespace IRI (e.g. "http://chess.cornell.edu/ns#").
//
// Fields that appear in multiple schema files are deduplicated; the first
// occurrence wins.  An error is returned only if schemasDir is non-empty and
// cannot be read at all; individual file parse errors are logged and skipped.
func LoadFieldMaps(schemaFiles []string, chessNS string) (*FieldMaps, error) {
	fm := &FieldMaps{
		Scalar:       make(map[string]string),
		Bool:         make(map[string]string),
		Array:        make(map[string]string),
		NumericArray: make(map[string]string),
		NumericType:  make(map[string]string),
	}

	if len(schemaFiles) == 0 {
		return fm, nil
	}

	loaded := 0
	for _, path := range schemaFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("foxden schema: skip %s: %v", path, err)
			continue
		}
		var fields []schemaField
		if err := json.Unmarshal(data, &fields); err != nil {
			log.Printf("foxden schema: parse %s: %v", path, err)
			continue
		}
		for _, f := range fields {
			if staticSkip[f.Key] {
				continue
			}
			// Skip if already registered from a previous schema file.
			if _, dup := fm.Scalar[f.Key]; dup {
				continue
			}
			if _, dup := fm.Bool[f.Key]; dup {
				continue
			}
			if _, dup := fm.Array[f.Key]; dup {
				continue
			}
			if _, dup := fm.NumericArray[f.Key]; dup {
				continue
			}

			pred := chessNS + toCamelCase(f.Key)

			switch {
			case f.Type == "bool":
				fm.Bool[f.Key] = pred

			case (f.Type == "float64" || f.Type == "int64") && f.Multiple:
				fm.NumericArray[f.Key] = pred
				fm.NumericType[f.Key] = f.Type

			case f.Type == "float64" || f.Type == "int64":
				fm.Scalar[f.Key] = pred
				fm.NumericType[f.Key] = f.Type

			case (f.Type == "list_str" || f.Type == "list_float") && f.Multiple:
				fm.Array[f.Key] = pred

			case f.Type == "string" && f.Multiple:
				fm.Array[f.Key] = pred

			default:
				// string single, list_str single → scalar
				fm.Scalar[f.Key] = pred
			}
		}
		loaded++
		log.Printf("foxden schema: loaded %s (%d fields)", path, len(fields))
	}

	return fm, nil
}
