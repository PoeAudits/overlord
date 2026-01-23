# Solidity Makefile (Foundry)
.DEFAULT_GOAL := help
.PHONY: help install build test test-v snapshot gas coverage lint fmt clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	forge install

build: ## Build contracts
	forge build

test: ## Run tests
	forge test

test-v: ## Run tests with verbosity
	forge test -vvv

snapshot: ## Update gas snapshot
	forge snapshot

gas: ## Show gas report
	forge test --gas-report

coverage: ## Run coverage
	forge coverage

lint: ## Check formatting
	forge fmt --check

fmt: ## Format contracts
	forge fmt

clean: ## Clean build artifacts
	forge clean
