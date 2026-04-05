"""Server class for handling JSON-RPC 2.0 requests"""

from typing import Any, Dict, List, Optional
from .rpc import RPCError
from .contract import Contract


class Server:
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, FastAPI, etc.)
    """

    def __init__(self, contract: Contract, validate_requests: bool = True,
                 validate_responses: bool = True):
        """Initialize Server

        Args:
            contract: Contract instance for validation
            validate_requests: Validate request parameters against IDL
            validate_responses: Validate response values against IDL
        """
        self.handlers: Dict[str, Any] = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses

    def add_handler(self, iface_name: str, handler: Any) -> None:
        """Register a handler instance for an interface

        Args:
            iface_name: Interface name (e.g., "UserService")
            handler: Handler instance with methods matching the interface
        """
        self.handlers[iface_name] = handler

    def call(self, req: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """Process a single JSON-RPC request

        Args:
            req: JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'

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

        # Normalize params to dict if it's a list
        if isinstance(params, list):
            # Validate request using positional params
            if self.validate_requests:
                try:
                    self.contract.validate_request(iface_name, func_name, params)
                except (TypeError, ValueError) as e:
                    return self._error_response(req_id, -32602, "Invalid params", str(e))

            # Convert positional params to named params using IDL signature
            try:
                params = self._positional_to_named_params(iface_name, func_name, params)
            except ValueError as e:
                return self._error_response(req_id, -32602, "Invalid params", str(e))
        elif params is None:
            params = {}

        if not isinstance(params, dict):
            return self._error_response(req_id, -32602, "Invalid params",
                                      "Parameters must be an object or array")

        # Validate request if using named params (dict)
        if self.validate_requests and isinstance(req.get('params'), dict):
            # Convert dict to list for validation
            param_list = self._named_to_positional_params(iface_name, func_name, params)
            if param_list is not None:
                try:
                    self.contract.validate_request(iface_name, func_name, param_list)
                except (TypeError, ValueError) as e:
                    return self._error_response(req_id, -32602, "Invalid params", str(e))

        # Invoke handler method
        try:
            # Call handler function with params as kwargs
            result = func(**params)
        except TypeError as e:
            return self._error_response(req_id, -32602, "Invalid params",
                                      f"Parameter mismatch: {e}")
        except Exception as e:
            # Convert application errors to RPC errors
            if isinstance(e, RPCError):
                return self._error_response(req_id, e.code, e.message, e.data)
            else:
                # Log unexpected errors and return internal error
                import traceback
                traceback.print_exc()
                return self._error_response(req_id, -32603, "Internal error",
                                          f"Handler exception: {e}")

        # Validate response if validation is enabled
        if self.validate_responses and result is not None:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except (TypeError, ValueError) as e:
                return self._error_response(req_id, -32603, "Internal error",
                                          f"Response validation failed: {e}")

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

    def _positional_to_named_params(self, iface_name: str, func_name: str,
                                     positional_params: List[Any]) -> Dict[str, Any]:
        """Convert positional parameters to named parameters using IDL signature

        Args:
            iface_name: Interface name
            func_name: Function name
            positional_params: List of positional parameter values

        Returns:
            Dict mapping parameter names to values

        Raises:
            ValueError: If parameter count doesn't match or interface not found
        """
        interface = self.contract.get_interface(iface_name)
        if not interface:
            # Without contract, can't map positional to named
            return {str(i): v for i, v in enumerate(positional_params)}

        func = interface.get_function(func_name)
        if not func:
            return {str(i): v for i, v in enumerate(positional_params)}

        # Get parameter names from IDL
        param_defs = func.get('parameters', [])

        # Check parameter count
        if len(positional_params) != len(param_defs):
            # Allow fewer params if trailing ones are optional
            # But require at least the required params
            required_count = sum(1 for p in param_defs if not p.get('optional', False))
            if len(positional_params) < required_count or len(positional_params) > len(param_defs):
                raise ValueError(f"Parameter count mismatch: expected {len(param_defs)}, got {len(positional_params)}")

        # Map positional params to names
        named_params = {}
        for i, param_value in enumerate(positional_params):
            if i < len(param_defs):
                param_name = param_defs[i]['name']
                named_params[param_name] = param_value
            else:
                # Fallback for extra params (shouldn't happen with validation)
                named_params[str(i)] = param_value

        return named_params

    def _named_to_positional_params(self, iface_name: str, func_name: str,
                                     named_params: Dict[str, Any]) -> Optional[List[Any]]:
        """Convert named parameters to positional parameters using IDL signature

        Args:
            iface_name: Interface name
            func_name: Function name
            named_params: Dict mapping parameter names to values

        Returns:
            List of parameter values in IDL order, or None if interface not found
        """
        interface = self.contract.get_interface(iface_name)
        if not interface:
            return None

        func = interface.get_function(func_name)
        if not func:
            return None

        # Get parameter names from IDL
        param_defs = func.get('parameters', [])

        # Build positional list in IDL order
        positional_params = []
        for param_def in param_defs:
            param_name = param_def['name']
            positional_params.append(named_params.get(param_name))

        return positional_params
