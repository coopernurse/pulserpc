.PHONY: build build-linux test cover lint quality quality-full clean install-tools test-runtime-python test-runtime-python2 test-runtime-ts test-runtime-ts-cjs test-runtime-csharp test-runtime-java test-runtime-go test-runtimes test-all-runtimes test-integration test-integration-java test-integration-scripts test-numeric-types test-enum-case test-raises-propagation test-static-client-ts test-java-compile test-errors test-limits test-generator-python test-generator-ts test-generator-csharp test-generator-java test-generator-go test-generators build-webui lint-webui test-webui start-test-servers stop-test-servers status-test-servers docs-build docs-serve docs-clean test-quickstarts test-quickstart-go test-quickstart-python test-quickstart-python2 test-quickstart-java test-quickstart-ts test-quickstart-csharp test-quickstart-csharp-docker test-openapi

# Variables
BINARY_NAME=pulserpc
TARGET_DIR=target
BINARY_PATH=$(TARGET_DIR)/$(BINARY_NAME)
BINARY_PATH_LINUX=$(TARGET_DIR)/pulserpc-amd64
COVERAGE_FILE=$(TARGET_DIR)/coverage.out
COVERAGE_HTML=$(TARGET_DIR)/coverage.html

# Default target
.DEFAULT_GOAL := build

# Build the web UI
build-webui:
	@echo "Building web UI..."
	@cd pkg/webui && npm install && npm run build

# Build the binary
build: build-webui
	go build -o $(BINARY_PATH) ./cmd/pulse
	@echo "Built successfully at $(BINARY_PATH)"
	@echo "Building Linux binary for Docker containers..."
	@mkdir -p $(TARGET_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_PATH_LINUX) ./cmd/pulse
	@echo "Built Linux binary successfully at $(BINARY_PATH_LINUX)"

# Build Linux binary for Docker containers (cross-compile) - only if it doesn't exist
build-linux:
	@if [ -f $(BINARY_PATH_LINUX) ]; then \
		echo "Linux binary already exists at $(BINARY_PATH_LINUX), skipping build"; \
	else \
		$(MAKE) build-webui; \
		echo "Building Linux binary for Docker containers..."; \
		mkdir -p $(TARGET_DIR); \
		GOOS=linux GOARCH=amd64 go build -o $(BINARY_PATH_LINUX) ./cmd/pulse; \
		echo "Built Linux binary successfully at $(BINARY_PATH_LINUX)"; \
	fi

# Run tests
test:
	@echo "Running tests..."
	go test -v ./cmd/... ./pkg/generator/... ./pkg/openapi/... ./pkg/parser/... ./pkg/runtime/

# Run tests with coverage
cover:
	@echo "Running tests with coverage..."
	@mkdir -p $(TARGET_DIR)
	go test -v -coverprofile=$(COVERAGE_FILE) ./cmd/... ./pkg/generator/... ./pkg/openapi/... ./pkg/parser/... ./pkg/runtime/
	go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated at $(COVERAGE_HTML)"
	@go tool cover -func=$(COVERAGE_FILE) | tail -1

# Run linter
lint: install-tools
	@echo "Running linter..."
	@GOPATH=$$(go env GOPATH); \
	if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --enable=unparam ./cmd/... ./pkg/generator/... ./pkg/parser/... ./pkg/webui/...; \
	elif [ -f "$$GOPATH/bin/golangci-lint" ]; then \
		$$GOPATH/bin/golangci-lint run --enable=unparam ./cmd/... ./pkg/generator/... ./pkg/parser/... ./pkg/webui/...; \
	else \
		echo "Error: golangci-lint not found. Run 'make install-tools' first."; \
		exit 1; \
	fi

# Run linter for webui
lint-webui:
	@echo "Running webui linter..."
	@cd pkg/webui && npm run lint

# Run tests for webui
test-webui:
	@echo "Running webui tests..."
	@cd pkg/webui && npm run test 2>/dev/null || echo "No test script configured for webui"

# Run quality checks (lint + test + webui lint + webui test)
quality: lint test lint-webui test-webui
	@echo "Quality checks completed"

# Run quality checks plus slower generator, quickstart, and cross-cutting
# integration scripts.
quality-full: build quality test-runtimes test-quickstarts test-generators test-integration-scripts test-openapi
	@echo "Full quality checks completed"

# Install required tools
install-tools:
	@echo "Checking for required tools..."
	@GOPATH=$$(go env GOPATH); \
	if [ ! -f "$$GOPATH/bin/golangci-lint" ]; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		echo "golangci-lint installed to $$GOPATH/bin/golangci-lint"; \
		echo "Make sure $$GOPATH/bin is in your PATH"; \
	else \
		echo "golangci-lint already installed"; \
	fi

# Test Python runtime
test-runtime-python:
	@echo "Testing Python runtime..."
	@cd pkg/runtime/runtimes/python && $(MAKE) test

# Test Python 2 runtime (delegates to the runtime Makefile, which uses a local
# Python 2.7 when available and a pinned python:2.7-slim image otherwise).
test-runtime-python2:
	@echo "Testing Python 2 runtime..."
	@cd pkg/runtime/runtimes/python2 && $(MAKE) test
	@echo "Python 2 runtime tests passed"

# Test TypeScript runtime (ESM)
test-runtime-ts:
	@echo "Testing TypeScript ESM runtime..."
	@cd pkg/runtime/runtimes/ts-node && $(MAKE) test

# Test TypeScript runtime (CJS)
test-runtime-ts-cjs:
	@echo "Testing TypeScript CJS runtime..."
	@cd pkg/runtime/runtimes/ts-cjs && $(MAKE) test

# Test C# runtime
test-runtime-csharp:
	@echo "Testing C# runtime..."
	@cd pkg/runtime/runtimes/csharp && $(MAKE) test

# Test Java runtime
test-runtime-java:
	@echo "Testing Java runtime..."
	@cd pkg/runtime/runtimes/java && $(MAKE) test

# Test Go runtime
test-runtime-go:
	@echo "Testing Go runtime..."
	@cd pkg/runtime/runtimes/go && $(MAKE) test

# Test all runtimes
test-runtimes: test-runtime-python test-runtime-python2 test-runtime-ts test-runtime-ts-cjs test-runtime-csharp test-runtime-java test-runtime-go
	@echo "All runtime tests passed"

# Run the multi-runtime HTTP API integration harness (all advertised servers)
test-all-runtimes:
	@echo "Running HTTP API integration tests against all runtimes..."
	@bash tests/integration/test_all_runtimes.sh

# Canonical integration target: protocol, limit, and matrix checks.
test-integration: test-all-runtimes

# The Java runtime's historical `test-integration` target referenced generated
# TestServer/TestClient classes that no longer exist. This is the supported
# replacement (the generated-server harness is owned by this repository root).
test-integration-java: test-generator-java

# Resource limits (D5) against a running server:
#   make test-limits SERVER_URL=http://localhost:9000
SERVER_URL ?= http://localhost:8080
test-limits:
	@echo "Testing resource limits against $(SERVER_URL)..."
	@SERVER_URL=$(SERVER_URL) bash tests/integration/test_limits.sh

# Orphaned harness scripts, now reachable from `make`.
test-numeric-types: build
	@bash tests/integration/test_numeric_types.sh

test-enum-case: build
	@bash tests/integration/test_enum_case.sh

test-raises-propagation: build
	@bash tests/integration/test_raises_propagation.sh

test-static-client-ts: build
	@bash tests/integration/test_static_client_ts.sh

test-java-compile: build
	@bash tests/integration/test_java_compile.sh

test-errors: build
	@bash tests/integration/test_errors.sh

test-integration-scripts: test-numeric-types test-enum-case test-raises-propagation test-static-client-ts test-java-compile test-errors
	@echo "All integration scripts passed"

# Test Python generator integration
test-generator-python:
	@echo "Testing Python generator integration..."
	@cd pkg/runtime/runtimes/python && $(MAKE) test-integration

# Test TypeScript generator integration
test-generator-ts:
	@echo "Testing TypeScript generator integration..."
	@cd pkg/runtime/runtimes/ts-node && $(MAKE) test-integration

# Test C# generator integration
test-generator-csharp:
	@echo "Testing C# generator integration..."
	@cd pkg/runtime/runtimes/csharp && $(MAKE) test-integration

# Test Java generator integration
test-generator-java:
	@echo "Testing Java generator integration..."
	@bash tests/integration/test_generator_java.sh

# Test Go generator integration
test-generator-go:
	@echo "Testing Go generator integration..."
	@cd pkg/runtime/runtimes/go && $(MAKE) test-integration

# Test all generators
test-generators: build test-generator-python test-generator-ts test-generator-csharp test-generator-java test-generator-go
	@echo "All generator tests passed"

# Start all test servers for web UI
start-test-servers:
	@./scripts/test-servers.sh start

# Stop all test servers
stop-test-servers:
	@./scripts/test-servers.sh stop

# Show status of test servers
status-test-servers:
	@./scripts/test-servers.sh status

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(TARGET_DIR)
	go clean ./...
	rm -rf pkg/webui/dist
	rm -rf pkg/webui/node_modules
	rm -rf docs/_site
	@echo "Cleaned build artifacts and documentation"

# Build documentation site (Jekyll)
docs-build:
	@echo "Building Jekyll documentation..."
	@cd docs && bundle install > /dev/null 2>&1 || bundle install
	@cd docs && SASS_SILENCE_DEPRECATIONS=darken bundle exec jekyll build
	@echo "Documentation built at docs/_site/"
	@echo "Preview with: make docs-serve"

# Serve documentation site locally (Jekyll)
docs-serve:
	@echo "Starting Jekyll dev server on http://localhost:4000"
	@echo "Press Ctrl+C to stop"
	@cd docs && bundle exec jekyll serve --host 0.0.0.0 --port 4000

# Clean documentation build artifacts
docs-clean:
	@echo "Cleaning documentation build..."
	rm -rf docs/_site
	@echo "Documentation cleaned"

# Quickstart testing targets
test-quickstarts: build
	@echo "Testing all quickstart guides..."
	@bash tests/integration/test_quickstart_all.sh

test-quickstart-go:
	@bash tests/integration/test_quickstart_go.sh

test-quickstart-python:
	@bash tests/integration/test_quickstart_python.sh

test-quickstart-python2:
	@bash tests/integration/test_quickstart_python2.sh

test-quickstart-java:
	@bash tests/integration/test_quickstart_java.sh

test-quickstart-ts:
	@bash tests/integration/test_quickstart_ts.sh

test-quickstart-csharp:
	@bash tests/integration/test_quickstart_csharp.sh

test-quickstart-csharp-docker:
	@USE_DOCKER=1 bash tests/integration/test_quickstart_csharp.sh

# OpenAPI translation tests
test-openapi: build
	@echo "Testing OpenAPI to Pulse conversion..."
	@bash tests/integration/test_openapi_to_pulse.sh
	@echo "Testing Pulse to OpenAPI conversion..."
	@bash tests/integration/test_pulse_to_openapi.sh
	@echo "Testing OpenAPI round-trip conversion..."
	@bash tests/integration/test_openapi_roundtrip.sh
	@echo "All OpenAPI integration tests passed"

