# Python Plan — Python 3 + Python 2

Worktree: `pulserpc-fix-python` · Branch: `fix-invariants-python`
Owns: `pkg/generator/python_client_server.go`, `pkg/generator/python_*_test.go`,
`pkg/runtime/runtimes/python/**`, `pkg/runtime/runtimes/python2/**`,
`docs/languages/python/**`.

Read [README.md](README.md) for D1–D16. Python 2 is a deliberately
feature-reduced legacy runtime; apply the safety/contract fixes but do not
force full Python 3 parity (verification/diff remains a documented carve-out).

## P0 — fix

### R15 — no internal detail leakage (both runtimes)
- `pkg/runtime/runtimes/python/pulserpc/server.py` and
  `python2/pulserpc/server.py`: replace `f"Handler exception: {e}"` /
  `"Handler exception: %s" % e` with the fixed `"Internal error"` message and
  no `data`; log the exception with `logging`/stderr server-side.
- Keep `-32602` validation messages and declared `RPCError` data.
- Add tests with a throwing handler; assert the wire response has no exception
  text.

### R20 — request-id increments are atomic
- Wrap `_request_id += 1` in a `threading.Lock` in both clients (or use
  `itertools.count` with a lock). Add a threaded test asserting unique ids.

### R21 — resolution-index cache is thread-safe
- `pkg/runtime/runtimes/python/pulserpc/rpctypes.py`: guard
  `_index_cache` get/put/move_to_end/popitem with a `threading.Lock`.
- Add a threaded test that validates concurrently and asserts no
  `RuntimeError`/corruption.

### R22 — non-POST shape
- Generated Python 3 test server: add `do_GET`/`do_PUT`/`do_DELETE` (or a
  generic method check) returning HTTP **405** with a JSON-RPC `-32600` body
  (`id:null`). Apply the same to the Python 2 generated server.
- Add a test.

### R24 / R25 — id type and int range
- Servers reject boolean/object/array ids with `-32600`.
- `validate_int` (both `validation.py` files): reject integral values outside
  signed 32-bit, in addition to bool/NaN/Inf/fractional. Update tests.

### R16 / R18 — generated server limits (Python 2 gaps)
- Python 3 already caps bodies at 1 MiB and batches at 100. Add the same body
  cap to the Python 2 generated server; add a batch cap if the Python 2 server
  supports batches (otherwise document the carve-out).
- Add generator tests.

### M19 / M20 — batches and params
- Confirm non-object batch entries produce a per-entry `-32600`; add tests.
- Object-form params: the Python client converts named→positional; the server
  rejects non-list params with `-32602`. Add tests asserting this explicit
  behavior (no silent drop).

## P1 — verify and test

- C1: sort `namespaceMap` iterations that affect emitted content.
- C4/C5: `<ns>/idl.json` includes `rootNamespace`/`checksum`; the served
  `pulserpc-idl` result is an object.
- R17: cap response bodies at 10 MiB in `HttpTransport` (both runtimes) and
  reject with a clear error.
- R19: keep the 30 s timeout; add tests.
- R29/R30: custom headers work; normalize a trailing `/` on the base URL.
- R31: Python 3 `verify_compatibility` fetches the server IDL at bootstrap
  (already); add a test with a divergent IDL. Python 2: document the
  verification/diff carve-out.
- C11/R26/R27: enums are strings; float is double; add tests.
- M18/M21–M28: empty batch, `id:null`, `error.data` nil, null result, numeric
  ids, error shape, `-32700`, `jsonrpc`, batch notifications.
- Update `docs/languages/python/python2.md` with the legacy carve-outs and
  D1/D3/D4/D5/D7.

## Test commands

```powershell
$env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
python -m pytest pkg/runtime/runtimes/python/tests pkg/runtime/runtimes/python/pulserpc/tests -q
docker run --rm -v "C:\Users\james\src\pulserpc:/workspace" -w /workspace/pkg/runtime/runtimes/python2 python:2.7-slim sh -c "PYTHONPATH=/workspace/pkg/runtime/runtimes/python2 python tests/run_tests.py"
```

Generated e2e: build the `pulserpc` binary (stub `pkg/webui/dist` if needed),
generate `examples/conform.pulse` with `-generate-test-files`, start the
generated Python test server, run the generated client; remove any stub.

## Acceptance

- Python 3 pytest 136+ passes; Python 2 Docker suite passes; generated Python
  e2e passes.
- Every P0 behavior has a test that fails before the change.
- No edits outside the Python generator/runtime/docs.
