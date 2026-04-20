#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# discovery.sh — FabricNode dataset discovery and SPARQL verification
#
# Usage:
#   ./discovery.sh                          # discover all beamlines from VoID
#   ./discovery.sh --dids dids              # verify specific DIDs from a file
#   ./discovery.sh --dids dids --ingest     # ingest DIDs first, then verify
#
# Environment:
#   CATALOG_URL   catalog-service base URL  (default: http://localhost:8781)
#   DATA_URL      data-service base URL     (default: http://localhost:8782)
#   LIMIT         max datasets per beamline in discovery mode  (default: 3)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ─── CONFIG ──────────────────────────────────────────────────────────────────
CATALOG_URL="${CATALOG_URL:-http://localhost:8781}"
DATA_URL="${DATA_URL:-http://localhost:8782}"
LIMIT="${LIMIT:-3}"

# ─── ARGUMENT PARSING ────────────────────────────────────────────────────────
DIDS_FILE=""
DO_INGEST=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dids)
            DIDS_FILE="$2"
            shift 2
            ;;
        --ingest)
            DO_INGEST=true
            shift
            ;;
        --catalog)
            CATALOG_URL="$2"
            shift 2
            ;;
        --data)
            DATA_URL="$2"
            shift 2
            ;;
        --limit)
            LIMIT="$2"
            shift 2
            ;;
        -h|--help)
            sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            exit 1
            ;;
    esac
done

# ─── COLOUR HELPERS ──────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
    GREEN='\033[32m'; RED='\033[31m'; CYAN='\033[36m'
    YELLOW='\033[33m'; BOLD='\033[1m'; RESET='\033[0m'
else
    GREEN=''; RED=''; CYAN=''; YELLOW=''; BOLD=''; RESET=''
fi

ok()   { echo -e "  ${GREEN}✓${RESET} $*"; }
fail() { echo -e "  ${RED}✗${RESET} $*"; }
sep()  { echo -e "\n${CYAN}══${RESET} ${BOLD}$*${RESET} ${CYAN}══${RESET}"; }
warn() { echo -e "  ${YELLOW}⚠${RESET} $*"; }

PASS=0; FAIL=0
check_ok()   { PASS=$((PASS+1)); ok "$*"; }
check_fail() { FAIL=$((FAIL+1)); fail "$*"; }

# ─── HELPERS ─────────────────────────────────────────────────────────────────

# Extract lower-case beamline ID from a DID string
# /beamline=3a/btr=... → 3a
beamline_from_did() {
    echo "$1" | grep -oP '(?<=/beamline=)[^/]+' | tr '[:upper:]' '[:lower:]'
}

# Percent-encode a DID for use as a URL path segment (encodes / and =)
encode_did() {
    python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "$1"
}

# Run a SPARQL query for one dataset DID and report results
query_sparql() {
    local bl="$1" did="$2" graph_iri="$3"
    local enc_did
    enc_did=$(encode_did "$did")
    local url="${DATA_URL}/beamlines/${bl}/datasets/${enc_did}/sparql"

    echo "  SPARQL endpoint: $url"

    local resp
    resp=$(curl -ks "$url")

    local bindings
    bindings=$(echo "$resp" | python3 -c "
import json,sys
try:
    b = json.load(sys.stdin).get('results',{}).get('bindings') or []
    print(len(b))
except Exception as e:
    print(0)
" 2>/dev/null || echo "0")

    if [[ "$bindings" == "0" || -z "$bindings" ]]; then
        check_fail "SPARQL returned no triples for $did"
        echo "  Raw response: $(echo "$resp" | head -c 200)"
        return
    fi
    check_ok "SPARQL returned $bindings triple(s)"

    # Show first 3 bindings
    echo "$resp" | python3 -c "
import json,sys
try:
    b = json.load(sys.stdin).get('results',{}).get('bindings',[])
    print(json.dumps(b[:3], indent=2))
except:
    pass
" 2>/dev/null || true

    # Verify all triples are in the expected named graph (if we know it)
    if [[ -n "$graph_iri" ]]; then
        local wrong
        wrong=$(echo "$resp" | python3 -c "
import json,sys
expected='$graph_iri'
try:
    bindings=json.load(sys.stdin).get('results',{}).get('bindings',[])
    bad=[b.get('g',{}).get('value') for b in bindings if b.get('g',{}).get('value') != expected]
    print(len(bad))
except:
    print(0)
" 2>/dev/null || echo "0")
        if [[ "$wrong" -gt 0 ]]; then
            check_fail "$wrong triple(s) have wrong graph IRI (expected $graph_iri)"
        else
            check_ok "All triples in correct named graph"
        fi
    fi
}

# Ingest one DID via POST /beamlines/{bl}/datasets/{did}/foxden/ingest
ingest_did() {
    local bl="$1" did="$2"
    local enc_did
    enc_did=$(encode_did "$did")
    local url="${DATA_URL}/beamlines/${bl}/datasets/${enc_did}/foxden/ingest"

    echo "  POST $url"
    local resp
    resp=$(curl -ks -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${FOXDEN_TOKEN:-}" \
        "$url")

    local count graph_iri
    count=$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin).get('ingested',0))" 2>/dev/null || echo "0")
    graph_iri=$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin).get('graphIRI',''))" 2>/dev/null || echo "")

    if [[ "$count" -gt 0 ]]; then
        check_ok "Ingested $count triples → graphIRI=$graph_iri"
        echo "$graph_iri"  # return value
    else
        check_fail "Ingest returned 0 triples for $did — response: $(echo "$resp" | head -c 200)"
        echo ""
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# MODE A — DID-file mode
# ─────────────────────────────────────────────────────────────────────────────

if [[ -n "$DIDS_FILE" ]]; then

    echo -e "\n${BOLD}== FabricNode DID-file discovery ==${RESET}"
    echo "  catalog: $CATALOG_URL"
    echo "  data:    $DATA_URL"
    echo "  file:    $DIDS_FILE"
    [[ "$DO_INGEST" == true ]] && echo "  ingest:  yes"
    echo

    if [[ ! -f "$DIDS_FILE" ]]; then
        echo -e "${RED}Error: DIDs file not found: $DIDS_FILE${RESET}" >&2
        exit 1
    fi

    # Read non-blank, non-comment lines
    mapfile -t DIDS < <(grep -v '^\s*#' "$DIDS_FILE" | grep -v '^\s*$' || true)

    if [[ ${#DIDS[@]} -eq 0 ]]; then
        echo -e "${RED}No DIDs found in $DIDS_FILE${RESET}" >&2
        exit 1
    fi

    sep "DIDs to process (${#DIDS[@]})"
    for d in "${DIDS[@]}"; do echo "  · $d"; done

    # Optionally ingest first
    if [[ "$DO_INGEST" == true ]]; then
        sep "Ingest DIDs → data-service"
        declare -A GRAPH_IRIS=()
        for did in "${DIDS[@]}"; do
            bl=$(beamline_from_did "$did")
            if [[ -z "$bl" ]]; then
                check_fail "Cannot extract beamline from DID: $did"
                continue
            fi
            echo
            echo "  DID:      $did"
            echo "  Beamline: $bl"
            graph_iri=$(ingest_did "$bl" "$did")
            GRAPH_IRIS["$did"]="$graph_iri"
        done
    fi

    sep "SPARQL verification"
    for did in "${DIDS[@]}"; do
        bl=$(beamline_from_did "$did")
        if [[ -z "$bl" ]]; then
            check_fail "Cannot extract beamline from DID: $did"
            continue
        fi
        echo
        echo "  DID:      $did"
        echo "  Beamline: $bl"
        graph_iri=""
        if [[ "$DO_INGEST" == true ]]; then
            graph_iri="${GRAPH_IRIS[$did]:-}"
        fi
        query_sparql "$bl" "$did" "$graph_iri"
    done

    sep "Catalog verification"
    declare -A SEEN_BL=()
    for did in "${DIDS[@]}"; do
        bl=$(beamline_from_did "$did")
        [[ -z "$bl" || -n "${SEEN_BL[$bl]:-}" ]] && continue
        SEEN_BL["$bl"]=1

        url="${CATALOG_URL}/catalog/beamlines/${bl}/datasets"
        echo
        echo "  GET $url"
        resp=$(curl -ks "$url")
        count=$(echo "$resp" | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin).get('dcat:dataset',[])
    print(len(d) if isinstance(d,list) else 0)
except:
    print(0)
" 2>/dev/null || echo "0")
        if [[ "$count" -gt 0 ]]; then
            check_ok "Catalog lists $count dataset(s) for beamline $bl"
        else
            check_fail "Catalog returned 0 datasets for beamline $bl"
        fi
    done

    echo
    echo -e "${BOLD}Results: ${GREEN}${PASS} passed${RESET}${BOLD}, ${RED}${FAIL} failed${RESET}"
    [[ "$FAIL" -eq 0 ]] && exit 0 || exit 1
fi

# ─────────────────────────────────────────────────────────────────────────────
# MODE B — VoID discovery mode (original behaviour)
# ─────────────────────────────────────────────────────────────────────────────

echo "== FabricNode discovery via .well-known/void =="
echo "  catalog: $CATALOG_URL"
echo "  data:    $DATA_URL"
echo

# ── Step 0 — Get VoID description ────────────────────────────────────────────
sep "Step 0: VoID discovery"

VOID=$(curl -ks -H "Accept: application/ld+json" "$CATALOG_URL/.well-known/void")
echo "$VOID" | python3 -m json.tool 2>/dev/null || echo "$VOID"

# Extract beamline IDs from catalog @id fields ending in /catalog/<bl>
BEAMLINES=$(echo "$VOID" | python3 -c "
import json,sys,re
try:
    def walk(obj):
        if isinstance(obj,dict):
            v=obj.get('@id','')
            m=re.search(r'/catalog/([^/]+)$',v)
            if m: print(m.group(1))
            for val in obj.values(): walk(val)
        elif isinstance(obj,list):
            for item in obj: walk(item)
    walk(json.load(sys.stdin))
except Exception as e:
    sys.stderr.write(str(e)+'\n')
" 2>/dev/null | sort -u)

echo
echo "Discovered beamlines:"
echo "$BEAMLINES"
echo

# ── Step 1 — Discover datasets per beamline ───────────────────────────────────
sep "Step 1: Dataset discovery"

DATASETS=()   # entries: "bl<TAB>sparql_url<TAB>did"

for BL in $BEAMLINES; do
    URL="$CATALOG_URL/catalog/beamlines/$BL/datasets"
    echo "GET $URL"

    RESP=$(curl -ks "$URL")

    # Print @id and dataset count
    echo "$RESP" | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin)
    print(d.get('@id','(no @id)'))
    ds=d.get('dcat:dataset',[])
    print(len(ds) if isinstance(ds,list) else 0,'dataset(s)')
except Exception as e:
    print('parse error:',e)
" 2>/dev/null

    echo "Extract dataset access URLs (SPARQL endpoints)..."

    # Extract up to $LIMIT SPARQL URLs and their DIDs
    ENTRIES=$(echo "$RESP" | python3 -c "
import json,sys,urllib.parse
limit=$LIMIT
try:
    ds=json.load(sys.stdin).get('dcat:dataset',[]) or []
    for entry in ds[:limit]:
        did=entry.get('dct:title','')
        dist=entry.get('dcat:distribution',{})
        acc=dist.get('dcat:accessURL','') if isinstance(dist,dict) else ''
        if acc and did:
            print(did+'\t'+acc)
except Exception as e:
    sys.stderr.write(str(e)+'\n')
" 2>/dev/null)

    while IFS=$'\t' read -r DID ACC_URL; do
        [[ -z "$ACC_URL" ]] && continue
        DATASETS+=("${BL}"$'\t'"${ACC_URL}"$'\t'"${DID}")
    done <<< "$ENTRIES"

    echo
done

# ── Step 2 — Query datasets (SPARQL) ─────────────────────────────────────────
sep "Step 2: SPARQL queries"

for ITEM in "${DATASETS[@]:-}"; do
    BL=$(echo "$ITEM"   | cut -f1)
    URL=$(echo "$ITEM"  | cut -f2)
    DID=$(echo "$ITEM"  | cut -f3)

    echo "Beamline: $BL"
    echo "DID:      $DID"
    echo "SPARQL endpoint: $URL"

    RESP=$(curl -ks "$URL")

    # Print first 3 bindings
    echo "$RESP" | python3 -c "
import json,sys
try:
    b=json.load(sys.stdin).get('results',{}).get('bindings',[])
    print(json.dumps(b[:3],indent=2))
except:
    print('(could not parse response)')
" 2>/dev/null

    echo
done

echo "== DONE =="

echo
echo -e "${BOLD}Results: ${GREEN}${PASS} passed${RESET}${BOLD}, ${RED}${FAIL} failed${RESET}"
[[ "$FAIL" -eq 0 ]] && exit 0 || exit 1
