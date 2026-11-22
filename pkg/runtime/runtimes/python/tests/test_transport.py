"""HTTP transport invariants: response cap (R17), timeout (R19), custom headers
(R29) and base-URL normalization (R30)."""

import json

import pytest

from pulserpc import HttpTransport, DEFAULT_TIMEOUT, MAX_RESPONSE_BODY_SIZE


class FakeResponse:
    def __init__(self, body):
        self._body = body

    def read(self, n=-1):
        if n is None or n < 0:
            return self._body
        return self._body[:n]

    def __enter__(self):
        return self

    def __exit__(self, *exc_info):
        return False


class RecordingUrlopen:
    def __init__(self, response_body=b'{}', status_error=None):
        self.response_body = response_body
        self.status_error = status_error
        self.calls = []

    def __call__(self, request, timeout=None):
        self.calls.append((request, timeout))
        if self.status_error is not None:
            raise self.status_error
        return FakeResponse(self.response_body)


def install_urlopen(monkeypatch, fake):
    monkeypatch.setattr(
        'pulserpc.transport.urllib.request.urlopen', fake
    )


def header_items(request):
    return {key.lower(): value for key, value in request.header_items()}


def test_default_timeout_and_response_cap():
    transport = HttpTransport('http://example.com/api')
    assert transport.timeout == DEFAULT_TIMEOUT == 30
    assert transport.max_response_size == MAX_RESPONSE_BODY_SIZE == 10 * 1024 * 1024


def test_timeout_is_passed_to_urlopen(monkeypatch):
    fake = RecordingUrlopen(
        json.dumps({'jsonrpc': '2.0', 'result': 1, 'id': 1}).encode('utf-8')
    )
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport('http://example.com/api', timeout=7)
    transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1})

    assert len(fake.calls) == 1
    assert fake.calls[0][1] == 7


def test_custom_headers_are_merged(monkeypatch):
    fake = RecordingUrlopen()
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport(
        'http://example.com/api',
        headers={'Authorization': 'Bearer token', 'X-Trace': 'abc'},
    )
    transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1})

    headers = header_items(fake.calls[0][0])
    assert headers['authorization'] == 'Bearer token'
    assert headers['x-trace'] == 'abc'
    assert headers['content-type'] == 'application/json'


def test_trailing_slash_is_normalized(monkeypatch):
    fake = RecordingUrlopen()
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport('http://example.com/api/')
    assert transport.url == 'http://example.com/api'
    transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1})
    assert fake.calls[0][0].full_url == 'http://example.com/api'


def test_response_at_the_cap_is_accepted(monkeypatch):
    payload = b'{"pad": "' + b'a' * 100 + b'"}'
    fake = RecordingUrlopen(payload)
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport('http://example.com/api', max_response_size=len(payload))
    assert transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1}) == {
        'pad': 'a' * 100
    }
    assert fake.calls[0][0] is not None


def test_oversized_response_is_rejected(monkeypatch):
    payload = b'{"pad": "' + b'a' * 100 + b'"}'
    fake = RecordingUrlopen(payload)
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport(
        'http://example.com/api', max_response_size=len(payload) - 1
    )
    with pytest.raises(ValueError) as exc:
        transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1})
    assert 'maximum size' in str(exc.value)


def test_empty_response_returns_empty_dict(monkeypatch):
    fake = RecordingUrlopen(b'')
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport('http://example.com/api')
    assert transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1}) == {}


def test_invalid_json_raises_value_error(monkeypatch):
    fake = RecordingUrlopen(b'not json')
    install_urlopen(monkeypatch, fake)

    transport = HttpTransport('http://example.com/api')
    with pytest.raises(ValueError) as exc:
        transport.request({'jsonrpc': '2.0', 'method': 'x', 'id': 1})
    assert 'Invalid JSON response' in str(exc.value)
