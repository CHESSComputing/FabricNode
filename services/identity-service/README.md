# identity-service

The **identity-service** provides decentralized identity and trust for the
CHESS fabric node. It generates a `did:web` identity at startup, self-issues a
`FabricConformanceCredential` (a W3C Verifiable Credential attesting that this
node conforms to `fabric:CoreProfile`), and exposes DID resolution and VC
verification endpoints.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/.well-known/did.json` | W3C DID document |
| `GET` | `/credentials/conformance` | Self-issued FabricConformanceCredential |
| `POST` | `/credentials/verify` | Verify a VC against this node's public key |
| `GET` | `/did/{did}` | DID Resolution HTTP API |
| `GET` | `/keys/node-key-1` | Public key export |
| `GET` | `/health` | Health check |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_BASE_URL` | `http://localhost:8783` | Public base URL |
| `NODE_ID` | `chess-node` | Node identifier |
| `NODE_NAME` | `CHESS Federated Knowledge Fabric Node` | Human label |
| `CATALOG_URL` | `http://localhost:8781` | URL of the catalog service |
| `DATA_URL` | `http://localhost:8782` | URL of the data service |
| `NOTIFICATION_URL` | `http://localhost:8784` | URL of the notification service |
| `PORT` | `8783` | Listen port |

## curl Examples

```bash
# Retrieve the DID document
curl http://localhost:8783/.well-known/did.json | jq .

# Retrieve the self-issued FabricConformanceCredential
curl http://localhost:8783/credentials/conformance | jq .

# Verify the conformance credential (round-trip test)
CRED=$(curl -s http://localhost:8783/credentials/conformance)
curl -X POST http://localhost:8783/credentials/verify \
  -H "Content-Type: application/json" \
  -d "$CRED" | jq .
# → {"verified":true,"issuer":"did:web:...","verificationMethod":"...#node-key-1"}

# Retrieve the node's Ed25519 public key
curl http://localhost:8783/keys/node-key-1 | jq .

# Health check (confirms DID)
curl http://localhost:8783/health | jq .
```

## What the DID Document Contains

The DID document declares four service endpoints pointing to the other services:

| Service Type | Endpoint |
|-------------|----------|
| `SPARQLEndpoint` | `data-service/sparql` |
| `VoIDDescription` | `catalog-service/.well-known/void` |
| `SHACLShapes` | `catalog-service/.well-known/shacl` |
| `LDNInbox` | `notification-service/inbox` |

An agent resolving this node's DID can discover all fabric endpoints without
any out-of-band configuration.

## Conformance Credential

The `FabricConformanceCredential` is a W3C VC 2.0 with:
- An `eddsa-jcs-2022` Data Integrity proof (Ed25519 signature)
- `dct:conformsTo fabric:CoreProfile` claim
- `relatedResource` array binding self-description artifacts to SHA-256 hashes

This credential can be submitted to any fabric bootstrap witness for node
admission, and can be verified offline using only the node's DID document.
