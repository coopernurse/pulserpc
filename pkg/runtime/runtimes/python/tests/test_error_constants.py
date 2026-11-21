"""Tests for named JSON-RPC error constants (audit python-11)."""

from pathlib import Path

import pulserpc
from pulserpc import (
    PARSE_ERROR,
    INVALID_REQUEST,
    METHOD_NOT_FOUND,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)


def test_constants_are_exported_with_standard_values():
    assert PARSE_ERROR == -32700
    assert INVALID_REQUEST == -32600
    assert METHOD_NOT_FOUND == -32601
    assert INVALID_PARAMS == -32602
    assert INTERNAL_ERROR == -32603
    for name in ('INVALID_REQUEST', 'METHOD_NOT_FOUND', 'INVALID_PARAMS', 'INTERNAL_ERROR'):
        assert name in pulserpc.__all__


def test_runtime_modules_have_no_bare_error_codes():
    runtime_dir = Path(pulserpc.__file__).parent
    for module in ('server.py', 'client.py', 'contract.py'):
        source = (runtime_dir / module).read_text(encoding='utf-8')
        for literal in ('-32600', '-32601', '-32602', '-32603'):
            assert literal not in source, f'{literal} still in {module}'
