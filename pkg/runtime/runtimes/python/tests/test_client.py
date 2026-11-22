"""Unit tests for Client: response envelope, named params, null results and
InProcTransport round trips (audit python-03, python-04, python-07, python-08).
"""

import json
import threading
import time

import pytest

from pulserpc import (
    Client,
    Contract,
    InProcTransport,
    RPCError,
    INVALID_REQUEST,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)
from tests.test_server import make_contract, make_server, request


class RecordingTransport:
    """Wraps an InProcTransport and records every outgoing request."""

    def __init__(self, server):
        self.inner = InProcTransport(server)
        self.requests = []

    def request(self, req):
        self.requests.append(dict(req))
        return self.inner.request(req)


class ScriptedTransport:
    """Serves the IDL on bootstrap, then returns canned responses."""

    def __init__(self, idl, responses=None):
        self.idl = idl
        self.responses = list(responses or [])

    def request(self, req):
        if req.get('method') == 'pulserpc-idl':
            return {'jsonrpc': '2.0', 'result': self.idl, 'id': req.get('id')}
        if not self.responses:
            raise AssertionError(f'Unexpected request: {req}')
        return self.responses.pop(0)


def response(result=None, req_id=1, **extra):
    body = {'jsonrpc': '2.0', 'result': result, 'id': req_id}
    body.update(extra)
    return body


def scripted_client(responses, **kwargs):
    return Client(ScriptedTransport(make_contract().idl_parsed, responses), **kwargs)


# --- python-07 / python-08: round trips and named params -----------------------

def test_positional_call_round_trip():
    client = Client(RecordingTransport(make_server()),
                    validate_request=True, validate_response=True)
    assert client.Demo.echo('hello') == 'hello'


def test_kwargs_call_sends_positional_array():
    transport = RecordingTransport(make_server())
    client = Client(transport, validate_request=True, validate_response=True)
    assert client.Demo.echo(value='hello') == 'hello'

    outgoing = [r for r in transport.requests if r['method'] == 'Demo.echo']
    assert len(outgoing) == 1
    assert isinstance(outgoing[0]['params'], list)
    assert outgoing[0]['params'] == ['hello']


def test_scalar_param_is_normalized_to_array():
    transport = RecordingTransport(make_server())
    client = Client(transport, validate_request=True)
    assert client.call('Demo.echo', 'hello') == 'hello'
    outgoing = [r for r in transport.requests if r['method'] == 'Demo.echo']
    assert outgoing[0]['params'] == ['hello']


def test_server_error_becomes_rpcerror():
    client = Client(RecordingTransport(make_server()))
    with pytest.raises(RPCError) as exc:
        client.call('Demo.boom')
    assert exc.value.code == INTERNAL_ERROR


# --- python-04: response envelope validation ----------------------------------

def test_rejects_response_with_both_result_and_error():
    client = scripted_client([
        {'jsonrpc': '2.0', 'result': 'x', 'error': {'code': -1, 'message': 'bad'}, 'id': 1}
    ])
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INVALID_REQUEST


def test_rejects_response_with_neither_result_nor_error():
    client = scripted_client([{'jsonrpc': '2.0', 'id': 1}])
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INVALID_REQUEST


def test_rejects_wrong_jsonrpc_version():
    client = scripted_client([{'jsonrpc': '1.0', 'result': 'x', 'id': 1}])
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INVALID_REQUEST


def test_rejects_mismatched_id():
    client = scripted_client([response(result='x', req_id=999)])
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INVALID_REQUEST


def test_error_response_is_surfaced_with_its_code():
    client = scripted_client([
        {'jsonrpc': '2.0', 'error': {'code': INVALID_PARAMS, 'message': 'bad'}, 'id': 1}
    ])
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INVALID_PARAMS


# --- python-03: client-side validation of null results -------------------------

def test_client_rejects_null_for_required_return():
    client = scripted_client([response(result=None)], validate_response=True)
    with pytest.raises(RPCError) as exc:
        client.call('Demo.required')
    assert exc.value.code == INTERNAL_ERROR


def test_client_accepts_null_for_optional_return():
    client = scripted_client([response(result=None)], validate_response=True)
    assert client.call('Demo.optional') is None


def test_client_validates_non_null_result_type():
    client = scripted_client([response(result=42)], validate_response=True)
    with pytest.raises(RPCError) as exc:
        client.call('Demo.echo', ['x'])
    assert exc.value.code == INTERNAL_ERROR


# --- R20: request ids are unique under concurrency ----------------------------

class _SlowCounter(int):
    """An int whose increment sleeps, exposing an unprotected read-modify-write.

    ``client._request_id += 1`` therefore spans a real interval: without the
    lock every thread reads the same value and they all store the same result.
    """

    def __add__(self, other):
        time.sleep(0.002)
        return _SlowCounter(int(self) + other)

    __radd__ = __add__


def test_concurrent_calls_get_unique_and_contiguous_request_ids():
    transport = RecordingTransport(make_server())
    client = Client(transport)
    # Deterministic interleaving instead of relying on GIL switch timing: the
    # slow increment makes a missing lock lose updates every run.
    client._request_id = _SlowCounter(0)

    thread_count = 8
    per_thread = 25
    barrier = threading.Barrier(thread_count)
    errors = []

    def worker():
        try:
            barrier.wait()
            for _ in range(per_thread):
                assert client.Demo.echo('x') == 'x'
        except Exception as exc:  # pragma: no cover - surfaced below
            errors.append(exc)

    threads = [threading.Thread(target=worker) for _ in range(thread_count)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    assert errors == []
    total = thread_count * per_thread
    ids = [
        r['id'] for r in transport.requests
        if r.get('method') == 'Demo.echo' and r.get('id') is not None
    ]
    assert len(ids) == total
    assert len(set(ids)) == len(ids), 'request ids must be unique'
    # Ids are handed out exactly once: no gaps, no duplicates.
    assert sorted(ids) == list(range(1, total + 1))


# --- M24: numeric ids are compared by value ----------------------------------

def test_numeric_id_comparison_is_type_tolerant():
    client = scripted_client([response(result='x', req_id=1.0)])
    assert client.call('Demo.echo', ['x']) == 'x'


# --- R31: verification diffs against the fetched server IDL -------------------

def test_verify_compatibility_detects_divergent_server_idl():
    server_idl = make_contract().idl_parsed
    client = scripted_client([])
    client.set_local_idl(json.dumps({
        'interfaces': [{
            'name': 'Demo',
            'methods': [{
                'name': 'onlyInClient',
                'parameters': [],
                'returnType': {'builtIn': 'string'},
            }],
        }],
        'structs': [],
        'enums': [],
        'rootNamespace': '',
        'checksum': 'client-checksum',
    }))

    result = client.verify_compatibility()

    assert result.compatible is False
    assert any(d.severity.value == 'Error' for d in result.deltas)
    assert result.server_checksum != result.client_checksum


def test_verify_compatibility_matches_identical_idl():
    server_idl = make_contract().idl_parsed
    client = scripted_client([])
    client.set_local_idl(json.dumps(server_idl))

    result = client.verify_compatibility()

    assert result.compatible is True
    assert result.deltas == []


def test_verify_compatibility_requires_local_idl():
    client = scripted_client([])
    client._local_idl = None
    with pytest.raises(RuntimeError):
        client.verify_compatibility()
