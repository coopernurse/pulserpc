"""Client class for making JSON-RPC 2.0 requests"""

from typing import Any, Dict, Optional
from .transport import Transport
from .rpc import RPCError


class Client:
    """JSON-RPC 2.0 client with transport abstraction and optional validation

    The Client class sends JSON-RPC requests via a Transport implementation.
    It supports both single requests and notifications.
    """

    def __init__(self, transport: Transport, validate_requests: bool = False,
                 validate_responses: bool = False):
        """Initialize Client

        Args:
            transport: Transport implementation (HttpTransport, InProcTransport, etc.)
            validate_requests: Validate request parameters against IDL before sending
            validate_responses: Validate response values against IDL after receiving
        """
        self.transport = transport
        self.idl_data: Optional[Dict[str, Any]] = None
        self.all_structs: Dict[str, Any] = {}
        self.all_enums: Dict[str, Any] = {}
        self.validate_requests = validate_requests
        self.validate_responses = validate_responses
        self._request_id = 0

    def load_idl(self, idl_data: Dict[str, Any], all_structs: Dict[str, Any] = None,
                 all_enums: Dict[str, Any] = None) -> None:
        """Load IDL metadata for validation

        Args:
            idl_data: Full IDL data dict from parser
            all_structs: Optional merged structs dict
            all_enums: Optional merged enums dict
        """
        self.idl_data = idl_data

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

    def call(self, method: str, params: Optional[Any] = None,
             expect_response: bool = True) -> Any:
        """Make a JSON-RPC call

        Args:
            method: Method name (e.g., "UserService.getUser")
            params: Parameters (dict for named params, list for positional)
            expect_response: If False, send as notification (no response expected)

        Returns:
            Result value from the server

        Raises:
            RPCError: For JSON-RPC errors returned by server
            ValueError: For invalid responses
            urllib.error.URLError: For network/HTTP errors (via HttpTransport)

        Example:
            >>> result = client.call("UserService.getUser", {"user_id": "123"})
        """
        # Generate request ID
        self._request_id += 1
        req_id = self._request_id if expect_response else None

        # Build request
        req: Dict[str, Any] = {
            'jsonrpc': '2.0',
            'method': method,
        }
        if params is not None:
            req['params'] = params
        if req_id is not None:
            req['id'] = req_id

        # Send request via transport
        response = self.transport.request(req)

        # Handle notification (no response expected)
        if not expect_response:
            return None

        # Check for error response
        if 'error' in response:
            error = response['error']
            raise RPCError(
                code=error.get('code', -32603),
                message=error.get('message', 'Unknown error'),
                data=error.get('data')
            )

        # Return result
        return response.get('result')

    def notify(self, method: str, params: Optional[Any] = None) -> None:
        """Send a JSON-RPC notification (no response expected)

        Args:
            method: Method name
            params: Parameters

        Example:
            >>> client.notify("UserService.logEvent", {"event": "user_login"})
        """
        self.call(method, params, expect_response=False)

    def get_interface(self, iface_name: str, iface_class: type) -> Any:
        """Get a dynamic proxy for an interface

        This method creates an instance of a generated interface proxy class
        that provides type-safe method calls.

        Args:
            iface_name: Name of the interface (e.g., "UserService")
            iface_class: The generated proxy class for this interface

        Returns:
            An instance of the proxy class

        Example:
            >>> user_service = client.get_interface("UserService", UserServiceClient)
            >>> user = user_service.get_user("123")
        """
        return iface_class(self)


class InterfaceProxy:
    """Base class for generated interface proxy classes

    Generated interface proxy classes should inherit from this class
    to get common functionality.
    """

    def __init__(self, client: Client, iface_name: str):
        """Initialize interface proxy

        Args:
            client: Client instance
            iface_name: Interface name (used for method routing)
        """
        self._client = client
        self._iface_name = iface_name

    def _call(self, method_name: str, **kwargs) -> Any:
        """Make a call through the client

        Args:
            method_name: Method name (without interface prefix)

        Returns:
            Result from the RPC call

        Raises:
            RPCError: For JSON-RPC errors
        """
        full_method = f"{self._iface_name}.{method_name}"
        return self._client.call(full_method, kwargs if kwargs else None)
