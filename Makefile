.PHONY: build test install clean run-tests dev-setup help

# Binary name and directories
BINARY_NAME=docker-cr
BUILD_DIR=./bin
DIST_DIR=./dist
SRC_DIR=./cmd
PKG_DIR=./pkg
TEST_DIR=./test

# Go configuration
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_TEST=$(GO_CMD) test
GO_GET=$(GO_CMD) get
GO_MOD=$(GO_CMD) mod

# Build flags
BUILD_FLAGS=-v -ldflags="-s -w"
DEBUG_FLAGS=-v -race
TEST_FLAGS=-v -race -cover

# Version information
VERSION?=1.0.0
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# LDFLAGS for version info
LDFLAGS=-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)

## build: Build the binary
build: deps $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(shell find $(SRC_DIR) $(PKG_DIR) -name "*.go")
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)/main.go

## build-debug: Build with debug symbols and race detection
build-debug: deps
	@echo "Building $(BINARY_NAME) with debug symbols..."
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) $(DEBUG_FLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(SRC_DIR)/main.go

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO_TEST) $(TEST_FLAGS) ./...

## test-unit: Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GO_TEST) $(TEST_FLAGS) ./pkg/...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO_TEST) -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## benchmark: Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	$(GO_TEST) -bench=. -benchmem ./...

## install: Install the binary to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete!"

## uninstall: Remove binary from system
uninstall:
	@echo "Removing $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GO_CLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO_CMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO_CMD) vet ./...

## lint: Run golint (requires golint to be installed)
lint:
	@echo "Running golint..."
	@which golint > /dev/null || (echo "golint not installed. Run: go install golang.org/x/lint/golint@latest" && exit 1)
	golint ./...

## check: Run fmt, vet, and lint
check: fmt vet lint

## dev-setup: Setup development environment
dev-setup:
	@echo "Setting up development environment..."
	$(GO_MOD) download
	@echo "Installing development tools..."
	$(GO_CMD) install golang.org/x/lint/golint@latest
	$(GO_CMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development environment ready!"

## run-tests: Run integration tests with Docker containers
run-tests: build
	@echo "Running integration tests..."
	@mkdir -p $(TEST_DIR)/checkpoints
	@echo "Starting test container..."
	docker run -d --name docker-cr-test alpine:latest sleep 300 || true
	@sleep 2
	@echo "Testing checkpoint..."
	sudo $(BUILD_DIR)/$(BINARY_NAME) checkpoint docker-cr-test --output $(TEST_DIR)/checkpoints --name test-checkpoint
	@echo "Testing restore..."
	docker stop docker-cr-test && docker rm docker-cr-test || true
	sudo $(BUILD_DIR)/$(BINARY_NAME) restore --from $(TEST_DIR)/checkpoints/docker-cr-test/test-checkpoint --new-name docker-cr-restored
	@echo "Testing inspect..."
	$(BUILD_DIR)/$(BINARY_NAME) inspect $(TEST_DIR)/checkpoints/docker-cr-test/test-checkpoint --summary
	@echo "Cleaning up..."
	docker stop docker-cr-restored && docker rm docker-cr-restored || true
	@echo "Integration tests completed!"

## quick-test: Quick test with minimal container
quick-test: build
	@echo "Running quick test..."
	@mkdir -p $(TEST_DIR)/quick-checkpoints
	docker run -d --name quick-test busybox:latest sleep 60 || true
	@sleep 1
	sudo $(BUILD_DIR)/$(BINARY_NAME) checkpoint quick-test --output $(TEST_DIR)/quick-checkpoints --leave-running=false
	docker stop quick-test && docker rm quick-test || true
	@echo "Quick test completed! Check $(TEST_DIR)/quick-checkpoints/"

## dist: Create distribution packages
dist: clean build
	@echo "Creating distribution packages..."
	@mkdir -p $(DIST_DIR)
	@tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	@echo "Distribution package created: $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz"

## release: Build release version
release: clean
	@echo "Building release version $(VERSION)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) -a -installsuffix cgo $(BUILD_FLAGS) -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC_DIR)/main.go
	@echo "Release build completed: $(DIST_DIR)/$(BINARY_NAME)-linux-amd64"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t docker-cr:$(VERSION) .
	docker tag docker-cr:$(VERSION) docker-cr:latest

## check-deps: Check if required tools are installed
check-deps:
	@echo "Checking dependencies..."
	@which criu > /dev/null || (echo "ERROR: CRIU not found. Please install CRIU first." && exit 1)
	@which docker > /dev/null || (echo "ERROR: Docker not found. Please install Docker first." && exit 1)
	@echo "All dependencies satisfied!"

## demo: Run a complete demo
demo: build check-deps
	@echo "Running complete demo..."
	@echo "1. Creating test container..."
	docker run -d --name demo-container nginx:alpine
	@sleep 2
	@echo "2. Checkpointing container..."
	sudo $(BUILD_DIR)/$(BINARY_NAME) checkpoint demo-container --output ./demo-checkpoints
	@echo "3. Inspecting checkpoint..."
	$(BUILD_DIR)/$(BINARY_NAME) inspect ./demo-checkpoints/demo-container/checkpoint --summary
	@echo "4. Stopping original container..."
	docker stop demo-container && docker rm demo-container
	@echo "5. Restoring container..."
	sudo $(BUILD_DIR)/$(BINARY_NAME) restore --from ./demo-checkpoints/demo-container/checkpoint --new-name demo-restored
	@echo "6. Verifying restored container..."
	docker ps | grep demo-restored || true
	@echo "Demo completed! Clean up with: docker stop demo-restored && docker rm demo-restored"

## help: Show this help message
help:
	@echo "Docker-CR Build System"
	@echo "====================="
	@echo
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo
	@echo "Examples:"
	@echo "  make build          # Build the binary"
	@echo "  make test           # Run all tests"
	@echo "  make install        # Install to system"
	@echo "  make demo           # Run complete demo"
	@echo "  make quick-test     # Quick functionality test"

# Default target
.DEFAULT_GOAL := help