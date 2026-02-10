# Makefile per compilare plugin GoLeapAI

PLUGIN_NAME := example_provider
PLUGIN_FILE := $(PLUGIN_NAME).so
GO := go
BUILD_FLAGS := -buildmode=plugin
LDFLAGS := -s -w

# Directory
PLUGIN_DIR := $(shell pwd)
OUTPUT_DIR := $(PLUGIN_DIR)/build

# Go module path
MODULE_PATH := github.com/biodoia/goleapifree

# Colori per output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

.PHONY: all build clean install test help deps

all: build

## build: Compila il plugin
build:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Building plugin $(PLUGIN_NAME)...$(COLOR_RESET)"
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(PLUGIN_FILE) .
	@echo "$(COLOR_BOLD)$(COLOR_GREEN)Plugin built successfully: $(OUTPUT_DIR)/$(PLUGIN_FILE)$(COLOR_RESET)"
	@ls -lh $(OUTPUT_DIR)/$(PLUGIN_FILE)

## clean: Rimuove i file compilati
clean:
	@echo "$(COLOR_BOLD)$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(OUTPUT_DIR)
	@echo "$(COLOR_GREEN)Clean complete$(COLOR_RESET)"

## install: Installa il plugin nella directory principale
install: build
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Installing plugin...$(COLOR_RESET)"
	@mkdir -p ../../plugins
	@cp $(OUTPUT_DIR)/$(PLUGIN_FILE) ../../plugins/
	@echo "$(COLOR_GREEN)Plugin installed to ../../plugins/$(PLUGIN_FILE)$(COLOR_RESET)"

## test: Testa il plugin
test:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Testing plugin...$(COLOR_RESET)"
	@$(GO) test -v ./...
	@echo "$(COLOR_GREEN)Tests complete$(COLOR_RESET)"

## deps: Verifica e scarica le dipendenze
deps:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Checking dependencies...$(COLOR_RESET)"
	@$(GO) mod download
	@$(GO) mod verify
	@echo "$(COLOR_GREEN)Dependencies OK$(COLOR_RESET)"

## verify: Verifica che il plugin sia valido
verify: build
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Verifying plugin...$(COLOR_RESET)"
	@if [ -f $(OUTPUT_DIR)/$(PLUGIN_FILE) ]; then \
		echo "$(COLOR_GREEN)✓ Plugin file exists$(COLOR_RESET)"; \
		file $(OUTPUT_DIR)/$(PLUGIN_FILE); \
		nm -D $(OUTPUT_DIR)/$(PLUGIN_FILE) | grep -q "NewPlugin" && \
		echo "$(COLOR_GREEN)✓ NewPlugin symbol found$(COLOR_RESET)" || \
		echo "$(COLOR_YELLOW)⚠ Warning: NewPlugin symbol not found$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)✗ Plugin file not found$(COLOR_RESET)"; \
		exit 1; \
	fi

## rebuild: Pulisce e ricompila
rebuild: clean build

## watch: Watch mode per sviluppo (richiede entr o inotifywait)
watch:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Watching for changes...$(COLOR_RESET)"
	@while true; do \
		inotifywait -e modify,create,delete -r . && \
		make build; \
	done

## info: Mostra informazioni sul plugin
info:
	@echo "$(COLOR_BOLD)Plugin Information:$(COLOR_RESET)"
	@echo "  Name:        $(PLUGIN_NAME)"
	@echo "  Output:      $(OUTPUT_DIR)/$(PLUGIN_FILE)"
	@echo "  Module Path: $(MODULE_PATH)"
	@echo "  Go Version:  $(shell $(GO) version)"
	@if [ -f $(OUTPUT_DIR)/$(PLUGIN_FILE) ]; then \
		echo "  File Size:   $(shell ls -lh $(OUTPUT_DIR)/$(PLUGIN_FILE) | awk '{print $$5}')"; \
		echo "  Modified:    $(shell ls -l $(OUTPUT_DIR)/$(PLUGIN_FILE) | awk '{print $$6, $$7, $$8}')"; \
	fi

## help: Mostra questo messaggio di aiuto
help:
	@echo "$(COLOR_BOLD)GoLeapAI Plugin Builder$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)Usage:$(COLOR_RESET)"
	@echo "  make [target]"
	@echo ""
	@echo "$(COLOR_BOLD)Targets:$(COLOR_RESET)"
	@grep -E '^##' Makefile | sed 's/## /  /' | column -t -s ':'

# Target di default
.DEFAULT_GOAL := help
