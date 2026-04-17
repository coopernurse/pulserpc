## Why

PulseRPC has a Python 2.7 runtime and generator target, but lacks documentation for users who need to write PulseRPC servers on legacy Python 2 systems. The existing Python docs cover Python 3 only.

## What Changes

- New documentation page `docs/languages/python/python2.md` - a mini-quickstart for Python 2 server implementation
- New Jekyll include `docs/_includes/quickstart/python2-server.md` with the server example code
- New quickstart test `tests/integration/test_quickstart_python2.sh` that runs the Python 2 server/client example in Docker
- Navigation update to add Python 2 page under Language Guides → Python

## Capabilities

### New Capabilities

- `python2-server-docs`: Document how to use the Python 2 runtime to implement PulseRPC servers. Covers CLI generation for Python 2 target, runtime architecture, handler pattern, and integration with Python stdlib `BaseHTTPServer`.

### Modified Capabilities

- `python-quickstart`: The existing Python quickstart uses Python 3 syntax and features. The new Python 2 page provides parallel coverage for legacy systems without modifying Python 3 requirements.

## Impact

- **New files**:
  - `docs/languages/python/python2.md` - documentation page
  - `docs/_includes/quickstart/python2-server.md` - server example include
  - `examples/quickstart/python2/` - example server/client source (if extracted for testing)
  - `tests/integration/test_quickstart_python2.sh` - integration test

- **Modified files**:
  - `docs/_data/navigation.yml` - add Python 2 page to navigation

- **Dependencies**:
  - Uses existing `checkout.pulse` IDL for the example
  - Uses existing `moxel/python2` Docker image for testing
  - Runtime already exists (embedded in CLI binary)
