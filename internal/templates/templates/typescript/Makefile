# TypeScript Makefile (pnpm/bun)
.DEFAULT_GOAL := help
.PHONY: help install build dev test lint fmt typecheck clean

# Detect package manager (prefer bun if bun.lockb exists, else pnpm)
PM := $(shell if [ -f bun.lockb ]; then echo "bun"; else echo "pnpm"; fi)

help: ## Show available commands
	@echo "Using package manager: $(PM)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	$(PM) install

build: ## Build the project
	$(PM) run build

dev: ## Run development server
	$(PM) run dev

test: ## Run tests
	$(PM) run test

lint: ## Lint code
	$(PM) run lint

fmt: ## Format code
	$(PM) run format

typecheck: ## TypeScript type checking
	$(PM) run typecheck

clean: ## Clean build artifacts
	rm -rf dist node_modules/.cache .turbo .next out
