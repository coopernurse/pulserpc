"""Inbound request-body cap in the production server path (audit wave4)."""

import io

import pulserpc
from pulserpc import (
    DEFAULT_MAX_BODY_SIZE,
    BodyTooLargeError,
    read_request_body,
)


class _FakeRfile:
    """Minimal rfile double that records whether a read was attempted."""

    def __init__(self, data: bytes):
        self._stream = io.BytesIO(data)
        self.read_called = False

    def read(self, size: int) -> bytes:
        self.read_called = True
        return self._stream.read(size)


def test_default_cap_is_one_mib_and_exported():
    assert DEFAULT_MAX_BODY_SIZE == 1024 * 1024
    for name in ("DEFAULT_MAX_BODY_SIZE", "BodyTooLargeError", "read_request_body"):
        assert name in pulserpc.__all__


def test_body_exactly_at_cap_is_read():
    payload = b"x" * DEFAULT_MAX_BODY_SIZE
    rfile = _FakeRfile(payload)
    assert read_request_body(rfile, len(payload)) == payload
    assert rfile.read_called


def test_body_one_byte_over_cap_is_rejected_before_read():
    payload = b"x" * (DEFAULT_MAX_BODY_SIZE + 1)
    rfile = _FakeRfile(payload)
    try:
        read_request_body(rfile, len(payload))
    except BodyTooLargeError as exc:
        assert exc.limit == DEFAULT_MAX_BODY_SIZE
        assert "exceeds maximum size" in str(exc)
    else:
        raise AssertionError("oversized body must raise BodyTooLargeError")
    # The adapter answers with a JSON-RPC error; the body is never buffered.
    assert not rfile.read_called


def test_custom_cap_is_honored():
    rfile = _FakeRfile(b"abcd")
    assert read_request_body(rfile, 3, max_body_size=3) == b"abc"
    rfile_over = _FakeRfile(b"abcd")
    try:
        read_request_body(rfile_over, 4, max_body_size=3)
    except BodyTooLargeError:
        pass
    else:
        raise AssertionError("body over custom cap must raise BodyTooLargeError")
