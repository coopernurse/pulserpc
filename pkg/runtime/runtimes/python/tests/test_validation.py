"""Tests for validation functions"""

import pytest
from pulserpc import (
    validate_string,
    validate_int,
    validate_float,
    validate_bool,
    validate_array,
    validate_map,
    validate_enum,
    validate_struct,
    validate_type,
)


class TestBuiltInTypes:
    """Test built-in type validators"""
    
    def test_validate_string_success(self):
        validate_string("hello")
        validate_string("")
    
    def test_validate_string_failure(self):
        errs = validate_string(123)
        assert len(errs) == 1
        assert "Expected string" in errs[0].message

        errs2 = validate_string(None)
        assert len(errs2) == 1
        assert "Expected string" in errs2[0].message
    
    def test_validate_int_success(self):
        validate_int(0)
        validate_int(42)
        validate_int(-100)
    
    def test_validate_int_failure(self):
        errs = validate_int("123")
        assert len(errs) == 1
        assert "Expected int" in errs[0].message

        errs2 = validate_int(3.14)
        assert len(errs2) == 1
        assert "Expected int" in errs2[0].message

        # Test bool rejection
        errs3 = validate_int(True)
        assert len(errs3) == 1
        assert "Expected int" in errs3[0].message

        # Test infinity
        errs4 = validate_int(float('inf'))
        assert len(errs4) == 1
        assert "Expected int" in errs4[0].message

        # Test NaN
        errs5 = validate_int(float('nan'))
        assert len(errs5) == 1
        assert "Expected int" in errs5[0].message
    
    def test_validate_float_success(self):
        validate_float(3.14)
        validate_float(42)  # int is acceptable
        validate_float(-1.5)
    
    def test_validate_float_failure(self):
        errs = validate_float("3.14")
        assert len(errs) == 1
        assert "Expected float" in errs[0].message

        errs2 = validate_float(None)
        assert len(errs2) == 1
        assert "Expected float" in errs2[0].message
    
    def test_validate_bool_success(self):
        validate_bool(True)
        validate_bool(False)
    
    def test_validate_bool_failure(self):
        errs = validate_bool(1)
        assert len(errs) == 1
        assert "Expected bool" in errs[0].message

        errs2 = validate_bool("true")
        assert len(errs2) == 1
        assert "Expected bool" in errs2[0].message


class TestArrayValidation:
    """Test array validation"""
    
    def test_validate_array_success(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array(["a", "b", "c"], element_validator)
        assert len(errs) == 0
        errs2 = validate_array([], element_validator)
        assert len(errs2) == 0
    
    def test_validate_array_wrong_type(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array("not a list", element_validator)
        assert len(errs) == 1
        assert "Expected list" in errs[0].message

        errs2 = validate_array({}, element_validator)
        assert len(errs2) == 1
        assert "Expected list" in errs2[0].message
    
    def test_validate_array_element_validation_fails(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array(["a", 123, "c"], element_validator)
        assert len(errs) == 1
        assert "[1]" in errs[0].path
        assert "Expected string" in errs[0].message


class TestMapValidation:
    """Test map validation"""
    
    def test_validate_map_success(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map({"a": 1, "b": 2}, value_validator)
        assert len(errs) == 0
        errs2 = validate_map({}, value_validator)
        assert len(errs2) == 0
    
    def test_validate_map_wrong_type(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map("not a dict", value_validator)
        assert len(errs) == 1
        assert "Expected dict" in errs[0].message

        errs2 = validate_map([], value_validator)
        assert len(errs2) == 1
        assert "Expected dict" in errs2[0].message
    
    def test_validate_map_non_string_key(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map({123: 1}, value_validator)
        assert len(errs) == 1
        assert "Map key must be string" in errs[0].message
    
    def test_validate_map_value_validation_fails(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map({"a": "not an int"}, value_validator)
        assert len(errs) == 1
        assert "[a]" in errs[0].path


class TestEnumValidation:
    """Test enum validation"""
    
    def test_validate_enum_success(self):
        errs = validate_enum("kindle", "Platform", ["kindle", "nook"])
        assert len(errs) == 0
        errs2 = validate_enum("nook", "Platform", ["kindle", "nook"])
        assert len(errs2) == 0
    
    def test_validate_enum_wrong_type(self):
        errs = validate_enum(123, "Platform", ["kindle", "nook"])
        assert len(errs) == 1
        assert "Expected string for enum" in errs[0].message
    
    def test_validate_enum_invalid_value(self):
        errs = validate_enum("invalid", "Platform", ["kindle", "nook"])
        assert len(errs) == 1
        assert "Invalid value for enum" in errs[0].message


class TestStructValidation:
    """Test struct validation"""
    
    def test_validate_struct_success(self):
        all_structs = {
            'User': {
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'string'}, 'optional': False},
                    {'name': 'name', 'type': {'builtIn': 'string'}, 'optional': False},
                ]
            }
        }
        all_enums = {}
        struct_def = all_structs['User']
        
        errs = validate_struct(
            {'id': '123', 'name': 'Alice'},
            'User',
            struct_def,
            all_structs,
            all_enums
        )
        assert len(errs) == 0
    
    def test_validate_struct_missing_required_field(self):
        all_structs = {
            'User': {
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'string'}, 'optional': False},
                ]
            }
        }
        all_enums = {}
        struct_def = all_structs['User']
        
        errs = validate_struct({}, 'User', struct_def, all_structs, all_enums)
        assert len(errs) == 1
        assert "Missing required field" in errs[0].message
    
    def test_validate_struct_optional_field(self):
        all_structs = {
            'User': {
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'string'}, 'optional': False},
                    {'name': 'email', 'type': {'builtIn': 'string'}, 'optional': True},
                ]
            }
        }
        all_enums = {}
        struct_def = all_structs['User']
        
        # Should work without optional field
        errs = validate_struct({'id': '123'}, 'User', struct_def, all_structs, all_enums)
        assert len(errs) == 0
        
        # Should work with optional field
        errs2 = validate_struct(
            {'id': '123', 'email': 'alice@example.com'},
            'User',
            struct_def,
            all_structs,
            all_enums
        )
        assert len(errs2) == 0
    
    def test_validate_struct_with_extends(self):
        all_structs = {
            'Base': {
                'fields': [
                    {'name': 'id', 'type': {'builtIn': 'string'}, 'optional': False},
                ]
            },
            'User': {
                'extends': 'Base',
                'fields': [
                    {'name': 'name', 'type': {'builtIn': 'string'}, 'optional': False},
                ]
            }
        }
        all_enums = {}
        struct_def = all_structs['User']
        
        # Should validate both parent and child fields
        errs = validate_struct(
            {'id': '123', 'name': 'Alice'},
            'User',
            struct_def,
            all_structs,
            all_enums
        )
        assert len(errs) == 0
        
        # Should fail if parent field missing
        errs2 = validate_struct({'name': 'Alice'}, 'User', struct_def, all_structs, all_enums)
        assert len(errs2) == 1
        assert "Missing required field" in errs2[0].message


class TestTypeValidation:
    """Test main validate_type function"""
    
    def test_validate_type_string(self):
        all_structs = {}
        all_enums = {}
        errs = validate_type("hello", {'builtIn': 'string'}, all_structs, all_enums)
        assert len(errs) == 0
    
    def test_validate_type_optional_none(self):
        all_structs = {}
        all_enums = {}
        
        # null is valid for optional fields
        errs = validate_type(None, {'builtIn': 'string'}, all_structs, all_enums, is_optional=True)
        assert len(errs) == 0
        
        # null is NOT valid for non-optional fields
        errs2 = validate_type(None, {'builtIn': 'string'}, all_structs, all_enums, is_optional=False)
        assert len(errs2) == 1
        assert "cannot be None" in errs2[0].message
    
    def test_validate_type_array(self):
        all_structs = {}
        all_enums = {}
        type_def = {'array': {'builtIn': 'string'}}
        
        errs = validate_type(["a", "b"], type_def, all_structs, all_enums)
        assert len(errs) == 0
        
        errs2 = validate_type(["a", 123], type_def, all_structs, all_enums)
        assert len(errs2) == 1
        assert "[1]" in errs2[0].path
    
    def test_validate_type_map(self):
        all_structs = {}
        all_enums = {}
        type_def = {'mapValue': {'builtIn': 'int'}}
        
        errs = validate_type({"a": 1, "b": 2}, type_def, all_structs, all_enums)
        assert len(errs) == 0
        
        errs2 = validate_type({"a": "not int"}, type_def, all_structs, all_enums)
        assert len(errs2) == 1
        assert "[a]" in errs2[0].path

