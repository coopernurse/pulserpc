"""Validation functions for PulseRPC types"""

from typing import Any, Callable, Dict, List

from .rpctypes import find_struct, find_enum, get_struct_fields, ValidationError


def _join_path(parent: str, child: str) -> str:
    return f"{parent}.{child}" if parent else f".{child}"


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
        return []
    if isinstance(value, float):
        if value != value:  # NaN check
            return [_make_error(path, f"Expected int, got float NaN")]
        if value == float('inf') or value == float('-inf'):
            return [_make_error(path, f"Expected int, got float infinity")]
        if value == int(value):
            return []
    return [_make_error(path, f"Expected int, got {type(value).__name__}")]


def validate_float(value: Any, path: str = "") -> List[ValidationError]:
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
    path: str = ""
) -> List[ValidationError]:
    if not isinstance(value, str):
        return [_make_error(path, f"Expected string for enum {enum_name}, got {type(value).__name__}")]
    if value not in allowed_values:
        return [_make_error(path, f"Invalid value for enum {enum_name}: '{value}'. Allowed values: {allowed_values}")]
    return []


def validate_struct(
    value: Any,
    struct_name: str,
    struct_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any],
    path: str = ""
) -> List[ValidationError]:
    if not isinstance(value, dict):
        return [_make_error(path, f"Expected dict for struct {struct_name}, got {type(value).__name__}")]

    fields = get_struct_fields(struct_name, all_structs)
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
                    validate_type(field_value, field_type, all_structs, all_enums, is_optional, field_path)
                )

    return errors


def validate_type(
    value: Any,
    type_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any],
    is_optional: bool = False,
    path: str = ""
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
        element_validator = lambda v, p: validate_type(v, element_type, all_structs, all_enums, False, p)
        return validate_array(value, element_validator, path)
    elif type_def.get('mapValue'):
        value_type = type_def['mapValue']
        value_validator = lambda v, p: validate_type(v, value_type, all_structs, all_enums, False, p)
        return validate_map(value, value_validator, path)
    elif type_def.get('userDefined'):
        user_type = type_def['userDefined']
        struct_def = find_struct(user_type, all_structs)
        if not struct_def and '.' not in user_type:
            for qualified_key in all_structs:
                if qualified_key.endswith('.' + user_type):
                    struct_def = all_structs[qualified_key]
                    user_type = qualified_key
                    break
        if struct_def:
            return validate_struct(value, user_type, struct_def, all_structs, all_enums, path)
        else:
            enum_def = find_enum(user_type, all_enums)
            if not enum_def and '.' not in user_type:
                for qualified_key in all_enums:
                    if qualified_key.endswith('.' + user_type):
                        enum_def = all_enums[qualified_key]
                        break
            if enum_def:
                allowed_values = [v['name'] for v in enum_def.get('values', [])]
                return validate_enum(value, user_type, allowed_values, path)
            else:
                return [_make_error(path, f"Unknown user-defined type: {user_type}")]
    else:
        return [_make_error(path, f"Invalid type definition: {type_def}")]
