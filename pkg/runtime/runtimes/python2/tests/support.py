"""Shared fixtures for the Python 2 runtime tests.

All fixtures are plain dicts/classes so they run under Python 2.7 with no
third-party dependencies (no pytest, no dataclasses, no enum).
"""

from pulserpc import RPCError


def sample_idl():
    """Return a minimal IDL dict exercising all validation paths."""
    return {
        "interfaces": [
            {
                "name": "CalcService",
                "methods": [
                    {
                        "name": "add",
                        "parameters": [
                            {"name": "a", "type": {"builtIn": "int"}},
                            {"name": "b", "type": {"builtIn": "int"}},
                        ],
                        "returnType": {"builtIn": "int"},
                    },
                    {
                        "name": "echo",
                        "parameters": [
                            {"name": "value", "type": {"builtIn": "string"}}
                        ],
                        "returnType": {"builtIn": "string"},
                    },
                    {"name": "ping", "parameters": [], "returnType": None},
                    {
                        "name": "getPerson",
                        "parameters": [],
                        "returnType": {"userDefined": "Person"},
                    },
                    {
                        "name": "fail",
                        "parameters": [],
                        "returnType": {"builtIn": "string"},
                    },
                ],
            }
        ],
        "structs": [
            {
                "name": "Person",
                "fields": [
                    {"name": "name", "type": {"builtIn": "string"}},
                    {"name": "age", "type": {"builtIn": "int"}, "optional": True},
                ],
            }
        ],
        "enums": [],
    }


class CalcServiceImpl(object):
    """Handler used by the contract/server/client tests."""

    def __init__(self, person=None, fail_error=None, boom=False):
        self.person = person
        self.fail_error = fail_error
        self.boom = boom

    def add(self, ctx, a, b):
        return a + b

    def echo(self, ctx, value):
        return value

    def ping(self, ctx):
        return None

    def getPerson(self, ctx):
        return self.person

    def fail(self, ctx):
        if self.boom:
            raise RuntimeError("kaboom")
        if self.fail_error is not None:
            raise self.fail_error
        return "ok"


def assert_rpc_error(code, fn, *args, **kwargs):
    """Run ``fn`` and assert it raises an RPCError with the expected code."""
    try:
        fn(*args, **kwargs)
    except RPCError as e:
        assert e.code == code, "expected code %r, got %r" % (code, e.code)
        return e
    raise AssertionError("expected RPCError with code %r" % code)
