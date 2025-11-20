.PHONY: help build test test-short test-storage test-coverage clean run dev migrate-up migrate-down docker-build docker-run install lint fmt vet

# Default target
.DEFAULT_GOAL := help

# Application configuration
APP_NAME := fixity
BINARY := ./$(APP_NAME)
DOCKER_IMAGE := $(APP_NAME):latest

# Build configuration
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/fixity
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags="-s -w"

# Test configuration
TEST_FLAGS := -v -race -timeout 180s
TEST_SHORT_FLAGS := -v -short -timeout 60s
COVERAGE_FILE := coverage.out

# Database configuration (override with environment variables)
DATABASE_URL ?= postgres://fixity:fixity@localhost/fixity?sslmode=disable
LISTEN_ADDR ?= :8080

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application binary
	@echo "Building $(APP_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

build-all: ## Build for all platforms (Linux, macOS, Windows)
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Multi-platform build complete"

install: ## Install the application to $GOPATH/bin
	@echo "Installing $(APP_NAME)..."
	$(GO) install $(GOFLAGS) $(MAIN_PATH)
	@echo "Installed to $$(go env GOPATH)/bin/$(APP_NAME)"

test: ## Run all tests
	@echo "Running all tests..."
	$(GO) test ./... $(TEST_FLAGS)

test-short: ## Run tests in short mode (skip long-running tests)
	@echo "Running tests in short mode..."
	$(GO) test ./... $(TEST_SHORT_FLAGS)

test-storage: ## Run storage backend tests only
	@echo "Running storage backend tests..."
	$(GO) test ./internal/storage $(TEST_FLAGS)

test-storage-short: ## Run storage backend tests in short mode
	@echo "Running storage backend tests (short)..."
	$(GO) test ./internal/storage $(TEST_SHORT_FLAGS)

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GO) test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-verbose: ## Run tests with verbose output
	@echo "Running tests with verbose output..."
	$(GO) test ./... -v -count=1

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GO) test ./... -bench=. -benchmem

clean: ## Clean build artifacts and test cache
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) coverage.html
	$(GO) clean -cache -testcache -modcache
	@echo "Clean complete"

run: build ## Build and run the application
	@echo "Starting $(APP_NAME)..."
	DATABASE_URL=$(DATABASE_URL) LISTEN_ADDR=$(LISTEN_ADDR) $(BUILD_DIR)/$(APP_NAME) serve

dev: ## Run the application in development mode (with live rebuild)
	@echo "Running in development mode..."
	DATABASE_URL=$(DATABASE_URL) LISTEN_ADDR=$(LISTEN_ADDR) $(GO) run $(MAIN_PATH) serve

migrate-up: build ## Run database migrations up
	@echo "Running database migrations..."
	DATABASE_URL=$(DATABASE_URL) $(BUILD_DIR)/$(APP_NAME) migrate up

migrate-down: build ## Rollback last database migration
	@echo "Rolling back last migration..."
	DATABASE_URL=$(DATABASE_URL) $(BUILD_DIR)/$(APP_NAME) migrate down

migrate-list: build ## List database migrations
	@echo "Listing migrations..."
	DATABASE_URL=$(DATABASE_URL) $(BUILD_DIR)/$(APP_NAME) migrate list

user-create: build ## Create a new user (requires USERNAME, PASSWORD, and optionally ADMIN=true)
	@echo "Creating user..."
	@if [ -z "$(USERNAME)" ]; then echo "Error: USERNAME required"; exit 1; fi
	@if [ -z "$(PASSWORD)" ]; then echo "Error: PASSWORD required"; exit 1; fi
	DATABASE_URL=$(DATABASE_URL) $(BUILD_DIR)/$(APP_NAME) user create \
		--username $(USERNAME) \
		--password $(PASSWORD) \
		$(if $(ADMIN),--admin,) \
		$(if $(EMAIL),--email $(EMAIL),)

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 \
		-e DATABASE_URL=$(DATABASE_URL) \
		-e LISTEN_ADDR=$(LISTEN_ADDR) \
		$(DOCKER_IMAGE)

docker-compose-up: ## Start services with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop services with docker-compose
	docker-compose down

lint: ## Run linters (requires golangci-lint)
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	$(GO) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

mod-download: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download

mod-tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GO) mod tidy

mod-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GO) mod verify

deps: mod-download mod-tidy mod-verify ## Download, tidy, and verify dependencies

check: fmt vet test-short ## Run quick checks (fmt, vet, short tests)

ci: lint vet test ## Run CI checks (lint, vet, all tests)

version: ## Show version information
	@echo "$(APP_NAME) version information:"
	@$(GO) version
	@echo "Build target: $(BUILD_DIR)/$(APP_NAME)"

# Quick commands for common workflows
quick-test: test-storage-short ## Alias for quick storage tests

full-check: clean deps lint vet test ## Full check: clean, deps, lint, vet, and test

# Development database setup
db-create: ## Create development database (requires PostgreSQL)
	@echo "Creating development database..."
	createdb fixity || echo "Database may already exist"
	createuser fixity || echo "User may already exist"
	psql -d fixity -c "ALTER USER fixity WITH PASSWORD 'fixity';" || true
	psql -d fixity -c "GRANT ALL PRIVILEGES ON DATABASE fixity TO fixity;" || true

db-drop: ## Drop development database
	@echo "WARNING: This will delete the development database!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		dropdb --if-exists fixity; \
		echo "Database dropped"; \
	else \
		echo "Cancelled"; \
	fi

db-reset: db-drop db-create migrate-up ## Reset database (drop, create, migrate)

# Common developer workflows
first-run: deps build db-create migrate-up ## First-time setup (deps, build, db setup)
	@echo ""
	@echo "Setup complete! Create an admin user with:"
	@echo "  make user-create USERNAME=admin PASSWORD=yourpassword ADMIN=true"
	@echo ""
	@echo "Then run the server with:"
	@echo "  make run"

# Performance and profiling
profile-cpu: ## Run CPU profiling
	@echo "Running with CPU profiling..."
	$(GO) test ./... -cpuprofile=cpu.prof -bench=.

profile-mem: ## Run memory profiling
	@echo "Running with memory profiling..."
	$(GO) test ./... -memprofile=mem.prof -bench=.

# Security scanning
security-scan: ## Run security scanner (requires gosec)
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest" && exit 1)
	gosec ./...
