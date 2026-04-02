2. notification-service store uses store.Inbox but config.go declares
   *store.Inbox — the type name in handlers/config.go is *store.Inbox but the
   actual store file may export Store not Inbox

3. void.NodeConfig is a second config struct that duplicates
   fabricconfig.NodeConfig — catalog-service carries both. void.NodeConfig has
   BaseURL, NodeID, NodeName, DataServiceURL; fabricconfig.NodeConfig has ID,
   Name, BaseURL. These should be unified or void.NodeConfig should be
   eliminated and handlers should take *fabricconfig.Config directly. Please
   suggest proper way to handle this.

4. No tests at all for any handler — the service Makefiles have test and
   test/cover targets, but there are no *_test.go files in any
   internal/handlers/ directory. pkg/config has a test but nothing else does.
   Given the CORS, SHACL, and ingest logic, at minimum table-driven unit tests
   for the SHACL validator and the FOXDEN RDF conversion would prevent
   regressions.

5. instead of docker-compose provide me separate k8s directory which should
   contains dockerfiles for all services; helm charts; k8s manifest files;
   provide a way to mount config/fabric.yaml to appropriate containers; and
   appropriate README.md to outline deployment procedure on k8s infrastructure
