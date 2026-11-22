#!/usr/bin/env python
"""Python 2.7 conformance server for the shared HTTP harness.

The Python 2 generator intentionally emits only idl.json plus the runtime
library (no generated server), so the harness supplies the thin conform
implementation and HTTP envelope handling here.
"""

import json
import os
import sys

from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer

from pulserpc import Contract, Server, RPCError

MAX_BODY_SIZE = 1024 * 1024
MAX_BATCH_SIZE = 100
STRING_TYPES = (str, unicode)
NUMBER_TYPES = (int, long, float)


def is_valid_id(value):
    if value is None:
        return True
    if isinstance(value, bool):
        return False
    return isinstance(value, STRING_TYPES + NUMBER_TYPES)


def error_response(code, message, req_id=None):
    return {"jsonrpc": "2.0", "error": {"code": code, "message": message}, "id": req_id}


class AImpl(object):
    def add(self, ctx, a, b):
        return a + b

    def calc(self, ctx, nums, operation):
        if operation == "add":
            return sum(nums)
        if operation == "multiply":
            result = 1.0
            for value in nums:
                result *= value
            return result
        return 0.0

    def divide(self, ctx, a, b):
        if b == 0:
            raise RPCError(1001, "Validation failed")
        return float(a) / float(b)

    def sqrt(self, ctx, a):
        return float(a) ** 0.5

    def repeat(self, ctx, req1):
        count = req1.get("count", 0)
        if count < 0:
            # Exercises the internal-error redaction invariant (D4/R15).
            raise RuntimeError("PULSERPC_SECRET_TOKEN_7F3A")
        text = req1.get("to_repeat", "")
        if req1.get("force_uppercase", False):
            text = text.upper()
        return {"status": "ok", "count": count, "items": [text] * count}

    def say_hi(self, ctx):
        return {"hi": "hi"}

    def repeat_num(self, ctx, num, count):
        return [num] * count

    def putPerson(self, ctx, person):
        return person.get("personId", "")

    def findItem(self, ctx, item_id):
        return None


class BImpl(object):
    def echo(self, ctx, value):
        if value == "return-null":
            return None
        return value


def load_contract():
    here = os.path.dirname(os.path.abspath(__file__))
    with open(os.path.join(here, "idl.json")) as handle:
        return Contract(json.load(handle))


def process_single(server, item):
    if not isinstance(item, dict):
        return error_response(-32600, "Invalid Request")

    req_id = item.get("id")
    if "id" in item and not is_valid_id(req_id):
        return error_response(-32600, "Invalid Request - id must be a string or number")

    # The runtime treats a missing/null id as a notification.
    return server.call(item)


def process_payload(server, payload):
    if isinstance(payload, list):
        if len(payload) == 0:
            return error_response(-32600, "Invalid Request - empty batch")
        if len(payload) > MAX_BATCH_SIZE:
            return error_response(-32600, "Batch exceeds maximum size")

        responses = []
        for item in payload:
            response = process_single(server, item)
            if response is not None:
                responses.append(response)
        return responses if responses else None

    return process_single(server, payload)


def make_handler(server):
    class Handler(BaseHTTPRequestHandler):
        def _send_json(self, status, payload):
            body = "" if payload is None else json.dumps(payload)
            self.send_response(status)
            if body:
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
            else:
                self.send_header("Content-Length", "0")
            self.end_headers()
            if body:
                self.wfile.write(body)

        def _reject_non_post(self, include_body=True):
            payload = {"jsonrpc": "2.0",
                       "error": {"code": -32600, "message": "Invalid Request - only POST allowed"},
                       "id": None}
            self._send_json(405, payload if include_body else None)

        def do_GET(self):
            self._reject_non_post()

        def do_PUT(self):
            self._reject_non_post()

        def do_DELETE(self):
            self._reject_non_post()

        def do_PATCH(self):
            self._reject_non_post()

        def do_OPTIONS(self):
            self._reject_non_post()

        def do_HEAD(self):
            self._reject_non_post(include_body=False)

        def do_POST(self):
            try:
                length = int(self.headers.get("Content-Length", "0"))
            except ValueError:
                length = 0

            if length > MAX_BODY_SIZE:
                self._send_json(
                    200,
                    error_response(-32600, "Request body too large"),
                )
                return

            raw = self.rfile.read(length) if length > 0 else ""

            try:
                payload = json.loads(raw)
            except Exception:
                self._send_json(200, error_response(-32700, "Parse error"))
                return

            response = process_payload(server, payload)
            if response is None:
                self._send_json(204, None)
            elif isinstance(response, list) and len(response) == 0:
                self._send_json(204, None)
            else:
                self._send_json(200, response)

        def log_message(self, fmt, *args):
            pass

    return Handler


def main():
    port = int(os.environ.get("SERVER_PORT", "8080"))
    server = Server(load_contract(), validate_requests=True, validate_responses=True)
    server.add_handler("A", AImpl())
    server.add_handler("B", BImpl())

    httpd = HTTPServer(("0.0.0.0", port), make_handler(server))
    sys.stderr.write("Python 2 conformance server listening on port %d\n" % port)
    httpd.serve_forever()


if __name__ == "__main__":
    main()
