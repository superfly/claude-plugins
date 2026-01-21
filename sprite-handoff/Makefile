.PHONY: all build test test-unit test-integration test-e2e clean install lint fmt help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary name and paths
BINARY_NAME=sprite-handoff
BINARY_DIR=bin
BINARY_PATH=$(BINARY_DIR)/$(BINARY_NAME)
CMD_PATH=./cmd/sprite-handoff

# Build flags
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

## Build targets

build: ## Build the binary
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) $(CMD_PATH)
	@echo "Built $(BINARY_PATH)"

build-all: ## Build for multiple platforms
	@mkdir -p $(BINARY_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	@echo "Built binaries for all platforms"

## Test targets

test: test-unit ## Run all tests (unit only by default)

test-unit: ## Run unit tests
	$(GOTEST) -v -short ./internal/...

test-integration: build ## Run integration tests (requires SPRITE_TOKEN and SPRITE_NAME)
	$(GOTEST) -v ./tests/integration/...

test-e2e: build ## Run E2E tests (requires SPRITE_TOKEN and SPRITE_NAME)
	$(GOTEST) -v ./tests/e2e/...

test-all: build ## Run all tests including integration and E2E
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./internal/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Code quality

fmt: ## Format code
	$(GOFMT) -s -w .

lint: ## Run linter
	$(GOLINT) run ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

## Dependencies

deps: ## Download dependencies
	$(GOMOD) download

tidy: ## Tidy dependencies
	$(GOMOD) tidy

## Install

install: build ## Install binary to plugin bin directory
	@mkdir -p bin
	@cp $(BINARY_PATH) bin/
	@echo "Installed to bin/$(BINARY_NAME)"

install-global: build ## Install binary to /usr/local/bin
	@cp $(BINARY_PATH) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

## Clean

clean: ## Remove build artifacts
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

## Development

dev: build ## Build and run with verbose output
	./$(BINARY_PATH) --help

watch: ## Watch for changes and rebuild (requires entr)
	find . -name '*.go' | entr -c make build

## Help

help: ## Show this help
	@echo "sprite-handoff - CLI tool for syncing files to Sprites.dev"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Environment variables for integration/E2E tests:"
	@echo "  SPRITE_TOKEN  - Sprites.dev API token (or use sprite CLI auth)"
	@echo "  SPRITE_NAME   - Name of sprite to use for testing"
