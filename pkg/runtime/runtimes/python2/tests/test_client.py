"""Client tests: in-process round trips, RPCError propagation, envelopes."""

import threading

from pulserpc import Client, Contract, InProcTransport, RPCError, Server, Transport

from support import assert_rpc_error, sample_idl


class EchoTransport(Transport):
    """Serves the IDL, then echoes the request id with a fixed result."""

    def __init__(self):
        self.idl = sample_idl()
        self.requests = []

    def request(self, req):
        self.requests.append(req)
        if req.get("method") == "pulserpc-idl":
            return {"jsonrpc": "2.0", "result": self.idl, "id": req.get("id")}
        return {"jsonrpc": "2.0", "result": "ok", "id": req.get("id")}


class ScriptedTransport(Transport):
    """Returns a fixed response for non-bootstrap requests and records them."""

    def __init__(self, response):
        self.idl = sample_idl()
        self.response = response
        self.requests = []

    def request(self, req):
        self.requests.append(req)
        if req.get("method") == "pulserpc-idl":
            return {"jsonrpc": "2.0", "result": self.idl, "id": req.get("id")}
        return self.response


class RecordingTransport(Transport):
    """Returns an empty response and records all requests (notifications)."""

    def __init__(self):
        self.idl = sample_idl()
        self.requests = []

    def request(self, req):
        self.requests.append(req)
        if req.get("method") == "pulserpc-idl":
            return {"jsonrpc": "2.0", "result": self.idl, "id": req.get("id")}
        return {}


def _in_proc_client(fail_error=None):
    contract = Contract(sample_idl())
    server = Server(contract)
    server.add_handler("CalcService", _Handler(fail_error))
    return Client(InProcTransport(server))


class _Handler(object):
    def __init__(self, fail_error=None):
        self.fail_error = fail_error

    def add(self, ctx, a, b):
        return a + b

    def echo(self, ctx, value):
        return value

    def ping(self, ctx):
        return None

    def getPerson(self, ctx):
        return {"name": "Ada"}

    def fail(self, ctx):
        raise self.fail_error


def test_in_proc_round_trip():
    print("Testing Client round trip over InProcTransport...")
    client = _in_proc_client()
    assert client.CalcService.add(2, 3) == 5
    assert client.CalcService.echo("hi") == "hi"
    assert client.CalcService.ping() is None
    assert client.CalcService.getPerson() == {"name": "Ada"}
    print("  PASS: Client round trip over InProcTransport")


def test_rpc_error_raised_on_server_error():
    print("Testing RPCError propagation to the client...")
    custom = RPCError(1234, "custom failure", {"detail": "boom"})
    client = _in_proc_client(fail_error=custom)
    err = assert_rpc_error(1234, client.CalcService.fail)
    assert err.message == "custom failure"
    assert err.data == {"detail": "boom"}
    print("  PASS: RPCError propagation to the client")


def test_response_envelope_validation():
    print("Testing response envelope validation...")

    def client_for(response):
        return Client(ScriptedTransport(response))

    # Both result and error together are invalid.
    err = assert_rpc_error(
        -32600,
        client_for({"jsonrpc": "2.0", "result": 1, "error": {"code": 1}, "id": 1}).call,
        "CalcService.add",
        [1, 1],
    )
    assert "exactly one" in err.data

    # Neither result nor error is invalid.
    assert_rpc_error(
        -32600,
        client_for({"jsonrpc": "2.0", "id": 1}).call,
        "CalcService.add",
        [1, 1],
    )

    # Wrong jsonrpc version.
    assert_rpc_error(
        -32600,
        client_for({"jsonrpc": "1.0", "result": 2, "id": 1}).call,
        "CalcService.add",
        [1, 1],
    )

    # Mismatched id.
    err = assert_rpc_error(
        -32600,
        client_for({"jsonrpc": "2.0", "result": 2, "id": 99}).call,
        "CalcService.add",
        [1, 1],
    )
    assert "does not match" in err.data

    # A well-formed response still returns its result.
    assert (
        client_for({"jsonrpc": "2.0", "result": 2, "id": 1}).call(
            "CalcService.add", [1, 1]
        )
        == 2
    )
    print("  PASS: response envelope validation")


def test_notify_sends_no_id():
    print("Testing client notifications...")
    transport = RecordingTransport()
    client = Client(transport)
    assert client.notify("CalcService.add", [1, 2]) is None
    call_requests = [r for r in transport.requests if r.get("method") != "pulserpc-idl"]
    assert len(call_requests) == 1
    assert "id" not in call_requests[0]
    assert call_requests[0]["params"] == [1, 2]
    print("  PASS: client notifications")


# --- M20: named params are converted to positional before going on the wire ---

def test_named_params_are_converted_to_positional():
    print("Testing named->positional conversion (M20)...")
    transport = EchoTransport()
    client = Client(transport)  # validation disabled on purpose

    assert client.CalcService.add(a=2, b=3) == "ok"

    calls = [r for r in transport.requests if r.get("method") == "CalcService.add"]
    assert len(calls) == 1
    assert calls[0]["params"] == [2, 3], calls[0]["params"]
    print("  PASS: named->positional conversion")


# --- R20: request ids are unique under concurrency ---------------------------

def test_concurrent_calls_get_unique_request_ids():
    print("Testing concurrent request ids (R20)...")
    transport = EchoTransport()
    client = Client(transport)
    # The id counter is shared; it must be guarded by a lock.
    assert hasattr(client, "_request_id_lock")

    errors = []

    def worker():
        try:
            for _ in range(100):
                assert client.CalcService.add(1, 2) == "ok"
        except Exception as exc:  # pragma: no cover - surfaced below
            errors.append(exc)

    threads = [threading.Thread(target=worker) for _ in range(8)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    assert errors == []
    ids = [
        r.get("id") for r in transport.requests
        if r.get("method") == "CalcService.add" and r.get("id") is not None
    ]
    assert len(ids) == 8 * 100
    assert len(set(ids)) == len(ids), "request ids must be unique"
    print("  PASS: concurrent request ids")


def main():
    test_in_proc_round_trip()
    test_rpc_error_raised_on_server_error()
    test_response_envelope_validation()
    test_notify_sends_no_id()
    test_named_params_are_converted_to_positional()
    test_concurrent_calls_get_unique_request_ids()
    print("  test_client: ALL PASSED")
