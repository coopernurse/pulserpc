"""Client class for making JSON-RPC 2.0 requests"""

import threading

from transport import Transport
from rpc import RPCError
from contract import Contract


class InterfaceClientProxy(object):
    """Proxy for an interface that provides callable methods"""

    def __init__(self, client, iface):
        """Initialize interface proxy"""
        self._client = client
        self._iface = iface
        self._iface_name = iface.name

        for func_name in iface.functions.keys():
            setattr(self, func_name, self._create_methodcaller(func_name))

    def _create_methodcaller(self, func_name):
        """Create a method that calls the RPC function"""

        def methodcaller(*params, **kwargs):
            if kwargs:
                return self._client.call(
                    "%s.%s" % (self._iface_name, func_name), kwargs if kwargs else None
                )
            else:
                return self._client.call(
                    "%s.%s" % (self._iface_name, func_name),
                    list(params) if params else None,
                )

        return methodcaller


class Client(object):
    """JSON-RPC 2.0 client with automatic interface discovery"""

    def __init__(self, transport, validate_request=False, validate_response=False):
        """Initialize Client"""
        self.transport = transport
        self.validate_request = validate_request
        self.validate_response = validate_response
        self._request_id = 0
        # Shared clients may be used from several threads; the read-modify-write
        # increment must be atomic or ids can be reused (R20).
        self._request_id_lock = threading.Lock()
        self.contract = None

        self._bootstrap()

    def _bootstrap(self):
        """Fetch IDL from server and create interface proxies"""
        req = {"jsonrpc": "2.0", "method": "pulserpc-idl", "id": "bootstrap"}

        resp = self.transport.request(req)

        if "error" in resp:
            error = resp["error"]
            raise RuntimeError(
                "Failed to fetch IDL from server: %s"
                % error.get("message", "Unknown error")
            )

        idl_json = resp.get("result")
        if not idl_json:
            raise RuntimeError("Server returned empty IDL")

        self.contract = Contract(idl_json)

        for iface_name, iface in list(self.contract.interfaces.items()):
            setattr(self, iface_name, InterfaceClientProxy(self, iface))

    def call(self, method, params=None, expect_response=True):
        """Make a JSON-RPC call"""
        try:
            iface_name, func_name = method.rsplit(".", 1)
        except ValueError:
            raise ValueError("Invalid method name format: %s" % method)

        if isinstance(params, dict):
            # Named params are converted to a positional list using the IDL
            # signature before they go on the wire: JSON-RPC 'params' is always
            # an array (M20).  This mirrors the Python 3 client.
            converted = self._named_to_positional(
                iface_name, func_name, params
            )
            params = converted if converted is not None else []
        elif params is not None and not isinstance(params, list):
            params = [params]

        if self.validate_request and self.contract:
            param_list = params if params is not None else []

            try:
                self.contract.validate_request(iface_name, func_name, param_list)
            except RPCError:
                raise
            except (TypeError, ValueError) as e:
                raise ValueError("Request validation failed: %s" % e)

        with self._request_id_lock:
            self._request_id += 1
            req_id = self._request_id if expect_response else None

        req = {
            "jsonrpc": "2.0",
            "method": method,
        }
        if params is not None:
            req["params"] = params
        if req_id is not None:
            req["id"] = req_id

        response = self.transport.request(req)

        if not expect_response:
            return None

        self._validate_envelope(response, req_id)

        if "error" in response:
            error = response["error"]
            raise RPCError(
                code=error.get("code", -32603),
                message=error.get("message", "Unknown error"),
                data=error.get("data"),
            )

        result = response.get("result")

        if self.validate_response and self.contract:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except RPCError:
                raise
            except (TypeError, ValueError) as e:
                raise ValueError("Response validation failed: %s" % e)

        return result

    def _validate_envelope(self, response, req_id):
        """Check the JSON-RPC 2.0 response envelope before reading the result."""
        if not isinstance(response, dict):
            raise RPCError(-32600, "Invalid Response", "Response must be an object")
        if response.get("jsonrpc") != "2.0":
            raise RPCError(
                -32600, "Invalid Response", "jsonrpc version must be '2.0'"
            )

        has_result = "result" in response
        has_error = "error" in response
        if has_result == has_error:
            raise RPCError(
                -32600,
                "Invalid Response",
                "Response must contain exactly one of 'result' or 'error'",
            )
        if response.get("id") != req_id:
            raise RPCError(
                -32600,
                "Invalid Response",
                "Response id %r does not match request id %r"
                % (response.get("id"), req_id),
            )

    def notify(self, method, params=None):
        """Send a JSON-RPC notification (no response expected)"""
        self.call(method, params, expect_response=False)

    def _named_to_positional(self, iface_name, func_name, named_params):
        """Convert named parameters to positional parameters using IDL signature"""
        if not self.contract:
            return None

        interface = self.contract.get_interface(iface_name)
        if not interface:
            return None

        func = interface.get_function(func_name)
        if not func:
            return None

        param_defs = func.get("parameters", [])

        positional_params = []
        for param_def in param_defs:
            param_name = param_def["name"]
            positional_params.append(named_params.get(param_name))

        return positional_params
