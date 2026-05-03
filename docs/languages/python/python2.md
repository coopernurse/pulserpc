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

- **PulseRPCServer** - Main server class
- **CatalogService**, **CartService**, **OrderService** - Base classes for handlers
- **RPCError** - For returning structured errors

Handlers inherit from generated base classes and override interface methods.

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
        
        # Process via PulseRPC server
        response = server.call(request_body.decode('utf-8'))
        
        # Send response
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(response.encode('utf-8'))
```

Start the server:

```bash
python my_server.py
```

Server runs on http://localhost:8080

## Next Steps

- [Python Reference](reference.html) - Type mappings, patterns, best practices
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL language reference