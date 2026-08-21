.PHONY: help install lint test build

help: ## Shows the available commands
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf " %-20s %s\n", $$1, $$2}'

install: ## Installs dependencies
	npm ci

lint: ## Runs the linter
	npm run lint

test: ## Runs the test suite
	npm run test -- --run

build: ## Builds the production bundle
	npm run build
