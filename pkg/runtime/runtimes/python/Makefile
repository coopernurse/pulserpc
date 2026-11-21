.PHONY: test test-docker test-integration install clean

# Variables
PYTHON_IMAGE=python:3.11-slim
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Test using local Python if available, otherwise Docker
test:
ifeq ($(DOCKER_AVAILABLE),yes)
	@echo "Using Docker for testing..."
	@$(MAKE) test-docker
else
	@echo "Using local Python for testing..."
	@python3 -m pytest tests/ pulserpc/tests/ -v
endif

# Test using Docker
test-docker:
	@echo "Testing Python runtime in Docker..."
	@docker run --rm -v $(PWD):/workspace -w /workspace \
		$(PYTHON_IMAGE) \
		/bin/bash -c "pip install -q pytest && python -m pytest tests/ pulserpc/tests/ -v"

# Test generator integration (requires Docker)
test-integration:
	@echo "Testing Python generator integration..."
	@DOCKER_AVAILABLE=$$(command -v docker >/dev/null 2>&1 && echo "yes" || echo "no"); \
	if [ "$$DOCKER_AVAILABLE" != "yes" ]; then \
		echo "Error: Docker is required for integration tests"; \
		exit 1; \
	fi
	@cd ../../../.. && $(MAKE) build-linux && docker run --rm \
		-v $$(pwd):/workspace \
		-w /workspace \
		-e GOTOOLCHAIN=auto \
		golang:1.23 \
		/bin/bash -c "apt-get update -qq && apt-get install -y -qq python3 curl >/dev/null 2>&1 && bash tests/integration/test_generator.sh"

# Install runtime in development mode
install:
	pip install -e .

# Clean Python cache files
clean:
	find . -type d -name __pycache__ -exec rm -r {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete
	find . -type d -name "*.egg-info" -exec rm -r {} + 2>/dev/null || true


