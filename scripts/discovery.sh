#!/usr/bin/env bash

set -euo pipefail

# ─── CONFIG ─────────────────────────────────────────────────────────────
CATALOG_URL="${CATALOG_URL:-http://localhost:8781}"
DATA_URL="${DATA_URL:-http://localhost:8782}"

LIMIT="${LIMIT:-3}"

echo "== FabricNode discovery via .well-known/void =="
echo

# ────────────────────────────────────────────────────────────────────────
# Step 0 — Get VOID description
# ────────────────────────────────────────────────────────────────────────
echo "== Step 0: VOID discovery =="

VOID=$(curl -ks -H "Accept: application/ld+json" "$CATALOG_URL/.well-known/void")

echo "$VOID" | jq .

# Extract beamline identifiers from VOID
# Assumes dataset URIs like: /catalog/beamlines/{bl}
BEAMLINES=$(echo "$VOID" | jq -r '
  .. | objects | select(has("@id")) | .["@id"] |
  select(test("/catalog/[^/]+$")) |
  capture("/catalog/(?<bl>[^/]+)$") |
  .bl
' | sort -u)

echo
echo "Discovered beamlines:"
echo "$BEAMLINES"
echo

# ────────────────────────────────────────────────────────────────────────
# Step 1 — Discover datasets per beamline
# ────────────────────────────────────────────────────────────────────────
echo "== Step 1: Dataset discovery =="

DATASETS=()

for BL in $BEAMLINES; do
  URL="$CATALOG_URL/catalog/beamlines/$BL/datasets"

  echo "GET $URL"

  RESP=$(curl -ks "$URL")
  echo "$RESP" | jq '.["@id"], (.["dcat:dataset"] | length)'

  echo "Extract dataset access URLs (SPARQL endpoints)..."
  URLS=$(echo "$RESP" | jq -r '.["dcat:dataset"][]? | .["dcat:distribution"]["dcat:accessURL"]')
  URLS=$(echo "$URLS" | head -n "$LIMIT")

  for U in $URLS; do
    DATASETS+=("$BL|$U")
  done

  echo
done

# ────────────────────────────────────────────────────────────────────────
# Step 2 — Query datasets (SPARQL)
# ────────────────────────────────────────────────────────────────────────
echo "== Step 2: SPARQL queries =="

for ITEM in "${DATASETS[@]}"; do
  BL="${ITEM%%|*}"
  URL="${ITEM##*|}"

  echo "Beamline: $BL"
  echo "SPARQL endpoint: $URL"

  # Default GET returns triples (as your service already does)
  RESP=$(curl -ks "$URL")

  echo "$RESP" | jq '.results.bindings[:3]'

  echo
done

echo "== DONE =="
