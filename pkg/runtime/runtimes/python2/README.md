# PulseRPC Python 2.7 Runtime

Runtime library copied into code generated with
`pulserpc -plugin python-client-server -python-version 2`.

This is an actively maintained, deliberately feature-reduced legacy runtime.
See the [Python 2 support policy](../../../../docs/languages/python/python2.md)
for the full list of intentionally waived capabilities.

## Layout

```
python2/
├── pulserpc/        # Runtime package (embedded into generated output)
│   ├── __init__.py
│   ├── rpc.py
│   ├── validation.py
│   ├── rpctypes.py
│   ├── contract.py
│   ├── server.py
│   ├── client.py
│   ├── transport.py
│   └── test_validation.py
├── tests/           # Unit tests (not embedded into generated output)
└── Makefile         # test / test-docker / clean
```

## Running the tests

```bash
cd pkg/runtime/runtimes/python2
make test          # uses a local Python 2.7, or Docker when none is installed
make test-docker   # always uses the pinned python:2.7-slim image
make clean
```

`tests/run_tests.py` runs every module with plain asserts and needs no
third-party packages. It can also be invoked directly:

```bash
PYTHONPATH=. python tests/run_tests.py
```

The root target `make test-runtime-python2` delegates here.

## API surface

The package exports 19 symbols: `RPCError`, the nine `validate_*` helpers,
the three type helpers (`find_struct`, `find_enum`, `get_struct_fields`),
`Server`, `Client`, `Contract`, `Transport`, `HttpTransport`, and
`InProcTransport`. Contract verification (diff/auditors/checksum),
`ValidationResult`/`ValidationError`, and generated stubs are intentionally
absent; `tests/test_api_surface.py` guards against drift.

## Notes

- Handlers receive `ctx` as their first positional argument.
- Contract validation failures raise `RPCError` with code `-32602` (request)
  or `-32603` (response) and structured `data`.
- Notifications (requests without `id`) never receive a response.
