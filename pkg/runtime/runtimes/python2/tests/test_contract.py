"""Contract validation tests for the Python 2 runtime."""

import io
import os
import shutil
import tempfile

from pulserpc import Contract, RPCError

from support import assert_rpc_error, sample_idl

EXPECTED_STRUCTURED_KEYS = ("error", "invalid_fields")


def _contract():
    return Contract(sample_idl())


def test_validate_request_success():
    print("Testing validate_request success...")
    contract = _contract()
    contract.validate_request("CalcService", "add", [1, 2])
    contract.validate_request("CalcService", "echo", ["hello"])
    contract.validate_request("CalcService", "ping", [])
    print("  PASS: validate_request success")


def test_validate_request_errors_are_rpc_errors():
    print("Testing validate_request error codes and data...")
    contract = _contract()

    err = assert_rpc_error(-32602, contract.validate_request, "Missing", "add", [1, 2])
    assert isinstance(err.data, dict)
    for key in EXPECTED_STRUCTURED_KEYS:
        assert key in err.data, "missing structured key %s" % key

    err = assert_rpc_error(-32602, contract.validate_request, "CalcService", "nope", [])
    assert isinstance(err.data, dict)

    err = assert_rpc_error(-32602, contract.validate_request, "CalcService", "add", [1])
    assert isinstance(err.data, dict)

    err = assert_rpc_error(
        -32602, contract.validate_request, "CalcService", "add", ["bad", 2]
    )
    fields = err.data.get("invalid_fields")
    assert isinstance(fields, list) and len(fields) == 1
    assert fields[0] == "param[0]"
    print("  PASS: validate_request error codes and data")


def test_validate_response_success_and_errors():
    print("Testing validate_response...")
    contract = _contract()

    contract.validate_response("CalcService", "add", 3)
    contract.validate_response("CalcService", "ping", None)
    contract.validate_response("CalcService", "getPerson", {"name": "Ada"})

    err = assert_rpc_error(-32603, contract.validate_response, "Missing", "add", 3)
    assert isinstance(err.data, dict)

    err = assert_rpc_error(-32603, contract.validate_response, "CalcService", "nope", 3)
    assert isinstance(err.data, dict)

    err = assert_rpc_error(
        -32603, contract.validate_response, "CalcService", "add", "not an int"
    )
    fields = err.data.get("invalid_fields")
    assert isinstance(fields, list) and len(fields) == 1
    assert fields[0] == "result"

    err = assert_rpc_error(
        -32603, contract.validate_response, "CalcService", "ping", "unexpected"
    )
    assert isinstance(err.data, dict)
    print("  PASS: validate_response")


def test_contract_helpers_and_from_file():
    print("Testing Contract helpers and from_file...")
    contract = _contract()
    assert contract.has_interface("CalcService")
    assert not contract.has_interface("Missing")
    assert contract.get_interface("CalcService").get_function("add") is not None

    tmpdir = tempfile.mkdtemp(prefix="pulserpc_py2_contract_")
    try:
        path = os.path.join(tmpdir, "idl.json")
        with io.open(path, "w", encoding="utf-8") as f:
            f.write(u'{"interfaces": [], "structs": [], "enums": []}')
        from_file = Contract.from_file(path)
        assert not from_file.has_interface("CalcService")
    finally:
        shutil.rmtree(tmpdir)
    print("  PASS: Contract helpers and from_file")


def main():
    test_validate_request_success()
    test_validate_request_errors_are_rpc_errors()
    test_validate_response_success_and_errors()
    test_contract_helpers_and_from_file()
    print("  test_contract: ALL PASSED")
