#!/bin/bash

set -e

echo
echo "------"
echo "FOXDEN+FabricNode ingest integration test..."
make probe KEY=btr VALUE=test-123-a VERBOSE=1

echo
echo "------"
echo "FOXDEN+FabricNode DOI integration test..."
did=/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-15E0C5BB-D36A-47D2-A91E-5CF90707885C
make probe-doi DID=$did DOI="10.5281/zenodo.123456" DOI_URL="https://doi.org/10.52
81/zenodo.123456" INGEST=1 VERBOSE=1
