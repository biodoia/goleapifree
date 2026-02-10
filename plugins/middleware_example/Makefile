# Makefile per compilare middleware plugin

PLUGIN_NAME := rate_limiter
PLUGIN_FILE := $(PLUGIN_NAME).so
GO := go
BUILD_FLAGS := -buildmode=plugin
LDFLAGS := -s -w

PLUGIN_DIR := $(shell pwd)
OUTPUT_DIR := $(PLUGIN_DIR)/build

COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

.PHONY: all build clean install verify help

all: build

build:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Building middleware plugin $(PLUGIN_NAME)...$(COLOR_RESET)"
	@mkdir -p $(OUTPUT_DIR)
	@$(GO) build $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(PLUGIN_FILE) .
	@echo "$(COLOR_BOLD)$(COLOR_GREEN)Plugin built: $(OUTPUT_DIR)/$(PLUGIN_FILE)$(COLOR_RESET)"
	@ls -lh $(OUTPUT_DIR)/$(PLUGIN_FILE)

clean:
	@echo "$(COLOR_BOLD)$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	@rm -rf $(OUTPUT_DIR)
	@echo "$(COLOR_GREEN)Clean complete$(COLOR_RESET)"

install: build
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Installing plugin...$(COLOR_RESET)"
	@mkdir -p ../../plugins
	@cp $(OUTPUT_DIR)/$(PLUGIN_FILE) ../../plugins/
	@echo "$(COLOR_GREEN)Plugin installed$(COLOR_RESET)"

verify: build
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Verifying plugin...$(COLOR_RESET)"
	@nm -D $(OUTPUT_DIR)/$(PLUGIN_FILE) | grep -q "NewPlugin" && \
		echo "$(COLOR_GREEN)✓ NewPlugin symbol found$(COLOR_RESET)" || \
		echo "$(COLOR_YELLOW)⚠ NewPlugin symbol not found$(COLOR_RESET)"

help:
	@echo "$(COLOR_BOLD)Middleware Plugin Builder$(COLOR_RESET)"
	@echo "  make build   - Build the plugin"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make install - Install to plugins directory"
	@echo "  make verify  - Verify plugin is valid"
