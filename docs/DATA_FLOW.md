# FabricNode — Data Flow & Service Integration

## Domain Model

```
Node
└── Beamlines         (e.g. id1, id3a, fast, qm2)
      └── Datasets    (identified by DID path)
            └── Triples  (RDF observations, stored in named graphs)
```

### Beamline ID

A lower-case alphanumeric string — letters and digits only, starting with a
letter.  Length 1–16.

| Valid       | Invalid      |
|-------------|--------------|
| `id1`       | `ID1`        |
| `id3a`      | `id-3a`      |
| `fast`      | `beamline1`  |
| `qm2`       | `123abc`     |

### Dataset DID

A slash-separated `key=value` path.  The first segment **must** be
`beamline=<id>` and must match the containing beamline.

```
/beamline=id3a/btr=val123/cycle=2024-3/sample_name=thaumatin-a
```

Additional segments are ordered by convention: `btr`, `cycle`, `sample_name`,
followed by any experiment-specific keys.

The DID maps to a named-graph IRI in the triple store:

```
/beamline=id3a/btr=val123/cycle=2024-3/sample_name=thaumatin-a
  → http://chess.cornell.edu/graph/id3a/btr=val123/cycle=2024-3/sample_name=thaumatin-a
```

---

## Service Responsibilities

| Service              | Port  | Responsibility                                  |
|----------------------|-------|-------------------------------------------------|
| catalog-service      | 8081  | VoID/SHACL/PROF self-description; beamline + dataset catalogue |
| data-service         | 8082  | RDF triple store; SPARQL; SHACL-validated writes |
| identity-service     | 8083  | DID documents; Verifiable Credentials           |
| notification-service | 8084  | W3C LDN inbox; event delivery                  |

---

## Data Flow: Writing Observations

```
External client / instrument software
        │
        │  POST /beamlines/{beamline}/datasets/{did}/triples
        │  Body: JSON array of store.Triple
        ▼
   data-service
        │
        ├─ shacl.ValidateForDataset(ref, triples)
        │     checks: resultTime, madeBySensor, observedProperty
        │     checks: sensor IRI prefix matches beamline
        │
        ├─ store.InsertForDataset(ref, triples)
        │     writes to named graph:
        │     http://chess.cornell.edu/graph/{beamline}/{did-segments}
        │
        └─ 201 Created  { inserted, conforms, graphIRI }
```

### URL encoding of DID

Because the DID contains `/` and `=` characters, it must be URL-encoded when
used as a path parameter:

```
DID:      /beamline=id3a/btr=val123/cycle=2024-3/sample_name=thaumatin-a
Encoded:  %2Fbeamline%3Did3a%2Fbtr%3Dval123%2Fcycle%3D2024-3%2Fsample_name%3Dthaumatin-a
```

Full URL:
```
POST http://localhost:8082/beamlines/id3a/datasets/%2Fbeamline%3Did3a%2Fbtr%3Dval123%2Fcycle%3D2024-3%2Fsample_name%3Dthaumatin-a/triples
```

Using the Go client:
```go
client := client.New("http://localhost:8082")
ref := model.DatasetRef{
    Beamline: "id3a",
    DID:      "/beamline=id3a/btr=val123/cycle=2024-3/sample_name=thaumatin-a",
}
resp, err := client.InsertTriples(ref, triples)
```

---

## Data Flow: Querying

### Dataset-scoped SPARQL
```
GET /beamlines/{beamline}/datasets/{did}/sparql?s=…&p=…&o=…
```
Returns triples from the single named graph for that DID.

### Beamline-scoped SPARQL
```
GET /beamlines/{beamline}/sparql?s=…&p=…&o=…
```
Returns triples from **all** named graphs belonging to that beamline —
i.e. every dataset.

### Global SPARQL
```
GET /sparql?s=…&p=…&o=…&g=…
```
Queries the entire store.  Optional `g` parameter scopes to one graph.

### Special modes (all scopes)
```
?describe=<iri>   — all triples where subject or object = IRI
?search=<text>    — case-insensitive keyword search across object literals
```

---

## Data Flow: Catalog → Data-Service

The catalog-service acts as a **discovery layer** on top of the data-service.

```
Browser / agent
      │
      │  GET /catalog/beamlines/{beamline}/datasets
      ▼
 catalog-service
      │
      │  client.GraphsForBeamline(bl)
      │  GET http://data-service/beamlines/{beamline}/graphs
      ▼
 data-service  →  returns list of named-graph IRIs (= dataset DIDs)
      │
      ▼
 catalog-service  →  wraps as DCAT JSON-LD with distribution links
      │
      └─  Each dataset entry carries:
          dcat:accessURL → data-service SPARQL endpoint for that DID
```

This means the catalog never stores data — it discovers datasets from the
data-service at request time.

---

## Named-Graph IRI Scheme

```
http://chess.cornell.edu/graph/{beamlineID}/{did-without-beamline-prefix}
```

Examples:

| DID                                                                  | Named-graph IRI                                                                         |
|----------------------------------------------------------------------|-----------------------------------------------------------------------------------------|
| `/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std`      | `http://chess.cornell.edu/graph/id1/btr=btr001/cycle=2024-3/sample_name=silicon-std`   |
| `/beamline=id3a/btr=btr101/cycle=2024-3/sample_name=thaumatin-a`     | `http://chess.cornell.edu/graph/id3a/btr=btr101/cycle=2024-3/sample_name=thaumatin-a`  |
| `/beamline=fast/btr=btr201/cycle=2024-3/sample_name=fe3o4-nanoparticles` | `http://chess.cornell.edu/graph/fast/btr=btr201/cycle=2024-3/sample_name=fe3o4-nanoparticles` |

Beamline-scope queries match any graph with prefix
`http://chess.cornell.edu/graph/{beamlineID}/`.

---

## Quick-start curl examples

```bash
# List beamlines
curl http://localhost:8081/catalog/beamlines

# List datasets for beamline id3a
curl http://localhost:8081/catalog/beamlines/id3a/datasets

# Insert an observation
DID_ENC=$(python3 -c "import urllib.parse; print(urllib.parse.quote('/beamline=id3a/btr=btr101/cycle=2024-3/sample_name=thaumatin-a'))")
curl -X POST "http://localhost:8082/beamlines/id3a/datasets/${DID_ENC}/triples" \
  -H 'Content-Type: application/json' \
  -d '[{
    "subject": "http://chess.cornell.edu/observation/test-01",
    "predicate": "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
    "object": "http://www.w3.org/ns/sosa/Observation",
    "objectType": "uri"
  }]'

# Query a dataset
curl "http://localhost:8082/beamlines/id3a/datasets/${DID_ENC}/sparql"

# Query all datasets for a beamline
curl "http://localhost:8082/beamlines/id3a/sparql"

# List named graphs for a beamline
curl "http://localhost:8082/beamlines/id3a/graphs"
```
