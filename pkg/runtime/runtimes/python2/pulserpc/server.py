"""Server class for handling JSON-RPC 2.0 requests"""

from rpc import RPCError
from contract import Contract

_string_types = (str, type(u""))


class Server(object):
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, etc.)
    """

    def __init__(self, contract, validate_requests=True, validate_responses=True):
        """Initialize Server"""
        self.handlers = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses

    def add_handler(self, iface_name, handler):
        """Register a handler instance for an interface"""
        self.handlers[iface_name] = handler

    def call(self, req, ctx=None):
        """Process a single JSON-RPC request

        Args:
            req: JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'
            ctx: Optional context dict for transport-level metadata (headers, auth, etc.)
                Passed as the last positional argument to handler methods.
        """
        if not isinstance(req, dict):
            return self._error_response(None, -32600, "Invalid Request", "Request must be an object")

        if req.get('jsonrpc') != '2.0':
            return self._error_response(req.get('id'), -32600, "Invalid Request",
                                      "jsonrpc version must be '2.0'")

        method = req.get('method')
        if not method or not isinstance(method, _string_types):
            return self._error_response(req.get('id'), -32600, "Invalid Request",
                                      "Method must be a string")

        if method == 'pulserpc-idl':
            req_id = req.get('id')
            return {
                'jsonrpc': '2.0',
                'result': self.contract.idl_parsed,
                'id': req_id
            }

        req_id = req.get('id')
        is_notification = req_id is None

        try:
            iface_name, func_name = method.rsplit('.', 1)
        except ValueError:
            return self._error_response(req_id, -32601, "Method not found",
                                      "Invalid method name format: %s" % method)

        if iface_name not in self.handlers:
            return self._error_response(req_id, -32601, "Method not found",
                                      "Unknown interface: %s" % iface_name)

        handler = self.handlers[iface_name]

        if not hasattr(handler, func_name):
            return self._error_response(req_id, -32601, "Method not found",
                                      "Unknown method: %s" % method)

        func = getattr(handler, func_name)

        params = req.get('params')

        if params is None:
            params = []

        if not isinstance(params, list):
            return self._error_response(req_id, -32602, "Invalid params",
                                      "Parameters must be an array")

        if self.validate_requests:
            try:
                self.contract.validate_request(iface_name, func_name, params)
            except (TypeError, ValueError) as e:
                return self._error_response(req_id, -32602, "Invalid params", str(e))

        try:
            args = [ctx] + list(params)
            result = func(*args)
        except TypeError as e:
            return self._error_response(req_id, -32602, "Invalid params",
                                      "Parameter mismatch: %s" % e)
        except Exception as e:
            if isinstance(e, RPCError):
                return self._error_response(req_id, e.code, getattr(e, 'message'), e.data)
            else:
                import traceback
                traceback.print_exc()
                return self._error_response(req_id, -32603, "Internal error",
                                          "Handler exception: %s" % e)

        if self.validate_responses and result is not None:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except (TypeError, ValueError) as e:
                return self._error_response(req_id, -32603, "Internal error",
                                          "Response validation failed: %s" % e)

        if is_notification:
            return None

        return {
            'jsonrpc': '2.0',
            'result': result,
            'id': req_id
        }

    def _error_response(self, req_id, code, message, data=None):
        """Create a JSON-RPC error response"""
        error = {
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

    
