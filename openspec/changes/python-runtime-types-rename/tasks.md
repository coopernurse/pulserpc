## 1. Python 2.7 Runtime Source Changes

- [ ] 1.1 Rename `pkg/runtime/runtimes/python2/pulserpc/types.py` to `rpctypes.py`
- [ ] 1.2 Update `pkg/runtime/runtimes/python2/pulserpc/__init__.py` import: `from types import` → `from rpctypes import`
- [ ] 1.3 Update `pkg/runtime/runtimes/python2/pulserpc/validation.py` import: `from types import` → `from rpctypes import`
- [ ] 1.4 Update `pkg/runtime/runtimes/python2/pulserpc/test_validation.py` import: `from types import` → `from rpctypes import`

## 2. Python 3 Runtime Source Changes

- [ ] 2.1 Rename `pkg/runtime/runtimes/python/pulserpc/types.py` to `rpctypes.py`
- [ ] 2.2 Update `pkg/runtime/runtimes/python/pulserpc/__init__.py` import: `from .types import` → `from .rpctypes import`
- [ ] 2.3 Update `pkg/runtime/runtimes/python/pulserpc/client.py` import: `from .types import` → `from .rpctypes import`
- [ ] 2.4 Update `pkg/runtime/runtimes/python/pulserpc/contract.py` import: `from .types import` → `from .rpctypes import`
- [ ] 2.5 Update `pkg/runtime/runtimes/python/pulserpc/diff.py` import: `from .types import` → `from .rpctypes import`
- [ ] 2.6 Update `pkg/runtime/runtimes/python/pulserpc/validation.py` import: `from .types import` → `from .rpctypes import`
- [ ] 2.7 Update `pkg/runtime/runtimes/python/pulserpc/tests/test_diff.py` import: `from pulserpc.types import` → `from pulserpc.rpctypes import`

## 3. Examples Quickstart Changes

- [ ] 3.1 Rename `examples/quickstart/python/pulserpc/types.py` to `rpctypes.py`
- [ ] 3.2 Update `examples/quickstart/python/pulserpc/__init__.py` import: `from .types import` → `from .rpctypes import`
- [ ] 3.3 Update `examples/quickstart/python/pulserpc/validation.py` import: `from .types import` → `from .rpctypes import`

## 4. Go Test File Updates

- [ ] 4.1 Update `pkg/generator/python_client_server_test.go` line 697: `types.py` → `rpctypes.py` in runtimeFiles list
- [ ] 4.2 Update `pkg/generator/python_namespace_paths_test.go` references to `types.py` → `rpctypes.py`

## 5. Verification

- [ ] 5.1 Run `make test-runtime-python2` to verify Python 2.7 runtime tests pass
- [ ] 5.2 Run `make test-quickstart-python2` to verify Python 2.7 quickstart test passes
- [ ] 5.3 Run `make test-quickstart-python` to verify Python 3 quickstart test passes
- [ ] 5.4 Rebuild CLI binary: `make build`
