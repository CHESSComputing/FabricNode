#!/usr/bin/env bash
# scripts/demo.sh
# End-to-end demo of the CHESS Federated Knowledge Fabric Node.
# Requires: curl, jq
# Run after: docker compose up --build  OR  all four services running locally
set -euo pipefail

CATALOG="http://localhost:8781"
DATA="http://localhost:8782"
IDENTITY="http://localhost:8783"
NOTIFY="http://localhost:8784"
JTYPE="Content-Type: application/ld+json"
JJSON="Content-Type: application/json"

# Beamline and dataset DID used throughout the write/query demos.
BEAMLINE="id1"
DID="/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"
DID_ENC=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "$DID")

sep()  { printf "\n\033[1;36m══ %s ══\033[0m\n" "$1"; }
ok()   { printf "\033[32m✓\033[0m %s\n" "$1"; }

# ── Health checks ─────────────────────────────────────────────────────────────
sep "Health checks"
for svc in "$CATALOG" "$DATA" "$IDENTITY" "$NOTIFY"; do
  result=$(curl -sf "$svc/health")
  ok "$svc → $result"
done

# ── L1: VoID discovery ────────────────────────────────────────────────────────
sep "L1 — VoID dataset description (Turtle)"
curl -s "$CATALOG/.well-known/void"

sep "L1 — VoID as JSON-LD"
curl -s -H "Accept: application/ld+json" "$CATALOG/.well-known/void" | jq .

sep "L1 — PROF capability profile"
curl -s "$CATALOG/.well-known/profile"

# ── L3: SHACL shapes ─────────────────────────────────────────────────────────
sep "L3 — SHACL shapes"
curl -s "$CATALOG/.well-known/shacl"

# ── L4: SPARQL examples ───────────────────────────────────────────────────────
sep "L4 — SPARQL examples catalog"
curl -s "$CATALOG/.well-known/sparql-examples"

# ── Catalog: beamline + dataset discovery ────────────────────────────────────
sep "Catalog — list beamlines"
curl -s "$CATALOG/catalog/beamlines" | jq .

sep "Catalog — datasets for beamline $BEAMLINE"
echo "$CATALOG/catalog/beamlines/$BEAMLINE/datasets"
curl -s "$CATALOG/catalog/beamlines/$BEAMLINE/datasets" | jq .

# ── Data: global query ────────────────────────────────────────────────────────
sep "Data — list all named graphs"
curl -s "$DATA/graphs" | jq .

sep "Data — list named graphs for beamline $BEAMLINE"
curl -s "$DATA/beamlines/$BEAMLINE/graphs" | jq .

sep "Data — beamline-scoped SPARQL (all triples for $BEAMLINE)"
curl -s "$DATA/beamlines/$BEAMLINE/sparql" \
  | jq '.results.bindings | length'

sep "Data — global SPARQL: beamline descriptor graph"
curl -s "$DATA/sparql?g=http://chess.cornell.edu/graph/beamlines" \
  | jq '.results.bindings | length'

sep "Data — describe beamline id1"
curl -s "$DATA/sparql?describe=http://chess.cornell.edu/beamline/id1" | jq .

sep "Data — keyword search for 'crystallography'"
curl -s "$DATA/sparql?search=crystallography" | jq .

sep "Data — SOSA observations (global)"
curl -s "$DATA/sparql?p=http://www.w3.org/1999/02/22-rdf-syntax-ns%23type&o=http://www.w3.org/ns/sosa/Observation" \
  | jq '.results.bindings | length'

# ── Data: dataset-scoped SPARQL ───────────────────────────────────────────────
sep "Data — dataset-scoped SPARQL for DID: $DID"
curl -s "$DATA/beamlines/$BEAMLINE/datasets/$DID_ENC/sparql" \
  | jq '.results.bindings | length'

# ── Data: SHACL-validated write ───────────────────────────────────────────────
sep "Data — insert valid observation into $BEAMLINE / $DID"
curl -s -X POST "$DATA/beamlines/$BEAMLINE/datasets/$DID_ENC/triples" \
  -H "$JJSON" \
  -d '[
    {
      "subject":    "http://chess.cornell.edu/observation/demo-01",
      "predicate":  "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
      "object":     "http://www.w3.org/ns/sosa/Observation",
      "objectType": "uri"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/demo-01",
      "predicate":  "http://www.w3.org/ns/sosa/resultTime",
      "object":     "2025-03-06T10:00:00Z^^http://www.w3.org/2001/XMLSchema#dateTime",
      "objectType": "literal"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/demo-01",
      "predicate":  "http://www.w3.org/ns/sosa/madeBySensor",
      "object":     "http://chess.cornell.edu/sensor/id1-detector-01",
      "objectType": "uri"
    },
    {
      "subject":    "http://chess.cornell.edu/observation/demo-01",
      "predicate":  "http://www.w3.org/ns/sosa/observedProperty",
      "object":     "http://chess.cornell.edu/property/lattice-parameter-a",
      "objectType": "uri"
    }
  ]' | jq .

sep "Data — dry-run validate INVALID observation (missing resultTime)"
curl -s -X POST "$DATA/beamlines/$BEAMLINE/datasets/$DID_ENC/validate" \
  -H "$JJSON" \
  -d '[
    {
      "subject":    "http://chess.cornell.edu/observation/bad-01",
      "predicate":  "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
      "object":     "http://www.w3.org/ns/sosa/Observation",
      "objectType": "uri"
    }
  ]' | jq .

sep "Data — insert INVALID observation (missing resultTime → SHACL rejected)"
curl -s -X POST "$DATA/beamlines/$BEAMLINE/datasets/$DID_ENC/triples" \
  -H "$JJSON" \
  -d '[
    {
      "subject":    "http://chess.cornell.edu/observation/bad-01",
      "predicate":  "http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
      "object":     "http://www.w3.org/ns/sosa/Observation",
      "objectType": "uri"
    }
  ]' | jq .

# ── Identity ──────────────────────────────────────────────────────────────────
sep "Identity — DID document"
curl -s "$IDENTITY/.well-known/did.json" \
  | jq '{id:.id,services:[.service[].type]}'

sep "Identity — FabricConformanceCredential"
curl -s "$IDENTITY/credentials/conformance" \
  | jq '{id:.id,issuer:.issuer,type:.type}'

sep "Identity — round-trip VC verification"
CRED=$(curl -s "$IDENTITY/credentials/conformance")
echo "$CRED" | curl -s -X POST "$IDENTITY/credentials/verify" \
  -H "$JJSON" -d @- | jq .

# ── Notifications (LDN inbox) ─────────────────────────────────────────────────
sep "Notifications — send new-run event (scoped to beamline $BEAMLINE)"
curl -s -X POST "$NOTIFY/inbox" \
  -H "$JTYPE" \
  -d '{
    "@context": "https://www.w3.org/ns/activitystreams",
    "@type":    "chess:NewRun",
    "actor":    "http://chess.cornell.edu/sensor/id1-detector-01",
    "object": {
      "chess:runNumber": 1027,
      "chess:beamline":  "http://chess.cornell.edu/beamline/id1",
      "chess:did":       "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std"
    },
    "target": "http://chess.cornell.edu/beamline/id1"
  }' | jq .

sep "Notifications — send trust-gap (PendingTask)"
curl -s -X POST "$NOTIFY/inbox" \
  -H "$JTYPE" \
  -d '{
    "@context": "https://w3id.org/cogitarelink/fabric/v1",
    "@type":    "fabric:PendingTask",
    "actor":    "did:web:chess-node",
    "object":   {"fabric:reason": "missing AgentAuthorizationCredential"}
  }' | jq .

sep "Notifications — list inbox"
curl -s "$NOTIFY/inbox" | jq '{totalItems:.totalItems}'

sep "Notifications — stats"
curl -s "$NOTIFY/inbox/stats" | jq .

printf "\n\033[1;32m✅  Demo complete.\033[0m\n\n"
printf "All four fabric layers demonstrated:\n"
printf "  L1 VoID + PROF      → catalog:8781/.well-known/void\n"
printf "  L3 SHACL            → catalog:8781/.well-known/shacl\n"
printf "  L4 SPARQL examples  → catalog:8781/.well-known/sparql-examples\n"
printf "  Catalog beamlines   → catalog:8781/catalog/beamlines\n"
printf "  Data (global)       → data:8782/sparql\n"
printf "  Data (beamline)     → data:8782/beamlines/{id}/sparql\n"
printf "  Data (dataset)      → data:8782/beamlines/{id}/datasets/{did}/sparql\n"
printf "  Identity (DID+VC)   → identity:8783/.well-known/did.json\n"
printf "  Notifications (LDN) → notify:8784/inbox\n"
