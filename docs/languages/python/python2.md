---
title: Python 2 Server Guide
layout: default
---

# Python 2 Server Guide

Build a PulseRPC server in Python 2.7 using the standard library.

## Python 2 support policy

Python 2 is **actively maintained for legacy systems**, not frozen, and **not
expected to reach feature parity with the Python 3 runtime**. Python 2 reached
end of life in 2020, but legacy deployments persist, so this runtime continues
to receive bug fixes and targeted features. The canonical
[RUNTIME_IMPLEMENTATION_GUIDE.md](../../RUNTIME_IMPLEMENTATION_GUIDE.md)
checklist applies to Python 2 except for the capabilities explicitly waived
below.

### Intentionally waived capabilities

- **Contract verification (diff / auditors / checksum)** — Python 2 has no diff
  engine, no auditors, and no checksum helper. The following Python 3 symbols
  are intentionally absent from `pulserpc.__all__`: `extract_checksum`,
  `diff_idl`, `EntityType`, `ChangeType`, `Direction`, `Severity`,
  `ContractDelta`, `VerificationResult`, `ContractAuditor`, `NoOpAuditor`,
  `LoggingAuditor`, and `FailFastAuditor`. A client therefore cannot perform the
  fetch-`pulserpc-idl`-and-diff compatibility check described in R31/D15; this
  remains a documented carve-out.
- **Structured validation classes** — `ValidationResult` and `ValidationError`
  are Python 3 only. Contract-level validation failures instead raise
  `RPCError` with code `-32602` (request) or `-32603` (response) and a
  plain-dict `data` payload (`{"error": ..., "invalid_fields": [...]}`).
- **Batch requests** — the Python 2 `Server.call` accepts one request object.
  Batch arrays are rejected with `-32600` ("Request must be an object"); there
  is no batch cap because there is no batch dispatch to bound (R18 carve-out).
- **Generated stubs and HTTP adapter** — Python 2 generates only `idl.json` and
  the runtime package. No `server.py`, `client.py`, `test_server.py` or
  `test_client.py` is generated. The 1 MiB body cap and 405 handling therefore
  live in your adapter, not in generated code (see below).
- **Type resolution** — the Python 2 helpers use exact dict keys; the cached,
  namespace-aware resolution index introduced for Python 3 (R21) is not
  present.

`tests/test_api_surface.py` pins the 19 exported symbols and fails on drift.
An API difference that is not listed here is a bug, not an intentional waiver.

### Shared wire-contract rules (implemented here too)

These rules are identical to Python 3 and the other runtimes:

- **D1/R25 — `int` is signed 32-bit.** `validate_int` accepts integral values
  in `[-2147483648, 2147483647]`; fractional, NaN/Inf, bool and out-of-range
  values are rejected. Python 2 `long` values are range-checked explicitly.
- **D2/R24 — `id` is a string, number or null.** Booleans, objects and arrays
  are rejected with `-32600` and a response `id` of `null`.
- **D4/R15 — internal errors never leak details.** An unhandled handler
  exception returns `{"code": -32603, "message": "Internal error"}` with no
  `data`. The exception and traceback are logged server-side through the module
  logger, `logging.getLogger(__name__)` — `pulserpc.server` for the runtime
  imported under its normal package name — or passed to `on_error`. Declared
  `RPCError` messages/data and `-32602` validation messages are preserved.
- **D5 — resource limits.** The request body cap is
  `pulserpc.server.DEFAULT_MAX_BODY_SIZE` (1 MiB) for your adapter to enforce;
  `HttpTransport` caps response bodies at `MAX_RESPONSE_BODY_SIZE` (10 MiB) and
  raises `ValueError` beyond it.
- **D6/R19 — 30 s HTTP timeout** (`pulserpc.transport.DEFAULT_TIMEOUT`).
- **D13/R29/R30 — custom headers and trailing-slash normalization.**
  `HttpTransport(url, headers={...})` merges headers over `Content-Type`, and
  **every** trailing `/` is stripped from the base URL: the runtime applies
  `url.rstrip("/")`, so `HttpTransport("http://host/api///")` is normalized to
  `http://host/api`.

## Prerequisites

- Python 2.7
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Generate Code

Generate Python 2 targeted code:

```bash
pulserpc -plugin python-client-server -python-version 2 -dir ./output checkout.pulse
```

This creates:
- `output/checkout/idl.json` - IDL metadata (in the namespace directory)
- `output/pulserpc/` - Runtime library (RPCError, validation, types)

Note: Python 2 generates only metadata + runtime. No stub classes are generated (Python 2 uses plain dicts and strings for structs and enums).

## 2. Runtime Architecture

The Python 2 runtime provides:

- **Server** - Main server class (from `pulserpc`)
- **Contract** - IDL validation (from `pulserpc`)
- **RPCError** - For returning structured errors (from `pulserpc`)

Handlers are plain classes (no generated base classes in Python 2). All handler methods receive `ctx` as the **first positional argument** for transport-level metadata.

The `Server` constructor accepts an optional `on_error` callback for custom error handling:

```python
def handle_error(e):
    # Log to your monitoring system, metrics, etc.
    print("Handler error:", e)

server = Server(contract, on_error=handle_error)
```

When `on_error` is set, it is invoked with unhandled handler exceptions instead of logging them. If `on_error` is not set (default `None`), unhandled exceptions are logged (with traceback) through the `pulserpc.server` logger — `server.py` uses `logging.getLogger(__name__)`, which resolves to that name for the normally imported runtime — and the client receives only `-32603 "Internal error"` with no `data`.

## Transport Context (ctx)

All handler methods receive a `ctx` parameter as the first positional argument. This contains transport-level metadata (headers, auth tokens, etc.) passed automatically by the runtime.

**Important**: Python 2 runtime passes `ctx` as a first positional argument, so it must be the first parameter after `self` in your method signature. See `pkg/runtime/runtimes/python2/pulserpc/server.py` for implementation details.

```python
class CatalogServiceImpl(object):
    def listProducts(self, ctx):
        # ctx may be None if no transport context is provided
        # ctx is a dict with transport metadata (e.g., headers)
        return products_db

    def getProduct(self, ctx, productId):
        # ctx is passed as first positional argument
        return None
```

## 3. Implement the Server

Create `my_server.py`:

{% include quickstart/python2-server.md %}

## 4. Integrate with BaseHTTPServer

The server integrates with Python 2's `BaseHTTPServer`. Build a `ctx` dict from request headers and pass it to `server.call()`:

```python
import json
from BaseHTTPServer import HTTPServer, BaseHTTPRequestHandler
from pulserpc import RPCError
from pulserpc.server import DEFAULT_MAX_BODY_SIZE

class PulseRPCHandler(BaseHTTPRequestHandler):
    MAX_BODY_SIZE = DEFAULT_MAX_BODY_SIZE  # 1 MiB (R16)

    def do_POST(self):
        # Reject an oversized body before reading it into memory.
        try:
            content_length = int(self.headers.getheader('Content-Length', 0))
        except (TypeError, ValueError):
            content_length = -1
        if content_length <= 0:
            self._send_error(None, -32700, "Parse error")
            return
        if content_length > self.MAX_BODY_SIZE:
            self._send_error(None, -32600, "Request body too large")
            return

        request_body = self.rfile.read(content_length)
        try:
            request_data = json.loads(request_body)
        except ValueError:
            self._send_error(None, -32700, "Parse error")
            return

        # Build context dict from request headers
        ctx = {
            'headers': dict(self.headers),
            'remote_addr': self.client_address[0],
        }
        # Example: extract auth token
        auth_header = self.headers.getheader('Authorization')
        if auth_header:
            ctx['auth'] = auth_header

        # Pass ctx to server.call() - forwarded to all handler methods
        response = server.call(request_data, ctx)

        # Send response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        if response:
            self.wfile.write(json.dumps(response))

    def _send_error(self, request_id, code, message):
        body = json.dumps({
            'jsonrpc': '2.0',
            'error': {'code': code, 'message': message},
            'id': request_id,
        })
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    # Only POST is a valid JSON-RPC transport (D3/R22).  Every other method
    # must return HTTP 405 with a JSON-RPC -32600 body.
    def do_GET(self):
        self._method_not_allowed()

    def do_PUT(self):
        self._method_not_allowed()

    def do_DELETE(self):
        self._method_not_allowed()

    def _method_not_allowed(self):
        body = json.dumps({
            'jsonrpc': '2.0',
            'error': {
                'code': -32600,
                'message': 'Invalid Request - only POST allowed',
            },
            'id': None,
        })
        self.send_response(405)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)
```

Handler methods receive `ctx` as the first positional argument:

```python
class OrderServiceImpl(object):
    def createOrder(self, ctx, request):
        # ctx contains transport metadata (headers, auth, remote_addr)
        if ctx and ctx.get('auth'):
            print("Authenticated request:", ctx['auth'])
        # ... rest of handler logic
```

Start the server:

```bash
python my_server.py
```

Server runs on http://localhost:8080

## Next Steps

- [Python Reference](reference.html) - Type mappings, patterns, best practices
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL language reference