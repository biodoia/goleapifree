# GoLeapAI Configure Tool Makefile

.PHONY: build install clean test help

# Variables
BINARY_NAME=goleapai-configure
INSTALL_PATH=$(GOPATH)/bin
BUILD_FLAGS=-ldflags="-s -w"
GO=go

help: ## Show this help message
	@echo "GoLeapAI Configure Tool - Build Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the configure tool
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(BUILD_FLAGS) -o $(BINARY_NAME)
	@echo "✓ Built: ./$(BINARY_NAME)"

install: build ## Build and install to GOPATH/bin
	@echo "Installing to $(INSTALL_PATH)..."
	@mkdir -p $(INSTALL_PATH)
	@cp $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "✓ Installed: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo ""
	@echo "You can now run: $(BINARY_NAME)"

clean: ## Remove built binaries
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Cleaned"

test: ## Test the configure tool
	@echo "Testing configuration tool..."
	$(GO) test -v ./...

run: build ## Build and run the tool
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME) --help

run-all: build ## Build and run with --all flag
	@echo "Running $(BINARY_NAME) --all..."
	@./$(BINARY_NAME) --all

run-test: build ## Build and run connectivity tests
	@echo "Running connectivity tests..."
	@./$(BINARY_NAME) --test

run-dry: build ## Build and run in dry-run mode
	@echo "Running in dry-run mode..."
	@./$(BINARY_NAME) --dry-run

check: ## Check Go code
	@echo "Checking code..."
	$(GO) fmt ./...
	$(GO) vet ./...
	@echo "✓ Code check passed"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download
	@echo "✓ Dependencies downloaded"
