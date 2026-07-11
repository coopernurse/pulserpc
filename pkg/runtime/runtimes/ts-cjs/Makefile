.PHONY: test test-docker test-integration clean install

# Variables
TS_IMAGE=node:18-slim
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Install dev dependencies (tsx, typescript, @types/node).
# Idempotent — skips if node_modules already populated.
install:
	@if [ ! -d node_modules/tsx ]; then \
		echo "Installing ts-cjs runtime devDependencies..."; \
		npm install --no-audit --no-fund --silent; \
	else \
		echo "node_modules already populated; skipping npm install"; \
	fi

# Test using local Node.js if available, otherwise Docker.
# Uses `tsx` so that the .ts runtime files (which use module.exports / require)
# and the plain .js tests can be exercised together.
test: install
	@echo "Using local Node.js for testing..."
	@node --version >/dev/null 2>&1 || (echo "Error: Node.js not found. Please install Node.js 18+ or use Docker." && exit 1)
	@cd pulserpc && npx tsx tests/test_rpc.js
	@cd pulserpc && npx tsx tests/test_types.js
	@cd pulserpc && npx tsx tests/test_validation.js
	@cd pulserpc && npx tsx tests/test_contract.js
	@cd pulserpc && npx tsx tests/test_client.js
	@cd pulserpc && npx tsx tests/test_transport.js
	@cd pulserpc && npx tsx tests/test_shared.ts

# Test using Docker
test-docker:
	@echo "Testing TypeScript CJS runtime in Docker..."
	@docker run --rm -v $(PWD):/workspace -w /workspace \
		$(TS_IMAGE) \
		/bin/bash -c "npm install -g tsx typescript @types/node >/dev/null 2>&1 && cd pulserpc && tsx tests/test_rpc.js && tsx tests/test_types.js && tsx tests/test_validation.js && tsx tests/test_contract.js && tsx tests/test_client.js && tsx tests/test_transport.js && tsx tests/test_shared.ts"

# Test generator integration (requires Docker)
test-integration:
	@echo "Testing TypeScript CJS generator integration..."
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
