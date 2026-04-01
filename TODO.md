1. No go.work file — all four services use replace directives to find pkg/config and now pkg/server. This works but means go.sum files go stale independently and go mod tidy must be run per-service. A go.work file at the repo root would remove all the replace directives and let a single go work sync keep everything consistent.

2. generalize graph db store to interface with common APIs such that
I can have different implementations: in-memory and oxigraph graph stores;
adjust configuration to choose either one; provide implementation
for oxigraph store

3. void.NodeConfig is a second config struct that duplicates fabricconfig.NodeConfig — catalog-service carries both. void.NodeConfig has BaseURL, NodeID, NodeName, DataServiceURL; fabricconfig.NodeConfig has ID, Name, BaseURL. These should be unified or void.NodeConfig should be eliminated and handlers should take *fabricconfig.Config directly.

4. loadConfig() in catalog-service main.go has a convoluted double-load fallback — when Load fails it calls Load("") again which will also fail the same way, and the nil check after that can never be reached. The error handling logic there needs simplifying to match the cleaner pattern in data-service.

5. data-service/cmd/server/auth.go — GetToken panics on empty input — if tokenSource is empty it panics with "tokenSource is empty", but in main.go the call is inside a fallback path that already returned early on success. The panic is reachable if cfg.Foxden.Token is "" and GetTokenFromFoxden fails. This needs to become a returned error rather than a panic.

6. foxden/client.go — Search response decoding is fragile — it decodes into []map[string]any and synthesises a ServiceResponse, but checks resp.Status (the HTTP status string like "200 OK") with strings.Contains(…, "ok") which is case-sensitive and would match partial strings. The HTTP StatusCode field should be used instead.

7. pkg/config/config.go — Load("") double-error path — when resolvePath returns an error (no file found), Load immediately returns nil, err, so MustLoad("") panics rather than returning defaults. The intended behaviour (use defaults when no file is found) is not what's implemented. Load should return defaults(), nil when no file is found rather than an error.

8. pkg/config/config.go — ApplyEnv not in the doc comment — FOXDEN_TOKEN and FOXDEN_METADATA_URL are applied but missing from the fabric.yaml comments section, and the new authz_url/client_id/client_secret/token_scope fields in FoxdenConfig have no corresponding env overrides in ApplyEnv.

9. instead of docker-compose provide me separate k8s directory which should
    contains dockerfiles for all services; helm charts; k8s manifest files;
    provide a way to mount config/fabric.yaml to appropriate containers;
    and appropriate README.md to outline deployment procedure on k8s
    infrastructure

10. foxden/client.go — dead NewClientFromConfig function — it accepts a never-implemented interface (GetMetadataURL(), GetToken(), GetTimeout()), making it unreachable. It should either be removed or implemented using the concrete fabricconfig.FoxdenConfig type.


11. notification-service store uses store.Inbox but config.go declares *store.Inbox — the type name in handlers/config.go is *store.Inbox but the actual store file may export Store not Inbox (worth verifying after the renaming history of this project).

12. No tests at all for any handler — the service Makefiles have test and test/cover targets, but there are no *_test.go files in any internal/handlers/ directory. pkg/config has a test but nothing else does. Given the CORS, SHACL, and ingest logic, at minimum table-driven unit tests for the SHACL validator and the FOXDEN RDF conversion would prevent regressions.

