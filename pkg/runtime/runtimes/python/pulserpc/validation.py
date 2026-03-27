"""Validation functions for PulseRPC types"""

from typing import Any, Callable, Dict, List

from .types import find_struct, find_enum, get_struct_fields


def validate_string(value: Any) -> None:
    """Validate that value is a string"""
    if not isinstance(value, str):
        raise TypeError(f"Expected string, got {type(value).__name__}")


def validate_int(value: Any) -> None:
    """Validate that value is an int"""
    if not isinstance(value, int):
        raise TypeError(f"Expected int, got {type(value).__name__}")


def validate_float(value: Any) -> None:
    """Validate that value is a float or int"""
    if not isinstance(value, (int, float)):
        raise TypeError(f"Expected float, got {type(value).__name__}")


def validate_bool(value: Any) -> None:
    """Validate that value is a bool"""
    if not isinstance(value, bool):
        raise TypeError(f"Expected bool, got {type(value).__name__}")


def validate_array(value: Any, element_validator: Callable[[Any], None]) -> None:
    """Validate that value is an array and each element passes validation"""
    if not isinstance(value, list):
        raise TypeError(f"Expected list, got {type(value).__name__}")
    for i, elem in enumerate(value):
        try:
            element_validator(elem)
        except Exception as e:
            raise ValueError(f"Array element at index {i} validation failed: {e}") from e


def validate_map(value: Any, value_validator: Callable[[Any], None]) -> None:
    """Validate that value is a map (dict) with string keys and validated values"""
    if not isinstance(value, dict):
        raise TypeError(f"Expected dict, got {type(value).__name__}")
    for key, val in value.items():
        if not isinstance(key, str):
            raise TypeError(f"Map key must be string, got {type(key).__name__}")
        try:
            value_validator(val)
        except Exception as e:
            raise ValueError(f"Map value for key '{key}' validation failed: {e}") from e


def validate_enum(value: Any, enum_name: str, allowed_values: List[str]) -> None:
    """Validate that value is a string and matches one of the allowed enum values"""
    if not isinstance(value, str):
        raise TypeError(f"Expected string for enum {enum_name}, got {type(value).__name__}")
    if value not in allowed_values:
        raise ValueError(f"Invalid value for enum {enum_name}: '{value}'. Allowed values: {allowed_values}")


def validate_struct(
    value: Any,
    struct_name: str,
    struct_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any]
) -> None:
    """Validate that value is a dict matching the struct definition"""
    if not isinstance(value, dict):
        raise TypeError(f"Expected dict for struct {struct_name}, got {type(value).__name__}")
    
    # Get all fields including parent fields
    fields = get_struct_fields(struct_name, all_structs)
    
    # Check required fields
    for field in fields:
        field_name = field['name']
        field_type = field['type']
        is_optional = field.get('optional', False)
        
        if field_name not in value:
            if not is_optional:
                raise ValueError(f"Missing required field '{field_name}' in struct {struct_name}")
        else:
            # Field is present, validate it
            field_value = value[field_name]
            if field_value is None:
                if not is_optional:
                    raise ValueError(f"Field '{field_name}' in struct {struct_name} cannot be None")
            else:
                # Create validator for this field type
                field_validator = lambda v: validate_type(v, field_type, all_structs, all_enums, is_optional)
                try:
                    field_validator(field_value)
                except Exception as e:
                    raise ValueError(f"Field '{field_name}' in struct {struct_name} validation failed: {e}") from e


def validate_type(
    value: Any,
    type_def: Dict[str, Any],
    all_structs: Dict[str, Any],
    all_enums: Dict[str, Any],
    is_optional: bool = False
) -> None:
    """Validate a value against a type definition"""
    # Handle optional types
    if value is None:
        if is_optional:
            return
        else:
            raise ValueError("Value cannot be None for non-optional type")
    
    # Built-in types
    if type_def.get('builtIn') == 'string':
        validate_string(value)
    elif type_def.get('builtIn') == 'int':
        validate_int(value)
    elif type_def.get('builtIn') == 'float':
        validate_float(value)
    elif type_def.get('builtIn') == 'bool':
        validate_bool(value)
    # Array types
    elif type_def.get('array'):
        element_type = type_def['array']
        element_validator = lambda v: validate_type(v, element_type, all_structs, all_enums, False)
        validate_array(value, element_validator)
    # Map types
    elif type_def.get('mapValue'):
        value_type = type_def['mapValue']
        value_validator = lambda v: validate_type(v, value_type, all_structs, all_enums, False)
        validate_map(value, value_validator)
    # User-defined types
    elif type_def.get('userDefined'):
        user_type = type_def['userDefined']
        # Check if it's a struct
        # Try direct lookup first (for qualified names like "inc.Response")
        struct_def = find_struct(user_type, all_structs)
        # If not found and it's a simple name (no dot), try qualifying with each namespace
        if not struct_def and '.' not in user_type:
            # Try to find the struct by looking for any qualified key that ends with this simple name
            for qualified_key in all_structs:
                if qualified_key.endswith('.' + user_type):
                    struct_def = all_structs[qualified_key]
                    user_type = qualified_key  # Update to use the qualified name
                    break
        if struct_def:
            validate_struct(value, user_type, struct_def, all_structs, all_enums)
        # Check if it's an enum
        else:
            enum_def = find_enum(user_type, all_enums)
            # If not found and it's a simple name (no dot), try qualifying with each namespace
            if not enum_def and '.' not in user_type:
                # Try to find the enum by looking for any qualified key that ends with this simple name
                for qualified_key in all_enums:
                    if qualified_key.endswith('.' + user_type):
                        enum_def = all_enums[qualified_key]
                        break
            if enum_def:
                allowed_values = [v['name'] for v in enum_def.get('values', [])]
                validate_enum(value, user_type, allowed_values)
            else:
                raise ValueError(f"Unknown user-defined type: {user_type}")
    else:
        raise ValueError(f"Invalid type definition: {type_def}")

