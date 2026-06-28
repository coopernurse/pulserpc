"""Server class for handling JSON-RPC 2.0 requests"""

from typing import Any, Callable, Dict, List, Optional
from .rpc import RPCError
from .contract import Contract


class Server:
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, FastAPI, etc.)
    """

    def __init__(self, contract: Contract, validate_requests: bool = True,
                 validate_responses: bool = True,
                 on_error: Optional[Callable[[Exception], None]] = None):
        """Initialize Server

        Args:
            contract: Contract instance for validation
            validate_requests: Validate request parameters against IDL
            validate_responses: Validate response values against IDL
            on_error: Optional callback invoked on unhandled handler exceptions
        """
        self.handlers: Dict[str, Any] = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses
        self.on_error = on_error

    def add_handler(self, iface_name: str, handler: Any) -> None:
        """Register a handler instance for an interface

        Args:
            iface_name: Interface name (e.g., "UserService")
            handler: Handler instance with methods matching the interface
        """
        self.handlers[iface_name] = handler

    def call(self, req: Dict[str, Any], ctx: Any = None) -> Optional[Dict[str, Any]]:
        """Process a single JSON-RPC request

        Args:
            req: JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'
            ctx: Optional context dict for transport-level metadata (headers, auth, etc.)
                Passed as the first positional argument to handler methods.

        Returns:
            JSON-RPC response dict, or None for notification (requests without 'id')

        Example:
            >>> request = {
            ...     'jsonrpc': '2.0',
            ...     'method': 'UserService.getUser',
            ...     'params': {'user_id': '123'},
            ...     'id': 1
            ... }
            >>> response = server.call(request)
        """
        # Validate request format
        if not isinstance(req, dict):
            return self._error_response(None, -32600, "Invalid Request", "Request must be an object")

        # Check JSON-RPC version
        if req.get('jsonrpc') != '2.0':
            return self._error_response(req.get('id'), -32600, "Invalid Request",
                                      "jsonrpc version must be '2.0'")

        # Check for method
        method = req.get('method')
        if not method or not isinstance(method, str):
            return self._error_response(req.get('id'), -32600, "Invalid Request",
                                      "Method must be a string")

        # Handle pulserpc-idl request
        if method == 'pulserpc-idl':
            req_id = req.get('id')
            return {
                'jsonrpc': '2.0',
                'result': self.contract.idl_parsed,
                'id': req_id
            }

        # Check for notification (no 'id' means no response expected)
        req_id = req.get('id')
        is_notification = req_id is None

        # Parse method name (e.g., "UserService.getUser")
        try:
            iface_name, func_name = method.rsplit('.', 1)
        except ValueError:
            return self._error_response(req_id, -32601, "Method not found",
                                      f"Invalid method name format: {method}")

        # Look up handler
        if iface_name not in self.handlers:
            return self._error_response(req_id, -32601, "Method not found",
                                      f"Unknown interface: {iface_name}")

        handler = self.handlers[iface_name]

        # Get function from handler
        if not hasattr(handler, func_name):
            return self._error_response(req_id, -32601, "Method not found",
                                      f"Unknown method: {method}")

        func = getattr(handler, func_name)

        # Get params
        params = req.get('params')

        if params is None:
            params = []

        if not isinstance(params, list):
            return self._error_response(req_id, -32602, "Invalid params",
                                      "Parameters must be an array")

        if self.validate_requests:
            try:
                self.contract.validate_request(iface_name, func_name, params)
            except RPCError as e:
                return self._error_response(req_id, e.code, e.message, e.data)

        # Invoke handler method
        try:
            result = func(ctx, *params)
        except TypeError as e:
            return self._error_response(req_id, -32602, "Invalid params",
                                      f"Parameter mismatch: {e}")
        except Exception as e:
            # Convert application errors to RPC errors
            if isinstance(e, RPCError):
                return self._error_response(req_id, e.code, e.message, e.data)
            else:
                # Log unexpected errors and return internal error
                if self.on_error is not None:
                    self.on_error(e)
                else:
                    import traceback
                    traceback.print_exc()
                return self._error_response(req_id, -32603, "Internal error",
                                          f"Handler exception: {e}")

        # Validate response if validation is enabled
        if self.validate_responses and result is not None:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except RPCError as e:
                return self._error_response(req_id, e.code, e.message, e.data)

        # Don't respond to notifications
        if is_notification:
            return None

        # Return success response
        return {
            'jsonrpc': '2.0',
            'result': result,
            'id': req_id
        }

    def _error_response(self, req_id: Any, code: int, message: str, data: Any = None) -> Dict[str, Any]:
        """Create a JSON-RPC error response

        Args:
            req_id: Request ID (can be None for notifications)
            code: Error code
            message: Error message
            data: Optional error data

        Returns:
            JSON-RPC error response dict
        """
        error: Dict[str, Any] = {
            'code': code,
            'message': message
        }
        if data is not None:
            error['data'] = data

        response = {
            'jsonrpc': '2.0',
            'error': error
        }
        if req_id is not None:
            response['id'] = req_id

        return response
