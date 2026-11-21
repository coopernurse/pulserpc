"""Unit tests for Server.call: error codes, notifications, batches and data shape
(audit python-03, python-05, python-08, python-09, python-10, python-11).
"""

import pytest

from pulserpc import (
    Contract,
    Server,
    DEFAULT_MAX_BATCH_SIZE,
    INVALID_REQUEST,
    METHOD_NOT_FOUND,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)


def make_contract():
    return Contract({
        'interfaces': [{
            'name': 'Demo',
            'methods': [
                {
                    'name': 'echo',
                    'parameters': [{'name': 'value', 'type': {'builtIn': 'string'}}],
                    'returnType': {'builtIn': 'string'},
                },
                {
                    'name': 'required',
                    'parameters': [],
                    'returnType': {'builtIn': 'string'},
                },
                {
                    'name': 'optional',
                    'parameters': [],
                    'returnType': {'builtIn': 'string'},
                    'returnOptional': True,
                },
                {'name': 'void', 'parameters': []},
                {
                    'name': 'boom',
                    'parameters': [],
                    'returnType': {'builtIn': 'string'},
                },
                {
                    'name': 'nullable',
                    'parameters': [],
                    'returnType': {'userDefined': 'Payload'},
                },
            ],
        }],
        'structs': [{
            'name': 'Payload',
            'fields': [
                {'name': 'email', 'type': {'builtIn': 'string'}, 'optional': False}
            ],
        }],
        'enums': [],
    })


class DemoHandler:
    def __init__(self):
        self.calls = []

    def echo(self, ctx, value):
        self.calls.append(('echo', ctx, value))
        return value

    def required(self, ctx):
        return None

    def optional(self, ctx):
        return None

    def void(self, ctx):
        return None

    def boom(self, ctx):
        raise RuntimeError('kaboom')

    def nullable(self, ctx):
        return {'email': None}


def make_server(**kwargs):
    server = Server(make_contract(), **kwargs)
    server.add_handler('Demo', DemoHandler())
    return server


def request(method, params=None, req_id=1, jsonrpc='2.0'):
    req = {'jsonrpc': jsonrpc, 'method': method}
    if params is not None:
        req['params'] = params
    if req_id is not None:
        req['id'] = req_id
    return req


def error_code(response):
    return response['error']['code']


# --- python-08: all four standard error branches -------------------------------

def test_bad_jsonrpc_returns_invalid_request():
    response = make_server().call(request('Demo.echo', ['x'], jsonrpc='1.0'))
    assert error_code(response) == INVALID_REQUEST == -32600
    assert response['id'] == 1


def test_missing_method_returns_invalid_request():
    response = make_server().call({'jsonrpc': '2.0', 'method': '', 'id': 1})
    assert error_code(response) == INVALID_REQUEST


def test_unknown_interface_returns_method_not_found():
    response = make_server().call(request('Nope.foo', ['x']))
    assert error_code(response) == METHOD_NOT_FOUND == -32601


def test_unknown_method_returns_method_not_found():
    response = make_server().call(request('Demo.nope', ['x']))
    assert error_code(response) == METHOD_NOT_FOUND


def test_bad_params_type_returns_invalid_params():
    response = make_server().call(request('Demo.echo', {'value': 'x'}))
    assert error_code(response) == INVALID_PARAMS == -32602


def test_handler_exception_returns_internal_error():
    errors = []
    server = make_server(on_error=errors.append)
    response = server.call(request('Demo.boom'))
    assert error_code(response) == INTERNAL_ERROR == -32603
    assert len(errors) == 1


def test_non_dict_request_returns_invalid_request():
    response = make_server().call('not-a-dict')
    assert error_code(response) == INVALID_REQUEST


# --- python-03: null results must be validated --------------------------------

def test_required_null_result_is_rejected():
    response = make_server().call(request('Demo.required'))
    assert error_code(response) == INTERNAL_ERROR
    assert 'invalid response' in response['error']['message']


def test_optional_null_result_is_allowed():
    response = make_server().call(request('Demo.optional'))
    assert response['result'] is None
    assert 'error' not in response


def test_void_result_is_allowed():
    response = make_server().call(request('Demo.void'))
    assert response['result'] is None


# --- python-05: notifications never receive a response ------------------------

def test_pulserpc_idl_notification_returns_none():
    assert make_server().call({'jsonrpc': '2.0', 'method': 'pulserpc-idl'}) is None


def test_bad_jsonrpc_notification_returns_none():
    req = request('Demo.echo', ['x'], req_id=None, jsonrpc='1.0')
    assert make_server().call(req) is None


def test_bad_method_notification_returns_none():
    assert make_server().call({'jsonrpc': '2.0', 'method': 123}) is None


def test_normal_method_notification_returns_none_but_runs_handler():
    server = make_server()
    assert server.call(request('Demo.echo', ['x'], req_id=None)) is None
    assert server.handlers['Demo'].calls[0][2] == 'x'


def test_idl_request_with_id_still_returns_idl():
    response = make_server().call(request('pulserpc-idl', req_id='bootstrap'))
    assert response['id'] == 'bootstrap'
    assert response['result'] == make_contract().idl_parsed


# --- python-09: batch requests -------------------------------------------------

def test_batch_returns_one_response_per_request():
    responses = make_server().call([
        request('Demo.echo', ['a'], req_id=1),
        request('Demo.echo', ['b'], req_id=2),
    ])
    assert isinstance(responses, list)
    assert [r['id'] for r in responses] == [1, 2]
    assert [r['result'] for r in responses] == ['a', 'b']


def test_batch_of_notifications_returns_none():
    responses = make_server().call([
        request('Demo.echo', ['a'], req_id=None),
        request('Demo.echo', ['b'], req_id=None),
    ])
    assert responses is None


def test_mixed_batch_omits_notification_responses():
    responses = make_server().call([
        request('Demo.echo', ['a'], req_id=None),
        request('Demo.echo', ['b'], req_id=7),
    ])
    assert len(responses) == 1
    assert responses[0]['id'] == 7


def test_empty_batch_is_invalid_request():
    assert error_code(make_server().call([])) == INVALID_REQUEST


def test_batch_includes_errors_for_invalid_entries():
    responses = make_server().call([
        'not-a-dict',
        request('Demo.echo', ['ok'], req_id=2),
    ])
    assert len(responses) == 2
    assert error_code(responses[0]) == INVALID_REQUEST
    assert responses[1]['result'] == 'ok'


# --- review finding 1: bounded batch processing -------------------------------

def test_oversized_batch_is_rejected_without_running_handlers():
    server = make_server()
    batch = [
        request('Demo.echo', ['x'], req_id=i)
        for i in range(DEFAULT_MAX_BATCH_SIZE + 1)
    ]
    response = server.call(batch)
    assert error_code(response) == INVALID_REQUEST
    assert 'id' not in response  # a rejected batch has no request id
    # The whole batch is refused up front: no per-entry amplification.
    assert server.handlers['Demo'].calls == []


def test_max_batch_size_is_configurable():
    server = make_server(max_batch_size=2)
    batch = [
        request('Demo.echo', ['a'], 1),
        request('Demo.echo', ['b'], 2),
        request('Demo.echo', ['c'], 3),
    ]
    assert error_code(server.call(batch)) == INVALID_REQUEST
    assert server.handlers['Demo'].calls == []


def test_normal_batch_within_limit_still_works():
    server = make_server(max_batch_size=2)
    responses = server.call([
        request('Demo.echo', ['a'], 1),
        request('Demo.echo', ['b'], 2),
    ])
    assert [r['id'] for r in responses] == [1, 2]
    assert [r['result'] for r in responses] == ['a', 'b']
    assert len(server.handlers['Demo'].calls) == 2


def test_max_batch_size_must_be_positive_integer():
    with pytest.raises(ValueError):
        Server(make_contract(), max_batch_size=0)
    with pytest.raises(ValueError):
        Server(make_contract(), max_batch_size=-1)


# --- python-10: structured validation-error data on the wire -------------------

def test_request_validation_error_data_shape():
    response = make_server().call(request('Demo.echo', [42]))
    assert error_code(response) == INVALID_PARAMS
    data = response['error']['data']
    assert set(data) == {'valid', 'error', 'invalid_fields'}
    assert data['valid'] is False
    assert data['invalid_fields'] == ['param[0]']
    assert 'Expected string' in data['error']


def test_response_validation_error_data_shape():
    response = make_server().call(request('Demo.nullable'))
    assert error_code(response) == INTERNAL_ERROR
    data = response['error']['data']
    assert set(data) == {'valid', 'error', 'invalid_fields'}
    assert data['valid'] is False
    assert data['invalid_fields'] == ['.email']
    assert '.email' in data['error']
