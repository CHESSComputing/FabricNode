#!/usr/bin/env python
"""
scripts/probe.py — CHESS FabricNode integration probe.

Traces the full data flow for a key=value attribute known in FOXDEN:
  1. Search FOXDEN for matching metadata records
  2. Ingest each matching record into data-service (POST /foxden/ingest)
  3. SPARQL-query the resulting named graph to verify triples arrived
  4. Validate the response shapes and report pass/fail for each step

Usage:
  python3 scripts/probe.py --key btr          --value test-123-a
  python3 scripts/probe.py --key cycle        --value 2026-1
  python3 scripts/probe.py --key beamline     --value 3a
  python3 scripts/probe.py --key sample_name  --value silicon-std

Options:
  --key        FOXDEN field name to search on
  --value      Value to match
  --foxden     FOXDEN base URL          (default: http://localhost:8300)
  --data       data-service base URL    (default: http://localhost:8082)
  --catalog    catalog-service base URL (default: http://localhost:8081)
  --limit      Max FOXDEN records to process (default: 5)
  --dry-run    Search and validate FOXDEN only; skip ingest and SPARQL steps
  --verbose    Print full JSON bodies at each step
"""

import os
import argparse
import json
import sys
import urllib.parse
import urllib.request
import urllib.error
from typing import Any

# ──────────────────────────────────────────────────────────────────────────────
# Colour helpers (auto-disabled when not a tty)
# ──────────────────────────────────────────────────────────────────────────────

USE_COLOR = sys.stdout.isatty()

def _c(code: str, text: str) -> str:
    return f"\033[{code}m{text}\033[0m" if USE_COLOR else text

def green(t):  return _c("32", t)
def red(t):    return _c("31", t)
def cyan(t):   return _c("36", t)
def yellow(t): return _c("33", t)
def bold(t):   return _c("1",  t)

# ──────────────────────────────────────────────────────────────────────────────
# HTTP helpers
# ──────────────────────────────────────────────────────────────────────────────

def _request(method: str, url: str, body: Any = None, timeout: int = 10) -> tuple[int, dict | list | str]:
    data = json.dumps(body).encode() if body is not None else None
    auth = "Bearer: {}".format(os.environ.get("FOXDEN_TOKEN"))
    headers = {"Content-Type": "application/json", "Accept": "application/json", "Authorization": auth}
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode()
            try:
                return resp.status, json.loads(raw)
            except json.JSONDecodeError:
                return resp.status, raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except json.JSONDecodeError:
            return e.code, raw
    except urllib.error.URLError as e:
        raise ConnectionError(f"Cannot reach {url}: {e.reason}") from e


def get(url: str, timeout: int = 10) -> tuple[int, Any]:
    return _request("GET", url, timeout=timeout)


def post(url: str, body: Any, timeout: int = 10) -> tuple[int, Any]:
    return _request("POST", url, body=body, timeout=timeout)


def encode_did(did: str) -> str:
    """URL-encode a DID so slashes and equals signs are safe in path segments."""
    return urllib.parse.quote(did, safe="")


# ──────────────────────────────────────────────────────────────────────────────
# Result tracking
# ──────────────────────────────────────────────────────────────────────────────

class Results:
    def __init__(self):
        self.passed = 0
        self.failed = 0
        self.steps: list[tuple[bool, str]] = []

    def ok(self, label: str):
        self.passed += 1
        self.steps.append((True, label))
        print(f"  {green('✓')} {label}")

    def fail(self, label: str, detail: str = ""):
        self.failed += 1
        self.steps.append((False, label))
        print(f"  {red('✗')} {label}")
        if detail:
            for line in detail.strip().splitlines():
                print(f"      {red(line)}")

    def summary(self):
        total = self.passed + self.failed
        print()
        if self.failed == 0:
            print(bold(green(f"All {total} checks passed.")))
        else:
            print(bold(red(f"{self.failed}/{total} checks failed.")))

    @property
    def exit_code(self) -> int:
        return 0 if self.failed == 0 else 1


# ──────────────────────────────────────────────────────────────────────────────
# Step 0 — health checks
# ──────────────────────────────────────────────────────────────────────────────

def check_health(results: Results, data_url: str, catalog_url: str, foxden_url: str, dry_run: bool):
    sep("Health checks")

    for name, url in [
        ("catalog-service", f"{catalog_url}/health"),
        ("data-service",    f"{data_url}/health"),
    ]:
        try:
            code, body = get(url)
            if code == 200 and isinstance(body, dict) and body.get("status") == "ok":
                results.ok(f"{name} healthy")
            else:
                results.fail(f"{name} health check", f"HTTP {code}: {body}")
        except ConnectionError as e:
            results.fail(f"{name} reachable", str(e))

    # FOXDEN health is best-effort — it may not have a /health endpoint
    try:
        code, _ = get(f"{foxden_url}/health", timeout=3)
        if code == 200:
            results.ok("FOXDEN reachable")
        else:
            print(f"  {yellow('⚠')} FOXDEN returned HTTP {code} on /health (continuing)")
    except ConnectionError:
        if dry_run:
            results.fail("FOXDEN reachable", "Cannot connect — required for --dry-run")
        else:
            print(f"  {yellow('⚠')} FOXDEN unreachable — ingest steps will be skipped gracefully")


# ──────────────────────────────────────────────────────────────────────────────
# Step 1 — FOXDEN search
# ──────────────────────────────────────────────────────────────────────────────

def search_foxden(results: Results, foxden_url: str, key: str, value: str, limit: int, verbose: bool) -> list[dict]:
    sep(f"FOXDEN search  ({key}={value!r})")

    payload = {
        "client": "probe",
        "service_query": {
            "spec": {key: value},
            "limit": limit,
            "sort_keys": ["cycle"],
            "sort_order": -1,
        },
    }
    vprint(verbose, "POST", f"{foxden_url}/search", payload)

    try:
        code, body = post(f"{foxden_url}/search", payload)
    except ConnectionError as e:
        results.fail("FOXDEN /search reachable", str(e))
        return []

    # ── Shape validation ──────────────────────────────────────────────────────
    # Expected shape:
    # {
    #   "http_code": 200,
    #   "status": "ok",
    #   "error": "",
    #   "results": {
    #     "nrecords": <int>,
    #     "records": [ { "did": "...", "beamline": [...], "btr": "...", ... } ]
    #   }
    # }

    if code != 200:
        results.fail(f"FOXDEN search HTTP 200", f"Got HTTP {code}: {body}")
        return []
    results.ok("FOXDEN search HTTP 200")

    if isinstance(body, list):
        records = body
        nrecords = len(records)
    else:
        if not isinstance(body, dict):
            results.fail("FOXDEN response is JSON object", f"Got: {type(body).__name__}")
            return []

        status = body.get("status")
        if status != "ok":
            results.fail("FOXDEN status == ok", f"Got status={status!r}, error={body.get('error')!r}")
            return []
        results.ok("FOXDEN status == ok")

        results_obj = body.get("results", {})
        nrecords = results_obj.get("nrecords", 0)
        records = results_obj.get("records", [])

    if not isinstance(records, list):
        results.fail("results.records is array", f"Got: {type(records).__name__}")
        return []

    if nrecords == 0 or len(records) == 0:
        results.fail(f"At least one record found for {key}={value!r}", "nrecords=0 — check the key/value and FOXDEN data")
        return []

    results.ok(f"{nrecords} record(s) found")

    # Validate first record has required fields
    rec = records[0]
    required_fields = ["did", "beamline", "btr", "cycle"]
    missing = [f for f in required_fields if f not in rec]
    if missing:
        results.fail(f"Record has required fields {required_fields}", f"Missing: {missing}")
    else:
        results.ok(f"Record has required fields {required_fields}")

    did = rec.get("did", "")
    if did.startswith("/beamline="):
        results.ok(f"DID well-formed: {did[:60]}")
    else:
        results.fail("DID starts with /beamline=", f"Got: {did!r}")

    vprint(verbose, "FOXDEN records", None, records[:2])
    return records


# ──────────────────────────────────────────────────────────────────────────────
# Step 2 — Ingest into data-service
# ──────────────────────────────────────────────────────────────────────────────

def ingest_records(results: Results, data_url: str, records: list[dict], verbose: bool) -> list[dict]:
    sep("Ingest FOXDEN → data-service")

    # Expected response shape from POST /beamlines/{bl}/datasets/{did}/foxden/ingest:
    # {
    #   "ingested": <int>,       # number of RDF triples written
    #   "did":      "<string>",  # the dataset DID
    #   "graphIRI": "<string>"   # http://chess.cornell.edu/graph/<bl>/...
    # }

    ingested_ok = []
    for rec in records:
        did = rec.get("did", "")
        bl_raw = rec.get("beamline", [])
        # beamline field is an array like ["3A"]; normalise to lower-case
        bl = bl_raw[0].lower() if isinstance(bl_raw, list) and bl_raw else ""
        if not bl:
            # fall back to extracting from DID
            parts = did.lstrip("/").split("/")
            for part in parts:
                if part.startswith("beamline="):
                    bl = part[len("beamline="):].lower()
                    break

        if not did or not bl:
            results.fail(f"Record has DID and beamline", f"did={did!r} bl={bl!r}")
            continue

        enc_did = encode_did(did)
        url = f"{data_url}/beamlines/{bl}/datasets/{enc_did}/foxden/ingest"
        vprint(verbose, "POST", url)

        try:
            code, body = post(url, body=None)
        except ConnectionError as e:
            results.fail(f"Ingest reachable for {did[:40]}", str(e))
            continue

        label = f"Ingest {did[:50]}"
        if code not in (200, 201):
            results.fail(label, f"HTTP {code}: {body}")
            continue

        if not isinstance(body, dict):
            results.fail(f"{label} returns JSON object", f"Got {type(body).__name__}")
            continue

        ingested_count = body.get("ingested", 0)
        graph_iri = body.get("graphIRI", "")
        returned_did = body.get("did", "")

        if ingested_count > 0:
            results.ok(f"{label} → {ingested_count} triples, graph={graph_iri}")
        else:
            results.fail(label, f"ingested=0, body={body}")
            continue

        # graphIRI must follow the scheme:
        #   http://chess.cornell.edu/graph/<beamline>/<did-segments>
        if graph_iri.startswith(f"http://chess.cornell.edu/graph/{bl}/"):
            results.ok(f"graphIRI scheme correct")
        else:
            results.fail(f"graphIRI has expected prefix", f"Got: {graph_iri!r}")

        if returned_did == did:
            results.ok("Response DID matches input DID")
        else:
            results.fail("Response DID matches input DID", f"Got {returned_did!r}")

        vprint(verbose, "Ingest response", None, body)
        ingested_ok.append({"did": did, "beamline": bl, "graphIRI": graph_iri})

    return ingested_ok


# ──────────────────────────────────────────────────────────────────────────────
# Step 3 — SPARQL verification
# ──────────────────────────────────────────────────────────────────────────────

def verify_sparql(results: Results, data_url: str, ingested: list[dict], verbose: bool):
    sep("SPARQL verification")

    # Expected SPARQL response shape:
    # {
    #   "head": { "vars": ["s", "p", "o", "g"] },
    #   "results": {
    #     "bindings": [
    #       {
    #         "s": { "type": "uri",     "value": "http://..." },
    #         "p": { "type": "uri",     "value": "http://..." },
    #         "o": { "type": "literal", "value": "..." },
    #         "g": { "type": "uri",     "value": "http://chess.cornell.edu/graph/..." }
    #       }
    #     ]
    #   }
    # }

    for item in ingested:
        did = item["did"]
        bl  = item["beamline"]
        graph_iri = item["graphIRI"]

        enc_did = encode_did(did)
        url = f"{data_url}/beamlines/{bl}/datasets/{enc_did}/sparql"
        vprint(verbose, "GET", url)

        try:
            code, body = get(url)
        except ConnectionError as e:
            results.fail(f"SPARQL endpoint reachable for {did[:40]}", str(e))
            continue

        label = f"SPARQL for {did[:50]}"

        if code != 200:
            results.fail(f"{label} HTTP 200", f"Got HTTP {code}")
            continue
        results.ok(f"{label} HTTP 200")

        if not isinstance(body, dict):
            results.fail(f"{label} returns JSON object", f"Got {type(body).__name__}")
            continue

        # head.vars
        head_vars = body.get("head", {}).get("vars", [])
        expected_vars = {"s", "p", "o", "g"}
        if expected_vars.issubset(set(head_vars)):
            results.ok(f"head.vars contains {sorted(expected_vars)}")
        else:
            results.fail(f"head.vars contains {sorted(expected_vars)}", f"Got: {head_vars}")

        bindings = body.get("results", {}).get("bindings", [])
        if len(bindings) > 0:
            results.ok(f"{len(bindings)} triple(s) in named graph")
        else:
            results.fail(f"Named graph contains triples", "bindings=[]")
            continue

        # Spot-check first binding structure
        b = bindings[0]
        shape_ok = all(
            isinstance(b.get(v), dict) and "type" in b[v] and "value" in b[v]
            for v in ["s", "p", "o"]
        )
        if shape_ok:
            results.ok("Binding shape: each var has type and value")
        else:
            results.fail("Binding shape", f"First binding: {b}")

        # Every triple must belong to the expected graph
        wrong_graph = [
            b["g"]["value"] for b in bindings
            if b.get("g", {}).get("value") != graph_iri
        ]
        if not wrong_graph:
            results.ok(f"All triples in correct named graph")
        else:
            results.fail("All triples in correct named graph",
                         f"{len(wrong_graph)} triple(s) have wrong graph IRI: {wrong_graph[:2]}")

        # At least one triple should have the dataset IRI as subject
        dataset_iri = f"http://chess.cornell.edu/dataset{did}"
        subjects = {b["s"]["value"] for b in bindings}
        if dataset_iri in subjects:
            results.ok(f"Dataset IRI appears as subject: {dataset_iri[:60]}")
        else:
            results.fail("Dataset IRI appears as subject",
                         f"Expected {dataset_iri!r}\nFound subjects: {list(subjects)[:3]}")

        vprint(verbose, "SPARQL bindings (first 3)", None, bindings[:3])

        # Also verify beamline-scoped SPARQL returns these triples
        bl_url = f"{data_url}/beamlines/{bl}/sparql"
        try:
            bl_code, bl_body = get(bl_url)
            if bl_code == 200:
                bl_count = len(bl_body.get("results", {}).get("bindings", []))
                if bl_count > 0:
                    results.ok(f"Beamline SPARQL /beamlines/{bl}/sparql → {bl_count} triple(s)")
                else:
                    results.fail(f"Beamline SPARQL returns triples", "bindings=[]")
            else:
                results.fail(f"Beamline SPARQL HTTP 200", f"Got HTTP {bl_code}")
        except ConnectionError as e:
            results.fail("Beamline SPARQL reachable", str(e))


# ──────────────────────────────────────────────────────────────────────────────
# Step 4 — catalog-service dataset listing
# ──────────────────────────────────────────────────────────────────────────────

def verify_catalog(results: Results, catalog_url: str, ingested: list[dict], verbose: bool):
    sep("Catalog-service verification")

    # Expected shape from GET /catalog/beamlines/{beamline}/datasets:
    # {
    #   "@context": { "dcat": "...", "dct": "..." },
    #   "@id":      "http://localhost:8081/catalog/beamlines/3a",
    #   "@type":    "dcat:Catalog",
    #   "dcat:dataset": [
    #     {
    #       "@id":   "http://...",
    #       "@type": "dcat:Dataset",
    #       "dct:title": "...",
    #       "dcat:distribution": { "dcat:accessURL": "http://..." }
    #     }
    #   ]
    # }

    seen_beamlines: set[str] = set()
    for item in ingested:
        bl = item["beamline"]
        if bl in seen_beamlines:
            continue
        seen_beamlines.add(bl)

        url = f"{catalog_url}/catalog/beamlines/{bl}/datasets"
        vprint(verbose, "GET", url)

        try:
            code, body = get(url)
        except ConnectionError as e:
            results.fail(f"Catalog reachable for beamline {bl}", str(e))
            continue

        label = f"Catalog /catalog/beamlines/{bl}/datasets"

        if code != 200:
            results.fail(f"{label} HTTP 200", f"Got HTTP {code}")
            continue
        results.ok(f"{label} HTTP 200")

        if not isinstance(body, dict):
            results.fail(f"{label} returns JSON-LD object", f"Got {type(body).__name__}")
            continue

        # Must be a dcat:Catalog
        dtype = body.get("@type", "")
        if dtype == "dcat:Catalog":
            results.ok("@type is dcat:Catalog")
        else:
            results.fail("@type is dcat:Catalog", f"Got {dtype!r}")

        datasets = body.get("dcat:dataset", [])
        if isinstance(datasets, list) and len(datasets) > 0:
            results.ok(f"{len(datasets)} dataset(s) listed in catalog")
        else:
            results.fail("dcat:dataset is non-empty array", f"Got: {datasets!r}")
            continue

        # Each dataset entry must have @id, @type, dcat:distribution
        first = datasets[0]
        for field in ["@id", "@type", "dct:title"]:
            if field in first:
                results.ok(f"Dataset entry has {field!r}")
            else:
                results.fail(f"Dataset entry has {field!r}", f"Keys present: {list(first.keys())}")

        dist = first.get("dcat:distribution", {})
        access_url = dist.get("dcat:accessURL", "") if isinstance(dist, dict) else ""
        if access_url and "sparql" in access_url:
            results.ok(f"dcat:accessURL points to SPARQL: {access_url[:60]}")
        else:
            results.fail("dcat:distribution.dcat:accessURL points to SPARQL", f"Got: {access_url!r}")

        vprint(verbose, "Catalog response", None, body)


# ──────────────────────────────────────────────────────────────────────────────
# Utilities
# ──────────────────────────────────────────────────────────────────────────────

def sep(title: str):
    print(f"\n{cyan('══')} {bold(title)} {cyan('══')}")


def vprint(verbose: bool, label: str, url: str | None, body: Any = None):
    if not verbose:
        return
    if url:
        print(f"    {yellow(label + ':')} {url}")
    if body is not None:
        text = json.dumps(body, indent=2)
        for line in text.splitlines()[:30]:
            print(f"    {line}")
        lines = text.count("\n")
        if lines > 30:
            print(f"    … ({lines - 30} more lines)")


# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="FabricNode integration probe — traces FOXDEN → data-service → SPARQL data flow",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("Usage:")[1] if "Usage:" in __doc__ else "",
    )
    parser.add_argument("--key",     required=True,  help="FOXDEN field to search on (e.g. btr, cycle, beamline)")
    parser.add_argument("--value",   required=True,  help="Value to match")
    parser.add_argument("--foxden",  default="http://localhost:8300", help="FOXDEN base URL")
    parser.add_argument("--data",    default="http://localhost:8082", help="data-service base URL")
    parser.add_argument("--catalog", default="http://localhost:8081", help="catalog-service base URL")
    parser.add_argument("--limit",   type=int, default=5, help="Max FOXDEN records to process")
    parser.add_argument("--dry-run", action="store_true", help="Search FOXDEN only; skip ingest and SPARQL")
    parser.add_argument("--verbose", action="store_true", help="Print full JSON request/response bodies")
    args = parser.parse_args()

    print()
    print(bold("CHESS FabricNode — integration probe"))
    print(f"  FOXDEN:   {args.foxden}")
    print(f"  data:     {args.data}")
    print(f"  catalog:  {args.catalog}")
    print(f"  search:   {args.key}={args.value!r}  limit={args.limit}")
    if args.dry_run:
        print(f"  {yellow('dry-run: ingest and SPARQL steps will be skipped')}")

    results = Results()

    # Step 0 — health
    check_health(results, args.data, args.catalog, args.foxden, args.dry_run)

    # Step 1 — FOXDEN search
    records = search_foxden(results, args.foxden, args.key, args.value, args.limit, args.verbose)
    if not records:
        print(f"\n{red('No records returned from FOXDEN — stopping.')}")
        results.summary()
        sys.exit(results.exit_code)

    if args.dry_run:
        results.summary()
        sys.exit(results.exit_code)

    # Step 2 — ingest
    ingested = ingest_records(results, args.data, records, args.verbose)
    if not ingested:
        print(f"\n{red('No records ingested successfully — skipping SPARQL and catalog steps.')}")
        results.summary()
        sys.exit(results.exit_code)

    # Step 3 — SPARQL verification
    verify_sparql(results, args.data, ingested, args.verbose)

    # Step 4 — catalog verification
    verify_catalog(results, args.catalog, ingested, args.verbose)

    results.summary()
    sys.exit(results.exit_code)


if __name__ == "__main__":
    main()
