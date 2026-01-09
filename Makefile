.PHONY: build test lint clean install run help

# Build variables
BINARY_NAME=momorph
VERSION?=dev
COMMIT_SHA?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION?=$(shell go version | awk '{print $$3}')

# Go build flags
LDFLAGS=-ldflags "-X github.com/momorph/cli/internal/version.Version=$(VERSION) \
				  -X github.com/momorph/cli/internal/version.CommitSHA=$(COMMIT_SHA) \
				  -X github.com/momorph/cli/internal/version.BuildDate=$(BUILD_DATE) \
				  -X github.com/momorph/cli/internal/version.GoVersion=$(GO_VERSION)"

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) main.go
	@echo "Build complete: $(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf bin/ dist/
	@echo "Clean complete"

install: build ## Install binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS)
	@echo "Install complete"

run: build ## Build and run the binary
	@./$(BINARY_NAME)

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies ready"

dev: ## Run in development mode with debug logging
	@go run main.go --debug

.DEFAULT_GOAL := help
