# Scripts
This area contains few scripts used by FabricNode
- `manage.sh` is generic script covering multiple actions used by Makefile, e.g.
  build/start/stop all services
- `demo.sh` covers demonstration invoked by `make demo`
- `probe.py` covers integration between FabricNode and FOXDEN

## Integration tests:

```

make probe KEY=btr VALUE=test-123-a DRY_RUN=1 VERBOSE=1

CHESS FabricNode — integration probe
  FOXDEN:   http://localhost:8300
  data:     http://localhost:8782
  catalog:  http://localhost:8781
  search:   btr='test-123-a'  limit=5
  dry-run: ingest and SPARQL steps will be skipped

══ Health checks ══
  ✓ catalog-service healthy
  ✓ data-service healthy
  ✓ FOXDEN reachable

══ FOXDEN search  (btr='test-123-a') ══
    POST: http://localhost:8300/search
    {
      "client": "probe",
      "service_query": {
        "spec": {
          "btr": "test-123-a"
        },
        "limit": 5,
        "sort_keys": [
          "cycle"
        ],
        "sort_order": -1
      }
    }
  ✓ FOXDEN search HTTP 200
  ✓ 5 record(s) found
  ✓ Record has required fields ['did', 'beamline', 'btr', 'cycle']
  ✓ DID well-formed: /beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=test-de
    [
      {
        "application": "user-app",
        "beamline": "3a",
        "btr": "test-123-a",
        "cycle": "2026-1",
        "date": 1768591297,
        "description": "test",
        "did": "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=test-demo/user=vek3:20260116_142137",
        "doi": "",
        "doi_access_metadata": "",
        "doi_created_at": "",
        "doi_foxden_url": "",
        "doi_parents_dids": "",
        "doi_provider": "",
        "doi_public": false,
        "doi_url": "",
        "doi_user": "",
        "input_files": [
          "file.png"
        ],
        "output_files": [
          "file.jpg"
        ],
        "parent_did": "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=test-demo",
        "sample_name": "test-demo",
        "schema": "user",
        "schema_file": "/Users/vk/Work/CHESS/FOXDEN/FOXDEN/configs/user.json",
        "user": "vek3"
      },
    … (29 more lines)

All 7 checks passed.

```
