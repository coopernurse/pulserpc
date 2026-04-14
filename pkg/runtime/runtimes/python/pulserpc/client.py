"""Client class for making JSON-RPC 2.0 requests"""

import json
from datetime import datetime
from typing import Any, Dict, List, Optional, TYPE_CHECKING
from .transport import Transport
from .rpc import RPCError
from .contract import Contract
from .diff import diff_idl
from .types import (
    ContractDelta, VerificationResult, extract_checksum
)

if TYPE_CHECKING:
    from .contract import ContractAuditor


class InterfaceClientProxy:
    """Proxy for an interface that provides callable methods

    Created dynamically by Client for each interface in the IDL.
    """

    def __init__(self, client: 'Client', iface):
        """Initialize interface proxy

        Args:
            client: Client instance
            iface: Interface object from contract
        """
        self._client = client
        self._iface = iface
        self._iface_name = iface.name

        # Create callable methods for each function in the interface
        for func_name in iface.functions.keys():
            setattr(self, func_name, self._create_methodcaller(func_name))

    def _create_methodcaller(self, func_name: str):
        """Create a method that calls the RPC function

        Args:
            func_name: Name of the function

        Returns:
            Callable that invokes the RPC method
        """
        def methodcaller(*params, **kwargs):
            # If keyword args were provided, convert to positional list
            if kwargs:
                # Use named params - send as dict
                return self._client.call(
                    f"{self._iface_name}.{func_name}",
                    kwargs if kwargs else None
                )
            else:
                # Use positional params - send as list
                return self._client.call(
                    f"{self._iface_name}.{func_name}",
                    list(params) if params else None
                )
        return methodcaller


class Client:
    """JSON-RPC 2.0 client with automatic interface discovery

    The Client class sends JSON-RPC requests via a Transport implementation.
    On initialization, it fetches the IDL from the server and dynamically
    creates interface proxies.

    Example:
        >>> transport = HttpTransport("http://localhost:8080")
        >>> client = Client(transport)
        >>> result = client.UserService.getUser("123")
    """

    def __init__(self, transport: Transport, validate_request: bool = False,
                 validate_response: bool = False, options: Optional[Dict[str, Any]] = None):
        """Initialize Client

        Args:
            transport: Transport implementation (HttpTransport, InProcTransport, etc.)
            validate_request: Validate request parameters against IDL before sending
            validate_response: Validate response values against IDL after receiving
            options: Optional dict with 'auditor' and 'verify_on_bootstrap' keys
        """
        self.transport = transport
        self.validate_request = validate_request
        self.validate_response = validate_response
        self._request_id = 0
        self.contract: Optional[Contract] = None
        self._options = options or {}
        self._auditor: Optional[ContractAuditor] = self._options.get('auditor')
        self._verify_on_bootstrap: bool = self._options.get('verify_on_bootstrap', False)
        self._local_idl: Optional[Dict[str, Any]] = None

        # Try to auto-load IDL from idl.json
        self._find_idl_json()

        # Bootstrap: fetch IDL from server
        self._bootstrap()

        # Run verify_on_bootstrap if enabled
        if self._verify_on_bootstrap:
            self.verify_compatibility()

    def _bootstrap(self) -> None:
        """Fetch IDL from server and create interface proxies

        Makes a 'pulserpc-idl' request to get the IDL JSON,
        then creates Contract and interface proxies.
        """
        # Make request to get IDL
        req = {
            'jsonrpc': '2.0',
            'method': 'pulserpc-idl',
            'id': 'bootstrap'
        }

        resp = self.transport.request(req)

        if 'error' in resp:
            error = resp['error']
            raise RuntimeError(f"Failed to fetch IDL from server: {error.get('message', 'Unknown error')}")

        idl_json = resp.get('result')
        if not idl_json:
            raise RuntimeError("Server returned empty IDL")

        # Create contract
        self.contract = Contract(idl_json)

        # Create interface proxies as attributes
        for iface_name, iface in list(self.contract.interfaces.items()):
            setattr(self, iface_name, InterfaceClientProxy(self, iface))

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
        # Parse method name
        try:
            iface_name, func_name = method.rsplit('.', 1)
        except ValueError:
            raise ValueError(f"Invalid method name format: {method}")

        # Validate request if enabled
        if self.validate_request and self.contract:
            # Convert params to list for validation
            if isinstance(params, dict):
                # Named params - need to convert to positional
                param_list = self._named_to_positional(iface_name, func_name, params)
            elif isinstance(params, list):
                param_list = params
            else:
                param_list = [] if params is None else [params]

            try:
                self.contract.validate_request(iface_name, func_name, param_list)
            except (TypeError, ValueError) as e:
                raise ValueError(f"Request validation failed: {e}") from e

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

        # Get result
        result = response.get('result')

        # Validate response if enabled
        if self.validate_response and self.contract and result is not None:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except (TypeError, ValueError) as e:
                raise ValueError(f"Response validation failed: {e}") from e

        return result

    def notify(self, method: str, params: Optional[Any] = None) -> None:
        """Send a JSON-RPC notification (no response expected)

        Args:
            method: Method name
            params: Parameters

        Example:
            >>> client.notify("UserService.logEvent", {"event": "user_login"})
        """
        self.call(method, params, expect_response=False)

    def _named_to_positional(self, iface_name: str, func_name: str,
                              named_params: Dict[str, Any]) -> Optional[List[Any]]:
        """Convert named parameters to positional parameters using IDL signature

        Args:
            iface_name: Interface name
            func_name: Function name
            named_params: Dict mapping parameter names to values

        Returns:
            List of parameter values in IDL order, or None if interface not found
        """
        if not self.contract:
            return None

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

    def _find_idl_json(self) -> None:
        """Try to find and load idl.json by walking up from this module's location"""
        import os
        from pathlib import Path
        
        # Get the directory containing this file (the pulserpc package)
        module_dir = Path(__file__).parent
        
        # Walk up the directory tree looking for idl.json
        current_dir = module_dir
        for _ in range(10):  # Max 10 levels up
            idl_path = current_dir / 'idl.json'
            if idl_path.exists():
                try:
                    with open(idl_path, 'r') as f:
                        self._local_idl = json.load(f)
                    return
                except (json.JSONDecodeError, IOError):
                    pass
            parent = current_dir.parent
            if parent == current_dir:
                break
            current_dir = parent

    def set_local_idl(self, idl_json: str) -> None:
        """Set the local IDL for verification
        
        Args:
            idl_json: JSON string containing the local IDL
        """
        self._local_idl = json.loads(idl_json)

    def verify_compatibility(self) -> VerificationResult:
        """Verify compatibility between local and server IDL
        
        Returns:
            VerificationResult with deltas and compatibility status
        """
        # Get server IDL from contract
        if not self.contract or not self.contract.idl_parsed:
            raise RuntimeError("No server IDL available - client not bootstrapped")
        
        server_idl = self.contract.idl_parsed
        client_idl = self._local_idl
        
        if not client_idl:
            raise RuntimeError("No local IDL available - call set_local_idl() or ensure idl.json exists")
        
        # Compute diff
        deltas = diff_idl(client_idl, server_idl)
        
        # Determine compatibility (has Error-level deltas)
        has_error = any(d.severity.value == 'Error' for d in deltas)
        compatible = not has_error
        
        # Get checksums
        server_checksum = extract_checksum(server_idl)
        client_checksum = extract_checksum(client_idl)
        
        result = VerificationResult(
            compatible=compatible,
            server_checksum=server_checksum,
            client_checksum=client_checksum,
            deltas=deltas,
            timestamp=datetime.utcnow()
        )
        
        # Invoke auditor if configured
        if self._auditor:
            self._auditor.audit(result)
        
        return result
