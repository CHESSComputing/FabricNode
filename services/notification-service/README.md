# notification-service

The **notification-service** implements a [W3C Linked Data Notifications
(LDN)](https://www.w3.org/TR/ldn/) inbox for the CHESS fabric node.

LDN is the messaging protocol between fabric actors — nodes, agents, and human
researchers. It replaces custom webhook endpoints with a standards-based protocol
that any LDN-aware client can use without prior integration.

## Event Types

| Type | Description |
|------|-------------|
| `chess:NewRun` | A new experimental run has been recorded |
| `chess:DataReady` | A dataset is ready for analysis |
| `fabric:SchemaChange` | SHACL shape or vocabulary has been updated |
| `fabric:NodeAdmission` | A new node has joined the fabric |
| `fabric:PendingTask` | Trust gap requiring human attention |
| `as:Announce` | Generic ActivityStreams announcement |

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/inbox` | List notifications (LDN container) |
| `POST` | `/inbox` | Receive a notification (JSON-LD body) |
| `GET` | `/inbox/{id}` | Retrieve a specific notification |
| `POST` | `/inbox/{id}/ack` | Acknowledge / mark handled |
| `GET` | `/inbox/stats` | Count pending / total notifications |
| `GET` | `/health` | Health check |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_ID` | `chess-node` | Node identifier |
| `PORT` | `8084` | Listen port |

## curl Examples

```bash
# List all notifications
curl http://localhost:8084/inbox | jq .

# Send a new-run notification (JSON-LD body)
curl -X POST http://localhost:8084/inbox \
  -H "Content-Type: application/ld+json" \
  -d '{
    "@context": "https://www.w3.org/ns/activitystreams",
    "@type":    "chess:NewRun",
    "actor":    "http://chess.cornell.edu/sensor/id1-detector-01",
    "object": {
      "@type":           "chess:ExperimentalRun",
      "chess:runNumber": 1026,
      "chess:beamline":  "http://chess.cornell.edu/beamline/id1"
    },
    "target": "http://chess.cornell.edu/dataset/beamline-id1"
  }'

# Send a trust-gap notification (surfaces to researcher inbox)
curl -X POST http://localhost:8084/inbox \
  -H "Content-Type: application/ld+json" \
  -d '{
    "@context": "https://w3id.org/cogitarelink/fabric/v1",
    "@type":    "fabric:PendingTask",
    "actor":    "did:web:chess-node",
    "object": {
      "@type":           "fabric:TrustGap",
      "fabric:reason":   "missing AgentAuthorizationCredential",
      "fabric:entity":   "http://chess.cornell.edu/observation/unverified-01"
    }
  }'

# Retrieve a specific notification by ID
curl http://localhost:8084/inbox/urn:uuid:SOME-UUID | jq .

# Acknowledge a notification
curl -X POST http://localhost:8084/inbox/urn:uuid:SOME-UUID/ack | jq .

# Filter by type
curl "http://localhost:8084/inbox?type=fabric:PendingTask" | jq .

# Stats
curl http://localhost:8084/inbox/stats | jq .
```

## LDN Discovery

Per the W3C LDN specification, the inbox URL is advertised via an HTTP `Link`
header on the service root:

```
Link: </inbox>; rel="http://www.w3.org/ns/ldp#inbox"
```

Agents and other nodes discover this inbox by resolving the node's DID document,
which lists the LDN inbox as a `Service` endpoint. No out-of-band configuration
is required.
