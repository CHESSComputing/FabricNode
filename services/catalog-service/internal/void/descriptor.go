// Package void generates the four-layer Knowledge Fabric self-description
// artifacts for the CHESS node:
//
//	L1  VoID dataset description   (/.well-known/void)
//	L1  PROF capability profile    (/.well-known/profile)
//	L3  SHACL shapes               (/.well-known/shacl)
//	L4  SPARQL examples catalog    (/.well-known/sparql-examples)
//
// All returned strings are in Turtle (text/turtle) unless a JSON-LD variant
// is requested; the caller handles content negotiation.
package void

import "fmt"

// NodeConfig holds deployment-specific values injected at startup.
type NodeConfig struct {
	BaseURL  string // e.g. https://chess-node.example.org
	NodeID   string // e.g. chess-node
	NodeName string // human label
}

// DefaultConfig returns a config suitable for local development.
func DefaultConfig() NodeConfig {
	return NodeConfig{
		BaseURL:  "http://localhost:8081",
		NodeID:   "chess-node",
		NodeName: "CHESS Federated Knowledge Fabric Node",
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// L1 — VoID dataset description
// ──────────────────────────────────────────────────────────────────────────────

// VoIDTurtle returns a full VoID + DCAT service description in Turtle.
func VoIDTurtle(cfg NodeConfig) string {
	return fmt.Sprintf(`@prefix void:  <http://rdfs.org/ns/void#> .
@prefix dcat:  <http://www.w3.org/ns/dcat#> .
@prefix dct:   <http://purl.org/dc/terms/> .
@prefix rdf:   <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix xsd:   <http://www.w3.org/2001/XMLSchema#> .
@prefix sosa:  <http://www.w3.org/ns/sosa/> .
@prefix chess: <http://chess.cornell.edu/ns#> .
@prefix fabric: <https://w3id.org/cogitarelink/fabric#> .

# ── Root dataset ──────────────────────────────────────────────────────────────
<%s/dataset>
    a void:Dataset , dcat:Dataset ;
    rdfs:label "%s" ;
    dct:description "Federated knowledge fabric node for CHESS synchrotron beamline data." ;
    dct:publisher <https://www.chess.cornell.edu/> ;
    dct:license <https://creativecommons.org/licenses/by/4.0/> ;
    dct:conformsTo <https://w3id.org/cogitarelink/fabric#CoreProfile> ;
    void:sparqlEndpoint <%s/sparql> ;
    void:uriSpace "http://chess.cornell.edu/" ;
    dcat:landingPage <%s> ;

    # Named graph subsets — one per beamline
    void:subset <%s/dataset/beamline-id1> ,
                <%s/dataset/beamline-id3> ,
                <%s/dataset/beamline-fast> ;

    # Vocabularies used across all graphs
    void:vocabulary <http://www.w3.org/ns/sosa/> ,
                    <http://semanticscience.org/resource/> ,
                    <http://www.w3.org/ns/prov#> ,
                    <http://purl.org/dc/terms/> ;
    dct:modified "%s"^^xsd:date .

# ── Beamline subsets ──────────────────────────────────────────────────────────
<%s/dataset/beamline-id1>
    a void:Dataset ;
    rdfs:label "Beamline ID1 — X-ray Diffraction" ;
    dct:description "X-ray diffraction experiments at CHESS beamline ID1." ;
    dct:conformsTo <https://w3id.org/cogitarelink/fabric#ObservationShape> ;
    void:inDataset <%s/dataset> ;
    void:sparqlEndpoint <%s/sparql> ;
    void:exampleResource <http://chess.cornell.edu/beamline/id1> ;
    chess:beamlineType "x-ray-diffraction" ;
    chess:location "CHESS Wilson Laboratory, Cornell University" .

<%s/dataset/beamline-id3>
    a void:Dataset ;
    rdfs:label "Beamline ID3 — Protein Crystallography" ;
    dct:description "Macromolecular protein crystallography at CHESS beamline ID3." ;
    dct:conformsTo <https://w3id.org/cogitarelink/fabric#ObservationShape> ;
    void:inDataset <%s/dataset> ;
    void:sparqlEndpoint <%s/sparql> ;
    void:exampleResource <http://chess.cornell.edu/beamline/id3> ;
    chess:beamlineType "protein-crystallography" .

<%s/dataset/beamline-fast>
    a void:Dataset ;
    rdfs:label "Beamline FAST — Time-Resolved Scattering" ;
    dct:description "High-cadence time-resolved X-ray scattering at CHESS beamline FAST." ;
    dct:conformsTo <https://w3id.org/cogitarelink/fabric#ObservationShape> ;
    void:inDataset <%s/dataset> ;
    void:sparqlEndpoint <%s/sparql> ;
    void:exampleResource <http://chess.cornell.edu/beamline/fast> ;
    chess:beamlineType "time-resolved-scattering" .
`,
		cfg.BaseURL, cfg.NodeName,
		cfg.BaseURL, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL,
		"2025-01-01",
		cfg.BaseURL, cfg.BaseURL, cfg.BaseURL,
		cfg.BaseURL, cfg.BaseURL, cfg.BaseURL,
		cfg.BaseURL, cfg.BaseURL,
	)
}

// VoIDJSONLD returns a minimal JSON-LD representation of the VoID description.
func VoIDJSONLD(cfg NodeConfig) string {
	return fmt.Sprintf(`{
  "@context": {
    "void":  "http://rdfs.org/ns/void#",
    "dcat":  "http://www.w3.org/ns/dcat#",
    "dct":   "http://purl.org/dc/terms/",
    "rdfs":  "http://www.w3.org/2000/01/rdf-schema#",
    "chess": "http://chess.cornell.edu/ns#"
  },
  "@id": "%s/dataset",
  "@type": ["void:Dataset", "dcat:Dataset"],
  "rdfs:label": "%s",
  "void:sparqlEndpoint": {"@id": "%s/sparql"},
  "dct:conformsTo": {"@id": "https://w3id.org/cogitarelink/fabric#CoreProfile"},
  "void:subset": [
    {
      "@id": "%s/dataset/beamline-id1",
      "@type": "void:Dataset",
      "rdfs:label": "Beamline ID1 — X-ray Diffraction",
      "chess:beamlineType": "x-ray-diffraction"
    },
    {
      "@id": "%s/dataset/beamline-id3",
      "@type": "void:Dataset",
      "rdfs:label": "Beamline ID3 — Protein Crystallography",
      "chess:beamlineType": "protein-crystallography"
    },
    {
      "@id": "%s/dataset/beamline-fast",
      "@type": "void:Dataset",
      "rdfs:label": "Beamline FAST — Time-Resolved Scattering",
      "chess:beamlineType": "time-resolved-scattering"
    }
  ]
}`, cfg.BaseURL, cfg.NodeName, cfg.BaseURL,
		cfg.BaseURL, cfg.BaseURL, cfg.BaseURL)
}

// ──────────────────────────────────────────────────────────────────────────────
// L1 — PROF capability profile
// ──────────────────────────────────────────────────────────────────────────────

// ProfileTurtle returns the PROF resource descriptor for this node.
func ProfileTurtle(cfg NodeConfig) string {
	return fmt.Sprintf(`@prefix prof:  <http://www.w3.org/ns/dx/prof/> .
@prefix role:  <http://www.w3.org/ns/dx/prof/role/> .
@prefix dct:   <http://purl.org/dc/terms/> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .

<https://w3id.org/cogitarelink/fabric#CoreProfile>
    a prof:Profile ;
    rdfs:label "Knowledge Fabric Core Profile" ;
    dct:description "Minimum conformance requirements for a Knowledge Fabric node." ;

    # L1 — service description
    prof:hasResource [
        a prof:ResourceDescriptor ;
        prof:hasRole role:guidance ;
        prof:hasArtifact <%s/.well-known/void> ;
        dct:format "text/turtle"
    ] ;

    # L3 — SHACL shapes
    prof:hasResource [
        a prof:ResourceDescriptor ;
        prof:hasRole role:constraints ;
        prof:hasArtifact <%s/.well-known/shacl> ;
        dct:format "text/turtle"
    ] ;

    # L4 — SPARQL examples
    prof:hasResource [
        a prof:ResourceDescriptor ;
        prof:hasRole role:example ;
        prof:hasArtifact <%s/.well-known/sparql-examples> ;
        dct:format "text/turtle"
    ] .
`, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL)
}

// ──────────────────────────────────────────────────────────────────────────────
// L3 — SHACL shapes
// ──────────────────────────────────────────────────────────────────────────────

// SHACLTurtle returns SHACL NodeShapes for CHESS beamline observations.
func SHACLTurtle(cfg NodeConfig) string {
	return fmt.Sprintf(`@prefix sh:    <http://www.w3.org/ns/shacl#> .
@prefix sosa:  <http://www.w3.org/ns/sosa/> .
@prefix sio:   <http://semanticscience.org/resource/> .
@prefix xsd:   <http://www.w3.org/2001/XMLSchema#> .
@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
@prefix chess: <http://chess.cornell.edu/ns#> .
@prefix fabric: <https://w3id.org/cogitarelink/fabric#> .

# ── ObservationShape ─────────────────────────────────────────────────────────
# Governs all named graphs that contain SOSA Observations at this CHESS node.
# Every write to /graph/observations must pass SHACL validation against this shape.

fabric:ObservationShape
    a sh:NodeShape ;
    rdfs:label "CHESS Beamline Observation Shape" ;
    sh:targetClass sosa:Observation ;

    sh:agentInstruction """
Observations at this CHESS node follow two result patterns:

Pattern A — simple numeric result:
  <obs> sosa:hasSimpleResult "12.3"^^xsd:decimal .

Pattern B — SIO measured value (preferred for calibrated data):
  <obs> sio:has-attribute [
      a sio:MeasuredValue ;
      sio:has-value "12.3"^^xsd:decimal ;
      sio:has-unit <http://qudt.org/vocab/unit/NanoMETER>
  ] .

Always query resultTime (required) and madeBySensor (required).
Use ?p ?o scanning only as a last resort — prefer targeted property queries.
""" ;

    # Required: observation result timestamp
    sh:property [
        sh:path sosa:resultTime ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
        sh:datatype xsd:dateTime ;
        sh:message "Every Observation must have exactly one sosa:resultTime (xsd:dateTime)."
    ] ;

    # Required: sensor that made the observation
    sh:property [
        sh:path sosa:madeBySensor ;
        sh:minCount 1 ;
        sh:message "Every Observation must declare which sensor made it via sosa:madeBySensor."
    ] ;

    # Required: observable property
    sh:property [
        sh:path sosa:observedProperty ;
        sh:minCount 1 ;
        sh:message "Every Observation must link to an observable property via sosa:observedProperty."
    ] ;

    # Optional: simple numeric result
    sh:property [
        sh:path sosa:hasSimpleResult ;
        sh:maxCount 1 ;
        sh:datatype xsd:decimal
    ] ;

    # Optional: SIO measurement chain
    sh:property [
        sh:path sio:has-attribute ;
        sh:class sio:MeasuredValue
    ] ;

    # Optional: beamline run number
    sh:property [
        sh:path chess:runNumber ;
        sh:maxCount 1 ;
        sh:datatype xsd:integer
    ] .

# ── BeamlineShape ─────────────────────────────────────────────────────────────
fabric:BeamlineShape
    a sh:NodeShape ;
    rdfs:label "CHESS Beamline Descriptor Shape" ;
    sh:targetClass chess:Beamline ;

    sh:property [
        sh:path rdfs:label ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:message "Every Beamline must have an rdfs:label."
    ] ;

    sh:property [
        sh:path chess:beamlineType ;
        sh:minCount 1 ;
        sh:in ( "x-ray-diffraction" "protein-crystallography"
                "time-resolved-scattering" "small-angle-scattering" ) ;
        sh:message "chess:beamlineType must be one of the allowed beamline type strings."
    ] .

# ── NodeConformanceShape ──────────────────────────────────────────────────────
# Used by bootstrap verification to confirm this node publishes required endpoints.
fabric:NodeConformanceShape
    a sh:NodeShape ;
    rdfs:label "Knowledge Fabric Node Conformance Shape" ;
    sh:targetClass fabric:FabricNode ;

    sh:property [
        sh:path fabric:sparqlEndpoint ;
        sh:minCount 1 ;
        sh:message "A FabricNode must declare a sparqlEndpoint."
    ] ;

    sh:property [
        sh:path fabric:voidEndpoint ;
        sh:minCount 1 ;
        sh:message "A FabricNode must publish a VoID description."
    ] ;

    sh:property [
        sh:path fabric:shaclEndpoint ;
        sh:minCount 1 ;
        sh:message "A FabricNode must publish SHACL shapes."
    ] .
`)
}

// ──────────────────────────────────────────────────────────────────────────────
// L4 — SPARQL examples catalog
// ──────────────────────────────────────────────────────────────────────────────

// SPARQLExamplesTurtle returns a spex-pattern SPARQL examples catalog.
func SPARQLExamplesTurtle(cfg NodeConfig) string {
	return fmt.Sprintf(`@prefix spex:  <https://purl.expasy.org/sparql-examples/ontology#> .
	@prefix sh:    <http://www.w3.org/ns/shacl#> .
	@prefix rdfs:  <http://www.w3.org/2000/01/rdf-schema#> .
	@prefix schema: <https://schema.org/> .
	@prefix sosa:  <http://www.w3.org/ns/sosa/> .
	@prefix chess: <http://chess.cornell.edu/ns#> .

# ── Example 1: list all beamlines ────────────────────────────────────────────
<%s/sparql-examples/list-beamlines>
    a spex:SparqlExample ;
    rdfs:label "List all beamlines" ;
    schema:description "Returns the IRI and label of every beamline registered at this node." ;
    spex:query """
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
PREFIX chess: <http://chess.cornell.edu/ns#>

SELECT ?beamline ?label ?type WHERE {
    ?beamline a chess:Beamline ;
              rdfs:label ?label ;
              chess:beamlineType ?type .
}
ORDER BY ?label
""" .

# ── Example 2: list recent observations for a beamline ───────────────────────
<%s/sparql-examples/recent-observations>
    a spex:SparqlExample ;
    rdfs:label "Recent observations for a beamline" ;
    schema:description "Returns the 10 most recent SOSA Observations from a given beamline sensor." ;
    spex:query """
PREFIX sosa: <http://www.w3.org/ns/sosa/>
PREFIX xsd:  <http://www.w3.org/2001/XMLSchema#>
PREFIX chess: <http://chess.cornell.edu/ns#>

SELECT ?obs ?resultTime ?result WHERE {
    ?obs a sosa:Observation ;
         sosa:madeBySensor <http://chess.cornell.edu/sensor/id1-detector-01> ;
         sosa:resultTime ?resultTime .
    OPTIONAL { ?obs sosa:hasSimpleResult ?result . }
}
ORDER BY DESC(?resultTime)
LIMIT 10
""" .

# ── Example 3: cross-beamline property query ──────────────────────────────────
<%s/sparql-examples/cross-beamline-property>
    a spex:SparqlExample ;
    rdfs:label "Cross-beamline observed properties" ;
    schema:description "Lists all distinct observable properties across all beamlines." ;
    spex:query """
PREFIX sosa: <http://www.w3.org/ns/sosa/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>

SELECT DISTINCT ?property ?label WHERE {
    ?obs a sosa:Observation ;
         sosa:observedProperty ?property .
    OPTIONAL { ?property rdfs:label ?label . }
}
ORDER BY ?label
""" .

# ── Example 4: SIO measured value chain ──────────────────────────────────────
<%s/sparql-examples/sio-measured-value>
    a spex:SparqlExample ;
    rdfs:label "SIO measured value with unit" ;
    schema:description "Retrieves calibrated measurements using the SIO attribute/value/unit chain." ;
    spex:query """
PREFIX sosa: <http://www.w3.org/ns/sosa/>
PREFIX sio:  <http://semanticscience.org/resource/>

SELECT ?obs ?value ?unit WHERE {
    ?obs a sosa:Observation ;
         sio:has-attribute ?attr .
    ?attr a sio:MeasuredValue ;
          sio:has-value ?value ;
          sio:has-unit ?unit .
}
LIMIT 20
""" .
`, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL, cfg.BaseURL)
}
