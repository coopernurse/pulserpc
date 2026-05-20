"""Client class for making JSON-RPC 2.0 requests"""

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

        if self.validate_request and self.contract:
            if isinstance(params, dict):
                param_list = self._named_to_positional(iface_name, func_name, params)
            elif isinstance(params, list):
                param_list = params
            else:
                param_list = [] if params is None else [params]

            try:
                self.contract.validate_request(iface_name, func_name, param_list)
            except (TypeError, ValueError) as e:
                raise ValueError("Request validation failed: %s" % e)

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

        if "error" in response:
            error = response["error"]
            raise RPCError(
                code=error.get("code", -32603),
                message=error.get("message", "Unknown error"),
                data=error.get("data"),
            )

        result = response.get("result")

        if self.validate_response and self.contract and result is not None:
            try:
                self.contract.validate_response(iface_name, func_name, result)
            except (TypeError, ValueError) as e:
                raise ValueError("Response validation failed: %s" % e)

        return result

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
