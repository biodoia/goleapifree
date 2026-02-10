# GoLeapAI Makefile
# ==================

# Variables
BINARY_NAME=goleapai
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# Directories
BUILD_DIR=bin
DATA_DIR=data

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOCLEAN=$(GOCMD) clean

.PHONY: all build test clean install dev run help

all: clean build

.DEFAULT_GOAL := help

help: ## Display this help message
	@echo "GoLeapAI - Available Make Targets"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/backend
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run all tests
	@echo "Running tests..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./...

clean: ## Clean build artifacts
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

dev: build ## Run in development mode
	@mkdir -p $(DATA_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) serve --dev --verbose

run: build ## Build and run the server
	@mkdir -p $(DATA_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) serve

deps: ## Download and tidy dependencies
	@$(GOMOD) download
	@$(GOMOD) tidy

fmt: ## Format code
	@$(GOCMD) fmt ./...

init-db: build ## Initialize database
	@mkdir -p $(DATA_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) migrate up
	@$(BUILD_DIR)/$(BINARY_NAME) migrate seed

doctor: build ## Run health diagnostics
	@$(BUILD_DIR)/$(BINARY_NAME) doctor --verbose

# TUI targets
tui: ## Build the TUI
	@echo "Building TUI..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=1 $(GOBUILD) -o $(BUILD_DIR)/goleapai-tui ./cmd/tui
	@echo "✓ TUI build complete: $(BUILD_DIR)/goleapai-tui"

tui-run: tui ## Build and run the TUI
	@mkdir -p $(DATA_DIR)
	@$(BUILD_DIR)/goleapai-tui

tui-dev: tui ## Run TUI in development mode with hot reload
	@mkdir -p $(DATA_DIR)
	@$(BUILD_DIR)/goleapai-tui --config ./configs/config.yaml

build-all: ## Build both backend and TUI
	@echo "Building all binaries..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/backend
	@CGO_ENABLED=1 $(GOBUILD) -o $(BUILD_DIR)/goleapai-tui ./cmd/tui
	@echo "✓ All builds complete"

# Discovery targets
discovery-run: build ## Run discovery once
	@echo "Running discovery..."
	@$(BUILD_DIR)/$(BINARY_NAME) discovery run

discovery-start: build ## Start discovery service
	@echo "Starting discovery service..."
	@$(BUILD_DIR)/$(BINARY_NAME) discovery start

discovery-stats: build ## Show discovery statistics
	@echo "Fetching discovery statistics..."
	@$(BUILD_DIR)/$(BINARY_NAME) discovery stats

discovery-validate: build ## Validate a specific endpoint (requires URL=)
	@test -n "$(URL)" || (echo "ERROR: URL not specified. Usage: make discovery-validate URL=https://api.example.com"; exit 1)
	@$(BUILD_DIR)/$(BINARY_NAME) discovery validate $(URL)

discovery-verify: build ## Verify existing providers
	@echo "Verifying existing providers..."
	@$(BUILD_DIR)/$(BINARY_NAME) discovery verify

test-discovery: ## Run discovery module tests
	@echo "Running discovery tests..."
	@$(GOTEST) -v ./internal/discovery/...
