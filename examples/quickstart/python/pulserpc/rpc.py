"""RPC error handling for JSON-RPC 2.0"""

from typing import Any


class RPCError(Exception):
    """Exception class for JSON-RPC errors"""
    
    def __init__(self, code: int, message: str, data: Any = None):
        self.code = code
        self.message = message
        self.data = data
        super().__init__(f"RPCError {code}: {message}")

