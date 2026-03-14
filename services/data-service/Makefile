MODULE  := $(shell head -1 go.mod | awk '{print $$2}')
BINARY  := $(notdir $(MODULE))
BUILD_DIR := ./bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BOLD  := \033[1m
GREEN := \033[32m
CYAN  := \033[36m
RESET := \033[0m

.DEFAULT_GOAL := help

.PHONY: help
help:
	@printf '$(BOLD)$(BINARY)$(RESET)\n'
	@awk 'BEGIN {FS=":.*##"} /^[a-zA-Z_\/-]+:.*##/ {printf "  $(CYAN)%-18s$(RESET) %s\n",$$1,$$2}' $(MAKEFILE_LIST)

.PHONY: deps
deps: ## Download dependencies
	go mod download

.PHONY: deps/tidy
deps/tidy: ## Tidy go.mod/go.sum
	go mod tidy

.PHONY: build
build: deps ## Build binary → bin/$(BINARY)
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT)" \
		-o $(BUILD_DIR)/$(BINARY) ./cmd/server
	@printf '$(GREEN)✓ $(BUILD_DIR)/$(BINARY)$(RESET)\n'

.PHONY: run
run: build ## Build and run
	$(BUILD_DIR)/$(BINARY)

.PHONY: test
test: ## Run tests
	go test -v -race ./...

.PHONY: test/cover
test/cover: ## Coverage report
	go test -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -func=$(BUILD_DIR)/coverage.out

.PHONY: fmt
fmt: ## Format source
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: vet ## Run golangci-lint
	@which golangci-lint >/dev/null || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest
	golangci-lint run ./...

.PHONY: docker/build
docker/build: ## Build Docker image
	docker build -t $(BINARY):$(VERSION) -t $(BINARY):latest .

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR)
