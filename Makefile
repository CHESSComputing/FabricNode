# ─────────────────────────────────────────────────────────────────────────────
# CHESS Federated Knowledge Fabric Node — top-level Makefile
# ─────────────────────────────────────────────────────────────────────────────

SERVICES := catalog-service data-service identity-service notification-service
SVC_DIRS := $(addprefix services/,$(SERVICES))
MANAGE   := bash scripts/manage.sh

BOLD  := \033[1m
GREEN := \033[32m
CYAN  := \033[36m
RESET := \033[0m

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@printf '$(BOLD)CHESS Federated Knowledge Fabric Node$(RESET)\n\n'
	@awk 'BEGIN {FS=":.*##"} /^[a-zA-Z_\/-]+:.*##/ \
		{printf "  $(CYAN)%-26s$(RESET) %s\n",$$1,$$2}' $(MAKEFILE_LIST)

# ─────────────────────────────────────────────────────────────────────────────
# Build
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: build
build: ## Build all service binaries
	@$(MANAGE) build

.PHONY: deps/tidy
deps/tidy: ## Tidy go.mod/go.sum for all services
	@for svc in $(SVC_DIRS); do \
		printf '$(GREEN)→ tidy $$svc$(RESET)\n'; \
		$(MAKE) -C $$svc deps/tidy --no-print-directory; \
	done

# ─────────────────────────────────────────────────────────────────────────────
# Local process management (no Docker)
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: start
start: build ## Build then start all services locally (logs -> .logs/)
	@$(MANAGE) start

.PHONY: stop
stop: ## Stop all locally running services
	@$(MANAGE) stop

.PHONY: restart
restart: ## Restart all locally running services
	@$(MANAGE) restart

.PHONY: status
status: ## Show PID, port and health of every service
	@$(MANAGE) status

.PHONY: logs
logs: ## Tail all service logs merged  (use: make logs SVC=catalog-service)
ifdef SVC
	@$(MANAGE) logs $(SVC)
else
	@$(MANAGE) logs --all
endif

.PHONY: start/%
start/%: ## Start a single service  e.g. make start/catalog-service
	@$(MANAGE) build $*
	@$(MANAGE) start $*

.PHONY: stop/%
stop/%: ## Stop a single service   e.g. make stop/data-service
	@$(MANAGE) stop $*

.PHONY: restart/%
restart/%: ## Restart a single service  e.g. make restart/identity-service
	@$(MANAGE) restart $*

.PHONY: logs/%
logs/%: ## Tail log for a single service  e.g. make logs/catalog-service
	@$(MANAGE) logs $*

# ─────────────────────────────────────────────────────────────────────────────
# Test, Format, Lint
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run tests across all services
	@for svc in $(SVC_DIRS); do \
		printf '$(GREEN)→ test $$svc$(RESET)\n'; \
		$(MAKE) -C $$svc test --no-print-directory; \
	done

.PHONY: fmt
fmt: ## Format all Go source
	@for svc in $(SVC_DIRS); do \
		$(MAKE) -C $$svc fmt --no-print-directory; \
	done

.PHONY: vet
vet: ## Run go vet across all services
	@for svc in $(SVC_DIRS); do \
		$(MAKE) -C $$svc vet --no-print-directory; \
	done

.PHONY: lint
lint: ## Run golangci-lint across all services
	@for svc in $(SVC_DIRS); do \
		$(MAKE) -C $$svc lint --no-print-directory; \
	done

# ─────────────────────────────────────────────────────────────────────────────
# Docker Compose (alternative to local process management)
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: docker/up
docker/up: ## Start the full node with Docker Compose
	docker compose up --build -d
	@printf '$(GREEN)✓ Stack running$(RESET)\n'

.PHONY: docker/down
docker/down: ## Stop the Docker Compose stack
	docker compose down

.PHONY: docker/logs
docker/logs: ## Tail Docker Compose logs
	docker compose logs -f

.PHONY: docker/ps
docker/ps: ## Show Docker container status
	docker compose ps

# ─────────────────────────────────────────────────────────────────────────────
# Demo & health
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: demo
demo: ## Run the end-to-end curl demo (services must be running)
	@bash scripts/demo.sh

# Usage: make probe KEY=btr VALUE=test-123-a
#        make probe KEY=cycle VALUE=2026-1
#        make probe KEY=beamline VALUE=3a
#        make probe KEY=sample_name VALUE=silicon-std  VERBOSE=1
#        make probe KEY=btr VALUE=test-123-a DRY_RUN=1
.PHONY: probe
probe: ## Trace FOXDEN→data-service→SPARQL data flow  (KEY=<field> VALUE=<val>)
	@test -n "$(KEY)"   || (echo "Usage: make probe KEY=<field> VALUE=<val>"; exit 1)
	@test -n "$(VALUE)" || (echo "Usage: make probe KEY=<field> VALUE=<val>"; exit 1)
	@python scripts/probe.py \
		--key   "$(KEY)"   \
		--value "$(VALUE)" \
		$(if $(FOXDEN),  --foxden  "$(FOXDEN)")  \
		$(if $(DATA),    --data    "$(DATA)")    \
		$(if $(CATALOG), --catalog "$(CATALOG)") \
		$(if $(LIMIT),   --limit   "$(LIMIT)")   \
		$(if $(DRY_RUN), --dry-run)              \
		$(if $(VERBOSE), --verbose)

# Usage: make probe-doi DID="/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-7271" \
#                       DOI="10.5281/zenodo.123456" \
#                       DOI_URL="https://doi.org/10.5281/zenodo.123456"
#        make probe-doi DID="..." DOI="..." DOI_URL="..." INGEST=1 VERBOSE=1
.PHONY: probe-doi
probe-doi: ## Trace FOXDEN DOI→identity-service credential flow  (DID= DOI= DOI_URL=)
	@test -n "$(DID)"     || (echo "Usage: make probe-doi DID=<did> DOI=<doi> DOI_URL=<url>"; exit 1)
	@test -n "$(DOI)"     || (echo "Usage: make probe-doi DID=<did> DOI=<doi> DOI_URL=<url>"; exit 1)
	@test -n "$(DOI_URL)" || (echo "Usage: make probe-doi DID=<did> DOI=<doi> DOI_URL=<url>"; exit 1)
	@python scripts/probe_doi.py \
		--did     "$(DID)"     \
		--doi     "$(DOI)"     \
		--doi-url "$(DOI_URL)" \
		$(if $(IDENTITY), --identity "$(IDENTITY)") \
		$(if $(DATA),     --data     "$(DATA)")     \
		$(if $(FOXDEN),   --foxden   "$(FOXDEN)")   \
		$(if $(INGEST),   --ingest)                 \
		$(if $(VERBOSE),  --verbose)

.PHONY: health
health: ## HTTP health check for all services (alias for status)
	@$(MANAGE) status

# ─────────────────────────────────────────────────────────────────────────────
# Clean
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: clean
clean: ## Remove build artefacts (bin/) for all services
	@for svc in $(SVC_DIRS); do \
		$(MAKE) -C $$svc clean --no-print-directory; \
	done

.PHONY: clean/run
clean/run: ## Remove PID and log files (.run/ .logs/)
	@rm -rf .run .logs
	@printf '$(GREEN)✓ .run/ and .logs/ removed$(RESET)\n'

.PHONY: clean/all
clean/all: stop clean clean/run ## Stop services + remove all generated files
