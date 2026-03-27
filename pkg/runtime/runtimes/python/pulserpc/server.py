"""Server class for handling JSON-RPC 2.0 requests"""

from typing import Any, Dict, List, Optional
from .rpc import RPCError
from .validation import validate_type, find_struct, find_enum, get_struct_fields


class Server:
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, FastAPI, etc.)
    """

    def __init__(self, validate_requests: bool = True, validate_responses: bool = True):
        """Initialize Server

        Args:
            validate_requests: Validate request parameters against IDL
            validate_responses: Validate response values against IDL
        """
        self.handlers: Dict[str, Any] = {}
        self.idl_data: Optional[Dict[str, Any]] = None
        self.all_structs: Dict[str, Any] = {}
        self.all_enums: Dict[str, Any] = {}
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses

    def add_handler(self, iface_name: str, handler: Any) -> None:
        """Register a handler instance for an interface

        Args:
            iface_name: Interface name (e.g., "UserService")
            handler: Handler instance with methods matching the interface
        """
        self.handlers[iface_name] = handler

    def load_idl(self, idl_data: Dict[str, Any], all_structs: Dict[str, Any] = None,
                 all_enums: Dict[str, Any] = None) -> None:
        """Load IDL metadata for validation

        Args:
            idl_data: Full IDL data dict from parser
            all_structs: Optional merged structs dict (will be extracted from idl_data if not provided)
            all_enums: Optional merged enums dict (will be extracted from idl_data if not provided)
        """
        self.idl_data = idl_data

        # Extract structs and enums from IDL if not provided
        if all_structs is None:
            all_structs = {}
            for struct in idl_data.get('structs', []):
                all_structs[struct['name']] = struct

        if all_enums is None:
            all_enums = {}
            for enum in idl_data.get('enums', []):
                all_enums[enum['name']] = enum

        self.all_structs = all_structs
        self.all_enums = all_enums

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

        # Validate request if IDL is loaded and validation is enabled
        if self.validate_requests and self.idl_data:
            validation_error = self._validate_request_params(iface_name, func_name, params)
            if validation_error:
                return self._error_response(req_id, -32602, "Invalid params", validation_error)

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
        if self.validate_responses and self.idl_data and result is not None:
            validation_error = self._validate_response_result(iface_name, func_name, result)
            if validation_error:
                return self._error_response(req_id, -32603, "Internal error",
                                          f"Response validation failed: {validation_error}")

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

    def _validate_request_params(self, iface_name: str, func_name: str,
                                  params: Dict[str, Any]) -> Optional[str]:
        """Validate request parameters against IDL

        Args:
            iface_name: Interface name
            func_name: Function name
            params: Parameters dict

        Returns:
            Error message string, or None if validation passes
        """
        if not self.idl_data:
            return None

        # Find interface in IDL
        interface = None
        for iface in self.idl_data.get('interfaces', []):
            if iface['name'] == iface_name:
                interface = iface
                break

        if not interface:
            return f"Interface not found in IDL: {iface_name}"

        # Find function in interface
        func = None
        for f in interface.get('methods', []):
            if f['name'] == func_name:
                func = f
                break

        if not func:
            return f"Function not found in IDL: {iface_name}.{func_name}"

        # Validate each parameter
        for param in func.get('parameters', []):
            param_name = param['name']
            param_type = param['type']
            is_optional = param.get('optional', False)

            if param_name not in params:
                if not is_optional:
                    return f"Missing required parameter: {param_name}"
            else:
                param_value = params[param_name]
                try:
                    validate_type(param_value, param_type, self.all_structs,
                                self.all_enums, is_optional)
                except (TypeError, ValueError) as e:
                    return f"Parameter '{param_name}' validation failed: {e}"

        return None

    def _validate_response_result(self, iface_name: str, func_name: str,
                                   result: Any) -> Optional[str]:
        """Validate response result against IDL

        Args:
            iface_name: Interface name
            func_name: Function name
            result: Result value

        Returns:
            Error message string, or None if validation passes
        """
        if not self.idl_data:
            return None

        # Find interface in IDL
        interface = None
        for iface in self.idl_data.get('interfaces', []):
            if iface['name'] == iface_name:
                interface = iface
                break

        if not interface:
            return f"Interface not found in IDL: {iface_name}"

        # Find function in interface
        func = None
        for f in interface.get('methods', []):
            if f['name'] == func_name:
                func = f
                break

        if not func:
            return f"Function not found in IDL: {iface_name}.{func_name}"

        # Check if function has a return type
        return_type = func.get('returnType')
        if not return_type:
            # Function returns void/None
            if result is not None:
                return f"Function should return None, got: {type(result).__name__}"
            return None

        # Validate return type
        is_optional = func.get('returnOptional', False)
        try:
            validate_type(result, return_type, self.all_structs,
                        self.all_enums, is_optional)
        except (TypeError, ValueError) as e:
            return f"Return type validation failed: {e}"

        return None

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
            ValueError: If parameter count doesn't match or IDL not loaded
        """
        if not self.idl_data:
            # Without IDL, can't map positional to named
            return {str(i): v for i, v in enumerate(positional_params)}

        # Find interface in IDL
        interface = None
        for iface in self.idl_data.get('interfaces', []):
            if iface['name'] == iface_name:
                interface = iface
                break

        if not interface:
            return {str(i): v for i, v in enumerate(positional_params)}

        # Find function in interface
        func = None
        for f in interface.get('methods', []):
            if f['name'] == func_name:
                func = f
                break

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
