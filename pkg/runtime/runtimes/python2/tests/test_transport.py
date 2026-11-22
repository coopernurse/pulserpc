"""HTTP transport invariants for the Python 2 runtime: response cap (R17),
timeout (R19), custom headers (R29) and base-URL normalization (R30)."""

import json

from pulserpc.transport import (
    DEFAULT_TIMEOUT,
    MAX_RESPONSE_BODY_SIZE,
    HttpTransport,
)


class FakeResponse(object):
    def __init__(self, body):
        self._body = body

    def read(self, n=-1):
        if n is None or n < 0:
            return self._body
        return self._body[:n]


class FakeUrlopen(object):
    def __init__(self, body=b"{}"):
        self.body = body
        self.calls = []

    def __call__(self, request, timeout=None):
        self.calls.append((request, timeout))
        return FakeResponse(self.body)


def _install(fake):
    import pulserpc.transport as transport_module

    original = transport_module.urlopen
    transport_module.urlopen = fake
    return original


def _header_items(request):
    return dict((key.lower(), value) for key, value in request.header_items())


def test_defaults():
    print("Testing transport defaults (R17/R19)...")
    transport = HttpTransport("http://example.com/api")
    assert transport.timeout == DEFAULT_TIMEOUT == 30
    assert transport.max_response_size == MAX_RESPONSE_BODY_SIZE == 10 * 1024 * 1024
    print("  PASS: transport defaults")


def test_timeout_headers_and_url_normalization():
    print("Testing timeout/headers/trailing slash (R19/R29/R30)...")
    fake = FakeUrlopen(json.dumps({"jsonrpc": "2.0", "result": 1, "id": 1}))
    original = _install(fake)
    try:
        transport = HttpTransport(
            "http://example.com/api/",
            timeout=7,
            headers={"Authorization": "Bearer token", "X-Trace": "abc"},
        )
        assert transport.url == "http://example.com/api"
        transport.request({"jsonrpc": "2.0", "method": "x", "id": 1})
    finally:
        _install(original)

    assert len(fake.calls) == 1
    request, timeout = fake.calls[0]
    assert timeout == 7
    headers = _header_items(request)
    assert headers["authorization"] == "Bearer token"
    assert headers["x-trace"] == "abc"
    assert headers["content-type"] == "application/json"
    assert request.get_full_url() == "http://example.com/api"
    print("  PASS: timeout/headers/trailing slash")


def test_response_cap():
    print("Testing response body cap (R17)...")
    payload = '{"pad": "' + ("a" * 100) + '"}'
    fake = FakeUrlopen(payload)
    original = _install(fake)
    try:
        ok = HttpTransport(
            "http://example.com/api", max_response_size=len(payload)
        )
        assert ok.request({"jsonrpc": "2.0", "method": "x", "id": 1}) == {
            "pad": "a" * 100
        }

        too_small = HttpTransport(
            "http://example.com/api", max_response_size=len(payload) - 1
        )
        try:
            too_small.request({"jsonrpc": "2.0", "method": "x", "id": 1})
            assert False, "Should have raised ValueError for oversized response"
        except ValueError as exc:
            assert "maximum size" in str(exc), str(exc)
    finally:
        _install(original)
    print("  PASS: response body cap")


def test_empty_response():
    print("Testing empty response handling...")
    fake = FakeUrlopen(b"")
    original = _install(fake)
    try:
        transport = HttpTransport("http://example.com/api")
        assert transport.request({"jsonrpc": "2.0", "method": "x", "id": 1}) == {}
    finally:
        _install(original)
    print("  PASS: empty response handling")


def test_invalid_json_response_is_wrapped():
    print("Testing invalid JSON response wrapping...")
    fake = FakeUrlopen("not json")
    original = _install(fake)
    try:
        transport = HttpTransport("http://example.com/api")
        try:
            transport.request({"jsonrpc": "2.0", "method": "x", "id": 1})
            assert False, "Should have raised ValueError for invalid JSON"
        except ValueError as exc:
            # Python 2 has no JSONDecodeError; the bare message must not escape.
            assert "Invalid JSON response" in str(exc), str(exc)
    finally:
        _install(original)
    print("  PASS: invalid JSON response wrapping")


def main():
    test_defaults()
    test_timeout_headers_and_url_normalization()
    test_response_cap()
    test_empty_response()
    test_invalid_json_response_is_wrapped()
    print("  test_transport: ALL PASSED")
