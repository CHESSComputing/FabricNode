# CHESS Federated Knowledge Fabric Node

A **Knowledge Fabric node** for the
Cornell High Energy Synchrotron Source (CHESS). Each service implements a
specific layer of the W3C-standards-based self-description stack defined in the
[Federated Knowledge Fabric prototype](./CONCEPT.md).

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                External Agents / Researchers                    │
│          (LLM agents, curl, semantic web clients)               │
└──────┬──────────────┬─────────────────┬──────────────┬──────────┘
       │              │                 │              │
       ▼              ▼                 ▼              ▼
 :8081 catalog  :8082 data       :8083 identity  :8084 notifications
 ─────────────  ──────────────   ──────────────  ─────────────────
 L1 VoID        SPARQL endpoint  did:web DID doc  W3C LDN inbox
 L1 PROF        Named graphs     Conformance VC   PendingTask alerts
 L3 SHACL       SHACL-gated      VC verification  ActivityStreams
 L4 SPARQL      triple writes    DID resolution   event routing
    examples
```

## Services

| Service | Port | Role |
|---------|------|------|
| **catalog-service** | 8081 | Four-layer self-description (VoID/PROF/SHACL/SPARQL examples) |
| **data-service** | 8082 | SPARQL triple store with SHACL-validated writes |
| **identity-service** | 8083 | DID document + Verifiable Credential issuance/verification |
| **notification-service** | 8084 | W3C LDN inbox for inter-node messaging |

## Quick Start

```bash
# build all components
make build

# start all components
make start

# check components status
 make status

SERVICE                  STATUS   PID    PORT       HEALTH
──────────────────────────────────────────────────────────────────────
catalog-service          running               83022  8081       healthy
data-service             running               83030  8082       healthy
identity-service         running               83103  8083       healthy
notification-service     running               83111  8084       healthy

# Run the end-to-end demo
make demo
...
Demo complete.

All four fabric layers demonstrated:
  L1 VoID + PROF    → catalog:8081/.well-known/void
  L3 SHACL          → catalog:8081/.well-known/shacl
  L4 SPARQL examples → catalog:8081/.well-known/sparql-examples
  Data              → data:8082/sparql
  Identity (DID+VC) → identity:8083/.well-known/did.json
  Notifications(LDN)→ notifications:8084/inbox

# Check all service health
make health

# cleanup
make clean/all
```

## What an Agent Does with This Node

An LLM-based agent following the Knowledge Fabric progressive disclosure pattern:

```
1. GET /.well-known/void        (catalog:8081)
   → learns 3 beamlines exist; finds SPARQL endpoint URL

2. GET /.well-known/shacl       (catalog:8081)
   → learns Observations need resultTime, madeBySensor, observedProperty

3. GET /.well-known/sparql-examples (catalog:8081)
   → copies working query templates

4. GET /sparql?g=...observations (data:8082)
   → queries calibrated measurement data

5. GET /.well-known/did.json    (identity:8083)
   → resolves node identity, discovers all service endpoints

6. POST /inbox                  (notifications:8084)
   → subscribes to new-run events
```

Each step is self-contained; an agent can stop at any layer once it has enough
context to answer its query.

## Project Structure

```
chess-fabric-node/
├── CONCEPT.md                   ← Federated Knowledge Fabric concept summary
├── docker-compose.yml
├── Makefile
├── scripts/
│   └── demo.sh                  ← end-to-end curl demo
└── services/
    ├── catalog-service/         ← L1/L3/L4 self-description
    │   ├── cmd/server/main.go
    │   ├── internal/
    │   │   ├── rdf/             ← content negotiation
    │   │   └── void/            ← VoID, PROF, SHACL, SPARQL examples generators
    │   ├── go.mod
    │   ├── Makefile
    │   ├── Dockerfile
    │   └── README.md
    ├── data-service/            ← SPARQL + SHACL-validated writes
    │   ├── cmd/server/main.go
    │   ├── internal/
    │   │   ├── store/           ← in-memory triple store with named graphs
    │   │   ├── sparql/          ← SPARQL query handler
    │   │   └── shacl/           ← ObservationShape validator
    │   ├── go.mod
    │   ├── Makefile
    │   ├── Dockerfile
    │   └── README.md
    ├── identity-service/        ← DID + VC identity layer
    │   ├── cmd/server/main.go
    │   ├── internal/
    │   │   ├── did/             ← DID document generation (Ed25519)
    │   │   ├── vc/              ← Verifiable Credential issuance + verification
    │   │   └── integrity/       ← digestMultibase / digestSRI content checks
    │   ├── go.mod
    │   ├── Makefile
    │   ├── Dockerfile
    │   └── README.md
    └── notification-service/    ← W3C LDN inbox
        ├── cmd/server/main.go
        ├── internal/
        │   └── store/           ← notification inbox store
        ├── go.mod
        ├── Makefile
        ├── Dockerfile
        └── README.md
```
