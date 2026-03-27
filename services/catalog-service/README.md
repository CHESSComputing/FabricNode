# catalog-service

The **catalog-service** implements the four-layer Knowledge Fabric
self-description stack for the CHESS node. It exposes the machine-readable
metadata that allows autonomous agents (and human developers) to discover and
understand the CHESS beamline datasets without prior manual integration.

## Endpoints

| Endpoint | Layer | Description |
|----------|-------|-------------|
| `GET /.well-known/void` | L1 | VoID + DCAT dataset description |
| `GET /.well-known/profile` | L1 | PROF capability profile |
| `GET /.well-known/shacl` | L3 | SHACL shapes (data constraints) |
| `GET /.well-known/sparql-examples` | L4 | SPARQL examples catalog |
| `GET /health` | — | Health check |

All endpoints support **content negotiation** via `Accept` header:
- `text/turtle` (default)
- `application/ld+json`
- `application/n-triples`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_BASE_URL` | `http://localhost:8081` | Public base URL of this node |
| `NODE_ID` | `chess-node` | Node identifier |
| `NODE_NAME` | `CHESS Federated Knowledge Fabric Node` | Human label |
| `PORT` | `8081` | Listen port |

## Running

```bash
# Local
make run

# Docker
docker build -t catalog-service .
docker run -p 8081:8081 catalog-service
```

## curl Examples

```bash
# L1 — VoID dataset description (Turtle)
curl http://localhost:8081/.well-known/void

# L1 — VoID as JSON-LD
curl -H "Accept: application/ld+json" \
     http://localhost:8081/.well-known/void

# L1 — PROF capability profile
curl http://localhost:8081/.well-known/profile

# L3 — SHACL shapes (shows ObservationShape, BeamlineShape)
curl http://localhost:8081/.well-known/shacl

# L4 — SPARQL examples catalog
curl http://localhost:8081/.well-known/sparql-examples

# Health
curl http://localhost:8081/health
```

## What an Agent Sees

An LLM-based agent following the Knowledge Fabric progressive disclosure pattern
would read these endpoints in order:

1. `/.well-known/void` → discovers beamlines id1, id3, fast; learns SPARQL endpoint URL
2. `/.well-known/profile` → learns what artifacts the node provides
3. `/.well-known/shacl` → reads `ObservationShape`: needs `resultTime`, `madeBySensor`, `observedProperty`
4. `/.well-known/sparql-examples` → copies example queries as templates

Each step narrows the context before the agent issues a single data query.

## CHESS catalog
This catalog service represents CHESS catalog. The catalog
consists of different beamlines which by itself contains multiple
or produce datasets. Below you can see a hierarchical structure:

```
/catalog                     (dcat:Catalog)
 ├── /catalog/beamline-id1  (dcat:Catalog + chess:Beamline)
 │     └── datasets (future)
 ├── /catalog/beamline-id3
 └── /catalog/beamline-fast
 ```

 ---

# Catalog Service → Data Service Integration

## Flow

1. User registers dataset under beamline
2. Catalog extracts:
   - beamlineId (e.g. id3a)
   - DID path
3. Catalog calls data-service

## Example

```
SendTriples(
    "http://localhost:8081",
    "id3a",
    "/beamline=id3a/btr=val123/cycle=123/sample_name=bla",
    "<rdf triples>"
)
```

