"""Tests for quickstart ctx parameter support"""

import json
import os
import sys

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from pulserpc import Server, Contract, RPCError
from checkout.server import CatalogService, CartService, OrderService


class MockCatalogService(CatalogService):
    """Test implementation that captures ctx"""

    def __init__(self):
        self.last_ctx = None

    def listProducts(self, ctx):
        self.last_ctx = ctx
        return [{"productId": "prod001", "name": "Test Product"}]

    def getProduct(self, ctx, productId):
        self.last_ctx = ctx
        return {"productId": productId, "name": "Test Product"}


class TestQuickstartCtx:
    """Tests for ctx passing in quickstart examples"""

    def setup_method(self):
        """Set up test fixtures"""
        # Load IDL
        with open(os.path.join(os.path.dirname(__file__), 'idl.json'), 'r') as f:
            idl_data = json.load(f)
        self.contract = Contract(idl_data)
        self.handler = MockCatalogService()
        self.server = Server(self.contract, validate_requests=True)
        self.server.add_handler("CatalogService", self.handler)

    def test_handler_receives_ctx_when_provided(self):
        """Test that ctx is passed to handler when provided"""
        ctx_value = {"auth": "Bearer token123", "requestId": "req-456"}
        req = {
            "jsonrpc": "2.0",
            "method": "CatalogService.listProducts",
            "params": {},
            "id": 1
        }

        response = self.server.call(req, ctx=ctx_value)

        assert response is not None
        assert response["result"] is not None
        assert self.handler.last_ctx == ctx_value

    def test_handler_receives_none_when_ctx_not_provided(self):
        """Test that ctx defaults to None when not provided"""
        req = {
            "jsonrpc": "2.0",
            "method": "CatalogService.listProducts",
            "params": {},
            "id": 1
        }

        response = self.server.call(req)

        assert response is not None
        assert self.handler.last_ctx is None

    def test_handler_receives_various_ctx_types(self):
        """Test that handler receives various ctx types correctly"""
        test_cases = [
            {"headers": {"Authorization": "Bearer token"}},
            {"traceId": "abc123", "spanId": "def456"},
            None,
            {"custom": "value", "nested": {"key": "val"}},
        ]

        for ctx in test_cases:
            req = {
                "jsonrpc": "2.0",
                "method": "CatalogService.getProduct",
                "params": {"productId": "prod001"},
                "id": 1
            }
            self.server.call(req, ctx=ctx)
            assert self.handler.last_ctx == ctx

    def test_getProduct_with_ctx(self):
        """Test getProduct handler receives ctx"""
        ctx_value = {"userId": "user123"}
        req = {
            "jsonrpc": "2.0",
            "method": "CatalogService.getProduct",
            "params": {"productId": "prod001"},
            "id": 1
        }

        response = self.server.call(req, ctx=ctx_value)

        assert response is not None
        assert response["result"]["productId"] == "prod001"
        assert self.handler.last_ctx == ctx_value


if __name__ == "__main__":
    import pytest
    sys.exit(pytest.main([__file__, "-v"]))
