.PHONY: help install lint test build

help: ## Shows the available commands
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf " %-20s %s\n", $$1, $$2}'

install: ## Installs dependencies
	go mod download

lint: ## Runs the linter
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt'd:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	go vet ./...

test: ## Runs the test suite
	go test ./... -cover

build: ## Builds the production binary
	CGO_ENABLED=0 go build -o bin/backend ./cmd/backend
