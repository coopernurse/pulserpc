"""Unit tests for Client: response envelope, named params, null results and
InProcTransport round trips (audit python-03, python-04, python-07, python-08).
"""

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
