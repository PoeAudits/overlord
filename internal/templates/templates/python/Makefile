# Python Makefile (uv + ruff)
.DEFAULT_GOAL := help
.PHONY: help install sync lock run test lint fmt clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	uv sync

sync: ## Sync dependencies (alias for install)
	uv sync

lock: ## Update lock file
	uv lock

run: ## Run the project
	uv run python -m $(shell basename $(CURDIR) | tr '-' '_')

test: ## Run tests
	uv run pytest

lint: ## Lint code (ruff check)
	uv run ruff check .

fmt: ## Format code (ruff format)
	uv run ruff format .

clean: ## Clean build artifacts
	rm -rf __pycache__ .pytest_cache .ruff_cache dist build *.egg-info .venv
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
