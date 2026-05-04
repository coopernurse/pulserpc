"""Tests for RPC error handling"""

import pytest
from pulserpc import RPCError


def test_rpc_error_creation():
    """Test creating an RPCError"""
    error = RPCError(-32603, "Internal error", {"detail": "Something went wrong"})
    assert error.code == -32603
    assert error.message == "Internal error"
    assert error.data == {"detail": "Something went wrong"}


def test_rpc_error_without_data():
    """Test creating an RPCError without data"""
    error = RPCError(-32600, "Invalid Request")
    assert error.code == -32600
    assert error.message == "Invalid Request"
    assert error.data is None


def test_rpc_error_string_representation():
    """Test RPCError string representation"""
    error = RPCError(-32601, "Method not found")
    assert "RPCError" in str(error)
    assert "-32601" in str(error)
    assert "Method not found" in str(error)


class MockHandler:
    """Mock handler that captures the ctx parameter"""
    def __init__(self):
        self.last_ctx = None

    def test_method(self, ctx=None):
        self.last_ctx = ctx
        return "test_value"


class TestServerCtx:
    """Tests for ctx parameter passing in Server.call()"""

    def test_call_with_ctx_passes_to_handler(self):
        """Test that ctx is passed to handler when provided"""
        from pulserpc import Server, Contract
        import json

        # Create a minimal contract (we'll mock the validation)
        idl = {
            "interfaces": [
                {
                    "name": "TestService",
                    "methods": [
                        {
                            "name": "test_method",
                            "parameters": [],
                            "returnType": {"builtIn": "string"}
                        }
                    ]
                }
            ],
            "structs": [],
            "enums": []
        }

        contract = Contract(idl)
        server = Server(contract, validate_requests=False)
        handler = MockHandler()
        server.add_handler("TestService", handler)

        ctx_value = {"auth": "token123", "requestId": "abc"}
        req = {
            "jsonrpc": "2.0",
            "method": "TestService.test_method",
            "params": [],
            "id": 1
        }

        response = server.call(req, ctx=ctx_value)

        assert response is not None
        assert response["result"] == "test_value"
        assert handler.last_ctx == ctx_value

    def test_call_without_ctx_defaults_to_none(self):
        """Test that ctx defaults to None when not provided"""
        from pulserpc import Server, Contract

        idl = {
            "interfaces": [
                {
                    "name": "TestService",
                    "methods": [
                        {
                            "name": "test_method",
                            "parameters": [{"name": "value", "type": "string"}],
                            "returnType": {"builtIn": "string"}
                        }
                    ]
                }
            ],
            "structs": [],
            "enums": []
        }

        contract = Contract(idl)
        server = Server(contract, validate_requests=False)
        handler = MockHandler()
        server.add_handler("TestService", handler)

        req = {
            "jsonrpc": "2.0",
            "method": "TestService.test_method",
            "params": ["test_value"],
            "id": 1
        }

        response = server.call(req)

        assert response is not None
        assert handler.last_ctx is None

    def test_handler_receives_correct_ctx_value(self):
        """Test that handler receives the exact ctx value passed to Server.call()"""
        from pulserpc import Server, Contract

        idl = {
            "interfaces": [
                {
                    "name": "TestService",
                    "methods": [
                        {
                            "name": "test_method",
                            "parameters": [],
                            "returnType": {"builtIn": "string"}
                        }
                    ]
                }
            ],
            "structs": [],
            "enums": []
        }

        contract = Contract(idl)
        server = Server(contract, validate_requests=False)
        handler = MockHandler()
        server.add_handler("TestService", handler)

        test_cases = [
            {"key": "value"},
            None,
            {"headers": {"Authorization": "Bearer token"}},
            {"traceId": "xyz123", "spanId": "abc"},
        ]

        for ctx in test_cases:
            req = {
                "jsonrpc": "2.0",
                "method": "TestService.test_method",
                "params": [],
                "id": 1
            }
            server.call(req, ctx=ctx)
            assert handler.last_ctx == ctx

