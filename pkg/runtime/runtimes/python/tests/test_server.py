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
    RPCError,
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


# --- R15/D4: internal errors never leak implementation details ----------------

def test_handler_exception_does_not_leak_exception_text():
    response = make_server(on_error=lambda e: None).call(request('Demo.boom'))
    assert response['error']['message'] == 'Internal error'
    assert 'data' not in response['error']
    assert 'kaboom' not in str(response)
    assert 'RuntimeError' not in str(response)


def test_handler_exception_without_callback_is_logged_not_returned(caplog):
    server = make_server()
    with caplog.at_level('ERROR', logger='pulserpc.server'):
        response = server.call(request('Demo.boom'))
    assert response['error'] == {'code': INTERNAL_ERROR, 'message': 'Internal error'}
    assert 'kaboom' in caplog.text  # logged server-side only


def test_declared_rpc_error_keeps_message_and_data():
    class BoomHandler(DemoHandler):
        def boom(self, ctx):
            raise RPCError(1234, 'declared failure', {'detail': 'kept'})

    server = Server(make_contract())
    server.add_handler('Demo', BoomHandler())
    response = server.call(request('Demo.boom'))
    assert response['error'] == {
        'code': 1234,
        'message': 'declared failure',
        'data': {'detail': 'kept'},
    }


def test_handler_body_type_error_is_redacted_as_internal_error():
    class BoomHandler(DemoHandler):
        def boom(self, ctx):
            raise TypeError('secret internal detail')

    server = Server(make_contract(), on_error=lambda e: None)
    server.add_handler('Demo', BoomHandler())
    response = server.call(request('Demo.boom'))
    assert error_code(response) == INTERNAL_ERROR
    assert response['error']['message'] == 'Internal error'
    assert 'secret' not in str(response)


def test_call_signature_mismatch_is_invalid_params_without_detail():
    server = make_server(validate_requests=False)
    response = server.call(request('Demo.echo', []))
    assert error_code(response) == INVALID_PARAMS
    assert response['error'] == {
        'code': INVALID_PARAMS,
        'message': 'Invalid params',
        'data': 'Parameter mismatch',
    }


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


# --- C4/C5: pulserpc-idl serves the IDL as a JSON object ---------------------

def test_pulserpc_idl_result_is_object_with_root_namespace_and_checksum():
    idl = {
        'rootNamespace': 'demo',
        'checksum': 'A' * 43,
        'interfaces': [],
        'structs': [],
        'enums': [],
    }
    server = Server(Contract(idl))
    response = server.call(request('pulserpc-idl', req_id=3))
    assert isinstance(response['result'], dict)
    assert response['result']['rootNamespace'] == 'demo'
    assert response['result']['checksum'] == 'A' * 43


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
    # A rejected batch has no request id, but the key is still emitted as null
    # (M18/M26).
    assert 'id' in response and response['id'] is None
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


# --- R24/D2: ids are string, number or null -----------------------------------

def test_non_string_number_or_null_ids_are_invalid_request():
    for bad_id in (True, False, {}, [], ['x'], float('nan'), float('inf')):
        response = make_server().call(request('Demo.echo', ['x'], req_id=bad_id))
        assert error_code(response) == INVALID_REQUEST
        assert response['id'] is None
        assert response['error']['message'] == 'Invalid Request'


def test_string_and_numeric_ids_are_accepted_and_echoed():
    for good_id in ('abc', 0, 7, -3, 1.5):
        response = make_server().call(request('Demo.echo', ['x'], req_id=good_id))
        assert response['result'] == 'x'
        assert response['id'] == good_id


def test_invalid_id_notification_also_returns_error_object():
    # An invalid id is not a notification: it must produce a -32600 with
    # id:null rather than being silently dropped.
    req = {'jsonrpc': '2.0', 'method': 'Demo.echo', 'params': ['x'], 'id': True}
    response = make_server().call(req)
    assert response == {
        'jsonrpc': '2.0',
        'error': {
            'code': INVALID_REQUEST,
            'message': 'Invalid Request',
            'data': 'Request id must be a string, number, or null',
        },
        'id': None,
    }


# --- M18/M26: every error response carries an explicit id key ------------------

def test_error_responses_always_emit_an_id_key():
    server = make_server()

    # An id that cannot be echoed back is reported as null, not omitted.
    invalid = server.call({'jsonrpc': '2.0', 'method': 'Demo.echo', 'id': []})
    assert 'id' in invalid and invalid['id'] is None

    # Transport-level batch errors have no request id either.
    empty_batch = server.call([])
    assert 'id' in empty_batch and empty_batch['id'] is None

    # A normal error still echoes the request id it was given.
    normal = server.call(request('Demo.nope', req_id='abc'))
    assert normal['id'] == 'abc'


# --- review finding: handler signatures are introspected once per method -------

def test_handler_signature_is_cached_per_method(monkeypatch):
    import pulserpc.server as server_module

    real = server_module.inspect.signature
    calls = []

    def counting(func):
        calls.append(func)
        return real(func)

    monkeypatch.setattr(server_module.inspect, 'signature', counting)
    server = make_server()
    assert server.call(request('Demo.echo', ['a']))['result'] == 'a'
    assert server.call(request('Demo.echo', ['b']))['result'] == 'b'

    echo_lookups = [f for f in calls if getattr(f, '__name__', None) == 'echo']
    assert len(echo_lookups) == 1


def test_add_handler_clears_signature_cache():
    server = make_server(validate_requests=False)
    assert server.call(request('Demo.echo', ['x']))['result'] == 'x'

    class NarrowEcho(DemoHandler):
        def echo(self, ctx):
            return 'narrow'

    server.add_handler('Demo', NarrowEcho())
    # A stale cached signature would still accept the extra argument.
    assert error_code(server.call(request('Demo.echo', ['x']))) == INVALID_PARAMS
    assert server.call(request('Demo.echo', []))['result'] == 'narrow'


# --- M18/M21-M28: message-validation invariants -------------------------------

def test_m18_empty_batch_is_single_error_object_not_array():
    response = make_server().call([])
    assert isinstance(response, dict)
    assert error_code(response) == INVALID_REQUEST


def test_m19_non_object_batch_entries_get_per_entry_invalid_request():
    responses = make_server().call([
        1,
        request('Demo.echo', ['ok'], req_id=2),
        [],
    ])
    assert isinstance(responses, list)
    assert len(responses) == 3
    assert error_code(responses[0]) == INVALID_REQUEST
    assert responses[1]['result'] == 'ok'
    assert error_code(responses[2]) == INVALID_REQUEST


def test_m21_id_null_is_a_notification():
    req = {'jsonrpc': '2.0', 'method': 'Demo.echo', 'params': ['x'], 'id': None}
    assert make_server().call(req) is None


def test_m22_error_data_is_omitted_when_nil():
    response = make_server(on_error=lambda e: None).call(request('Demo.boom'))
    assert error_code(response) == INTERNAL_ERROR
    assert 'data' not in response['error']


def test_m23_null_result_is_explicit():
    response = make_server().call(request('Demo.optional'))
    assert 'result' in response and response['result'] is None


def test_m25_error_objects_have_numeric_code_and_string_message():
    response = make_server().call(request('Demo.nope'))
    assert isinstance(response['error']['code'], int)
    assert isinstance(response['error']['message'], str)


def test_m27_jsonrpc_is_always_2_0():
    server = make_server()
    assert server.call(request('Demo.echo', ['x']))['jsonrpc'] == '2.0'
    assert server.call(request('Demo.nope'))['jsonrpc'] == '2.0'
    assert server.call(request('Demo.echo', ['x'], jsonrpc='1.0'))['jsonrpc'] == '2.0'


def test_m28_batch_notifications_are_dropped():
    responses = make_server().call([
        request('Demo.echo', ['a'], req_id=None),
        request('Demo.nope', req_id=None),
        request('Demo.echo', ['b'], req_id=9),
    ])
    assert len(responses) == 1
    assert responses[0] == {'jsonrpc': '2.0', 'result': 'b', 'id': 9}


# --- M20: object-form params are rejected explicitly --------------------------

def test_m20_object_form_params_are_rejected_with_invalid_params():
    for params in ({'value': 'x'}, {}):
        response = make_server().call(request('Demo.echo', params))
        assert error_code(response) == INVALID_PARAMS
        assert response['error']['message'] == 'Invalid params'
        assert 'Parameters must be an array' in response['error']['data']
