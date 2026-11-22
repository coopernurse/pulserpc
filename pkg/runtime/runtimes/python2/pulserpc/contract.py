"""Contract class for IDL validation and interface metadata

This module provides the Contract class which parses IDL metadata
and provides validation for requests and responses.
"""

import json

from validation import validate_type
from rpc import RPCError


def _validation_error(code, message, invalid_fields=None):
    """Build an RPCError with structured, Python-2-compatible data.

    ``ValidationResult``/``ValidationError`` are Python-3-only, so the
    structured payload uses plain dicts/lists here.
    """
    if invalid_fields is None:
        invalid_fields = []
    return RPCError(
        code,
        message,
        {"error": message, "invalid_fields": invalid_fields},
    )


class Interface(object):
    """Represents an interface from the IDL"""

    def __init__(self, iface_data):
        """Initialize Interface from IDL data"""
        self.name = iface_data["name"]
        self.functions = {}
        for func in iface_data.get("methods", []):
            self.functions[func["name"]] = func

    def get_function(self, func_name):
        """Get function metadata by name"""
        return self.functions.get(func_name)


class Contract(object):
    """Represents a parsed IDL contract

    The Contract class parses IDL JSON and provides validation
    for requests and responses based on the interface definitions.
    """

    def __init__(self, idl_parsed):
        """Initialize Contract from parsed IDL"""
        if isinstance(idl_parsed, dict):
            self.idl_parsed = idl_parsed
            self.interfaces = {}
            self.structs = {}
            self.enums = {}
            self.meta = {}

            for iface_data in idl_parsed.get("interfaces", []):
                self.interfaces[iface_data["name"]] = Interface(iface_data)

            for struct_data in idl_parsed.get("structs", []):
                self.structs[struct_data["name"]] = struct_data

            for enum_data in idl_parsed.get("enums", []):
                self.enums[enum_data["name"]] = enum_data
        else:
            self.idl_parsed = idl_parsed
            self.interfaces = {}
            self.structs = {}
            self.enums = {}
            self.meta = {}

            for item in idl_parsed:
                item_type = item.get("type")
                if item_type == "struct":
                    self.structs[item["name"]] = item
                elif item_type == "enum":
                    self.enums[item["name"]] = item
                elif item_type == "interface":
                    self.interfaces[item["name"]] = Interface(item)
                elif item_type == "meta":
                    for key, value in item.items():
                        if key != "type":
                            self.meta[key] = value

    @classmethod
    def from_file(cls, path):
        """Load a Contract from a JSON file path

        Args:
            path: Path to the idl.json file

        Returns:
            Contract instance
        """
        import io
        with io.open(path, 'r', encoding='utf-8') as f:
            return cls(json.load(f))

    def has_interface(self, iface_name):
        """Check if interface exists"""
        return iface_name in self.interfaces

    def get_interface(self, iface_name):
        """Get interface by name"""
        return self.interfaces.get(iface_name)

    def validate_request(self, iface_name, func_name, params):
        """Validate request parameters against IDL.

        Raises:
            RPCError: with code -32602 and structured ``data`` on any violation.
        """
        interface = self.get_interface(iface_name)
        if not interface:
            message = "Unknown interface: '%s'" % iface_name
            raise _validation_error(-32602, message)

        func = interface.get_function(func_name)
        if not func:
            message = "%s: Unknown function: '%s'" % (iface_name, func_name)
            raise _validation_error(-32602, message)

        param_defs = func.get("parameters", [])

        if len(params) != len(param_defs):
            message = (
                "Function '%s.%s' expects %d param(s). %d given."
                % (iface_name, func_name, len(param_defs), len(params))
            )
            raise _validation_error(-32602, message)

        invalid_fields = []
        for i, param_value in enumerate(params):
            param_def = param_defs[i]
            param_type = param_def["type"]
            is_optional = param_def.get("optional", False)

            try:
                validate_type(
                    param_value, param_type, self.structs, self.enums, is_optional
                )
            except (TypeError, ValueError) as e:
                # Canonical M13 shape: invalid_fields is a list of path strings.
                invalid_fields.append("param[%d]" % i)

        if invalid_fields:
            message = "Function '%s.%s' invalid params" % (iface_name, func_name)
            raise _validation_error(-32602, message, invalid_fields)

    def validate_response(self, iface_name, func_name, result):
        """Validate response result against IDL.

        Raises:
            RPCError: with code -32603 and structured ``data`` on any violation.
        """
        interface = self.get_interface(iface_name)
        if not interface:
            message = "Unknown interface: '%s'" % iface_name
            raise _validation_error(-32603, message)

        func = interface.get_function(func_name)
        if not func:
            message = "%s: Unknown function: '%s'" % (iface_name, func_name)
            raise _validation_error(-32603, message)

        return_type = func.get("returnType")
        if not return_type:
            if result is not None:
                message = (
                    "Function '%s.%s' invalid response: '%s'. Expected None"
                    % (iface_name, func_name, result)
                )
                raise _validation_error(-32603, message, ["result"])
            return

        is_optional = func.get("returnOptional", False)
        try:
            validate_type(result, return_type, self.structs, self.enums, is_optional)
        except (TypeError, ValueError) as e:
            message = "Function '%s.%s' invalid response" % (iface_name, func_name)
            raise _validation_error(-32603, message, ["result"])
