# Base Makefile
.DEFAULT_GOAL := help
.PHONY: help install build test lint fmt clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	@echo "Override this target for your project"

build: ## Build the project
	@echo "Override this target for your project"

test: ## Run tests
	@echo "Override this target for your project"

lint: ## Lint code
	@echo "Override this target for your project"

fmt: ## Format code
	@echo "Override this target for your project"

clean: ## Clean build artifacts
	@echo "Override this target for your project"
