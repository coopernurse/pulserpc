"""RPC error handling for JSON-RPC 2.0"""

from typing import Any

# Standard JSON-RPC 2.0 error codes.  These named constants are shared by the
# server, client and contract validator so that no magic numbers are scattered
# through the runtime (audit python-11).
PARSE_ERROR = -32700
INVALID_REQUEST = -32600
METHOD_NOT_FOUND = -32601
INVALID_PARAMS = -32602
INTERNAL_ERROR = -32603


class RPCError(Exception):
    """Exception class for JSON-RPC errors"""
    
    def __init__(self, code: int, message: str, data: Any = None):
        self.code = code
        self.message = message
        self.data = data
        super().__init__(f"RPCError {code}: {message}")
