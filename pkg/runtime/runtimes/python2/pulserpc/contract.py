"""Contract class for IDL validation and interface metadata

This module provides the Contract class which parses IDL metadata
and provides validation for requests and responses.
"""

from validation import validate_type


class Interface(object):
    """Represents an interface from the IDL"""

    def __init__(self, iface_data):
        """Initialize Interface from IDL data"""
        self.name = iface_data['name']
        self.functions = {}
        for func in iface_data.get('methods', []):
            self.functions[func['name']] = func

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

            for iface_data in idl_parsed.get('interfaces', []):
                self.interfaces[iface_data['name']] = Interface(iface_data)

            for struct_data in idl_parsed.get('structs', []):
                self.structs[struct_data['name']] = struct_data

            for enum_data in idl_parsed.get('enums', []):
                self.enums[enum_data['name']] = enum_data
        else:
            self.idl_parsed = idl_parsed
            self.interfaces = {}
            self.structs = {}
            self.enums = {}
            self.meta = {}

            for item in idl_parsed:
                item_type = item.get('type')
                if item_type == 'struct':
                    self.structs[item['name']] = item
                elif item_type == 'enum':
                    self.enums[item['name']] = item
                elif item_type == 'interface':
                    self.interfaces[item['name']] = Interface(item)
                elif item_type == 'meta':
                    for key, value in item.items():
                        if key != 'type':
                            self.meta[key] = value

    def has_interface(self, iface_name):
        """Check if interface exists"""
        return iface_name in self.interfaces

    def get_interface(self, iface_name):
        """Get interface by name"""
        return self.interfaces.get(iface_name)

    def validate_request(self, iface_name, func_name, params):
        """Validate request parameters against IDL"""
        interface = self.get_interface(iface_name)
        if not interface:
            raise ValueError("Unknown interface: '%s'" % iface_name)

        func = interface.get_function(func_name)
        if not func:
            raise ValueError("%s: Unknown function: '%s'" % (iface_name, func_name))

        param_defs = func.get('parameters', [])

        if len(params) != len(param_defs):
            raise ValueError(
                "Function '%s.%s' expects %d param(s). %d given." %
                (iface_name, func_name, len(param_defs), len(params))
            )

        for i, param_value in enumerate(params):
            param_def = param_defs[i]
            param_name = param_def['name']
            param_type = param_def['type']
            is_optional = param_def.get('optional', False)

            try:
                validate_type(param_value, param_type, self.structs,
                            self.enums, is_optional)
            except (TypeError, ValueError) as e:
                raise ValueError(
                    "Function '%s.%s' invalid param '%s'. %s" %
                    (iface_name, func_name, param_name, e)
                )

    def validate_response(self, iface_name, func_name, result):
        """Validate response result against IDL"""
        interface = self.get_interface(iface_name)
        if not interface:
            raise ValueError("Unknown interface: '%s'" % iface_name)

        func = interface.get_function(func_name)
        if not func:
            raise ValueError("%s: Unknown function: '%s'" % (iface_name, func_name))

        return_type = func.get('returnType')
        if not return_type:
            if result is not None:
                raise ValueError(
                    "Function '%s.%s' invalid response: '%s'. Expected None" %
                    (iface_name, func_name, result)
                )
            return

        is_optional = func.get('returnOptional', False)
        try:
            validate_type(result, return_type, self.structs,
                        self.enums, is_optional)
        except (TypeError, ValueError) as e:
            raise ValueError(
                "Function '%s.%s' invalid response: '%s'. %s" %
                (iface_name, func_name, result, e)
            )
