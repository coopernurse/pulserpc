---
title: Python 2 Server Guide
layout: default
---

# Python 2 Server Guide

Build a PulseRPC server in Python 2.7 using the standard library.

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

When `on_error` is set, it is invoked with unhandled handler exceptions instead of printing a traceback. If `on_error` is not set (default `None`), unhandled exceptions print a traceback to stderr.

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
from BaseHTTPServer import HTTPServer, BaseHTTPRequestHandler
from pulserpc import RPCError

class PulseRPCHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        # Read request body
        content_length = int(self.headers.getheader('Content-Length', 0))
        request_body = self.rfile.read(content_length)
        request_data = json.loads(request_body)

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