"""Server class for handling JSON-RPC 2.0 requests"""

import inspect
import logging
import math
from typing import Any, Callable, Dict, List, Optional, Union
from .rpc import (
    RPCError,
    INVALID_REQUEST,
    METHOD_NOT_FOUND,
    INVALID_PARAMS,
    INTERNAL_ERROR,
)
from .contract import Contract

logger = logging.getLogger(__name__)


# Default upper bound on the number of entries accepted in a single JSON-RPC
# batch.  Without a bound an attacker can submit an arbitrarily large array and
# force one handler invocation (and one response object) per element, which is a
# cheap request-amplification / memory-exhaustion vector (review finding 1).
DEFAULT_MAX_BATCH_SIZE = 100


def _is_valid_request_id(req_id: Any) -> bool:
    """Return True when ``req_id`` is a legal JSON-RPC id (D2/R24).

    JSON-RPC 2.0 allows a String, Number, or Null id.  Python's ``bool`` is a
    subclass of ``int``, and objects/arrays are not numbers, so all three must
    be rejected explicitly rather than echoed back on the wire.
    """
    if req_id is None or isinstance(req_id, str):
        return True
    if isinstance(req_id, (int, float)) and not isinstance(req_id, bool):
        # JSON has no NaN/Infinity; json.loads accepts them by default, so a
        # non-finite number would be echoed as invalid JSON.
        return not isinstance(req_id, float) or math.isfinite(req_id)
    return False


class Server:
    """JSON-RPC 2.0 server with handler registration and optional validation

    The Server class provides transport-independent request processing.
    It can be used with any HTTP server (http.server, Flask, FastAPI, etc.)
    """

    def __init__(self, contract: Contract, validate_requests: bool = True,
                 validate_responses: bool = True,
                 on_error: Optional[Callable[[Exception], None]] = None,
                 max_batch_size: int = DEFAULT_MAX_BATCH_SIZE):
        """Initialize Server

        Args:
            contract: Contract instance for validation
            validate_requests: Validate request parameters against IDL
            validate_responses: Validate response values against IDL
            on_error: Optional callback invoked on unhandled handler exceptions
            max_batch_size: Maximum number of entries accepted in a batch
                request. Oversized batches are rejected with INVALID_REQUEST
                before any handler runs. Must be a positive integer.
        """
        if not isinstance(max_batch_size, int) or isinstance(max_batch_size, bool) \
                or max_batch_size <= 0:
            raise ValueError("max_batch_size must be a positive integer")

        self.handlers: Dict[str, Any] = {}
        self.contract = contract
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses
        self.on_error = on_error
        self.max_batch_size = max_batch_size

    def add_handler(self, iface_name: str, handler: Any) -> None:
        """Register a handler instance for an interface

        Args:
            iface_name: Interface name (e.g., "UserService")
            handler: Handler instance with methods matching the interface
        """
        self.handlers[iface_name] = handler

    def call(self, req: Union[Dict[str, Any], List[Any]], ctx: Any = None
             ) -> Optional[Union[Dict[str, Any], List[Dict[str, Any]]]]:
        """Process a JSON-RPC 2.0 request or batch of requests

        Args:
            req: JSON-RPC request dict (or a list of request dicts for a batch)
                with 'jsonrpc', 'method', 'params' and 'id' keys
            ctx: Optional context dict for transport-level metadata (headers, auth, etc.)
                Passed as the first positional argument to handler methods.

        Returns:
            A JSON-RPC response dict, a list of responses for a batch, or None
            for a notification (a request without 'id').

        Example:
            >>> request = {
            ...     'jsonrpc': '2.0',
            ...     'method': 'UserService.getUser',
            ...     'params': {'user_id': '123'},
            ...     'id': 1
            ... }
            >>> response = server.call(request)
        """
        if isinstance(req, list):
            return self._call_batch(req, ctx)
        return self._call_single(req, ctx)

    def _call_batch(self, reqs: List[Any], ctx: Any = None
                    ) -> Optional[List[Dict[str, Any]]]:
        """Process a JSON-RPC batch request (audit python-09).

        Batches are bounded by ``max_batch_size`` so an attacker cannot force an
        unbounded number of handler invocations or response objects with a
        single request. Oversized batches are rejected up front with
        INVALID_REQUEST; entries are never dispatched.
        """
        if not reqs:
            return self._error_response(
                None, INVALID_REQUEST, "Invalid Request", "Batch must not be empty"
            )

        if len(reqs) > self.max_batch_size:
            return self._error_response(
                None, INVALID_REQUEST, "Invalid Request",
                f"Batch exceeds maximum size of {self.max_batch_size}"
            )

        responses: List[Dict[str, Any]] = []
        for item in reqs:
            # A nested array is not a valid batch entry; _call_single rejects it.
            response = self._call_single(item, ctx)
            if response is not None:
                responses.append(response)

        # A batch containing only notifications produces no response at all.
        return responses if responses else None

    def _call_single(self, req: Any, ctx: Any = None) -> Optional[Dict[str, Any]]:
        """Process a single JSON-RPC request dict."""
        # Validate request format
        if not isinstance(req, dict):
            return self._error_response(
                None, INVALID_REQUEST, "Invalid Request", "Request must be an object"
            )

        # Notifications (requests with no 'id') must never receive a response,
        # including for format errors and the pulserpc-idl path (audit python-05).
        req_id = req.get('id')

        # An id that is not a string, number or null is an invalid request.  The
        # response cannot echo the bad value, so id is always null here (R24).
        if not _is_valid_request_id(req_id):
            return {
                'jsonrpc': '2.0',
                'error': {
                    'code': INVALID_REQUEST,
                    'message': 'Invalid Request',
                    'data': 'Request id must be a string, number, or null',
                },
                'id': None,
            }

        is_notification = req_id is None

        def error_response(code: int, message: str, data: Any = None
                           ) -> Optional[Dict[str, Any]]:
            if is_notification:
                return None
            return self._error_response(req_id, code, message, data)

        # Check JSON-RPC version
        if req.get('jsonrpc') != '2.0':
            return error_response(
                INVALID_REQUEST, "Invalid Request", "jsonrpc version must be '2.0'"
            )

        # Check for method
        method = req.get('method')
        if not method or not isinstance(method, str):
            return error_response(
                INVALID_REQUEST, "Invalid Request", "Method must be a string"
            )

        # Handle pulserpc-idl request
        if method == 'pulserpc-idl':
            if is_notification:
                return None
            return {
                'jsonrpc': '2.0',
                'result': self.contract.idl_parsed,
                'id': req_id
            }

        # Parse method name (e.g., "UserService.getUser")
        try:
            iface_name, func_name = method.rsplit('.', 1)
        except ValueError:
            return error_response(
                METHOD_NOT_FOUND, "Method not found",
                f"Invalid method name format: {method}"
            )

        # Look up handler
        if iface_name not in self.handlers:
            return error_response(
                METHOD_NOT_FOUND, "Method not found", f"Unknown interface: {iface_name}"
            )

        handler = self.handlers[iface_name]

        # Get function from handler
        if not hasattr(handler, func_name):
            return error_response(
                METHOD_NOT_FOUND, "Method not found", f"Unknown method: {method}"
            )

        func = getattr(handler, func_name)

        # Get params
        params = req.get('params')

        if params is None:
            params = []

        if not isinstance(params, list):
            return error_response(
                INVALID_PARAMS, "Invalid params", "Parameters must be an array"
            )

        if self.validate_requests:
            try:
                self.contract.validate_request(iface_name, func_name, params)
            except RPCError as e:
                return error_response(e.code, e.message, e.data)

        # Distinguish a call-signature mismatch (a client error) from a
        # TypeError raised inside the handler body (a server bug).  Binding
        # before the call keeps handler bugs out of the invalid-params path so
        # their text is never reflected to the client (R15).
        try:
            signature = inspect.signature(func)
        except (TypeError, ValueError):
            signature = None
        if signature is not None:
            try:
                signature.bind(ctx, *params)
            except TypeError:
                return error_response(
                    INVALID_PARAMS, "Invalid params", "Parameter mismatch"
                )

        # Invoke handler method
        try:
            result = func(ctx, *params)
        except Exception as e:
            # Convert application errors to RPC errors
            if isinstance(e, RPCError):
                return error_response(e.code, e.message, e.data)
            else:
                # Log unexpected errors server-side only.  The wire response
                # must not leak exception text, stack traces or handler
                # messages (R15 / D4).
                if self.on_error is not None:
                    self.on_error(e)
                else:
                    logger.exception(
                        "Unhandled handler exception in %s", method
                    )
                return error_response(INTERNAL_ERROR, "Internal error")

        # Validate response if validation is enabled.  A null result for a
        # required return type must be validated, not silently accepted
        # (audit python-03); Contract.validate_response decides based on the
        # IDL's returnType/returnOptional.
        if self.validate_responses:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except RPCError as e:
                return error_response(e.code, e.message, e.data)

        # Don't respond to notifications
        if is_notification:
            return None

        # Return success response
        return {
            'jsonrpc': '2.0',
            'result': result,
            'id': req_id
        }

    def _error_response(self, req_id: Any, code: int, message: str,
                        data: Any = None) -> Dict[str, Any]:
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
