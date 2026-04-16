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
 :8781 catalog  :8782 data       :8783 identity  :8784 notifications
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
| **catalog-service** | 8781 | Four-layer self-description (VoID/PROF/SHACL/SPARQL examples) |
| **data-service** | 8782 | SPARQL triple store with SHACL-validated writes |
| **identity-service** | 8783 | DID document + Verifiable Credential issuance/verification |
| **notification-service** | 8784 | W3C LDN inbox for inter-node messaging |

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
catalog-service          running               83022  8781       healthy
data-service             running               83030  8782       healthy
identity-service         running               83103  8783       healthy
notification-service     running               83111  8784       healthy

# Run the end-to-end demo
make demo
...
Demo complete.

All four fabric layers demonstrated:
  L1 VoID + PROF    → catalog:8781/.well-known/void
  L3 SHACL          → catalog:8781/.well-known/shacl
  L4 SPARQL examples → catalog:8781/.well-known/sparql-examples
  Data              → data:8782/sparql
  Identity (DID+VC) → identity:8783/.well-known/did.json
  Notifications(LDN)→ notifications:8784/inbox

# Check all service health
make health

# cleanup
make clean/all
```

## What an Agent Does with This Node

An LLM-based agent following the Knowledge Fabric progressive disclosure pattern:

```
1. GET /.well-known/void        (catalog:8781)
   → learns which CHESS beamlines exist; finds SPARQL endpoint URL

2. GET /.well-known/shacl       (catalog:8781)
   → learns Observations need resultTime, madeBySensor, observedProperty

3. GET /.well-known/sparql-examples (catalog:8781)
   → copies working query templates

4. GET /sparql?g=...observations (data:8782)
   → queries calibrated measurement data

5. GET /.well-known/did.json    (identity:8783)
   → resolves node identity, discovers all service endpoints

6. POST /inbox                  (notifications:8784)
   → subscribes to new-run events
```

Each step is self-contained; an agent can stop at any layer once it has enough
context to answer its query.

## Project Structure

```
FabricNode/
├── CONCEPT.md                   ← Federated Knowledge Fabric concept summary
└── services/
    ├── catalog-service/         ← L1/L3/L4 self-description
    │   ├── internal/
    │   │   ├── rdf/             ← content negotiation
    │   │   └── void/            ← VoID, PROF, SHACL, SPARQL examples generators
    ├── data-service/            ← SPARQL + SHACL-validated writes
    │   ├── internal/
    │   │   ├── store/           ← in-memory triple store with named graphs
    │   │   ├── sparql/          ← SPARQL query handler
    │   │   └── shacl/           ← ObservationShape validator
    ├── identity-service/        ← DID + VC identity layer
    │   ├── internal/
    │   │   ├── did/             ← DID document generation (Ed25519)
    │   │   ├── vc/              ← Verifiable Credential issuance + verification
    │   │   └── integrity/       ← digestMultibase / digestSRI content checks
    └── notification-service/    ← W3C LDN inbox
        ├── internal/
        │   └── store/           ← notification inbox store
```

### Local development

To work locally ensure that you have `go.work`

```bash
# if go.work file does not exist do the following
go work init ./pkg/... ./services/...
go work sync
make build
```

Here is an example of `go.work` file


```
go 1.26.1

use (
	./pkg/config
	./pkg/model
	./pkg/server
    ./services/catalog-service
    ./services/data-service
    ./services/identity-service
    ./services/notification-service
)
```

Ensure CI works without it

Test:

```bash
GOWORK=off make build
```
