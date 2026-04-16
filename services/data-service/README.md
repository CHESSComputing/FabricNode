# data-service

The **data-service** provides the SPARQL-compatible query interface for CHESS
beamline observation data. It stores RDF triples in named graphs (one per
beamline) and exposes them via a SPARQL-results+JSON endpoint.

The service also enforces **SHACL validation** on all write operations,
implementing the `fabric:ObservationShape` write contract: every observation
must have `sosa:resultTime`, `sosa:madeBySensor`, and `sosa:observedProperty`.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET/POST` | `/sparql` | Query triples (see parameters below) |
| `GET` | `/graphs` | List named graphs |
| `POST` | `/triples` | Insert triples (SHACL-validated) |
| `POST` | `/validate` | Dry-run SHACL validation |
| `GET` | `/health` | Health check |

### SPARQL Query Parameters

| Param | Description |
|-------|-------------|
| `s` | Subject filter (IRI) |
| `p` | Predicate filter (IRI) |
| `o` | Object filter |
| `g` | Graph filter (IRI) |
| `describe` | DESCRIBE a resource by IRI |
| `search` | Keyword search across literal values |

## Named Graphs

| Graph IRI | Content |
|-----------|---------|
| `http://chess.cornell.edu/graph/beamlines` | Beamline descriptors |
| `http://chess.cornell.edu/graph/observations` | SOSA Observations |
| `http://chess.cornell.edu/graph/sensors` | Sensor descriptors |

## curl Examples

```bash
# List all named graphs
curl http://localhost:8782/graphs

# Get all triples (default limit 100)
curl http://localhost:8782/sparql

# Filter by graph
curl "http://localhost:8782/sparql?g=http://chess.cornell.edu/graph/beamlines"

# Filter by subject
curl "http://localhost:8782/sparql?s=http://chess.cornell.edu/beamline/id1"

# Filter by predicate (find all SOSA Observations)
curl "http://localhost:8782/sparql?p=http://www.w3.org/1999/02/22-rdf-syntax-ns%23type&o=http://www.w3.org/ns/sosa/Observation"

# DESCRIBE a beamline resource
curl "http://localhost:8782/sparql?describe=http://chess.cornell.edu/beamline/id1"

# Keyword search
curl "http://localhost:8782/sparql?search=diffraction"

# Insert SHACL-validated observation triples
curl -X POST http://localhost:8782/triples \
  -H "Content-Type: application/json" \
  -d '[
    {
      "subject":    "http://chess.cornell.edu/observation/test-01",
      "predicate":  "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
      "object":     "http://www.w3.org/ns/sosa/Observation",
      "objectType": "uri",
      "graph":      "http://chess.cornell.edu/graph/observations"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/test-01",
      "predicate":  "http://www.w3.org/ns/sosa/resultTime",
      "object":     "2025-01-15T14:30:00Z^^http://www.w3.org/2001/XMLSchema#dateTime",
      "objectType": "literal",
      "graph":      "http://chess.cornell.edu/graph/observations"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/test-01",
      "predicate":  "http://www.w3.org/ns/sosa/madeBySensor",
      "object":     "http://chess.cornell.edu/sensor/id1-detector-01",
      "objectType": "uri",
      "graph":      "http://chess.cornell.edu/graph/observations"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/test-01",
      "predicate":  "http://www.w3.org/ns/sosa/observedProperty",
      "object":     "http://chess.cornell.edu/property/peak-intensity",
      "objectType": "uri",
      "graph":      "http://chess.cornell.edu/graph/observations"
    }
  ]'

# Dry-run validation (no insertion)
curl -X POST http://localhost:8782/validate \
  -H "Content-Type: application/json" \
  -d '[{"subject":"http://chess.cornell.edu/obs/x","predicate":"http://www.w3.org/1999/02/22-rdf-syntax-ns#type","object":"http://www.w3.org/ns/sosa/Observation","objectType":"uri","graph":"http://chess.cornell.edu/graph/observations"}]'
# → {"conforms":false,"errors":["missing required sosa:resultTime", ...]}
```

## Production Upgrade Path

Replace the in-memory store with [Oxigraph](https://github.com/oxigraph/oxigraph):

```go
// In main.go, swap:
db := store.New()
// For:
db := oxigraph.NewClient("http://oxigraph:7878")
```

The `sparql.Handler` interface remains unchanged.
