"""Validation functions for PulseRPC types"""

from typing import Any, Callable, Dict, List, Optional

from .rpctypes import (
    ValidationError,
    _get_resolution_index,
    _resolve_with_index,
    get_struct_fields,
)


def _join_path(parent: str, child: str) -> str:
    return f"{parent}.{child}" if parent else f".{child}"


# IDL `int` is a signed 64-bit integer (D1/R25).  Python integers are
# arbitrary precision, so the range must be enforced by validation even though
# generated native types may be wider.
INT_MIN = -2 ** 63
INT_MAX = 2 ** 63 - 1


def _validate_int_range(value: Any, path: str) -> List["ValidationError"]:
    if value < INT_MIN or value > INT_MAX:
        return [_make_error(
            path,
            f"Expected int in signed 64-bit range "
            f"[{INT_MIN}, {INT_MAX}], got {value}",
        )]
    return []


def _array_index_path(parent: str, index: Any) -> str:
    return f"{parent}[{index}]"


def _make_error(path: str, message: str) -> ValidationError:
    return ValidationError(path=path, message=message)


def validate_string(value: Any, path: str = "") -> List[ValidationError]:
    if not isinstance(value, str):
        return [_make_error(path, f"Expected string, got {type(value).__name__}")]
    return []


def validate_int(value: Any, path: str = "") -> List[ValidationError]:
    if isinstance(value, bool):
        return [_make_error(path, f"Expected int, got bool")]
    if isinstance(value, int):
        return _validate_int_range(value, path)
    if isinstance(value, float):
        if value != value:  # NaN check
            return [_make_error(path, f"Expected int, got float NaN")]
        if value == float('inf') or value == float('-inf'):
            return [_make_error(path, f"Expected int, got float infinity")]
        # Match the Go/C#/TS/Java acceptance set exactly: only integral values.
        # Near-integers with any fractional part are rejected (audit python-02,
        # invariant R2). Go no longer applies a 1e-9 tolerance.
        if value == int(value):
            return _validate_int_range(int(value), path)
    return [_make_error(path, f"Expected int, got {type(value).__name__}")]


def validate_float(value: Any, path: str = "") -> List[ValidationError]:
    # bool subclasses int in Python, but JSON `true`/`false` are not floats.
    # validate_int already applies the same rule (audit python-02).
    if isinstance(value, bool):
        return [_make_error(path, f"Expected float, got bool")]
    if not isinstance(value, (int, float)):
        return [_make_error(path, f"Expected float, got {type(value).__name__}")]
    return []


def validate_bool(value: Any, path: str = "") -> List[ValidationError]:
    if not isinstance(value, bool):
        return [_make_error(path, f"Expected bool, got {type(value).__name__}")]
    return []


def validate_array(
    value: Any,
    element_validator: Callable[[Any, str], List[ValidationError]],
    path: str = ""
) -> List[ValidationError]:
    if not isinstance(value, list):
        return [_make_error(path, f"Expected list, got {type(value).__name__}")]
    errors: List[ValidationError] = []
    for i, elem in enumerate(value):
        element_path = _array_index_path(path, i)
        errors.extend(element_validator(elem, element_path))
    return errors


def validate_map(
    value: Any,
    value_validator: Callable[[Any, str], List[ValidationError]],
    path: str = ""
) -> List[ValidationError]:
    if not isinstance(value, dict):
        return [_make_error(path, f"Expected dict, got {type(value).__name__}")]
    errors: List[ValidationError] = []
    for key, val in value.items():
        if not isinstance(key, str):
            errors.append(_make_error(path, f"Map key must be string, got {type(key).__name__}"))
            continue
        key_path = _array_index_path(path, key)
        errors.extend(value_validator(val, key_path))
    return errors


def validate_enum(
    value: Any,
    enum_name: str,
    allowed_values: List[str],
    path: str = "",
    _allowed_set: Optional[frozenset] = None,
) -> List[ValidationError]:
    """Validate an enum value.

    ``_allowed_set`` is an optional precomputed frozenset for O(1) membership.
    When it is omitted (the standalone/public case) the ordered list is used,
    which keeps the original signature and behaviour intact.  Error text always
    uses the ordered list.
    """
    if not isinstance(value, str):
        return [_make_error(path, f"Expected string for enum {enum_name}, got {type(value).__name__}")]
    membership = allowed_values if _allowed_set is None else _allowed_set
    if value not in membership:
        return [_make_error(path, f"Invalid value for enum {enum_name}: '{value}'. Allowed values: {allowed_values}")]
    return []


def validate_struct(
    value: Any,
    struct_name: str,
    struct_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any],
    path: str = "",
    _indexes: Optional[tuple] = None,
    _fields: Optional[Dict[str, List[Dict[str, Any]]]] = None,
    _enum_values: Optional[Dict[str, tuple]] = None,
) -> List[ValidationError]:
    if not isinstance(value, dict):
        return [_make_error(path, f"Expected dict for struct {struct_name}, got {type(value).__name__}")]

    if _indexes is None:
        _indexes = (_get_resolution_index(all_structs),
                    _get_resolution_index(all_enums))
    if _fields is not None and struct_name in _fields:
        # Precomputed by Contract.__init__: no recursive field rebuild here.
        fields = _fields[struct_name]
    else:
        fields = get_struct_fields(struct_name, all_structs, _index=_indexes[0])
    errors: List[ValidationError] = []

    for field in fields:
        field_name = field['name']
        field_type = field['type']
        is_optional = field.get('optional', False)
        field_path = _join_path(path, field_name)

        if field_name not in value:
            if not is_optional:
                errors.append(_make_error(
                    field_path,
                    f"Missing required field '{field_name}' in struct {struct_name}"
                ))
        else:
            field_value = value[field_name]
            if field_value is None:
                if not is_optional:
                    errors.append(_make_error(
                        field_path,
                        f"Field '{field_name}' in struct {struct_name} cannot be None"
                    ))
            else:
                errors.extend(
                    validate_type(field_value, field_type, all_structs, all_enums,
                                  is_optional, field_path, _indexes,
                                  _enum_values, _fields)
                )

    return errors


def validate_type(
    value: Any,
    type_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any],
    is_optional: bool = False,
    path: str = "",
    _indexes: Optional[tuple] = None,
    _enum_values: Optional[Dict[str, tuple]] = None,
    _fields: Optional[Dict[str, List[Dict[str, Any]]]] = None,
) -> List[ValidationError]:
    if value is None:
        if is_optional:
            return []
        else:
            return [_make_error(path, "Value cannot be None for non-optional type")]

    if type_def.get('builtIn') == 'string':
        return validate_string(value, path)
    elif type_def.get('builtIn') == 'int':
        return validate_int(value, path)
    elif type_def.get('builtIn') == 'float':
        return validate_float(value, path)
    elif type_def.get('builtIn') == 'bool':
        return validate_bool(value, path)
    elif type_def.get('array'):
        element_type = type_def['array']
        element_validator = lambda v, p: validate_type(
            v, element_type, all_structs, all_enums, False, p,
            _indexes, _enum_values, _fields
        )
        return validate_array(value, element_validator, path)
    elif type_def.get('mapValue'):
        value_type = type_def['mapValue']
        value_validator = lambda v, p: validate_type(
            v, value_type, all_structs, all_enums, False, p,
            _indexes, _enum_values, _fields
        )
        return validate_map(value, value_validator, path)
    elif type_def.get('userDefined'):
        # Resolve against prebuilt indexes instead of re-sorting both tables on
        # every validation (review finding 2).  Contract passes all three cache
        # objects; standalone callers still get the original lazy behaviour.
        if _indexes is None:
            _indexes = (_get_resolution_index(all_structs),
                        _get_resolution_index(all_enums))
        user_type = type_def['userDefined']
        struct_key = _resolve_with_index(user_type, all_structs, _indexes[0])
        if struct_key is not None:
            return validate_struct(
                value, struct_key, all_structs[struct_key], all_structs,
                all_enums, path, _indexes, _fields, _enum_values
            )
        enum_key = _resolve_with_index(user_type, all_enums, _indexes[1])
        if enum_key is not None:
            if _enum_values is not None and enum_key in _enum_values:
                # O(1) membership with the ordered list retained for error text.
                allowed_set, allowed_values = _enum_values[enum_key]
            else:
                allowed_values = [v['name'] for v in all_enums[enum_key].get('values', [])]
                allowed_set = frozenset(allowed_values)
            return validate_enum(value, enum_key, allowed_values, path, allowed_set)
        return [_make_error(path, f"Unknown user-defined type: {user_type}")]
    else:
        return [_make_error(path, f"Invalid type definition: {type_def}")]
