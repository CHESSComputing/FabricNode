# Federated Knowledge Fabric — Concept Summary

## What It Is

The **Federated Knowledge Fabric** (FKF) is an experimental cyberinfrastructure
for research data systems. It creates a network of independent, self-describing
knowledge nodes that can discover and interact with each other automatically —
without prior manual integration — using only shared open standards.

Each node is a self-contained **SPARQL endpoint** that publishes machine-readable
metadata about its data, structure, access patterns, and identity. Because the
description format is standardised, any software agent — including LLM-based
agents — can encounter an unfamiliar node and immediately understand how to use it.

The project simultaneously serves two purposes:

1. **Prototype cyberinfrastructure** — a working, federated research data layer
   built entirely on W3C and FAIR standards.
2. **Agent research platform** — a testbed for measuring how well LLM-based
   agents can autonomously navigate structured knowledge graphs.

---

## The Core Problem It Solves

Modern agentic coding tools make it cheap and fast to build custom research
software. The bottleneck has shifted from *can we build it* to *can it talk to
anything else*. Each research group's system works internally; none of them
interoperate. The Knowledge Fabric addresses this by enforcing **shared protocols
and metadata formats** as a governance layer — nodes built by independent teams
automatically understand each other because they all speak the same standards.

---

## Progressive Disclosure Instead of RAG

The key design principle is **progressive disclosure**: agents navigate knowledge
graphs layer by layer rather than downloading everything upfront (as in typical
RAG pipelines).

An agent encountering a node for the first time:

1. Reads a **compact service description** (VoID) — learns what data exists.
2. Examines **ontology definitions** (TBox) — learns the semantic vocabulary.
3. Inspects **data shapes and constraints** (SHACL) — learns what properties to expect.
4. Uses **example queries** (SPARQL examples catalog) — constructs its own queries.

This keeps the LLM context window bounded while allowing it to navigate datasets
far larger than any context limit.

---

## The Four-Layer Knowledge Representation Stack

Every conformant fabric node exposes four layers of structured metadata:

| Layer | Standard | Endpoint | What it provides |
|-------|----------|----------|------------------|
| **L1** Service Description | VoID + PROF | `/.well-known/void` | What datasets and vocabularies exist |
| **L2** TBox Ontologies | OWL/RDFS | `/ontology/{vocab}` | Class hierarchies, property domain/range |
| **L3** SHACL Shapes | SHACL 1.2 | `/.well-known/shacl` | Data constraints, required properties |
| **L4** Query Examples | SPARQL examples | `/.well-known/sparql-examples` | Working query patterns |

---

## Technology Stack

The fabric is built entirely on open standards — these act as **interoperability
contracts** ensuring independently-built nodes can interact without custom glue:

| Standard | Role |
|----------|------|
| **SPARQL** | Querying RDF knowledge graphs |
| **VoID** | Dataset and service description |
| **PROF** | Capability profiling and conformance |
| **SHACL** | Data validation and governance |
| **Linked Data Notifications (LDN)** | Standards-based messaging between nodes |
| **Decentralized Identifiers (DID)** | Cryptographic identity for nodes and agents |
| **Verifiable Credentials (VC)** | Trust, attestation, and delegation chains |

---

## FAIR Was Always About Machines

The FAIR Guiding Principles (Wilkinson et al., 2016) are widely treated as a
checklist for human researchers. The original paper is explicit that FAIR was
designed for **autonomous machines** that find, understand, and use data without
human intervention. The four capabilities FAIR requires of a machine explorer —
identify structure, determine utility, check access conditions, take appropriate
action — map directly onto the four-layer KR stack.

LLM-based agents are the machines FAIR was written for. The fabric tests whether
FAIR's design choices, taken seriously as an engineering specification, actually
produce the machine-navigable infrastructure the authors envisioned. Phase 3
experimental results (+0.167 score lift from structured TBox metadata for
unfamiliar vocabularies) provide the first direct evidence that they do.

---

## Research Integrity and the Responsibility Chain

Every data write in the fabric carries a **cryptographically verifiable
credential chain** linking it back to a named human researcher:

```
Data write
  └── AgentAuthorizationCredential (signed by agent DID)
        └── Delegated by FabricDelegationCredential (signed by researcher)
              └── Researcher identified by ORCID + institutional ROR affiliation
```

This matters for research integrity policy: NSF's updated misconduct definition
now explicitly covers AI-assisted research, and the CICI/IPAAI funding track
specifically funds verifiable provenance infrastructure for AI-ready datasets.
The fabric provides this at the infrastructure level, not as an audit layer bolted
on afterwards.

---

## Applicability to Cornell CHESS

The Cornell High Energy Synchrotron Source (CHESS) is an ideal deployment
scenario for a Knowledge Fabric node. CHESS operates multiple beamlines, each
producing distinct experimental datasets:

| Beamline | Experiment Type | Data Characteristics |
|----------|----------------|----------------------|
| `id1` | X-ray diffraction | Crystallographic structure data |
| `id3` | Protein crystallography | Macromolecular structure |
| `fast` | Time-resolved scattering | High-cadence sequential measurements |

Each beamline maps naturally to a **named graph** in the fabric node, with its
own SHACL shape governing what constitutes a valid observation record. External
agents — or remote researchers — can:

1. **Discover** available beamlines and their dataset descriptions via `/.well-known/void`.
2. **Understand** measurement semantics (e.g. SOSA `Observation`, SIO measured values)
   via the TBox ontologies.
3. **Validate** incoming data or query results against SHACL shapes.
4. **Query** specific run conditions, calibration records, or cross-beamline
   comparisons via SPARQL — without any prior manual integration.
5. **Subscribe** to new-run notifications via Linked Data Notifications.
6. **Verify** the node's identity and data integrity via its DID document and
   Verifiable Conformance Credential.

The governance model is also well-matched to CHESS operations: instrument
scientists hold the delegation credentials; automated data-collection agents act
under those delegations; every observation write is SHACL-validated before
admission; and trust gaps (missing provenance, ambiguous entity identity) surface
to the responsible scientist's LDN inbox rather than silently failing.

The result is a beamline data system that is **automatically discoverable by any
agent or external system** that speaks SPARQL and follows the Knowledge Fabric
standards — today, and without bilateral integration work, as new collaborators
arrive.

---

## Key References

- Wilkinson et al. (2016). The FAIR Guiding Principles. *Scientific Data*, 3, 160018.
- Janowicz et al. (2014). Five Stars of Linked Data Vocabulary Use. *Semantic Web*, 5(3).
- Zhang, Kraska & Khattab (2025). Recursive Language Models. arXiv:2512.24601.
- Ranaldi et al. (2025). Protoknowledge shapes behaviour of LLMs. arXiv:2505.15501.
- Allemang & Sequeda (2024). Increasing LLM Accuracy with Ontologies. arXiv:2405.11706.
- https://github.com/LA3D/cogitarelink-fabric
