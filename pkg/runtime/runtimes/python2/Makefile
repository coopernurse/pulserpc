.PHONY: test test-docker clean

# Python 2 is EOL, so use the official, version-pinned image for container runs.
PYTHON2_IMAGE=python:2.7-slim

# Prefer a local Python 2.7 when present; otherwise fall back to Docker.
LOCAL_PYTHON := $(shell command -v python2 >/dev/null 2>&1 && echo python2 || (command -v python >/dev/null 2>&1 && python -c 'import sys; sys.exit(0 if sys.version_info[0] == 2 else 1)' >/dev/null 2>&1 && echo python))
DOCKER_AVAILABLE := $(shell command -v docker >/dev/null 2>&1 && echo "yes" || echo "no")

# Bind-mount friendly paths (Windows/Git Bash first, POSIX fallback).
RUNTIME_DIR := $(shell pwd -W 2>/dev/null || pwd)
REPO_ROOT := $(shell cd ../../../.. && (pwd -W 2>/dev/null || pwd))

test:
ifeq ($(LOCAL_PYTHON),)
ifeq ($(DOCKER_AVAILABLE),yes)
	@echo "No local Python 2.7; using Docker..."
	@$(MAKE) test-docker
else
	@echo "ERROR: Python 2.7 or Docker is required to run the Python 2 runtime tests." >&2
	@exit 1
endif
else
	@echo "Using local Python 2 ($(LOCAL_PYTHON))..."
	@PYTHONPATH=$(RUNTIME_DIR) $(LOCAL_PYTHON) tests/run_tests.py
endif

# Run the full suite in a pinned official Python 2.7 image.
test-docker:
	@echo "Testing Python 2 runtime in Docker ($(PYTHON2_IMAGE))..."
	@docker run --rm \
		-v "$(REPO_ROOT):/workspace" \
		-w /workspace/pkg/runtime/runtimes/python2 \
		$(PYTHON2_IMAGE) \
		sh -c "PYTHONPATH=/workspace/pkg/runtime/runtimes/python2 python tests/run_tests.py"

clean:
	@echo "Cleaning Python 2 bytecode..."
	@find . -name "*.py[co]" -delete 2>/dev/null || true
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
