Great, this is a solid, well-structured codebase—nice separation of services already. I went through the layout and key components, and I’ll focus specifically on what needs to change conceptually and structurally so the **FabricNode becomes a “catalog of beamlines → each beamline → multiple datasets”** (instead of a flatter dataset-centric model).

---

# 🔧 Big Picture Shift (What You Already Started)

You’ve already refactored **catalog-service** toward:

```
Node
 └── Beamlines (logical grouping / sub-catalogs)
       └── Datasets
```

Now the rest of the services still behave more like:

```
Node
 └── Datasets (flat or loosely grouped)
```

So the work is about making **data-service, identity-service, notification-service** align with:

👉 “Beamline is a first-class entity”

---

# 1️⃣ data-service (MOST IMPORTANT)

## Current Structure (from code)

* `handlers/`

  * `triples.go`
  * `sparql.go`
* `store/triplestore.go`
* SPARQL endpoint
* SHACL validation

### ❗ Problem

Right now, the service is **dataset-agnostic or dataset-flat**:

* No explicit concept of **beamline ownership**
* Likely storing triples per dataset or globally
* SPARQL queries are not scoped by beamline

---

## ✅ What You Need to Introduce

### 1. Beamline as a Namespace / Partition

You need **logical partitioning**:

**Option A (recommended): named graphs**

```sparql
GRAPH <beamline/{id}/dataset/{id}> { ... }
```

**Option B: prefix-based URIs**

```
/beamlines/{beamlineId}/datasets/{datasetId}
```

👉 Named graphs are cleaner and align with RDF best practices.

---

### 2. Update Storage Layer (`triplestore.go`)

Add concept of:

```go
type DatasetRef struct {
    BeamlineID string
    DatasetID  string
}
```

Then enforce:

* Every write → requires `BeamlineID`
* Every query → scoped to beamline (unless global)

---

### 3. Update Handlers

#### `triples.go`

Currently probably:

```
POST /datasets/{id}/triples
```

👉 Change to:

```
POST /beamlines/{beamlineId}/datasets/{datasetId}/triples
```

And internally:

* Write to graph:

  ```
  beamline/{beamlineId}/dataset/{datasetId}
  ```

---

#### `sparql.go`

Add:

* Query scoping:

  * per dataset
  * per beamline
  * global

Example:

```
GET /beamlines/{beamlineId}/sparql
GET /beamlines/{beamlineId}/datasets/{datasetId}/sparql
```

Then rewrite query:

```sparql
FROM NAMED <beamline/{beamlineId}/*>
```

---

### 4. SHACL Validation

Right now:

```
validator.go
```

👉 Extend to support:

* Beamline-level constraints
* Dataset-level constraints

Example:

```ttl
:BeamlineShape { ... }
:DatasetShape { ... }
```

---

### 5. Metadata Enrichment

Every dataset should include:

```json
{
  "beamline_id": "...",
  "dataset_id": "...",
  "source_node": "...",
  "created_at": "...",
  "type": "experiment | simulation | derived"
}
```

---

## 🔥 Summary (data-service)

You need:

* Beamline-aware routing
* Named graph partitioning
* Scoped SPARQL
* Beamline-aware validation

---

# 2️⃣ notification-service

## Current Structure

* Simple store: `store/notification.go`

### ❗ Problem

Notifications are likely:

* global
* dataset-based (maybe)
* not grouped by beamline

---

## ✅ Required Changes

### 1. Introduce Notification Scope

```go
type NotificationScope struct {
    BeamlineID string
    DatasetID  *string // optional
}
```

---

### 2. Notification Types

Add structure:

```go
type Notification struct {
    ID          string
    Type        string
    Scope       NotificationScope
    Message     string
    Timestamp   time.Time
}
```

---

### 3. Add Beamline Events

Examples:

* `beamline.created`
* `beamline.updated`
* `dataset.added`
* `dataset.updated`
* `dataset.validated`
* `data.ingested`

---

### 4. API Changes

Instead of:

```
GET /notifications
```

Add:

```
GET /beamlines/{beamlineId}/notifications
GET /beamlines/{beamlineId}/datasets/{datasetId}/notifications
```

---

### 5. Event Producers

You’ll want:

* data-service → emits events
* catalog-service → emits structure changes

👉 Suggest introducing:

```
event bus abstraction (even if in-memory for now)
```

---

## 🔥 Summary (notification-service)

Make notifications:

* scoped to beamline/dataset
* event-driven
* structured (not just logs/messages)

---

# 3️⃣ identity-service

## Current Structure

* DID documents
* Verifiable credentials
* Key management

---

### ❗ Problem

Identity is likely:

* Node-level only
* Not tied to beamlines or datasets

---

## ✅ Required Changes

### 1. Beamline as Identity Subject

Introduce:

```go
type BeamlineIdentity struct {
    ID          string
    DID         string
    Owner       string
    Permissions []string
}
```

---

### 2. Dataset-Level Identity (Optional but Powerful)

Each dataset can have:

* provenance
* ownership
* access control

```go
type DatasetIdentity struct {
    DatasetID  string
    BeamlineID string
    DID        string
}
```

---

### 3. Extend DID Documents

In `did/document.go`:

Add relationships:

```json
{
  "id": "did:fabric:beamline:123",
  "controller": "did:fabric:node:xyz",
  "service": [
    {
      "type": "DatasetCatalog",
      "endpoint": "/beamlines/123/datasets"
    }
  ]
}
```

---

### 4. Credentials

Extend `vc/credential.go`:

Support:

* dataset provenance credential
* beamline ownership credential

Example:

```json
{
  "type": ["VerifiableCredential", "DatasetCredential"],
  "subject": {
    "dataset_id": "...",
    "beamline_id": "...",
    "owner": "..."
  }
}
```

---

### 5. Access Control Hooks

Eventually:

* data-service checks identity-service
* enforce:

  * read/write permissions
  * beamline ownership

---

## 🔥 Summary (identity-service)

Add:

* Beamline-level identity
* Dataset-level identity
* Credentials for provenance + ownership

---

# 🔗 Cross-Service Alignment (IMPORTANT)

You should standardize IDs across ALL services:

```text
beamline_id
dataset_id
node_id
```

And consistent URI scheme:

```
/beamlines/{beamlineId}
/beamlines/{beamlineId}/datasets/{datasetId}
```

---

# 🧠 Recommended Next Step (Practical)

Do this incrementally:

### Step 1

Refactor **data-service routes + storage**

### Step 2

Update **catalog-service → data-service integration**

### Step 3

Add **notifications scoped by beamline**

### Step 4

Extend **identity model**

---

# ⚡ Architectural Suggestion (Optional but Powerful)

Introduce a shared package:

```
/pkg/model
```

With:

```go
type Beamline struct { ... }
type Dataset struct { ... }
type DatasetRef struct { ... }
```

So all services speak the same language.

---

# If You Want Next

I can:

* Propose exact API specs (OpenAPI-style)
* Show concrete Go refactoring for one service (e.g., data-service)
* Or map how catalog-service should call data-service now

Just tell me where you want to go deeper.

