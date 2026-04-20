#!/usr/bin/env python
"""
scripts/probe.py — FabricNode integration probe.

Supports two modes:

  Key/value search (original):
    ./probe.py --key btr --value tomo-sim-0330144634

  DID-file mode (new):
    ./probe.py --dids dids
    ./probe.py --dids dids --no-foxden   # skip FOXDEN lookup; ingest directly from DIDs

  In DID-file mode the script reads one DID per line, optionally looks each one
  up in FOXDEN to confirm it exists, then ingests and verifies every DID.
"""

import os
import argparse
import json
import sys
import urllib.parse
import urllib.request
import urllib.error
from typing import Any, Union, Tuple, List, Dict, Set, Optional

# ──────────────────────────────────────────────────────────────────────────────
# Colour helpers
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

def _request(method: str, url: str, body: Any = None, timeout: int = 10) -> Tuple[int, Union[Dict, List, str]]:
    data = json.dumps(body).encode() if body is not None else None
    token = os.environ.get("FOXDEN_TOKEN")
    if not token:
        raise ValueError("FOXDEN_TOKEN is not set or empty")
    auth = f"Bearer {token}"
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


def get(url: str, timeout: int = 10) -> Tuple[int, Any]:
    return _request("GET", url, timeout=timeout)


def post(url: str, body: Any, timeout: int = 10) -> Tuple[int, Any]:
    return _request("POST", url, body=body, timeout=timeout)


def encode_did(did: str) -> str:
    """Percent-encode a DID for use as a URL path segment (encodes / and =)."""
    return urllib.parse.quote(did, safe="")


def beamline_from_did(did: str) -> str:
    """Extract the lower-case beamline ID from a DID string."""
    for seg in did.lstrip("/").split("/"):
        if seg.startswith("beamline="):
            return seg[len("beamline="):].lower()
    return ""

# ──────────────────────────────────────────────────────────────────────────────
# DID file reader
# ──────────────────────────────────────────────────────────────────────────────

def read_dids_file(path: str) -> List[str]:
    """Read a file of DIDs (one per line), skipping blank lines and comments."""
    dids = []
    try:
        with open(path) as f:
            for lineno, line in enumerate(f, 1):
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if not line.startswith("/beamline="):
                    print(f"  {yellow('⚠')} Line {lineno}: skipping malformed DID: {line!r}")
                    continue
                dids.append(line)
    except OSError as e:
        print(f"{red('Error reading DIDs file:')} {e}")
        sys.exit(1)
    return dids

# ──────────────────────────────────────────────────────────────────────────────
# Result tracking
# ──────────────────────────────────────────────────────────────────────────────

class Results:
    def __init__(self):
        self.passed = 0
        self.failed = 0
        self.warned = 0
        self.steps: List[Tuple[bool, str]] = []

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

    def warn(self, label: str, detail: str = ""):
        """Non-fatal issue: worth flagging but not counted as a failure."""
        self.warned += 1
        self.steps.append((None, label))
        print(f"  {yellow('⚠')} {label}")
        if detail:
            for line in detail.strip().splitlines():
                print(f"      {yellow(line)}")

    def summary(self):
        total = self.passed + self.failed
        print()
        if self.failed == 0 and self.warned == 0:
            print(bold(green(f"All {total} checks passed.")))
        elif self.failed == 0:
            print(bold(yellow(f"All {total} checks passed ({self.warned} warning(s)).")))
        else:
            msg = f"{self.failed}/{total} checks failed."
            if self.warned:
                msg += f"  {self.warned} warning(s)."
            print(bold(red(msg)))

    @property
    def exit_code(self) -> int:
        return 0 if self.failed == 0 else 1

# ──────────────────────────────────────────────────────────────────────────────
# Utilities
# ──────────────────────────────────────────────────────────────────────────────

def sep(title: str):
    print(f"\n{cyan('══')} {bold(title)} {cyan('══')}")


def vprint(verbose: bool, label: str, url: Optional[str], body: Any = None):
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
# Step 0 — health checks
# ──────────────────────────────────────────────────────────────────────────────

def check_health(results: Results, data_url: str, catalog_url: str, foxden_url: str,
                 dry_run: bool, skip_foxden: bool):
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

    if skip_foxden:
        print(f"  {yellow('⚠')} FOXDEN health skipped (--no-foxden)")
        return

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
# Step 1a — FOXDEN search by key/value (original mode)
# ──────────────────────────────────────────────────────────────────────────────

def search_foxden_by_key(results: Results, foxden_url: str, key: str, value: str,
                         limit: int, verbose: bool) -> List[dict]:
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

    if code != 200:
        results.fail("FOXDEN search HTTP 200", f"Got HTTP {code}: {body}")
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
        results.fail(f"At least one record found for {key}={value!r}",
                     "nrecords=0 — check the key/value and FOXDEN data")
        return []

    results.ok(f"{nrecords} record(s) found")

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
# Step 1b — FOXDEN lookup by DID (dids-file mode)
# ──────────────────────────────────────────────────────────────────────────────

def lookup_foxden_by_did(results: Results, foxden_url: str, did: str, verbose: bool) -> Optional[dict]:
    """
    Look up a single DID in FOXDEN via GET /records?did=<did> or POST /search with did spec.
    Returns the matching record dict, or None if not found.
    """
    payload = {
        "client": "probe",
        "service_query": {
            "spec": {"did": did},
            "limit": 1,
        },
    }
    vprint(verbose, "POST (DID lookup)", f"{foxden_url}/search", payload)

    try:
        code, body = post(f"{foxden_url}/search", payload)
    except ConnectionError as e:
        results.fail(f"FOXDEN lookup reachable for {did[:50]}", str(e))
        return None

    if code != 200:
        results.fail(f"FOXDEN lookup HTTP 200 for {did[:50]}", f"Got HTTP {code}: {body}")
        return None

    # Parse response — same shape as key/value search
    if isinstance(body, list):
        records = body
    elif isinstance(body, dict):
        status = body.get("status")
        if status and status != "ok":
            results.fail(f"FOXDEN lookup status ok for {did[:50]}",
                         f"Got status={status!r}, error={body.get('error')!r}")
            return None
        results_obj = body.get("results", {})
        records = results_obj.get("records", [])
    else:
        results.fail(f"FOXDEN lookup response shape for {did[:50]}", f"Got: {type(body).__name__}")
        return None

    if not records:
        results.fail(f"FOXDEN has record for {did[:60]}", "nrecords=0 — DID not found in FOXDEN")
        return None

    rec = records[0]
    # Confirm the returned DID matches (FOXDEN may return partial matches)
    returned_did = rec.get("did", "")
    if returned_did == did:
        results.ok(f"FOXDEN confirmed DID: {did[:60]}")
    else:
        results.fail(f"FOXDEN DID exact match",
                     f"Expected {did!r}\n     Got {returned_did!r}")
        return None

    vprint(verbose, "FOXDEN record", None, rec)
    return rec


# ──────────────────────────────────────────────────────────────────────────────
# Step 1c — Build synthetic records from DID list (--no-foxden mode)
# ──────────────────────────────────────────────────────────────────────────────

def records_from_dids(dids: List[str]) -> List[dict]:
    """
    Build minimal record dicts from a list of DIDs without contacting FOXDEN.
    The beamline is extracted from the /beamline=<id> segment of each DID.
    """
    records = []
    for did in dids:
        bl = beamline_from_did(did)
        if bl:
            records.append({"did": did, "beamline": [bl]})
        else:
            print(f"  {yellow('⚠')} Cannot extract beamline from DID, skipping: {did!r}")
    return records


# ──────────────────────────────────────────────────────────────────────────────
# Step 2 — Ingest into data-service
# ──────────────────────────────────────────────────────────────────────────────

def ingest_records(results: Results, data_url: str, records: List[dict], verbose: bool) -> List[dict]:
    sep("Ingest FOXDEN → data-service")

    # Expected response from POST /beamlines/{bl}/datasets/{did}/foxden/ingest:
    # { "ingested": <int>, "did": "<string>", "graphIRI": "<string>" }

    ingested_ok = []
    for rec in records:
        did = rec.get("did", "")
        bl_raw = rec.get("beamline", [])
        bl = bl_raw[0].lower() if isinstance(bl_raw, list) and bl_raw else ""
        if not bl:
            bl = beamline_from_did(did)

        if not did or not bl:
            results.fail("Record has DID and beamline", f"did={did!r} bl={bl!r}")
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
        graph_iri      = body.get("graphIRI", "")
        returned_did   = body.get("did", "")

        if ingested_count > 0:
            results.ok(f"{label} → {ingested_count} triples, graph={graph_iri}")
        else:
            results.fail(label, f"ingested=0, body={body}")
            continue

        expected_fragment = f"/graph/{bl}/"
        if graph_iri and expected_fragment in graph_iri:
            results.ok(f"graphIRI contains expected fragment '{expected_fragment}'")
        else:
            results.fail(f"graphIRI contains '{expected_fragment}'", f"Got: {graph_iri!r}")

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

def verify_sparql(results: Results, data_url: str, ingested: List[dict], verbose: bool):
    sep("SPARQL verification")

    for item in ingested:
        did       = item["did"]
        bl        = item["beamline"]
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

        head_vars = body.get("head", {}).get("vars", [])
        expected_vars = {"s", "p", "o", "g"}
        if expected_vars.issubset(set(head_vars)):
            results.ok(f"head.vars contains {sorted(expected_vars)}")
        else:
            results.fail(f"head.vars contains {sorted(expected_vars)}", f"Got: {head_vars}")

        bindings = body.get("results", {}).get("bindings", [])
        if bindings:
            results.ok(f"{len(bindings)} triple(s) in named graph")
        else:
            results.fail("Named graph contains triples", "bindings=[]")
            continue

        b = bindings[0]
        shape_ok = all(
            isinstance(b.get(v), dict) and "type" in b[v] and "value" in b[v]
            for v in ["s", "p", "o"]
        )
        if shape_ok:
            results.ok("Binding shape: each var has type and value")
        else:
            results.fail("Binding shape", f"First binding: {b}")

        wrong_graph = [
            b["g"]["value"] for b in bindings
            if b.get("g", {}).get("value") != graph_iri
        ]
        if not wrong_graph:
            results.ok("All triples in correct named graph")
        else:
            results.fail("All triples in correct named graph",
                         f"{len(wrong_graph)} triple(s) have wrong graph IRI: {wrong_graph[:2]}")

        subjects = {b["s"]["value"] for b in bindings}
        dataset_subject = next(
            (s for s in subjects if f"dataset{did}" in s or s.endswith(did)),
            None
        )
        if dataset_subject:
            results.ok(f"Dataset IRI appears as subject: {dataset_subject[:60]}")
        else:
            results.fail("Dataset IRI appears as subject",
                         f"No subject contains DID path\nFound subjects: {list(subjects)[:3]}")

        vprint(verbose, "SPARQL bindings (first 3)", None, bindings[:3])

        bl_url = f"{data_url}/beamlines/{bl}/sparql"
        try:
            bl_code, bl_body = get(bl_url)
            if bl_code == 200:
                bl_count = len(bl_body.get("results", {}).get("bindings", []))
                if bl_count > 0:
                    results.ok(f"Beamline SPARQL /beamlines/{bl}/sparql → {bl_count} triple(s)")
                else:
                    results.fail("Beamline SPARQL returns triples", "bindings=[]")
            else:
                results.fail(f"Beamline SPARQL HTTP 200", f"Got HTTP {bl_code}")
        except ConnectionError as e:
            results.fail("Beamline SPARQL reachable", str(e))


# ──────────────────────────────────────────────────────────────────────────────
# Step 4 — catalog-service dataset listing
# ──────────────────────────────────────────────────────────────────────────────

def verify_catalog(results: Results, catalog_url: str, ingested: List[dict], verbose: bool):
    sep("Catalog-service verification")

    # Build a set of DIDs we successfully ingested, keyed by beamline.
    # Used to cross-check: if SPARQL has data but catalog is empty, that is a
    # catalog/FOXDEN sync issue (not a probe failure).
    ingested_by_bl: Dict[str, List[str]] = {}
    for item in ingested:
        ingested_by_bl.setdefault(item["beamline"], []).append(item["did"])

    seen_beamlines: Set[str] = set()
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

        dtype = body.get("@type", "")
        if dtype == "dcat:Catalog":
            results.ok("@type is dcat:Catalog")
        else:
            results.fail("@type is dcat:Catalog", f"Got {dtype!r}")

        datasets = body.get("dcat:dataset", [])
        if not isinstance(datasets, list) or len(datasets) == 0:
            # Cross-check: we know we just ingested data for this beamline.
            # An empty catalog means getDids() in the catalog-service found
            # nothing in FOXDEN for this beamline — typically a case-sensitivity
            # or field-type mismatch (FOXDEN stores beamline as an array like
            # ["7A"] but the catalog queries with the scalar lower-case string).
            ingested_dids = ingested_by_bl.get(bl, [])
            results.warn(
                f"Catalog lists 0 dataset(s) for beamline {bl} "
                f"(store has {len(ingested_dids)} ingested DID(s))",
                f"The data-service store contains triples for this beamline but the\n"
                f"catalog-service found no matching DIDs in FOXDEN.\n"
                f"Likely cause: FOXDEN stores 'beamline' as an array (e.g. [\"{bl.upper()}\"]) but\n"
                f"the catalog queries FOXDEN with a scalar lower-case string (\"{bl}\").\n"
                f"Fix: check getDids() in catalog-service/internal/handlers/datasets.go —\n"
                f"the FOXDEN search spec may need  {{\"beamline\": [\"{bl}\"]}}  or a case-\n"
                f"insensitive match instead of  {{\"beamline\": \"{bl}\"}}.",
            )
            continue
        results.ok(f"{len(datasets)} dataset(s) listed in catalog")

        first = datasets[0]
        for field in ["@id", "@type", "dct:title"]:
            if field in first:
                results.ok(f"Dataset entry has {field!r}")
            else:
                results.fail(f"Dataset entry has {field!r}", f"Keys present: {list(first.keys())}")

        dist       = first.get("dcat:distribution", {})
        access_url = dist.get("dcat:accessURL", "") if isinstance(dist, dict) else ""
        if access_url and "sparql" in access_url:
            results.ok(f"dcat:accessURL points to SPARQL: {access_url[:60]}")
        else:
            results.fail("dcat:distribution.dcat:accessURL points to SPARQL",
                         f"Got: {access_url!r}")

        vprint(verbose, "Catalog response", None, body)


# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="FabricNode integration probe — traces FOXDEN → data-service → SPARQL data flow",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Search by field, ingest, and verify:
  ./probe.py --key btr --value tomo-sim-0330144634

  # Use a file of DIDs; confirm each in FOXDEN before ingesting:
  ./probe.py --dids dids

  # Use a file of DIDs; skip FOXDEN lookup and ingest directly:
  ./probe.py --dids dids --no-foxden
""",
    )

    # Mode selection — mutually exclusive: key/value vs dids file
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--dids",  metavar="FILE",
                      help="File containing one DID per line to ingest and verify")
    mode.add_argument("--key",   metavar="KEY",
                      help="FOXDEN field to search on (e.g. btr, cycle, beamline)")

    parser.add_argument("--value",     metavar="VALUE",
                        help="Value to match (required with --key)")
    parser.add_argument("--no-foxden", action="store_true",
                        help="With --dids: skip FOXDEN lookup; ingest DIDs directly into data-service")
    parser.add_argument("--foxden",    default="http://localhost:8300",
                        help="FOXDEN base URL  (default: http://localhost:8300)")
    parser.add_argument("--data",      default="http://localhost:8782",
                        help="data-service base URL  (default: http://localhost:8782)")
    parser.add_argument("--catalog",   default="http://localhost:8781",
                        help="catalog-service base URL  (default: http://localhost:8781)")
    parser.add_argument("--limit",     type=int, default=5,
                        help="Max FOXDEN records to process in key/value mode  (default: 5)")
    parser.add_argument("--dry-run",   action="store_true",
                        help="Search/lookup only; skip ingest and SPARQL verification")
    parser.add_argument("--verbose",   action="store_true",
                        help="Print full JSON request/response bodies")
    args = parser.parse_args()

    # Validate argument combinations
    if args.key and not args.value:
        parser.error("--key requires --value")
    if args.no_foxden and not args.dids:
        parser.error("--no-foxden is only valid with --dids")

    skip_foxden = bool(args.no_foxden)

    print()
    print(bold("CHESS FabricNode — integration probe"))
    print(f"  FOXDEN:   {args.foxden}")
    print(f"  data:     {args.data}")
    print(f"  catalog:  {args.catalog}")
    if args.dids:
        print(f"  mode:     DID file ({args.dids})")
        if skip_foxden:
            print(f"  {yellow('--no-foxden: FOXDEN lookup will be skipped')}")
    else:
        print(f"  search:   {args.key}={args.value!r}  limit={args.limit}")
    if args.dry_run:
        print(f"  {yellow('--dry-run: ingest and SPARQL steps will be skipped')}")

    results = Results()

    # ── Step 0 — health ───────────────────────────────────────────────────────
    check_health(results, args.data, args.catalog, args.foxden,
                 args.dry_run, skip_foxden)

    # ── Step 1 — obtain records ───────────────────────────────────────────────
    records: List[dict] = []

    if args.dids:
        # DID-file mode
        dids = read_dids_file(args.dids)
        if not dids:
            print(f"\n{red('No valid DIDs found in file — stopping.')}")
            results.summary()
            sys.exit(results.exit_code)

        sep(f"DID file: {args.dids}  ({len(dids)} DID(s))")
        for did in dids:
            print(f"  {cyan('·')} {did}")

        if skip_foxden:
            # Build minimal records directly from the DID list
            records = records_from_dids(dids)
            results.ok(f"{len(records)} record(s) prepared from DID file (FOXDEN skipped)")
        else:
            # Look up each DID in FOXDEN individually
            sep("FOXDEN DID lookup")
            for did in dids:
                rec = lookup_foxden_by_did(results, args.foxden, did, args.verbose)
                if rec:
                    records.append(rec)
            if records:
                results.ok(f"{len(records)}/{len(dids)} DID(s) confirmed in FOXDEN")
            else:
                print(f"\n{red('No DIDs confirmed in FOXDEN — stopping.')}")
                results.summary()
                sys.exit(results.exit_code)

    else:
        # Key/value mode (original behaviour)
        records = search_foxden_by_key(
            results, args.foxden, args.key, args.value, args.limit, args.verbose
        )
        if not records:
            print(f"\n{red('No records returned from FOXDEN — stopping.')}")
            results.summary()
            sys.exit(results.exit_code)

    if args.dry_run:
        results.summary()
        sys.exit(results.exit_code)

    # ── Step 2 — ingest ───────────────────────────────────────────────────────
    ingested = ingest_records(results, args.data, records, args.verbose)
    if not ingested:
        print(f"\n{red('No records ingested successfully — skipping SPARQL and catalog steps.')}")
        results.summary()
        sys.exit(results.exit_code)

    # ── Step 3 — SPARQL verification ──────────────────────────────────────────
    verify_sparql(results, args.data, ingested, args.verbose)

    # ── Step 4 — catalog verification ────────────────────────────────────────
    verify_catalog(results, args.catalog, ingested, args.verbose)

    results.summary()
    sys.exit(results.exit_code)


if __name__ == "__main__":
    main()
