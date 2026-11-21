.PHONY: test test-docker test-integration clean

# Variables
GO_IMAGE=golang:1.21-alpine
# The repository go.mod requires Go 1.25; use a matching image for unit tests.
TEST_GO_IMAGE=golang:1.25-alpine
REPO_ROOT=$(abspath ../../../..)
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Test using local Go if available, otherwise Docker. Both test packages are
# selected: the standalone `tests` module and the `pulserpc` package with the
# contract/diff/auditor/verification tests (audit go-11).
test:
ifeq ($(DOCKER_AVAILABLE),yes)
	@echo "Using Docker for testing..."
	@$(MAKE) test-docker
else
	@echo "Using local Go for testing..."
	@cd $(REPO_ROOT) && go test ./pkg/runtime/runtimes/go/... -count=1
endif

# Test using Docker
test-docker:
	@echo "Testing Go runtime in Docker..."
	@docker run --rm -v $(REPO_ROOT):/workspace -w /workspace \
		$(TEST_GO_IMAGE) \
		sh -c "go test ./pkg/runtime/runtimes/go/... -count=1"

# Test generator integration (requires Docker)
test-integration:
	@echo "Testing Go generator integration..."
	@DOCKER_AVAILABLE=$$(command -v docker >/dev/null 2>&1 && echo "yes" || echo "no"); \
	if [ "$$DOCKER_AVAILABLE" != "yes" ]; then \
		echo "Error: Docker is required for integration tests"; \
		exit 1; \
	fi
	@cd ../../../.. && $(MAKE) build-linux && docker run --rm \
		-v $$(pwd):/workspace \
		-w /workspace \
		-e GOTOOLCHAIN=auto \
		$(GO_IMAGE) \
		sh -c "apk add --no-cache bash curl && bash tests/integration/test_generator_go.sh"

# Clean build artifacts
clean:
	go clean ./...
	find . -name "*.test" -delete 2>/dev/null || true
