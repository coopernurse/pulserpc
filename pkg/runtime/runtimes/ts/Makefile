.PHONY: test test-docker test-integration clean

# Variables
TS_IMAGE=node:18-slim
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Test using local Node.js if available, otherwise Docker
test:
ifeq ($(DOCKER_AVAILABLE),yes)
	@echo "Using Docker for testing..."
	@$(MAKE) test-docker
else
	@echo "Using local Node.js for testing..."
	@node --version >/dev/null 2>&1 || (echo "Error: Node.js not found. Please install Node.js 18+ or use Docker." && exit 1)
	@npx tsx pulserpc/tests/test_rpc.ts && npx tsx pulserpc/tests/test_types.ts && npx tsx pulserpc/tests/test_validation.ts || (echo "Error: TypeScript runtime tests failed. Using Docker..." && $(MAKE) test-docker)
endif

# Test using Docker
test-docker:
	@echo "Testing TypeScript runtime in Docker..."
	@docker run --rm -v $(PWD):/workspace -w /workspace \
		$(TS_IMAGE) \
		/bin/bash -c "npm install -g tsx typescript @types/node >/dev/null 2>&1 && cd pulserpc/tests && tsx test_rpc.ts && tsx test_types.ts && tsx test_validation.ts"

# Test generator integration (requires Docker)
test-integration:
	@echo "Testing TypeScript generator integration..."
	@DOCKER_AVAILABLE=$$(command -v docker >/dev/null 2>&1 && echo "yes" || echo "no"); \
	if [ "$$DOCKER_AVAILABLE" != "yes" ]; then \
		echo "Error: Docker is required for integration tests"; \
		exit 1; \
	fi
	@cd ../../../.. && $(MAKE) build-linux && docker run --rm \
		-v $$(pwd):/workspace \
		-w /workspace \
		node:18-slim \
		/bin/bash -c "apt-get update -qq && apt-get install -y -qq curl >/dev/null 2>&1 && bash tests/integration/test_generator_ts.sh"

# Clean build artifacts
clean:
	find . -type f -name "*.js" -not -path "./node_modules/*" -delete 2>/dev/null || true
	find . -type f -name "*.js.map" -delete 2>/dev/null || true
	find . -type d -name "node_modules" -exec rm -r {} + 2>/dev/null || true

