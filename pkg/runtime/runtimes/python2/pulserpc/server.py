"""Server class for handling JSON-RPC 2.0 requests"""

from rpc import RPCError
from contract import Contract

_string_types = (str, type(u""))


class Server(object):
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, etc.)
    """

    def __init__(
        self, contract, validate_requests=True, validate_responses=True, on_error=None
    ):
        """Initialize Server"""
        self.handlers = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses
        self.on_error = on_error

    def add_handler(self, iface_name, handler):
        """Register a handler instance for an interface"""
        self.handlers[iface_name] = handler

    def call(self, req, ctx=None):
        """Process a single JSON-RPC request

        Args:
            req: JSON-RPC request dict with 'jsonrpc', 'method', 'params', 'id'
            ctx: Optional context dict for transport-level metadata (headers, auth, etc.)
                Passed as the first positional argument to handler methods.
        """
        if not isinstance(req, dict):
            return self._error_response(
                None, -32600, "Invalid Request", "Request must be an object"
            )

        # A request without an id is a notification: it must never produce a
        # response, not even an error response (JSON-RPC 2.0 section 4.4).
        req_id = req.get("id")
        is_notification = req_id is None

        def error_response(code, message, data=None):
            if is_notification:
                return None
            return self._error_response(req_id, code, message, data)

        if req.get("jsonrpc") != "2.0":
            return error_response(
                -32600, "Invalid Request", "jsonrpc version must be '2.0'"
            )

        method = req.get("method")
        if not method or not isinstance(method, _string_types):
            return error_response(-32600, "Invalid Request", "Method must be a string")

        if method == "pulserpc-idl":
            if is_notification:
                return None
            return {"jsonrpc": "2.0", "result": self.contract.idl_parsed, "id": req_id}

        try:
            iface_name, func_name = method.rsplit(".", 1)
        except ValueError:
            return error_response(
                -32601,
                "Method not found",
                "Invalid method name format: %s" % method,
            )

        if iface_name not in self.handlers:
            return error_response(
                -32601, "Method not found", "Unknown interface: %s" % iface_name
            )

        handler = self.handlers[iface_name]

        if not hasattr(handler, func_name):
            return error_response(
                -32601, "Method not found", "Unknown method: %s" % method
            )

        func = getattr(handler, func_name)

        params = req.get("params")

        if params is None:
            params = []

        if not isinstance(params, list):
            return error_response(
                -32602, "Invalid params", "Parameters must be an array"
            )

        if self.validate_requests:
            try:
                self.contract.validate_request(iface_name, func_name, params)
            except RPCError as e:
                return error_response(e.code, e.message, e.data)
            except (TypeError, ValueError) as e:
                return error_response(-32602, "Invalid params", str(e))

        try:
            args = [ctx] + list(params)
            result = func(*args)
        except TypeError as e:
            return error_response(-32602, "Invalid params", "Parameter mismatch: %s" % e)
        except Exception as e:
            if isinstance(e, RPCError):
                return error_response(e.code, getattr(e, "message"), e.data)
            else:
                if self.on_error is not None:
                    self.on_error(e)
                else:
                    import traceback

                    traceback.print_exc()
                return error_response(
                    -32603, "Internal error", "Handler exception: %s" % e
                )

        # Validate every response, including None. A None result for a required
        # (non-void) return must be rejected, not silently skipped.
        if self.validate_responses:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except RPCError as e:
                return error_response(e.code, e.message, e.data)
            except (TypeError, ValueError) as e:
                return error_response(
                    -32603, "Internal error", "Response validation failed: %s" % e
                )

        if is_notification:
            return None

        return {"jsonrpc": "2.0", "result": result, "id": req_id}

    def _error_response(self, req_id, code, message, data=None):
        """Create a JSON-RPC error response"""
        error = {"code": code, "message": message}
        if data is not None:
            error["data"] = data

        response = {"jsonrpc": "2.0", "error": error}
        if req_id is not None:
            response["id"] = req_id

        return response
