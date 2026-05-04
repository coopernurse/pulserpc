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
- **Contract** - IDL validation
- **RPCError** - For returning structured errors

Handlers are plain classes (no generated base classes in Python 2). All handler methods receive `ctx` as the **last positional argument** for transport-level metadata.

## Transport Context (ctx)

All handler methods receive a `ctx` parameter as the last positional argument. This contains transport-level metadata (headers, auth tokens, etc.) passed automatically by the runtime.

**Important**: Python 2 runtime passes `ctx` as a positional argument (not keyword), so it must be the last parameter in your method signature. See `pkg/runtime/runtimes/python2/pulserpc/server.py` for implementation details.

```python
class CatalogServiceImpl(object):
    def listProducts(self, ctx):
        # ctx may be None if no transport context is provided
        # ctx is a dict with transport metadata (e.g., headers)
        return products_db

    def getProduct(self, productId, ctx):
        # ctx is passed as last positional argument
        return None
```

## 3. Implement the Server

Create `my_server.py`:

{% include quickstart/python2-server.md %}

## 4. Integrate with BaseHTTPServer

The server integrates with Python 2's `BaseHTTPServer`:

```python
from BaseHTTPServer import HTTPServer, BaseHTTPRequestHandler
from pulserpc import RPCError

class PulseRPCHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        # Read request body
        content_length = int(self.headers.get('Content-Length', 0))
        request_body = self.rfile.read(content_length)
        request_data = json.loads(request_body)

        # Process via PulseRPC server (ctx is passed automatically)
        # The runtime passes transport metadata as ctx positional arg
        response = server.call(request_data)

        # Send response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        if response:
            self.wfile.write(json.dumps(response))
```

Start the server:

```bash
python my_server.py
```

Server runs on http://localhost:8080

## Next Steps

- [Python Reference](reference.html) - Type mappings, patterns, best practices
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL language reference