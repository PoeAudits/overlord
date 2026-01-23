# Go Makefile
.DEFAULT_GOAL := help
.PHONY: help install build run test bench lint fmt clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	go mod download

build: ## Build the application
	go build -o bin/$(shell basename $(CURDIR)) ./...

run: ## Run the application
	go run .

test: ## Run tests
	go test -v ./...

bench: ## Run benchmarks
	go test -bench=. -benchmem ./...

lint: ## Run static analysis (go vet)
	go vet ./...

fmt: ## Format code
	go fmt ./...

clean: ## Clean build artifacts
	rm -rf bin/ dist/ coverage.out
