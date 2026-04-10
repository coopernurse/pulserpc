"""Validation functions for PulseRPC types"""

from types import find_struct, find_enum, get_struct_fields


def validate_string(value):
    """Validate that value is a string"""
    if not isinstance(value, basestring):
        raise TypeError("Expected string, got %s" % type(value).__name__)


def validate_int(value):
    """Validate that value is an int"""
    if isinstance(value, int):
        return
    if isinstance(value, float) and value == int(value):
        return
    raise TypeError("Expected int, got %s" % type(value).__name__)


def validate_float(value):
    """Validate that value is a float or int"""
    if not isinstance(value, (int, float)):
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
        if not isinstance(key, basestring):
            raise TypeError("Map key must be string, got %s" % type(key).__name__)
        try:
            value_validator(val)
        except Exception as e:
            raise ValueError("Map value for key '%s' validation failed: %s" % (key, e))


def validate_enum(value, enum_name, allowed_values):
    """Validate that value is a string and matches one of the allowed enum values"""
    if not isinstance(value, basestring):
        raise TypeError("Expected string for enum %s, got %s" % (enum_name, type(value).__name__))
    if value not in allowed_values:
        raise ValueError("Invalid value for enum %s: '%s'. Allowed values: %s" % (enum_name, value, allowed_values))


def validate_struct(value, struct_name, struct_def, all_structs, all_enums):
    """Validate that value is a dict matching the struct definition"""
    if not isinstance(value, dict):
        raise TypeError("Expected dict for struct %s, got %s" % (struct_name, type(value).__name__))
    
    fields = get_struct_fields(struct_name, all_structs)
    
    for field in fields:
        field_name = field['name']
        field_type = field['type']
        is_optional = field.get('optional', False)
        
        if field_name not in value:
            if not is_optional:
                raise ValueError("Missing required field '%s' in struct %s" % (field_name, struct_name))
        else:
            field_value = value[field_name]
            if field_value is None:
                if not is_optional:
                    raise ValueError("Field '%s' in struct %s cannot be None" % (field_name, struct_name))
            else:
                def make_validator(ft, als, ale, io):
                    return lambda x: validate_type(x, ft, als, ale, io)
                try:
                    make_validator(field_type, all_structs, all_enums, is_optional)(field_value)
                except Exception as e:
                    raise ValueError("Field '%s' in struct %s validation failed: %s" % (field_name, struct_name, e))


def validate_type(value, type_def, all_structs, all_enums, is_optional=False):
    """Validate a value against a type definition"""
    if value is None:
        if is_optional:
            return
        else:
            raise ValueError("Value cannot be None for non-optional type")
    
    if type_def.get('builtIn') == 'string':
        validate_string(value)
    elif type_def.get('builtIn') == 'int':
        validate_int(value)
    elif type_def.get('builtIn') == 'float':
        validate_float(value)
    elif type_def.get('builtIn') == 'bool':
        validate_bool(value)
    elif type_def.get('array'):
        element_type = type_def['array']
        def make_element_validator(et, als, ale):
            return lambda x: validate_type(x, et, als, ale, False)
        validate_array(value, make_element_validator(element_type, all_structs, all_enums))
    elif type_def.get('mapValue'):
        value_type = type_def['mapValue']
        def make_value_validator(vt, als, ale):
            return lambda x: validate_type(x, vt, als, ale, False)
        validate_map(value, make_value_validator(value_type, all_structs, all_enums))
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
            validate_struct(value, user_type, struct_def, all_structs, all_enums)
        else:
            enum_def = find_enum(user_type, all_enums)
            if not enum_def and '.' not in user_type:
                for qualified_key in all_enums:
                    if qualified_key.endswith('.' + user_type):
                        enum_def = all_enums[qualified_key]
                        break
            if enum_def:
                allowed_values = [v['name'] for v in enum_def.get('values', [])]
                validate_enum(value, user_type, allowed_values)
            else:
                raise ValueError("Unknown user-defined type: %s" % user_type)
    else:
        raise ValueError("Invalid type definition: %s" % type_def)
