# PulseRPC Python Runtime

This directory contains the Python runtime library for PulseRPC-generated code.

## Structure

- `pulserpc/` - Main runtime library package
  - `__init__.py` - Package exports
  - `rpc.py` - RPC error handling
  - `validation.py` - Type validation functions
  - `types.py` - Type helper functions
- `tests/` - Unit tests

## Installation

For development:
```bash
make install
```

Or:
```bash
pip install -e .
```

## Testing

Run tests locally (requires Python 3.7+):
```bash
make test
```

Run tests in Docker (no local Python required):
```bash
make test-docker
```

## Usage

Generated code imports from this library:
```python
from pulserpc import RPCError, validate_type
from pulserpc import ALL_STRUCTS, ALL_ENUMS
```

The runtime library provides:
- `RPCError` - Exception class for JSON-RPC errors
- `Contract` - Class for parsing IDL and validating data
  - `Contract.validate(type_name, value) -> ValidationResult` — manually validate against any named type
  - `Contract.from_file(path) -> Contract` — load IDL from JSON file
- `ValidationResult` - Result type with `valid`, `error`, `invalid_fields` fields
- `ValidationError` - Individual error with `path` and `message` fields
- `validate_type()` - Main validation function, returns `List[ValidationError]`
- `validate_struct()`, `validate_enum()`, etc. - Specific validators, return `List[ValidationError]`
- Helper functions for working with type definitions

**Note:** The runtime library is automatically bundled into the output directory when code is generated, so no separate installation is required.

