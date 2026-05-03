## ADDED Requirements

### Requirement: Python runtime types.py renamed to rpctypes.py

The Python runtime library SHALL use `rpctypes.py` as the module filename instead of `types.py` to avoid conflicts with Python's built-in `types` module in Python 2.7.

#### Scenario: Python 2.7 runtime imports work correctly
- **WHEN** Python 2.7 code does `from pulserpc import find_struct`
- **THEN** the function resolves correctly from the renamed `rpctypes.py` module

#### Scenario: Python 3 runtime imports work correctly
- **WHEN** Python 3 code does `from pulserpc import find_struct`
- **THEN** the function resolves correctly from the renamed `rpctypes.py` module

#### Scenario: Generated Python 2.7 projects use renamed runtime
- **WHEN** a user generates Python 2.7 code with `pulserpc -plugin python-client-server -python-version 2`
- **THEN** the generated `pulserpc/` directory contains `rpctypes.py` instead of `types.py`

#### Scenario: Generated Python 3 projects use renamed runtime
- **WHEN** a user generates Python 3 code with `pulserpc -plugin python-client-server`
- **THEN** the generated `pulserpc/` directory contains `rpctypes.py` instead of `types.py`
