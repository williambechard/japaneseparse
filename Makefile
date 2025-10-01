.PHONY: build test clean dev fmt lint help deps run

# Variables
BINARY_NAME=japanese-parser
MAIN_PATH=./cmd/parser
BUILD_DIR=./bin
CONFIG_FILE=configs/config.example.yaml

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-w -s"
BUILD_FLAGS=-v

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Development build with race detection and debug symbols
dev-build:
	@echo "Building $(BINARY_NAME) for development..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -race -v -o $(BUILD_DIR)/$(BINARY_NAME)-dev $(MAIN_PATH)
	@echo "Development build complete: $(BUILD_DIR)/$(BINARY_NAME)-dev"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		echo "Running goimports..."; \
		goimports -w .; \
	fi

# Lint code (requires golangci-lint)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install development tools
dev-deps:
	@echo "Installing development dependencies..."
	$(GOGET) golang.org/x/tools/cmd/goimports@latest
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run with default configuration
run: build
	@echo "Running $(BINARY_NAME) with default text..."
	./$(BUILD_DIR)/$(BINARY_NAME) -text "こんにちは世界"

# Run with custom text (using temporary file to avoid encoding issues)
run-text: build
	@if [ -z "$(TEXT)" ]; then \
		echo "Usage: make run-text TEXT='your japanese text here'"; \
		echo "Note: On Windows, use file input to avoid encoding issues"; \
		exit 1; \
	fi
	@echo "$(TEXT)" > .tmp_input.txt
	@./$(BUILD_DIR)/$(BINARY_NAME) -file .tmp_input.txt
	@rm -f .tmp_input.txt

# Run with text from file (recommended for Japanese input)
run-file: build
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make run-file FILE='path/to/file.txt'"; \
		echo "Example: echo 'こんにちは世界' > input.txt && make run-file FILE=input.txt"; \
		exit 1; \
	fi
	./$(BUILD_DIR)/$(BINARY_NAME) -file "$(FILE)"

# Run with configuration file
run-config: build
	@if [ ! -f "$(CONFIG_FILE)" ]; then \
		echo "Config file not found: $(CONFIG_FILE)"; \
		echo "Copy configs/config.example.yaml to $(CONFIG_FILE) and customize it"; \
		exit 1; \
	fi
	./$(BUILD_DIR)/$(BINARY_NAME) -config "$(CONFIG_FILE)" -text "秋田県仙北市の例文です"

# Run development version
dev: dev-build
	./$(BUILD_DIR)/$(BINARY_NAME)-dev -text "こんにちは世界" -verbose

# Install the binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Create a release build for multiple platforms
release:
	@echo "Building release versions..."
	@mkdir -p $(BUILD_DIR)/release
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	@echo "Release builds complete in $(BUILD_DIR)/release/"

# Setup project (copy example configs, create directories)
setup:
	@echo "Setting up project..."
	@mkdir -p logs dict
	@if [ ! -f "config.yaml" ] && [ -f "configs/config.example.yaml" ]; then \
		cp configs/config.example.yaml config.yaml; \
		echo "Created config.yaml from example"; \
	fi
	@if [ ! -f ".env" ] && [ -f ".env.example" ]; then \
		cp .env.example .env; \
		echo "Created .env from example"; \
	fi
	@echo "Setup complete. Please download dictionary files as described in docs/SETUP.md"

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Security audit
audit:
	@if command -v gosec >/dev/null 2>&1; then \
		echo "Running security audit..."; \
		gosec ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Help target
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  dev-build    - Build with race detection for development"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter (requires golangci-lint)"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  dev-deps     - Install development tools"
	@echo "  run          - Build and run with default text"
	@echo "  run-text     - Build and run with custom text (use TEXT='...')"
	@echo "  run-file     - Build and run with text from file (use FILE='...')"
	@echo "  run-config   - Build and run with configuration file"
	@echo "  dev          - Build and run development version"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  release      - Build for multiple platforms"
	@echo "  setup        - Initial project setup"
	@echo "  bench        - Run benchmark tests"
	@echo "  audit        - Run security audit (requires gosec)"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make run-text TEXT='日本語のテストです'  # May have encoding issues on Windows"
	@echo "  echo '私は学校に行きます' > input.txt && make run-file FILE=input.txt  # Recommended"
	@echo "  make test-coverage"
	@echo "  make release"
