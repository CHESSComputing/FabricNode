# FOXDEN and FabricNode Integration

**CHESS — Cornell High Energy Synchrotron Source**
**Document version:** 1.0 · 2026

---

## Table of Contents

1. [Overview](#1-overview)
2. [System Roles](#2-system-roles)
   - 2.1 [FOXDEN — Metadata Document Store](#21-foxden--metadata-document-store)
   - 2.2 [FabricNode — Knowledge Graph Layer](#22-fabricnode--knowledge-graph-layer)
   - 2.3 [How They Complement Each Other](#23-how-they-complement-each-other)
3. [FabricNode Architecture](#3-fabricnode-architecture)
   - 3.1 [catalog-service](#31-catalog-service-port-8081)
   - 3.2 [data-service](#32-data-service-port-8082)
   - 3.3 [identity-service](#33-identity-service-port-8083)
   - 3.4 [notification-service](#34-notification-service-port-8084)
4. [Core Concepts](#4-core-concepts)
   - 4.1 [Beamline IDs](#41-beamline-ids)
   - 4.2 [Dataset DIDs](#42-dataset-dids)
   - 4.3 [Named Graphs and Graph IRIs](#43-named-graphs-and-graph-iris)
   - 4.4 [RDF Triples](#44-rdf-triples)
   - 4.5 [Why Convert FOXDEN Records to Triples?](#45-why-convert-foxden-records-to-triples)
5. [Data Flow](#5-data-flow)
   - 5.1 [Acquiring and Registering a Dataset](#51-acquiring-and-registering-a-dataset)
   - 5.2 [Ingesting FOXDEN Metadata into the Graph](#52-ingesting-foxden-metadata-into-the-graph)
   - 5.3 [Querying the Knowledge Graph](#53-querying-the-knowledge-graph)
   - 5.4 [Catalog Discovery](#54-catalog-discovery)
6. [DOI Publication Credential Flow](#6-doi-publication-credential-flow)
   - 6.1 [Why a Verifiable Credential?](#61-why-a-verifiable-credential)
   - 6.2 [The Request](#62-the-request)
   - 6.3 [The Credential Structure](#63-the-credential-structure)
   - 6.4 [External Verification](#64-external-verification)
7. [API Reference](#7-api-reference)
   - 7.1 [catalog-service endpoints](#71-catalog-service-endpoints)
   - 7.2 [data-service endpoints](#72-data-service-endpoints)
   - 7.3 [identity-service endpoints](#73-identity-service-endpoints)
   - 7.4 [notification-service endpoints](#74-notification-service-endpoints)
8. [Configuration](#8-configuration)
9. [Integration Tests](#9-integration-tests)
   - 9.1 [FOXDEN Ingestion Probe](#91-foxden-ingestion-probe)
   - 9.2 [DOI Publication Credential Probe](#92-doi-publication-credential-probe)
10. [Glossary](#10-glossary)

---

## 1. Overview

CHESS produces synchrotron X-ray data across multiple beamlines. Each experiment generates both raw data files and rich metadata — who ran the experiment, which detectors were used, what technique was applied, what the sample was. This metadata is stored and validated in **FOXDEN**. However, once an experiment is complete, researchers need to ask questions that cut across many experiments: all datasets from a given cycle, all datasets involving a particular technique or sample type, or the full provenance chain behind a published result.

**FabricNode** addresses this need. It is a federated knowledge graph layer that ingests FOXDEN metadata and represents it as an interconnected RDF graph that can be queried using SPARQL. It does not replace FOXDEN; the two systems have complementary roles and are designed to work together.

```
Beamline instruments
        │
        │ data + metadata
        ▼
     FOXDEN                          FabricNode
  ┌──────────────────┐    ingest    ┌─────────────────────────────┐
  │ Metadata service │ ──────────→  │ data-service (RDF graph)    │
  │ Provenance svc   │ ──────────→  │ catalog-service (discovery) │
  │ DOI service      │ ──────────→  │ identity-service (VC / DID) │
  └──────────────────┘   VC issuance│ notification-service (LDN)  │
                                    └─────────────────────────────┘
                                              │
                                    SPARQL queries
                                              │
                                    Researchers / agents
```

---

## 2. System Roles

### 2.1 FOXDEN — Metadata Document Store

FOXDEN is a document-oriented metadata store built by and for the CHESS user community. It stores one JSON record per experiment, validated against a beamline-specific schema (e.g. `ID3A.json`). A typical record contains:

| Field | Example value |
|---|---|
| `beamline` | `["3A"]` |
| `btr` | `"test-123-a"` |
| `cycle` | `"2026-1"` |
| `pi` | `"Baggins"` |
| `technique` | `["high_energy_diffraction_microscopy_near_field", "tomography"]` |
| `detectors` | `["dual_dexelas", "retiga"]` |
| `beam_energy` | `41.991` |
| `did` | `"/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271..."` |

FOXDEN is excellent at answering:
- "Give me everything about experiment `test-123-a`."
- "Find all experiments by PI `Baggins` in cycle `2026-1`."

FOXDEN is not designed to answer questions that join across multiple records or link metadata to provenance chains in a single query.

### 2.2 FabricNode — Knowledge Graph Layer

FabricNode is a federated knowledge graph node. It consumes FOXDEN metadata and represents the same facts as RDF triples stored in named graphs — one named graph per dataset. This representation allows cross-record queries using SPARQL.

FabricNode is excellent at answering:
- "Find all datasets from cycle `2026-1` where beam energy was above 40 keV and the technique included tomography."
- "What is the full provenance chain for dataset X — who submitted the BTR, who ran the experiment, which processing steps were applied?"
- "Which datasets from beamline `3A` have been assigned a DOI?"

FabricNode also provides machine-readable self-description (VoID, SHACL, PROF) and cryptographic identity (DIDs, Verifiable Credentials) to support federated trust across nodes at different institutions.

### 2.3 How They Complement Each Other

| Concern | FOXDEN | FabricNode |
|---|---|---|
| Primary format | JSON documents | RDF triples |
| Query language | Field-value search | SPARQL |
| Cross-record queries | No (requires custom logic) | Yes (single SPARQL query) |
| Schema validation | Per-beamline JSON schema | SHACL shapes |
| Provenance | Provenance service (JSON) | PROV-O triples in graph |
| Identity / trust | User accounts | W3C DIDs + Verifiable Credentials |
| DOI publication | DOI service | Issues signed publication VC |
| Discovery | FOXDEN API | VoID, DCAT, PROF endpoints |

Neither system is a replacement for the other. The integration pattern is: **FOXDEN owns the record; FabricNode reasons over it.**

---

## 3. FabricNode Architecture

FabricNode consists of four microservices. Each service is independently deployable and communicates over HTTP.

```
┌─────────────────────────────────────────────────────────────────┐
│                        FabricNode                               │
│                                                                 │
│  catalog-service :8081    │  data-service :8082                 │
│  ─────────────────────    │  ─────────────────────              │
│  VoID · SHACL · PROF      │  RDF triple store                   │
│  DCAT beamline catalog    │  SPARQL endpoint                    │
│  Dataset listings         │  SHACL validation                   │
│                           │  FOXDEN ingest                      │
│  ───────────────────────────────────────────────                │
│  identity-service :8083   │  notification-service :8084         │
│  ─────────────────────    │  ─────────────────────              │
│  W3C DID document         │  W3C LDN inbox                      │
│  Verifiable Credentials   │  Event notifications                │
│  Key management           │                                     │
└─────────────────────────────────────────────────────────────────┘
              │                          │
         Named graph store         FOXDEN services
         (in-memory / Oxigraph)    (metadata, provenance, DOI)
```

### 3.1 catalog-service (port 8081)

The catalog-service is the public face of the node. It serves:

- **VoID** — a machine-readable description of what datasets this node holds and where to query them.
- **SHACL shapes** — the validation rules that incoming RDF data must satisfy.
- **PROF** — a capability profile listing all self-description artifacts.
- **DCAT catalog** — a structured listing of beamlines and their datasets, with links into the data-service for each dataset's SPARQL endpoint.

The list of beamlines is driven by the node's configuration file (`config/fabric.yaml`) rather than hard-coded values, so adding a new beamline requires only a configuration change.

### 3.2 data-service (port 8082)

The data-service is the RDF graph store. It:

- Stores triples in **named graphs** keyed by dataset DID.
- Accepts SPARQL-style queries at global, beamline-scoped, and dataset-scoped levels.
- Validates incoming triples against SHACL shapes before writing them.
- Pulls FOXDEN metadata records via the FOXDEN client, converts them to RDF triples, and inserts them into the correct named graph.

In the current implementation the store is in-memory. For production, the `store.Store` interface is designed for a one-line swap to Oxigraph or Apache Jena.

### 3.3 identity-service (port 8083)

The identity-service handles cryptographic identity for the node. It:

- Generates an **Ed25519 key pair** at startup.
- Publishes a **W3C DID document** at `/.well-known/did.json` that binds the node's public key to its service endpoints.
- Issues and verifies **Verifiable Credentials** — both the node-level conformance credential and dataset-level publication credentials requested by FOXDEN's DOI service.

### 3.4 notification-service (port 8084)

The notification-service implements a **W3C Linked Data Notifications (LDN)** inbox. Other services and external systems can send structured notifications to the node (e.g. "new run started", "dataset validated", "trust gap detected"), and the node processes or acknowledges them.

---

## 4. Core Concepts

### 4.1 Beamline IDs

Beamline identifiers in FabricNode are **lower-case alphanumeric strings**, starting with a letter, 1–16 characters long.

| FOXDEN value | FabricNode ID | Notes |
|---|---|---|
| `["3A"]` | `3a` | Upper-case normalised to lower |
| `["ID1"]` | `id1` | |
| `["FAST"]` | `fast` | |
| `["ID3A"]` | `id3a` | |

The canonical form is always lower-case. The `beamline=` segment in a dataset DID uses this canonical form.

### 4.2 Dataset DIDs

A **Dataset DID** (Dataset Identifier) is a slash-separated `key=value` path that uniquely identifies a dataset within a beamline. The first segment is always `beamline=<id>`.

```
/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271DD79-9845-48A4-A813-9FDA3F10A4B2
```

Conventional segment order:

| Position | Key | Example |
|---|---|---|
| 1 | `beamline` | `3a` |
| 2 | `btr` | `test-123-a` |
| 3 | `cycle` | `2026-1` |
| 4 | `sample_name` | `PAT-7271...` |
| 5+ | additional keys | experiment-specific |

When used as a URL path parameter, the DID must be **URL-encoded** because it contains `/` and `=` characters:

```
/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271
  →  %2Fbeamline%3D3a%2Fbtr%3Dtest-123-a%2Fcycle%3D2026-1%2Fsample_name%3DPAT-7271
```

In Python:
```python
import urllib.parse
encoded = urllib.parse.quote(did, safe="")
```

### 4.3 Named Graphs and Graph IRIs

An **IRI** (Internationalized Resource Identifier) is a URL that names a thing. A **named graph** is a container in RDF that groups a set of triples under one IRI, making it possible to query or describe that subset independently.

FabricNode assigns one named graph per dataset. The graph IRI is derived from the DID:

```
DID:       /beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271
Graph IRI: http://chess.cornell.edu/graph/3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271
```

The scheme is:
```
http://chess.cornell.edu/graph/<beamline>/<did-segments-without-beamline-prefix>
```

**Beamline-scoped queries** match any graph whose IRI starts with `http://chess.cornell.edu/graph/3a/`, covering all datasets for beamline `3a`. **Dataset-scoped queries** match exactly one graph.

### 4.4 RDF Triples

RDF (Resource Description Framework) expresses every fact as a three-part statement:

```
subject  →  predicate  →  object
```

- **Subject**: the thing being described (an IRI).
- **Predicate**: the property or relationship (an IRI).
- **Object**: a value (either an IRI or a literal like a string or number).

A concrete example from a FOXDEN record for dataset `/beamline=3a/btr=test-123-a/...`:

```turtle
<http://chess.cornell.edu/dataset/beamline=3a/btr=test-123-a/...>
    dct:creator           "Baggins" .
    chess:cycle           "2026-1" .
    chess:beamEnergy      "41.991"^^xsd:decimal .
    chess:technique       "high_energy_diffraction_microscopy_near_field" .
    chess:technique       "tomography" .
    chess:detector        "dual_dexelas" .
    chess:detector        "retiga" .
```

Each line is one triple. Array fields in FOXDEN (like `technique` and `detectors`) become multiple triples with the same subject and predicate.

### 4.5 Why Convert FOXDEN Records to Triples?

JSON describes one thing in isolation. RDF describes relationships between things across documents. The difference matters when a researcher asks a question like:

> "Find all datasets from cycle 2026-1 where beam energy was above 40 keV and the technique included tomography, and return their PI names."

In FOXDEN this requires: fetching all cycle 2026-1 records, filtering in application code for beam energy, filtering again for technique, extracting the PI field. Each new combination requires new application logic.

In FabricNode this is a single SPARQL query:

```sparql
SELECT ?dataset ?pi WHERE {
    ?dataset chess:cycle      "2026-1" ;
             chess:beamEnergy ?energy ;
             chess:technique  "tomography" ;
             dct:creator      ?pi .
    FILTER(?energy > 40.0)
}
```

No additional application code is needed. The same query engine handles any combination of fields. This is the core value proposition of the integration: FOXDEN provides validated, schema-compliant records; FabricNode makes those records queryable as a unified graph.

---

## 5. Data Flow

### 5.1 Acquiring and Registering a Dataset

```
Beamline instruments
        │
        │  raw data → filesystem
        │  metadata → FOXDEN metadata service
        │  lineage  → FOXDEN provenance service
        ▼
     FOXDEN
  (record stored, DID assigned)
```

When a beamline run completes, the beamline control system or staff scientist:

1. Writes raw and reduced data to the shared filesystem.
2. Submits a metadata record to FOXDEN (validated against the beamline schema).
3. FOXDEN assigns a dataset DID of the form `/beamline=<id>/btr=<btr>/cycle=<cycle>/sample_name=<name>`.
4. FOXDEN's provenance service records who submitted the BTR, who ran the experiment, and what processing steps were applied.

At this point FOXDEN holds the authoritative record. FabricNode is not yet involved.

### 5.2 Ingesting FOXDEN Metadata into the Graph

When the dataset is ready to be discoverable and queryable as a graph — typically after finalisation, provenance recording, or DOI minting — the ingest endpoint is called:

```
POST /beamlines/{beamline}/datasets/{did}/foxden/ingest
```

The data-service:

1. Calls `GET /search` on the FOXDEN metadata service with the dataset DID.
2. Converts the returned JSON record to RDF triples (see the field mapping table in §7.2).
3. Validates the triples against SHACL shapes.
4. Writes the triples to the named graph keyed by the dataset DID.

```
FOXDEN metadata service
        │
        │  JSON record
        ▼
  data-service /foxden/ingest
        │
        ├── convert JSON → RDF triples
        ├── SHACL validation
        └── write to named graph
              http://chess.cornell.edu/graph/3a/btr=test-123-a/...
```

**Design principle — loose coupling**: FOXDEN does not need to know FabricNode exists. The ingest is triggered externally (by an operator, a CI pipeline, or a webhook). This means the two systems remain independently deployable, and a FabricNode outage does not affect FOXDEN's ability to accept metadata.

### 5.3 Querying the Knowledge Graph

Three query scopes are available:

| Scope | Endpoint | Covers |
|---|---|---|
| Dataset | `GET /beamlines/{bl}/datasets/{did}/sparql` | One dataset's named graph |
| Beamline | `GET /beamlines/{bl}/sparql` | All named graphs for a beamline |
| Global | `GET /sparql` | Entire store |

All endpoints accept the same query parameters:

| Parameter | Meaning |
|---|---|
| `s` | Subject filter (IRI) |
| `p` | Predicate filter (IRI) |
| `o` | Object filter (literal or IRI) |
| `g` | Graph filter (IRI) — global endpoint only |
| `describe` | Return all triples where subject or object = this IRI |
| `search` | Case-insensitive keyword search across object literals |

Example — find all datasets for beamline `3a` that used the technique `tomography`:

```
GET /beamlines/3a/sparql?p=http://chess.cornell.edu/ns%23technique&o=tomography
```

Example — describe a specific dataset resource:

```
GET /sparql?describe=http://chess.cornell.edu/dataset/beamline=3a/btr=test-123-a/...
```

### 5.4 Catalog Discovery

The catalog-service exposes the node's datasets in DCAT JSON-LD format, making them discoverable by standard catalog tools:

```
GET /catalog/beamlines
GET /catalog/beamlines/{beamline}/datasets
```

Each dataset entry in the catalog includes a `dcat:accessURL` pointing to the dataset's SPARQL endpoint in the data-service, allowing a client to go from discovery to query in two steps.

---

## 6. DOI Publication Credential Flow

### 6.1 Why a Verifiable Credential?

When FOXDEN mints a DOI for a dataset, it creates a link between the dataset's scientific identity (the DID) and a permanent public identifier (the DOI). For this link to be trustworthy in a federated setting — where external institutions may want to verify that a dataset really came from CHESS and that it really is stored at the claimed SPARQL endpoint — the link needs a cryptographic signature.

The FabricNode identity-service provides this: it issues a **W3C Verifiable Credential** (VC) that binds the DID, the graph IRI, the DOI, and the SPARQL endpoint together under the node's Ed25519 signature. Anyone who can resolve the node's DID document (`/.well-known/did.json`) can independently verify this credential without contacting CHESS directly.

### 6.2 The Request

FOXDEN's DOI service calls the identity-service immediately after minting a DOI:

```http
POST /credentials/dataset
Content-Type: application/json

{
  "did":     "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271",
  "doi":     "10.5281/zenodo.123456",
  "doi_url": "https://doi.org/10.5281/zenodo.123456"
}
```

`graph_iri` and `sparql_endpoint` are optional — if omitted, the identity-service derives them from the DID and the node's DID document service endpoints.

### 6.3 The Credential Structure

The identity-service responds with a signed `DatasetPublicationCredential`:

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://w3id.org/cogitarelink/fabric/v1",
    "https://schema.org/",
    "http://www.w3.org/ns/dcat#",
    "http://www.w3.org/ns/prov#"
  ],
  "id": "did:web:chess-node/credentials/dataset/<uuid>",
  "type": ["VerifiableCredential", "DatasetPublicationCredential"],
  "issuer": "did:web:chess-node",
  "issuanceDate": "2026-03-31T10:00:00Z",
  "credentialSubject": {
    "id": "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271",
    "type": ["schema:Dataset", "dcat:Dataset"],
    "fabric:graphIRI":        "http://chess.cornell.edu/graph/3a/btr=test-123-a/...",
    "schema:identifier":      "10.5281/zenodo.123456",
    "schema:url":             "https://doi.org/10.5281/zenodo.123456",
    "dcat:accessURL":         "http://chess-node:8082/beamlines/3a/datasets/.../sparql",
    "chess:beamline":         "3a",
    "prov:wasAttributedTo":   "did:web:chess-node",
    "prov:generatedAtTime":   "2026-03-31T10:00:00Z"
  },
  "proof": {
    "type":               "DataIntegrityProof",
    "created":            "2026-03-31T10:00:00Z",
    "verificationMethod": "did:web:chess-node#node-key-1",
    "proofPurpose":       "assertionMethod",
    "proofValue":         "<base64url-encoded Ed25519 signature>"
  }
}
```

FOXDEN stores this credential alongside its DOI record. It can be shared with dataset depositors, included in data management plans, or published alongside the DOI landing page.

### 6.4 External Verification

Any external party can verify the credential without contacting CHESS:

1. Fetch the node's DID document: `GET https://chess-node/.well-known/did.json`
2. Extract the public key from `verificationMethod`.
3. Remove the `proof` field from the credential.
4. Verify the Ed25519 signature in `proof.proofValue` against the serialised credential body.

The identity-service also exposes a convenience verification endpoint:

```http
POST /credentials/dataset/verify
Content-Type: application/json

<the full credential JSON>
```

Response:
```json
{
  "verified":    true,
  "issuer":      "did:web:chess-node",
  "did":         "/beamline=3a/...",
  "doi":         "10.5281/zenodo.123456",
  "graphIRI":    "http://chess.cornell.edu/graph/3a/...",
  "publishedAt": "2026-03-31T10:00:00Z"
}
```

---

## 7. API Reference

### 7.1 catalog-service endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/.well-known/void` | VoID dataset description (Turtle or JSON-LD) |
| `GET` | `/.well-known/profile` | PROF capability profile (Turtle) |
| `GET` | `/.well-known/shacl` | SHACL shapes (Turtle) |
| `GET` | `/.well-known/sparql-examples` | SPARQL examples catalog (Turtle) |
| `GET` | `/catalog/beamlines` | List all beamlines (DCAT JSON-LD) |
| `GET` | `/catalog/beamlines/{beamline}/datasets` | List datasets for a beamline (DCAT JSON-LD) |
| `GET` | `/health` | Service liveness check |

Content negotiation on `/.well-known/void`: send `Accept: application/ld+json` to receive JSON-LD; default is Turtle.

### 7.2 data-service endpoints

| Method | Path | Description |
|---|---|---|
| `GET\|POST` | `/sparql` | Global SPARQL query |
| `GET` | `/graphs` | List all named graphs |
| `GET` | `/beamlines/{bl}/sparql` | Beamline-scoped SPARQL query |
| `GET` | `/beamlines/{bl}/graphs` | Named graphs for a beamline |
| `GET` | `/beamlines/{bl}/datasets/{did}/sparql` | Dataset-scoped SPARQL query |
| `POST` | `/beamlines/{bl}/datasets/{did}/triples` | Insert triples (SHACL-validated) |
| `POST` | `/beamlines/{bl}/datasets/{did}/validate` | Dry-run SHACL validation |
| `GET` | `/beamlines/{bl}/foxden/datasets` | List FOXDEN records for a beamline |
| `GET` | `/beamlines/{bl}/datasets/{did}/foxden` | Fetch one FOXDEN record by DID |
| `POST` | `/beamlines/{bl}/datasets/{did}/foxden/ingest` | Ingest FOXDEN record as RDF |
| `GET` | `/health` | Service liveness check |

**FOXDEN field → RDF predicate mapping** (selected fields):

| FOXDEN field | RDF predicate | Notes |
|---|---|---|
| `did` | `dct:identifier` | Dataset identifier |
| `pi` | `dct:creator` | Principal investigator |
| `cycle` | `chess:cycle` | Run cycle |
| `btr` | `chess:btr` | Beam time request ID |
| `beam_energy` | `chess:beamEnergy` | xsd:decimal |
| `technique[]` | `chess:technique` | One triple per element |
| `detectors[]` | `chess:detector` | One triple per element |
| `beamline[]` | `chess:beamline` | One triple per element |
| `in_situ` | `chess:inSitu` | xsd:boolean |
| `sample_name` | `chess:sampleName` | |
| `sample_common_name` | `chess:sampleCommonName` | |
| `data_location_raw` | `chess:dataLocationRaw` | |

### 7.3 identity-service endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/.well-known/did.json` | W3C DID document |
| `GET` | `/credentials/conformance` | Node conformance VC |
| `POST` | `/credentials/verify` | Verify a conformance VC |
| `POST` | `/credentials/dataset` | Issue a dataset publication VC |
| `POST` | `/credentials/dataset/verify` | Verify a dataset publication VC |
| `GET` | `/did/{did}` | DID resolution (simplified) |
| `GET` | `/keys/node-key-1` | Public key export |
| `GET` | `/health` | Service liveness check |

### 7.4 notification-service endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/inbox` | List notifications (`?type=` filter) |
| `POST` | `/inbox` | Receive a notification (JSON-LD body) |
| `GET` | `/inbox/{id}` | Retrieve a specific notification |
| `POST` | `/inbox/{id}/ack` | Acknowledge a notification |
| `GET` | `/inbox/stats` | Inbox statistics |
| `GET` | `/health` | Service liveness check |

---

## 8. Configuration

All services read from a single YAML configuration file. The file is searched in the following order:

1. The path given by the `FABRIC_CONFIG` environment variable
2. `./fabric.yaml` (working directory)
3. `./config/fabric.yaml`
4. `$HOME/.fabric/fabric.yaml`

If no file is found, safe defaults are used (all services run locally on their standard ports).

```yaml
# config/fabric.yaml

node:
  id: chess-node
  name: CHESS Federated Knowledge Fabric Node
  base_url: http://localhost:8081

catalog:
  port: 8081
  beamlines:
    - id: id1
      label: "Beamline ID1 — X-ray Diffraction"
      type: x-ray-diffraction
      location: CHESS Wilson Laboratory
    - id: id3a
      label: "Beamline ID3A — Protein Crystallography"
      type: protein-crystallography
    - id: fast
      label: "Beamline FAST — Time-Resolved Scattering"
      type: time-resolved-scattering

data_service:
  port: 8082
  sparql_result_limit: 100

identity:
  port: 8083

notification:
  port: 8084

foxden:
  metadata_url:   http://localhost:8300
  provenance_url: http://localhost:8301
  doi_url:        http://localhost:8302
  token: ""          # set via FOXDEN_TOKEN env var in production
  timeout: 10
```

**Environment variable overrides** (take precedence over file values):

| Variable | Config field |
|---|---|
| `FABRIC_CONFIG` | Path to config file |
| `NODE_ID` | `node.id` |
| `NODE_NAME` | `node.name` |
| `NODE_BASE_URL` | `node.base_url` |
| `FOXDEN_URL` | `foxden.metadata_url` |
| `FOXDEN_METADATA_URL` | `foxden.metadata_url` |
| `FOXDEN_PROVENANCE_URL` | `foxden.provenance_url` |
| `FOXDEN_TOKEN` | `foxden.token` |

Sensitive values (tokens, private keys) should always be set via environment variables rather than stored in the config file.

---

## 9. Integration Tests

Two probe scripts are provided under `scripts/`. Both require the relevant services to be running. Start them with `make start` before running probes.

### 9.1 FOXDEN Ingestion Probe

Tests the full flow from FOXDEN search → ingest → SPARQL verification → catalog listing.

```bash
# Search by BTR number
make probe KEY=btr VALUE=test-123-a

# Search by cycle, show full JSON bodies
make probe KEY=cycle VALUE=2026-1 VERBOSE=1

# Search by beamline
make probe KEY=beamline VALUE=3a LIMIT=10

# Only search FOXDEN, skip ingest (services not required)
make probe KEY=sample_name VALUE=silicon-std DRY_RUN=1
```

The probe validates at each step:

| Step | What is checked |
|---|---|
| Health | Both services return `{"status":"ok"}` |
| FOXDEN search | `status == "ok"`, `nrecords > 0`, DID starts with `/beamline=` |
| Ingest | HTTP 201, `ingested > 0`, `graphIRI` has expected prefix |
| SPARQL | SPARQL JSON Results format, triples in correct named graph, dataset IRI as subject |
| Catalog | DCAT JSON-LD shape, `dcat:dataset` array non-empty, `dcat:accessURL` points to SPARQL |

### 9.2 DOI Publication Credential Probe

Tests the FOXDEN DOI service → identity-service credential issuance flow.

```bash
# Basic DOI credential test
make probe-doi \
  DID="/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271" \
  DOI="10.5281/zenodo.123456" \
  DOI_URL="https://doi.org/10.5281/zenodo.123456"

# Ingest dataset first, then issue credential, with full output
make probe-doi \
  DID="/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std" \
  DOI="10.5281/zenodo.999999" \
  DOI_URL="https://doi.org/10.5281/zenodo.999999" \
  INGEST=1 \
  VERBOSE=1
```

The probe validates:

| Step | What is checked |
|---|---|
| Health | Both services respond |
| DID resolution | `did:web` method, Ed25519 key present, `SPARQLEndpoint` service declared |
| Credential issuance | HTTP 201, all required `credentialSubject` fields present, `fabric:graphIRI` prefix correct |
| Proof structure | `DataIntegrityProof` type, `assertionMethod` purpose, `proofValue` present |
| Round-trip verify | `verified == true`, all fields echoed back |
| SPARQL reachability | `dcat:accessURL` from credential responds HTTP 200 |
| Tamper test | Modifying the DOI causes `verified == false` |

---

## 10. Glossary

**DCAT** (Data Catalog Vocabulary)
: A W3C vocabulary for describing datasets and data services in a catalog. FabricNode uses DCAT to expose beamline and dataset listings at `/catalog/beamlines`.

**DID** (Decentralized Identifier)
: A W3C standard for globally unique identifiers that do not require a central registry. In FabricNode, the node's DID takes the form `did:web:<hostname>` and resolves to the DID document at `/.well-known/did.json`.

**Dataset DID**
: In the FabricNode/FOXDEN context, a dataset identifier is a slash-separated `key=value` path that uniquely addresses a dataset, e.g. `/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271`. Not to be confused with a W3C Decentralized Identifier — the acronym is overloaded in this codebase.

**DOI** (Digital Object Identifier)
: A persistent identifier for published datasets and documents. FOXDEN's DOI service mints DOIs via providers such as DataCite or Zenodo.

**Graph IRI**
: The IRI used to name a specific named graph in the triple store. In FabricNode the scheme is `http://chess.cornell.edu/graph/<beamline>/<did-segments>`.

**IRI** (Internationalized Resource Identifier)
: A generalisation of a URL that names a resource. In RDF, subjects and predicates are IRIs. Unlike a URL, an IRI need not resolve to a document — its role is to provide a globally unique name.

**Named graph**
: A container in RDF that groups a set of triples under a single IRI. FabricNode assigns one named graph per dataset, allowing queries to be scoped to a single dataset or to all datasets belonging to a beamline.

**PROV-O** (W3C Provenance Ontology)
: A standard vocabulary for describing provenance — who created something, using what inputs, at what time. Key predicates: `prov:wasGeneratedBy`, `prov:wasAssociatedWith`, `prov:used`.

**PROF** (W3C Profiles Vocabulary)
: A vocabulary for describing profiles — specifications that constrain how a standard is to be used. FabricNode publishes a PROF resource at `/.well-known/profile` listing the SHACL shapes and SPARQL examples that define the node's conformance requirements.

**RDF** (Resource Description Framework)
: A W3C standard data model where every fact is a triple: subject → predicate → object. The universal representation that allows knowledge from multiple sources to be merged and queried as a single graph.

**SHACL** (Shapes Constraint Language)
: A W3C standard for validating RDF data against a set of constraints. FabricNode validates every write to the triple store against SHACL shapes published at `/.well-known/shacl`. For example, every `sosa:Observation` triple must include `sosa:resultTime`, `sosa:madeBySensor`, and `sosa:observedProperty`.

**SPARQL** (SPARQL Protocol and RDF Query Language)
: The W3C standard query language for RDF graphs. Analogous to SQL for relational databases. FabricNode exposes SPARQL-style query endpoints on the data-service.

**Turtle**
: A compact, human-readable syntax for writing RDF triples. The default format for VoID, SHACL, and PROF responses from the catalog-service.

**VC** (Verifiable Credential)
: A W3C standard for tamper-evident digital credentials. A VC carries claims (e.g. "this dataset was published under this DOI") and a cryptographic proof that allows any party to verify the claims without contacting the issuer. FabricNode issues two kinds: `FabricConformanceCredential` (node-level) and `DatasetPublicationCredential` (per dataset).

**VoID** (Vocabulary of Interlinked Datasets)
: A vocabulary for describing RDF datasets, their access points (e.g. SPARQL endpoints), and their relationships to other datasets. FabricNode publishes a VoID description at `/.well-known/void`.

**W3C LDN** (Linked Data Notifications)
: A protocol for sending and receiving structured notifications between web resources using JSON-LD. FabricNode's notification-service implements an LDN inbox at `/inbox`.
