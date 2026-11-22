"""Server tests: standard error codes, validation, notifications."""

from pulserpc import Contract, RPCError, Server

from support import CalcServiceImpl, sample_idl


def _server(person=None, fail_error=None, boom=False):
    contract = Contract(sample_idl())
    server = Server(contract, on_error=lambda e: None)
    server.add_handler(
        "CalcService", CalcServiceImpl(person=person, fail_error=fail_error, boom=boom)
    )
    return server


def _call(server, method, params=None, req_id=1, jsonrpc="2.0"):
    req = {"jsonrpc": jsonrpc, "method": method, "id": req_id}
    if params is not None:
        req["params"] = params
    return server.call(req)


def _error_code(response):
    assert isinstance(response, dict), "expected a response dict"
    assert "error" in response, "expected an error response, got %r" % response
    return response["error"]["code"]


def test_standard_error_codes():
    print("Testing all four standard JSON-RPC error codes...")

    # -32600 Invalid Request
    assert _error_code(_call(_server(), "CalcService.add", [1, 2], jsonrpc="1.0")) == -32600
    assert _error_code(_call(_server(), 42)) == -32600

    # -32601 Method not found
    assert _error_code(_call(_server(), "NoService.add")) == -32601
    assert _error_code(_call(_server(), "CalcService.missing")) == -32601
    assert _error_code(_call(_server(), "malformed")) == -32601

    # -32602 Invalid params (including structured contract data)
    resp = _call(_server(), "CalcService.add", ["bad", 2])
    assert _error_code(resp) == -32602
    assert isinstance(resp["error"].get("data"), dict)
    assert _error_code(_call(_server(), "CalcService.add", {"a": 1})) == -32602

    # -32603 Internal error (null result for a required return, handler crash)
    assert _error_code(_call(_server(), "CalcService.getPerson")) == -32603
    assert _error_code(_call(_server(boom=True), "CalcService.fail")) == -32603

    print("  PASS: all four standard JSON-RPC error codes")


def test_invalid_jsonrpc_version_uses_null_id():
    print("Testing missing jsonrpc version uses null id...")
    # A request missing `jsonrpc` is an invalid request (-32600); its id must
    # be null on the wire rather than echoing the non-notification id of 1.
    req = {"method": "CalcService.add", "params": [1, 2], "id": 1}
    resp = _server().call(req)
    assert _error_code(resp) == -32600, repr(resp)
    assert "id" in resp and resp["id"] is None, repr(resp)
    print("  PASS: missing jsonrpc version uses null id")


def test_success_and_null_validation():
    print("Testing successful calls and null-result validation...")
    server = _server(person={"name": "Ada"})
    resp = _call(server, "CalcService.add", [2, 3])
    assert resp == {"jsonrpc": "2.0", "result": 5, "id": 1}

    resp = _call(server, "CalcService.getPerson")
    assert resp["result"] == {"name": "Ada"}

    # None for a required return must be rejected (-32603), not passed through.
    resp = _call(_server(), "CalcService.getPerson")
    assert _error_code(resp) == -32603

    # Void returns still accept None.
    resp = _call(server, "CalcService.ping")
    assert resp == {"jsonrpc": "2.0", "result": None, "id": 1}
    print("  PASS: successful calls and null-result validation")


def test_handler_rpc_error_propagation():
    print("Testing handler RPCError propagation...")
    custom = RPCError(1234, "custom failure", {"detail": "boom"})
    resp = _call(_server(fail_error=custom), "CalcService.fail")
    assert _error_code(resp) == 1234
    assert resp["error"]["message"] == "custom failure"
    assert resp["error"]["data"] == {"detail": "boom"}
    print("  PASS: handler RPCError propagation")


def test_internal_error_does_not_leak_details():
    print("Testing internal error redaction (R15/D4)...")
    resp = _call(_server(boom=True), "CalcService.fail")
    assert _error_code(resp) == -32603
    assert resp["error"]["message"] == "Internal error"
    assert "data" not in resp["error"]
    assert "kaboom" not in repr(resp)
    assert "RuntimeError" not in repr(resp)
    print("  PASS: internal error redaction")


def test_handler_body_type_error_is_redacted():
    print("Testing handler TypeError redaction...")

    class TypeErrorHandler(CalcServiceImpl):
        def fail(self, ctx):
            raise TypeError("secret internal detail")

    contract = Contract(sample_idl())
    server = Server(contract, on_error=lambda e: None)
    server.add_handler("CalcService", TypeErrorHandler())
    resp = _call(server, "CalcService.fail")
    assert _error_code(resp) == -32603
    assert resp["error"]["message"] == "Internal error"
    assert "secret" not in repr(resp)
    print("  PASS: handler TypeError redaction")


def test_call_signature_mismatch_is_invalid_params():
    print("Testing call-signature mismatch...")
    contract = Contract(sample_idl())
    server = Server(contract, validate_requests=False)
    server.add_handler("CalcService", CalcServiceImpl())
    resp = _call(server, "CalcService.echo", [])
    assert _error_code(resp) == -32602
    assert resp["error"]["message"] == "Invalid params"
    assert resp["error"]["data"] == "Parameter mismatch"
    print("  PASS: call-signature mismatch")


def test_request_id_types():
    print("Testing request id types (R24/D2)...")
    server = _server(person={"name": "Ada"})

    # Strings and numbers (including floats and `long`-sized integers, which
    # Python 2 promotes beyond sys.maxint) are accepted and echoed.
    for good_id in ("abc", 0, 7, -3, 1.5, 2 ** 70, -(2 ** 70)):
        resp = _call(server, "CalcService.add", [1, 2], req_id=good_id)
        assert resp["result"] == 3
        assert resp["id"] == good_id

    # Booleans, objects and arrays are invalid requests with id:null.
    for bad_id in (True, False, {}, [], ["x"], float("nan"), float("inf")):
        req = {
            "jsonrpc": "2.0",
            "method": "CalcService.add",
            "params": [1, 2],
            "id": bad_id,
        }
        resp = server.call(req)
        assert resp["error"]["code"] == -32600, repr(resp)
        assert "id" in resp, repr(resp)
        assert resp["id"] is None, repr(resp)
        assert resp["error"]["message"] == "Invalid Request"
    print("  PASS: request id types")


def test_large_long_id_from_json_is_accepted():
    print("Testing large `long` request ids (R24)...")
    import json

    # This is the real path: the HTTP adapter parses the body with json.loads,
    # which turns an integer beyond sys.maxint into `long` in Python 2.
    raw = (
        '{"jsonrpc": "2.0", "method": "CalcService.add",'
        ' "params": [1, 2], "id": %d}' % (2 ** 70)
    )
    req = json.loads(raw)
    assert isinstance(req["id"], long), type(req["id"])

    resp = _server(person={"name": "Ada"}).call(req)
    assert resp["result"] == 3, repr(resp)
    assert resp["id"] == req["id"], repr(resp)
    print("  PASS: large `long` request ids")


def test_error_responses_always_emit_an_id_key():
    print("Testing explicit id on error responses (M18/M26)...")
    server = _server(person={"name": "Ada"})

    # An id that cannot be echoed back is null, not omitted.
    resp = server.call({"jsonrpc": "2.0", "method": "CalcService.add", "id": []})
    assert "id" in resp and resp["id"] is None, repr(resp)

    # A transport-level error has no request id but keeps the key.
    resp = server.call("not-a-dict")
    assert "id" in resp and resp["id"] is None, repr(resp)

    # A normal error still echoes the request id.
    resp = _call(server, "CalcService.missing", req_id="abc")
    assert resp["id"] == "abc", repr(resp)
    print("  PASS: explicit id on error responses")


def test_signature_lookup_is_cached():
    print("Testing per-method signature caching...")
    import inspect

    import pulserpc.server as server_module

    calls = []
    real = inspect.getargspec

    def counting(func):
        calls.append(func)
        return real(func)

    server_module.inspect.getargspec = counting
    try:
        server = _server(person={"name": "Ada"})
        _call(server, "CalcService.add", [1, 2])
        _call(server, "CalcService.add", [3, 4])
    finally:
        server_module.inspect.getargspec = real

    add_lookups = [c for c in calls if getattr(c, "__name__", None) == "add"]
    assert len(add_lookups) == 1, add_lookups
    print("  PASS: per-method signature caching")


def test_add_handler_clears_signature_cache():
    print("Testing signature cache invalidation...")

    class NarrowEcho(CalcServiceImpl):
        def echo(self, ctx):
            return "narrow"

    contract = Contract(sample_idl())
    server = Server(contract, validate_requests=False, on_error=lambda e: None)
    server.add_handler("CalcService", CalcServiceImpl())
    assert _call(server, "CalcService.echo", ["x"])["result"] == "x"

    server.add_handler("CalcService", NarrowEcho())
    # A stale cached signature would still accept the extra argument.
    assert _error_code(_call(server, "CalcService.echo", ["x"])) == -32602
    assert _call(server, "CalcService.echo", [])["result"] == "narrow"
    print("  PASS: signature cache invalidation")


def test_body_size_constant_is_documented():
    print("Testing Python 2 body-size carve-out constant...")
    from pulserpc.server import DEFAULT_MAX_BODY_SIZE

    assert DEFAULT_MAX_BODY_SIZE == 1024 * 1024
    print("  PASS: Python 2 body-size carve-out constant")


def test_notifications_produce_no_response():
    print("Testing notification silence...")

    def notify(method, params=None, jsonrpc="2.0"):
        req = {"jsonrpc": jsonrpc, "method": method}
        if params is not None:
            req["params"] = params
        return _server().call(req)

    # Normal method.
    assert notify("CalcService.add", [1, 2]) is None
    # Built-in IDL method must also be silent (python2-08).
    assert notify("pulserpc-idl") is None
    # Malformed jsonrpc/method notifications are silent too.
    assert notify("CalcService.add", [1, 2], jsonrpc="1.0") is None
    assert notify(42) is None
    assert notify("malformed") is None
    assert notify("NoService.add") is None
    assert notify("CalcService.add", ["bad", 2]) is None
    print("  PASS: notification silence")


def main():
    test_standard_error_codes()
    test_invalid_jsonrpc_version_uses_null_id()
    test_success_and_null_validation()
    test_handler_rpc_error_propagation()
    test_internal_error_does_not_leak_details()
    test_handler_body_type_error_is_redacted()
    test_call_signature_mismatch_is_invalid_params()
    test_request_id_types()
    test_large_long_id_from_json_is_accepted()
    test_error_responses_always_emit_an_id_key()
    test_signature_lookup_is_cached()
    test_add_handler_clears_signature_cache()
    test_body_size_constant_is_documented()
    test_notifications_produce_no_response()
    print("  test_server: ALL PASSED")
