"""RPC error and transport tests for the Python 2 runtime."""

from pulserpc import (
    Contract,
    HttpTransport,
    InProcTransport,
    RPCError,
    Server,
    Transport,
)

from support import CalcServiceImpl, sample_idl


def test_rpc_error_fields():
    print("Testing RPCError fields...")
    err = RPCError(-32602, "bad params", {"invalid_fields": ["a"]})
    assert err.code == -32602
    assert err.message == "bad params"
    assert err.data == {"invalid_fields": ["a"]}
    assert "-32602" in str(err) and "bad params" in str(err)

    plain = RPCError(-32603, "boom")
    assert plain.data is None
    print("  PASS: RPCError fields")


def test_transport_base_and_http():
    print("Testing Transport abstraction and HttpTransport...")
    base = Transport()
    try:
        base.request({"jsonrpc": "2.0"})
        assert False, "Transport.request should be abstract"
    except NotImplementedError:
        pass

    http = HttpTransport("http://localhost:8080", timeout=5, headers={"X-A": "b"})
    assert http.url == "http://localhost:8080"
    assert http.timeout == 5
    assert http.headers == {"X-A": "b"}
    print("  PASS: Transport abstraction and HttpTransport")


def test_in_proc_transport():
    print("Testing InProcTransport round trip...")
    contract = Contract(sample_idl())
    server = Server(contract)
    server.add_handler("CalcService", CalcServiceImpl())
    transport = InProcTransport(server)

    resp = transport.request(
        {"jsonrpc": "2.0", "method": "CalcService.add", "params": [4, 5], "id": 7}
    )
    assert resp == {"jsonrpc": "2.0", "result": 9, "id": 7}

    # Notifications produce no response; the transport normalises that to {}.
    resp = transport.request(
        {"jsonrpc": "2.0", "method": "CalcService.add", "params": [4, 5]}
    )
    assert resp == {}
    print("  PASS: InProcTransport round trip")


def main():
    test_rpc_error_fields()
    test_transport_base_and_http()
    test_in_proc_transport()
    print("  test_rpc: ALL PASSED")
