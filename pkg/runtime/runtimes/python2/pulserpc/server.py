"""Server class for handling JSON-RPC 2.0 requests"""

import inspect
import logging

from rpc import RPCError
from contract import Contract

_string_types = (str, type(u""))
# Python 2 promotes large integer literals (e.g. ``2**70``) to ``long``, which
# ``isinstance(value, int)`` rejects.  Mirror validation.py so every legal
# JSON-RPC numeric id is accepted (R24).
_large_int = 2 ** 100
_int_types = (int,) if isinstance(_large_int, int) else (int, type(_large_int))

logger = logging.getLogger(__name__)

# Adapters are responsible for reading the request body (the Server is
# transport-independent).  They should refuse anything larger than this so an
# oversized payload cannot be buffered in memory (R16/D5).  Kept here so
# generated documentation and hand-written handlers share one constant.
DEFAULT_MAX_BODY_SIZE = 1024 * 1024  # 1 MiB


def _is_valid_request_id(req_id):
    """Return True when ``req_id`` is a legal JSON-RPC id (D2/R24).

    JSON-RPC 2.0 allows a string, number or null id.  ``bool`` subclasses
    ``int`` in Python 2 and dicts/lists are not numbers, so all three must be
    rejected explicitly instead of being echoed back on the wire.
    """
    if req_id is None or isinstance(req_id, _string_types):
        return True
    if isinstance(req_id, _int_types + (float,)) and not isinstance(req_id, bool):
        # JSON has no NaN/Infinity; reject non-finite numbers so they are not
        # echoed as invalid JSON.
        if isinstance(req_id, float):
            return req_id == req_id and req_id not in (float("inf"), float("-inf"))
        return True
    return False


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
        # Argument specs never change between calls, so cache them per
        # (interface, method) instead of introspecting on every request.
        self._signature_cache = {}

    def add_handler(self, iface_name, handler):
        """Register a handler instance for an interface"""
        self.handlers[iface_name] = handler
        self._signature_cache.clear()

    def _accepts_args(self, iface_name, func_name, func, args):
        """Return True when ``func`` can be called with positional ``args``.

        The argument spec is cached per (interface, method).  Simple
        signatures (no defaults/``*args``/``**kwargs``) are checked with a
        cheap arity comparison; anything richer falls back to
        ``inspect.getcallargs`` so binding semantics stay exact.
        """
        key = (iface_name, func_name)
        if key not in self._signature_cache:
            try:
                self._signature_cache[key] = inspect.getargspec(func)
            except TypeError:
                self._signature_cache[key] = None

        spec = self._signature_cache[key]
        if (
            spec is not None
            and not spec.varargs
            and not spec.keywords
            and not spec.defaults
        ):
            # getargspec exposes the underlying function's argument names, so
            # drop the implicit ``self`` for bound methods.
            bound = getattr(func, "im_self", None) is not None
            expected = len(spec.args) - (1 if bound else 0)
            return len(args) == expected

        try:
            inspect.getcallargs(func, *args)
        except TypeError:
            return False
        return True

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

        # An id that is not a string, number or null is an invalid request.  The
        # bad value cannot be echoed back, so id is always null here (R24).
        if not _is_valid_request_id(req_id):
            return self._error_response(
                None,
                -32600,
                "Invalid Request",
                "Request id must be a string, number, or null",
            )

        is_notification = req_id is None

        def error_response(code, message, data=None):
            if is_notification:
                return None
            return self._error_response(req_id, code, message, data)

        if req.get("jsonrpc") != "2.0":
            # A bad jsonrpc version is an invalid request, so the offending id
            # must not be echoed back: use null (while keeping notifications
            # silent).  Do not use the `error_response` closure, which echoes
            # `req_id`.
            if is_notification:
                return None
            return self._error_response(
                None, -32600, "Invalid Request", "jsonrpc version must be '2.0'"
            )

        method = req.get("method")
        if not method or not isinstance(method, _string_types):
            # An invalid method makes the envelope invalid; never echo the
            # caller-supplied id. Notifications still receive no response.
            if is_notification:
                return None
            return self._error_response(
                None, -32600, "Invalid Request", "Method must be a string"
            )

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

        # Distinguish a call-signature mismatch (a client error) from a
        # TypeError raised inside the handler body (a server bug) so handler
        # bugs are redacted as internal errors instead of returned as -32602
        # (R15).
        args = [ctx] + list(params)
        if not self._accepts_args(iface_name, func_name, func, args):
            return error_response(
                -32602, "Invalid params", "Parameter mismatch"
            )

        try:
            result = func(*args)
        except Exception as e:
            if isinstance(e, RPCError):
                return error_response(e.code, getattr(e, "message"), e.data)
            else:
                # Log server-side only; the wire response must not leak
                # exception text, traceback or handler messages (R15/D4).
                if self.on_error is not None:
                    self.on_error(e)
                else:
                    logger.exception(
                        "Unhandled handler exception in %s", method
                    )
                return error_response(-32603, "Internal error")

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
        """Create a JSON-RPC error response

        ``id`` is always present on the wire (M18/M26): notifications never
        reach this helper, so ``None`` only appears for requests whose id could
        not be accepted and for transport-level parse/batch errors.
        """
        error = {"code": code, "message": message}
        if data is not None:
            error["data"] = data

        return {"jsonrpc": "2.0", "error": error, "id": req_id}
