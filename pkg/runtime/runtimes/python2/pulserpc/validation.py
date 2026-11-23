"""Validation functions for PulseRPC types"""

from rpctypes import find_struct, find_enum, get_struct_fields


def _build_simple_name_map(definitions):
    """Map an unqualified type name to its first namespaced definition key.

    Mirrors the old ``qualified_key.endswith("." + user_type)`` scan: only keys
    that actually contain a namespace separator participate, and the first key
    in iteration order wins when several share a simple name.
    """
    mapping = {}
    for qualified_key in definitions:
        if "." in qualified_key:
            simple_name = qualified_key.rsplit(".", 1)[-1]
            if simple_name not in mapping:
                mapping[simple_name] = qualified_key
    return mapping


class ValidationContext(object):
    """Precomputed lookup metadata shared by all validations of a Contract.

    Keeping these tables on the Contract avoids re-scanning every struct/enum
    name and re-resolving every inheritance chain for each validated value.
    The public ``validate_*`` helpers still work without a context (they fall
    back to computing the same information on demand).
    """

    def __init__(self, structs, enums):
        self.struct_simple_names = _build_simple_name_map(structs)
        self.enum_simple_names = _build_simple_name_map(enums)

        self.enum_values = {}
        self.enum_value_sets = {}
        for key, enum_def in enums.items():
            values = [v["name"] for v in enum_def.get("values", [])]
            self.enum_values[key] = values
            self.enum_value_sets[key] = frozenset(values)

        self.struct_fields = {}
        for key in structs:
            self.struct_fields[key] = get_struct_fields(key, structs)


_string_types = (str, type(u""))
# In Python 2, 2**100 is `long`; in Python 3 it's `int`. Avoids naming `long` directly.
_large_int = 2 ** 100
_int_types = (int,) if isinstance(_large_int, int) else (int, type(_large_int))

# IDL `int` is a signed 32-bit integer (D1/R25).  Python 2 integers are
# arbitrary precision (`long`), so the range must be enforced by validation.
INT_MIN = -2147483648
INT_MAX = 2147483647


def _check_int_range(value):
    if value < INT_MIN or value > INT_MAX:
        raise TypeError(
            "Expected int in signed 32-bit range [%d, %d], got %s"
            % (INT_MIN, INT_MAX, value)
        )


def validate_string(value):
    """Validate that value is a string"""
    if not isinstance(value, _string_types):
        raise TypeError("Expected string, got %s" % type(value).__name__)


def validate_int(value):
    """Validate that value is an int.

    Booleans are rejected even though ``bool`` subclasses ``int`` in Python 2,
    and float NaN/infinity are rejected before ``int(value)`` can raise an
    unwrapped ValueError/OverflowError. This matches the Python 3 acceptance
    set (invariant R2).
    """
    if isinstance(value, bool):
        raise TypeError("Expected int, got bool")
    if isinstance(value, _int_types):
        _check_int_range(value)
        return
    if isinstance(value, float):
        if value != value:  # NaN
            raise TypeError("Expected int, got float NaN")
        if value == float("inf") or value == float("-inf"):
            raise TypeError("Expected int, got float infinity")
        if value == int(value):
            _check_int_range(int(value))
            return
    raise TypeError("Expected int, got %s" % type(value).__name__)


def validate_float(value):
    """Validate that value is a float or int.

    Booleans are rejected even though ``bool`` subclasses ``int`` in Python 2,
    matching Python 3 and the other runtimes (invariant R2).
    """
    if isinstance(value, bool):
        raise TypeError("Expected float, got bool")
    if not isinstance(value, _int_types + (float,)):
        raise TypeError("Expected float, got %s" % type(value).__name__)


def validate_bool(value):
    """Validate that value is a bool"""
    if not isinstance(value, bool):
        raise TypeError("Expected bool, got %s" % type(value).__name__)


def validate_array(value, element_validator):
    """Validate that value is an array and each element passes validation"""
    if not isinstance(value, list):
        raise TypeError("Expected list, got %s" % type(value).__name__)
    for i, elem in enumerate(value):
        try:
            element_validator(elem)
        except Exception as e:
            raise ValueError("Array element at index %d validation failed: %s" % (i, e))


def validate_map(value, value_validator):
    """Validate that value is a map (dict) with string keys and validated values"""
    if not isinstance(value, dict):
        raise TypeError("Expected dict, got %s" % type(value).__name__)
    for key, val in value.items():
        if not isinstance(key, _string_types):
            raise TypeError("Map key must be string, got %s" % type(key).__name__)
        try:
            value_validator(val)
        except Exception as e:
            raise ValueError("Map value for key '%s' validation failed: %s" % (key, e))


def validate_enum(value, enum_name, allowed_values, allowed_set=None):
    """Validate that value is a string and matches one of the allowed enum values.

    ``allowed_set`` is an optional precomputed frozenset for O(1) membership;
    error text still uses the ordered ``allowed_values`` list.  When omitted,
    membership is checked against the list directly (legacy helper behavior).
    """
    if not isinstance(value, _string_types):
        raise TypeError(
            "Expected string for enum %s, got %s" % (enum_name, type(value).__name__)
        )
    if allowed_set is not None:
        matches = value in allowed_set
    else:
        matches = value in allowed_values
    if not matches:
        raise ValueError(
            "Invalid value for enum %s: '%s'. Allowed values: %s"
            % (enum_name, value, allowed_values)
        )


def validate_struct(value, struct_name, struct_def, all_structs, all_enums,
                    fields=None, context=None):
    """Validate that value is a dict matching the struct definition.

    ``fields`` may be a pre-resolved field list (parent + child) supplied by a
    ValidationContext; when omitted it is computed as before.  ``context`` is
    forwarded to nested value validation so nested/user-defined types also
    reuse the precomputed lookups.
    """
    if not isinstance(value, dict):
        raise TypeError(
            "Expected dict for struct %s, got %s" % (struct_name, type(value).__name__)
        )

    if fields is None:
        fields = get_struct_fields(struct_name, all_structs)

    for field in fields:
        field_name = field["name"]
        field_type = field["type"]
        is_optional = field.get("optional", False)

        if field_name not in value:
            if not is_optional:
                raise ValueError(
                    "Missing required field '%s' in struct %s"
                    % (field_name, struct_name)
                )
        else:
            field_value = value[field_name]
            if field_value is None:
                if not is_optional:
                    raise ValueError(
                        "Field '%s' in struct %s cannot be None"
                        % (field_name, struct_name)
                    )
            else:

                def make_validator(ft, als, ale, io, ctx):
                    return lambda x: validate_type(x, ft, als, ale, io, ctx)

                try:
                    make_validator(
                        field_type, all_structs, all_enums, is_optional, context
                    )(field_value)
                except Exception as e:
                    raise ValueError(
                        "Field '%s' in struct %s validation failed: %s"
                        % (field_name, struct_name, e)
                    )


def validate_type(value, type_def, all_structs, all_enums, is_optional=False,
                  context=None):
    """Validate a value against a type definition.

    ``context`` is an optional ValidationContext carrying precomputed
    simple-name, enum-value and struct-field lookups.  Direct helper calls
    without a context compute the same information on demand, preserving the
    original observable behavior.
    """
    if value is None:
        if is_optional:
            return
        else:
            raise ValueError("Value cannot be None for non-optional type")

    if type_def.get("builtIn") == "string":
        validate_string(value)
    elif type_def.get("builtIn") == "int":
        validate_int(value)
    elif type_def.get("builtIn") == "float":
        validate_float(value)
    elif type_def.get("builtIn") == "bool":
        validate_bool(value)
    elif type_def.get("array"):
        element_type = type_def["array"]

        def make_element_validator(et, als, ale, ctx):
            return lambda x: validate_type(x, et, als, ale, False, ctx)

        validate_array(
            value, make_element_validator(element_type, all_structs, all_enums, context)
        )
    elif type_def.get("mapValue"):
        value_type = type_def["mapValue"]

        def make_value_validator(vt, als, ale, ctx):
            return lambda x: validate_type(x, vt, als, ale, False, ctx)

        validate_map(
            value, make_value_validator(value_type, all_structs, all_enums, context)
        )
    elif type_def.get("userDefined"):
        if context is not None:
            struct_simple_names = context.struct_simple_names
            enum_simple_names = context.enum_simple_names
        else:
            struct_simple_names = _build_simple_name_map(all_structs)
            enum_simple_names = _build_simple_name_map(all_enums)

        user_type = type_def["userDefined"]
        struct_def = find_struct(user_type, all_structs)
        if not struct_def and "." not in user_type:
            qualified_key = struct_simple_names.get(user_type)
            if qualified_key is not None:
                struct_def = all_structs[qualified_key]
                user_type = qualified_key
        if struct_def:
            cached_fields = None
            if context is not None:
                cached_fields = context.struct_fields.get(user_type)
            validate_struct(
                value, user_type, struct_def, all_structs, all_enums,
                fields=cached_fields, context=context,
            )
        else:
            enum_def = find_enum(user_type, all_enums)
            enum_key = user_type
            if not enum_def and "." not in user_type:
                qualified_key = enum_simple_names.get(user_type)
                if qualified_key is not None:
                    enum_def = all_enums[qualified_key]
                    enum_key = qualified_key
            if enum_def:
                allowed_set = None
                if context is not None:
                    allowed_values = context.enum_values.get(enum_key)
                    allowed_set = context.enum_value_sets.get(enum_key)
                    if allowed_values is None:
                        allowed_values = [v["name"] for v in enum_def.get("values", [])]
                else:
                    allowed_values = [v["name"] for v in enum_def.get("values", [])]
                validate_enum(value, user_type, allowed_values, allowed_set)
            else:
                raise ValueError("Unknown user-defined type: %s" % user_type)
    else:
        raise ValueError("Invalid type definition: %s" % type_def)
