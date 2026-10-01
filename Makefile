# iTurtle Makefile

# Variables
BINARY_NAME=iTurtle
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe
BINARY_MAC=$(BINARY_NAME)_darwin
MAIN_PATH=./cmd/iturtle-smart-fetcher
BUILD_DIR=./bin
DIST_DIR=./dist
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

.PHONY: all build clean test coverage install uninstall run help fmt vet lint deps tidy build-all release

# Default target
all: clean deps test build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux: ## Build for Linux (amd64)
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_UNIX) $(MAIN_PATH)

build-linux-arm64: ## Build for Linux ARM64
	@echo "Building for Linux ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_linux_arm64 $(MAIN_PATH)

build-windows: ## Build for Windows (amd64)
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_WINDOWS) $(MAIN_PATH)

build-darwin: ## Build for macOS (amd64)
	@echo "Building for macOS (Intel)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_darwin_amd64 $(MAIN_PATH)

build-darwin-arm64: ## Build for macOS (Apple Silicon)
	@echo "Building for macOS (Apple Silicon)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)_darwin_arm64 $(MAIN_PATH)

build-all: build-linux build-linux-arm64 build-windows build-darwin build-darwin-arm64 ## Build for all platforms
	@echo "All builds complete!"

release: clean deps test build-all ## Create release archives for all platforms
	@echo "Creating release archives..."
	@mkdir -p $(DIST_DIR)
	cd $(BUILD_DIR) && tar -czf ../$(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz $(BINARY_UNIX)
	cd $(BUILD_DIR) && tar -czf ../$(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_arm64.tar.gz $(BINARY_NAME)_linux_arm64
	cd $(BUILD_DIR) && zip -q ../$(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_windows_amd64.zip $(BINARY_WINDOWS)
	cd $(BUILD_DIR) && tar -czf ../$(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz $(BINARY_NAME)_darwin_amd64
	cd $(BUILD_DIR) && tar -czf ../$(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz $(BINARY_NAME)_darwin_arm64
	@echo "Release archives created in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/

install: build ## Install binary to $GOPATH/bin or $GOBIN
	@echo "Installing $(BINARY_NAME)..."
	@mkdir -p "$(shell go env GOPATH)/bin"
	@cp "$(BUILD_DIR)/$(BINARY_NAME)" "$(shell go env GOPATH)/bin/$(BINARY_NAME)"
	@echo "Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

uninstall: ## Remove installed binary
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f $(shell go env GOPATH)/bin/$(BINARY_NAME)
	@echo "Uninstalled"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_UNIX)
	@rm -f $(BINARY_WINDOWS)
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-short: ## Run tests in short mode
	$(GOTEST) -v -short ./...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: fmt vet ## Run formatters and linters
	@echo "Linting complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Dependencies downloaded"

tidy: ## Tidy and verify dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

run: build ## Build and run the application (requires command, e.g. ARGS='download -url ...')
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

run-example: build ## Run with example parameters
	@echo "Running example download..."
	@$(BUILD_DIR)/$(BINARY_NAME) download \
		-url "https://youtube.com/watch?v=EXAMPLE_VIDEO_ID" \
		-out ./downloads \
		-artist "Black Kids" \
		-album "Partie Traumatic" \
		-year "2008" \
		-genre "Indie Pop"

dev: ## Run directly with go run (for development)
	@echo "Running in dev mode..."
	$(GOCMD) run $(MAIN_PATH) $(ARGS)

check: fmt vet test ## Run all checks (format, vet, test)
	@echo "All checks passed!"

ci: deps lint test ## Run CI pipeline (deps, lint, test)
	@echo "CI pipeline complete!"

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Go version: $(shell go version)"
	@echo "Build dir: $(BUILD_DIR)"

.DEFAULT_GOAL := help
