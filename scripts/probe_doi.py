#!/usr/bin/env python
"""
scripts/probe_doi.py — FOXDEN DOI → FabricNode identity-service integration probe.

Traces the full DOI publication credential flow:

  1. Health-check identity-service and data-service
  2. Resolve the node DID document (proves identity-service is functioning)
  3. Simulate FOXDEN DOI service: POST /credentials/dataset to request a
     signed DatasetPublicationCredential from the identity-service
  4. Validate the returned credential structure and proof
  5. Round-trip verify the credential: POST /credentials/dataset/verify
  6. Confirm the credential's SPARQL endpoint is reachable on data-service
  7. Optionally ingest the dataset into data-service first (--ingest flag)

Usage:
  python scripts/probe_doi.py \\
      --did  "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271" \\
      --doi  "10.5281/zenodo.123456" \\
      --doi-url "https://doi.org/10.5281/zenodo.123456"

  python scripts/probe_doi.py \\
      --did "/beamline=id1/btr=btr001/cycle=2024-3/sample_name=silicon-std" \\
      --doi "10.5281/zenodo.999999" \\
      --doi-url "https://doi.org/10.5281/zenodo.999999" \\
      --ingest \\
      --verbose

Options:
  --did        Dataset DID (required)
  --doi        DOI string, e.g. 10.5281/zenodo.123456 (required)
  --doi-url    Resolvable DOI URL (required)
  --identity   identity-service base URL  (default: http://localhost:8083)
  --data       data-service base URL      (default: http://localhost:8082)
  --foxden     FOXDEN base URL            (default: http://localhost:8300)
  --ingest     Also ingest the dataset into data-service before issuing VC
  --verbose    Print full JSON request/response bodies
"""

import argparse
import json
import sys
import urllib.parse
import urllib.request
import urllib.error
from typing import Any

# ──────────────────────────────────────────────────────────────────────────────
# Colour helpers
# ──────────────────────────────────────────────────────────────────────────────

USE_COLOR = sys.stdout.isatty()

def _c(code, text): return f"\033[{code}m{text}\033[0m" if USE_COLOR else text
def green(t):  return _c("32", t)
def red(t):    return _c("31", t)
def cyan(t):   return _c("36", t)
def yellow(t): return _c("33", t)
def bold(t):   return _c("1",  t)

# ──────────────────────────────────────────────────────────────────────────────
# HTTP helpers
# ──────────────────────────────────────────────────────────────────────────────

def _request(method, url, body=None, timeout=10):
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode()
            try:    return resp.status, json.loads(raw)
            except: return resp.status, raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:    return e.code, json.loads(raw)
        except: return e.code, raw
    except urllib.error.URLError as e:
        raise ConnectionError(f"Cannot reach {url}: {e.reason}") from e

def get(url, timeout=10):  return _request("GET",  url, timeout=timeout)
def post(url, body, timeout=10): return _request("POST", url, body=body, timeout=timeout)

def encode_did(did): return urllib.parse.quote(did, safe="")
def beamline_from_did(did):
    seg = did.lstrip("/").split("/")[0]
    return seg.split("=")[1].lower() if "=" in seg else ""

# ──────────────────────────────────────────────────────────────────────────────
# Result tracker
# ──────────────────────────────────────────────────────────────────────────────

class Results:
    def __init__(self):
        self.passed = self.failed = 0

    def ok(self, label):
        self.passed += 1
        print(f"  {green('✓')} {label}")

    def fail(self, label, detail=""):
        self.failed += 1
        print(f"  {red('✗')} {label}")
        if detail:
            for line in str(detail).strip().splitlines():
                print(f"      {red(line)}")

    def summary(self):
        total = self.passed + self.failed
        print()
        if self.failed == 0:
            print(bold(green(f"All {total} checks passed.")))
        else:
            print(bold(red(f"{self.failed}/{total} checks failed.")))

    @property
    def exit_code(self): return 0 if self.failed == 0 else 1

def sep(title): print(f"\n{cyan('══')} {bold(title)} {cyan('══')}")

def vprint(verbose, label, body=None):
    if not verbose: return
    print(f"    {yellow(label)}")
    if body is not None:
        lines = json.dumps(body, indent=2).splitlines()
        for line in lines[:40]: print(f"    {line}")
        if len(lines) > 40: print(f"    … ({len(lines)-40} more lines)")

# ──────────────────────────────────────────────────────────────────────────────
# Step 0 — health checks
# ──────────────────────────────────────────────────────────────────────────────

def check_health(results, identity_url, data_url):
    sep("Health checks")
    for name, url in [("identity-service", f"{identity_url}/health"),
                      ("data-service",     f"{data_url}/health")]:
        try:
            code, body = get(url)
            if code == 200 and isinstance(body, dict) and body.get("status") == "ok":
                results.ok(f"{name} healthy")
            else:
                results.fail(f"{name} health check", f"HTTP {code}: {body}")
        except ConnectionError as e:
            results.fail(f"{name} reachable", str(e))

# ──────────────────────────────────────────────────────────────────────────────
# Step 1 — resolve DID document
# ──────────────────────────────────────────────────────────────────────────────

def resolve_did(results, identity_url, verbose):
    sep("DID document resolution")

    try:
        code, body = get(f"{identity_url}/.well-known/did.json")
    except ConnectionError as e:
        results.fail("DID document reachable", str(e))
        return None

    if code != 200:
        results.fail("DID document HTTP 200", f"Got {code}")
        return None
    results.ok("DID document HTTP 200")

    # Expected shape:
    # {
    #   "@context": ["https://www.w3.org/ns/did/v1", ...],
    #   "id": "did:web:localhost%3A8083",
    #   "verificationMethod": [{"id": "...", "type": "Ed25519VerificationKey2020", ...}],
    #   "service": [{"type": "SPARQLEndpoint", "serviceEndpoint": "..."}, ...]
    # }

    node_did = body.get("id", "")
    if node_did.startswith("did:web:"):
        results.ok(f"DID is did:web method: {node_did}")
    else:
        results.fail("DID uses did:web method", f"Got: {node_did!r}")

    vm = body.get("verificationMethod", [])
    if vm and vm[0].get("type") == "Ed25519VerificationKey2020":
        results.ok(f"Ed25519 verification key present: {vm[0]['id']}")
    else:
        results.fail("Ed25519 verification key present", f"verificationMethod: {vm}")

    services = {s["type"]: s["serviceEndpoint"] for s in body.get("service", [])}
    if "SPARQLEndpoint" in services:
        results.ok(f"SPARQLEndpoint service declared: {services['SPARQLEndpoint']}")
    else:
        results.fail("SPARQLEndpoint service in DID doc", f"services: {list(services.keys())}")

    vprint(verbose, "DID document:", body)
    return body

# ──────────────────────────────────────────────────────────────────────────────
# Step 2 — optional: ingest dataset into data-service first
# ──────────────────────────────────────────────────────────────────────────────

def ingest_dataset(results, data_url, foxden_url, did_str, verbose):
    sep(f"Dataset ingest (pre-requisite)")
    bl = beamline_from_did(did_str)
    enc = encode_did(did_str)
    url = f"{data_url}/beamlines/{bl}/datasets/{enc}/foxden/ingest"
    vprint(verbose, f"POST {url}")
    try:
        code, body = post(url, body=None)
        if code in (200, 201) and isinstance(body, dict) and body.get("ingested", 0) > 0:
            results.ok(f"Dataset ingested: {body.get('ingested')} triples → {body.get('graphIRI','')[:60]}")
        else:
            results.fail("Dataset ingest", f"HTTP {code}: {body}")
    except ConnectionError as e:
        results.fail("Data-service reachable for ingest", str(e))

# ──────────────────────────────────────────────────────────────────────────────
# Step 3 — issue DatasetPublicationCredential
# ──────────────────────────────────────────────────────────────────────────────

def issue_credential(results, identity_url, did_str, doi, doi_url, verbose):
    sep("Issue DatasetPublicationCredential")

    payload = {
        "did":     did_str,
        "doi":     doi,
        "doi_url": doi_url,
    }
    vprint(verbose, f"POST {identity_url}/credentials/dataset", payload)

    try:
        code, body = post(f"{identity_url}/credentials/dataset", payload)
    except ConnectionError as e:
        results.fail("/credentials/dataset reachable", str(e))
        return None

    if code != 201:
        results.fail("Issue credential HTTP 201", f"Got {code}: {body}")
        return None
    results.ok("Issue credential HTTP 201")

    if not isinstance(body, dict):
        results.fail("Response is JSON object", f"Got {type(body).__name__}")
        return None

    # ── Validate credential structure ─────────────────────────────────────────
    #
    # Expected shape:
    # {
    #   "@context": [...],
    #   "id": "did:web:...chess-node.../credentials/dataset/<uuid>",
    #   "type": ["VerifiableCredential", "DatasetPublicationCredential"],
    #   "issuer": "did:web:...",
    #   "issuanceDate": "2026-...",
    #   "credentialSubject": {
    #     "id": "/beamline=3a/btr=...",
    #     "type": ["schema:Dataset", "dcat:Dataset"],
    #     "fabric:graphIRI": "http://chess.cornell.edu/graph/3a/...",
    #     "schema:identifier": "10.5281/zenodo.123456",
    #     "schema:url": "https://doi.org/10.5281/zenodo.123456",
    #     "dcat:accessURL": "http://localhost:8082/beamlines/3a/datasets/.../sparql",
    #     "chess:beamline": "3a",
    #     "prov:wasAttributedTo": "did:web:...",
    #     "prov:generatedAtTime": "2026-..."
    #   },
    #   "proof": {
    #     "type": "DataIntegrityProof",
    #     "created": "...",
    #     "verificationMethod": "did:web:...#node-key-1",
    #     "proofPurpose": "assertionMethod",
    #     "proofValue": "<base64url>"
    #   }
    # }

    cred_types = body.get("type", [])
    if "DatasetPublicationCredential" in cred_types:
        results.ok("type includes DatasetPublicationCredential")
    else:
        results.fail("type includes DatasetPublicationCredential", f"Got: {cred_types}")

    if body.get("issuer", "").startswith("did:web:"):
        results.ok(f"issuer is did:web: {body['issuer']}")
    else:
        results.fail("issuer is did:web DID", f"Got: {body.get('issuer')!r}")

    subj = body.get("credentialSubject", {})
    checks = [
        ("credentialSubject.id == DID",          subj.get("id") == did_str),
        ("credentialSubject has fabric:graphIRI", bool(subj.get("fabric:graphIRI"))),
        ("credentialSubject has schema:identifier (DOI)", subj.get("schema:identifier") == doi),
        ("credentialSubject has schema:url (DOI URL)",    subj.get("schema:url") == doi_url),
        ("credentialSubject has dcat:accessURL",  bool(subj.get("dcat:accessURL"))),
        ("credentialSubject has chess:beamline",  bool(subj.get("chess:beamline"))),
        ("credentialSubject has prov:wasAttributedTo", bool(subj.get("prov:wasAttributedTo"))),
    ]
    for label, ok in checks:
        if ok: results.ok(label)
        else:  results.fail(label, f"credentialSubject: {json.dumps(subj, indent=2)}")

    graph_iri = subj.get("fabric:graphIRI", "")
    bl = beamline_from_did(did_str)
    expected_prefix = f"http://chess.cornell.edu/graph/{bl}/"
    if graph_iri.startswith(expected_prefix):
        results.ok(f"graphIRI has expected prefix: {graph_iri[:70]}")
    else:
        results.fail("graphIRI prefix", f"Expected prefix {expected_prefix!r}\nGot: {graph_iri!r}")

    proof = body.get("proof", {})
    proof_checks = [
        ("proof.type == DataIntegrityProof",        proof.get("type") == "DataIntegrityProof"),
        ("proof.proofPurpose == assertionMethod",    proof.get("proofPurpose") == "assertionMethod"),
        ("proof.verificationMethod contains #node-key", "#node-key" in proof.get("verificationMethod", "")),
        ("proof.proofValue present",                 bool(proof.get("proofValue"))),
    ]
    for label, ok in proof_checks:
        if ok: results.ok(label)
        else:  results.fail(label, f"proof: {proof}")

    vprint(verbose, "Issued credential:", body)
    return body

# ──────────────────────────────────────────────────────────────────────────────
# Step 4 — round-trip verify the credential
# ──────────────────────────────────────────────────────────────────────────────

def verify_credential(results, identity_url, cred, verbose):
    sep("Round-trip credential verification")

    vprint(verbose, f"POST {identity_url}/credentials/dataset/verify", cred)
    try:
        code, body = post(f"{identity_url}/credentials/dataset/verify", cred)
    except ConnectionError as e:
        results.fail("/credentials/dataset/verify reachable", str(e))
        return

    if code != 200:
        results.fail("Verify credential HTTP 200", f"Got {code}: {body}")
        return
    results.ok("Verify credential HTTP 200")

    # Expected shape:
    # { "verified": true, "issuer": "did:web:...", "did": "...", "doi": "...",
    #   "graphIRI": "...", "publishedAt": "..." }

    if body.get("verified") is True:
        results.ok("verified == true")
    else:
        results.fail("verified == true", f"Response: {body}")
        return

    for field in ["issuer", "did", "doi", "graphIRI", "publishedAt"]:
        if body.get(field):
            results.ok(f"Response has {field!r}: {str(body[field])[:60]}")
        else:
            results.fail(f"Response has {field!r}", f"Body: {body}")

    vprint(verbose, "Verify response:", body)

# ──────────────────────────────────────────────────────────────────────────────
# Step 5 — confirm SPARQL endpoint from credential is reachable
# ──────────────────────────────────────────────────────────────────────────────

def check_sparql_endpoint(results, cred, verbose):
    sep("SPARQL endpoint reachability")

    sparql_url = cred.get("credentialSubject", {}).get("dcat:accessURL", "")
    if not sparql_url:
        results.fail("credentialSubject.dcat:accessURL present", "Empty URL — skipping SPARQL check")
        return

    vprint(verbose, f"GET {sparql_url}")
    try:
        code, body = get(sparql_url)
        if code == 200 and isinstance(body, dict) and "results" in body:
            bindings = body["results"].get("bindings", [])
            results.ok(f"SPARQL endpoint responds HTTP 200, {len(bindings)} triple(s) in graph")
        elif code == 200:
            results.ok(f"SPARQL endpoint responds HTTP 200 (graph may be empty)")
        else:
            results.fail(f"SPARQL endpoint HTTP 200", f"Got {code}: {str(body)[:200]}")
    except ConnectionError as e:
        results.fail("SPARQL endpoint reachable", str(e))

# ──────────────────────────────────────────────────────────────────────────────
# Step 6 — tamper test: verify that a modified credential fails
# ──────────────────────────────────────────────────────────────────────────────

def tamper_test(results, identity_url, cred, verbose):
    sep("Tamper test (modified credential must fail verification)")

    import copy
    tampered = copy.deepcopy(cred)
    # Change the DOI — this invalidates the signature
    tampered["credentialSubject"]["schema:identifier"] = "10.0000/TAMPERED"

    vprint(verbose, f"POST {identity_url}/credentials/dataset/verify (tampered)", tampered)
    try:
        code, body = post(f"{identity_url}/credentials/dataset/verify", tampered)
        if code == 200 and body.get("verified") is False:
            results.ok("Tampered credential correctly rejected (verified=false)")
        else:
            results.fail("Tampered credential rejected", f"Got verified={body.get('verified')}, code={code}")
    except ConnectionError as e:
        results.fail("Tamper test reachable", str(e))

# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="FabricNode DOI publication credential integration probe",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--did",      required=True,  help="Dataset DID, e.g. /beamline=3a/btr=.../...")
    parser.add_argument("--doi",      required=True,  help="DOI string, e.g. 10.5281/zenodo.123456")
    parser.add_argument("--doi-url",  required=True,  help="Resolvable DOI URL")
    parser.add_argument("--identity", default="http://localhost:8083", help="identity-service URL")
    parser.add_argument("--data",     default="http://localhost:8082", help="data-service URL")
    parser.add_argument("--foxden",   default="http://localhost:8300", help="FOXDEN URL")
    parser.add_argument("--ingest",   action="store_true", help="Ingest dataset into data-service first")
    parser.add_argument("--verbose",  action="store_true", help="Print full JSON bodies")
    args = parser.parse_args()

    print()
    print(bold("CHESS FabricNode — DOI publication credential probe"))
    print(f"  identity: {args.identity}")
    print(f"  data:     {args.data}")
    print(f"  DID:      {args.did}")
    print(f"  DOI:      {args.doi}")
    print(f"  DOI URL:  {args.doi_url}")

    results = Results()

    check_health(results, args.identity, args.data)

    did_doc = resolve_did(results, args.identity, args.verbose)

    if args.ingest:
        ingest_dataset(results, args.data, args.foxden, args.did, args.verbose)

    cred = issue_credential(results, args.identity, args.did, args.doi, args.doi_url, args.verbose)

    if cred:
        verify_credential(results, args.identity, cred, args.verbose)
        check_sparql_endpoint(results, cred, args.verbose)
        tamper_test(results, args.identity, cred, args.verbose)

    results.summary()
    sys.exit(results.exit_code)

if __name__ == "__main__":
    main()
