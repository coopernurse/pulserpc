"""Comprehensive tests for PulseRPC Python 2.7 validation functions"""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from validation import (
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
from rpctypes import find_struct, find_enum, get_struct_fields


def test_string_validation():
    print("Testing string validation...")
    validate_string("hello")
    validate_string(u"unicode string")
    validate_string("")
    
    try:
        validate_string(123)
        assert False, "Should have raised TypeError for int"
    except TypeError:
        pass
    
    try:
        validate_string(None)
        assert False, "Should have raised TypeError for None"
    except TypeError:
        pass
    
    try:
        validate_string([])
        assert False, "Should have raised TypeError for list"
    except TypeError:
        pass
    
    print("  PASS: String validation")


def test_int_validation():
    print("Testing int validation...")
    validate_int(42)
    validate_int(-100)
    validate_int(0)
    validate_int(1.0)
    validate_int(100.0)
    
    try:
        validate_int("string")
        assert False, "Should have raised TypeError for string"
    except TypeError:
        pass
    
    try:
        validate_int(1.5)
        assert False, "Should have raised TypeError for float with decimals"
    except TypeError:
        pass
    
    try:
        validate_int(None)
        assert False, "Should have raised TypeError for None"
    except TypeError:
        pass
    
    # Note: In Python 2, bool is a subclass of int, so True/False are valid ints
    # This is expected behavior in Python 2
    validate_int(True)
    validate_int(False)
    
    print("  PASS: Int validation")


def test_float_validation():
    print("Testing float validation...")
    validate_float(1.5)
    validate_float(-3.14)
    validate_float(0.0)
    validate_float(42)
    validate_float(100)
    
    try:
        validate_float("string")
        assert False, "Should have raised TypeError for string"
    except TypeError:
        pass
    
    try:
        validate_float(None)
        assert False, "Should have raised TypeError for None"
    except TypeError:
        pass
    
    try:
        validate_float([])
        assert False, "Should have raised TypeError for list"
    except TypeError:
        pass
    
    print("  PASS: Float validation")


def test_bool_validation():
    print("Testing bool validation...")
    validate_bool(True)
    validate_bool(False)
    
    try:
        validate_bool(1)
        assert False, "Should have raised TypeError for int 1"
    except TypeError:
        pass
    
    try:
        validate_bool(0)
        assert False, "Should have raised TypeError for int 0"
    except TypeError:
        pass
    
    try:
        validate_bool("true")
        assert False, "Should have raised TypeError for string"
    except TypeError:
        pass
    
    try:
        validate_bool(None)
        assert False, "Should have raised TypeError for None"
    except TypeError:
        pass
    
    print("  PASS: Bool validation")


def test_array_validation():
    print("Testing array validation...")
    
    validate_array([], lambda x: None)
    validate_array([1, 2, 3], validate_int)
    validate_array(["a", "b", "c"], validate_string)
    validate_array([1.0, 2.0, 3.0], validate_float)
    validate_array([True, False], validate_bool)
    
    try:
        validate_array("not a list", validate_int)
        assert False, "Should have raised TypeError for string"
    except TypeError:
        pass
    
    try:
        validate_array(123, validate_int)
        assert False, "Should have raised TypeError for int"
    except TypeError:
        pass
    
    try:
        validate_array([1, 2, "three"], validate_int)
        assert False, "Should have raised ValueError for invalid element"
    except ValueError:
        pass
    
    print("  PASS: Array validation")


def test_map_validation():
    print("Testing map validation...")
    
    validate_map({}, validate_int)
    validate_map({"a": 1, "b": 2}, validate_int)
    validate_map({"x": "hello", "y": "world"}, validate_string)
    
    try:
        validate_map("not a dict", validate_int)
        assert False, "Should have raised TypeError for string"
    except TypeError:
        pass
    
    try:
        validate_map([1, 2, 3], validate_int)
        assert False, "Should have raised TypeError for list"
    except TypeError:
        pass
    
    try:
        validate_map({1: "a", 2: "b"}, validate_string)
        assert False, "Should have raised TypeError for non-string keys"
    except TypeError:
        pass
    
    print("  PASS: Map validation")


def test_enum_validation():
    print("Testing enum validation...")
    
    validate_enum("value1", "MyEnum", ["value1", "value2", "value3"])
    validate_enum("value2", "MyEnum", ["value1", "value2", "value3"])
    
    try:
        validate_enum(123, "MyEnum", ["value1", "value2"])
        assert False, "Should have raised TypeError for int"
    except TypeError:
        pass
    
    try:
        validate_enum("invalid", "MyEnum", ["value1", "value2"])
        assert False, "Should have raised ValueError for invalid value"
    except ValueError:
        pass
    
    try:
        validate_enum(None, "MyEnum", ["value1", "value2"])
        assert False, "Should have raised TypeError for None"
    except TypeError:
        pass
    
    print("  PASS: Enum validation")


def test_struct_validation():
    print("Testing struct validation...")
    
    all_structs = {
        "Person": {
            "name": "Person",
            "fields": [
                {"name": "name", "type": {"builtIn": "string"}},
                {"name": "age", "type": {"builtIn": "int"}, "optional": True},
            ]
        }
    }
    all_enums = {}
    
    validate_struct({"name": "John"}, "Person", all_structs["Person"], all_structs, all_enums)
    validate_struct({"name": "John", "age": 30}, "Person", all_structs["Person"], all_structs, all_enums)
    validate_struct({"name": "Jane", "age": None}, "Person", all_structs["Person"], all_structs, all_enums)
    
    try:
        validate_struct({}, "Person", all_structs["Person"], all_structs, all_enums)
        assert False, "Should have raised ValueError for missing required field"
    except ValueError:
        pass
    
    try:
        validate_struct({"name": "John", "age": "not an int"}, "Person", all_structs["Person"], all_structs, all_enums)
        assert False, "Should have raised ValueError for invalid type"
    except ValueError:
        pass
    
    # Note: Extra fields are allowed (not rejected) - this matches PulseRPC behavior
    validate_struct({"name": "John", "age": 30, "extra": "field"}, "Person", all_structs["Person"], all_structs, all_enums)
    
    print("  PASS: Struct validation")


def test_struct_validation_with_inheritance():
    print("Testing struct validation with inheritance...")
    
    all_structs = {
        "Entity": {
            "name": "Entity",
            "fields": [
                {"name": "id", "type": {"builtIn": "string"}},
            ]
        },
        "Person": {
            "name": "Person",
            "extends": "Entity",
            "fields": [
                {"name": "name", "type": {"builtIn": "string"}},
            ]
        }
    }
    all_enums = {}
    
    validate_struct({"id": "123", "name": "John"}, "Person", all_structs["Person"], all_structs, all_enums)
    validate_struct({"id": "456", "name": "Jane"}, "Person", all_structs["Person"], all_structs, all_enums)
    
    try:
        validate_struct({"name": "John"}, "Person", all_structs["Person"], all_structs, all_enums)
        assert False, "Should have raised ValueError for missing inherited field"
    except ValueError:
        pass
    
    print("  PASS: Struct validation with inheritance")


def test_nested_struct_validation():
    print("Testing nested struct validation...")
    
    all_structs = {
        "Address": {
            "name": "Address",
            "fields": [
                {"name": "street", "type": {"builtIn": "string"}},
                {"name": "city", "type": {"builtIn": "string"}},
            ]
        },
        "Person": {
            "name": "Person",
            "fields": [
                {"name": "name", "type": {"builtIn": "string"}},
                {"name": "address", "type": {"userDefined": "Address"}},
            ]
        }
    }
    all_enums = {}
    
    validate_struct({
        "name": "John",
        "address": {"street": "123 Main St", "city": "Anytown"}
    }, "Person", all_structs["Person"], all_structs, all_enums)
    
    try:
        validate_struct({
            "name": "John",
            "address": {"street": 123, "city": "Anytown"}
        }, "Person", all_structs["Person"], all_structs, all_enums)
        assert False, "Should have raised ValueError for invalid nested struct"
    except ValueError:
        pass
    
    print("  PASS: Nested struct validation")


def test_array_of_user_types():
    print("Testing array of user-defined types...")
    
    all_structs = {
        "Person": {
            "name": "Person",
            "fields": [
                {"name": "name", "type": {"builtIn": "string"}},
            ]
        }
    }
    all_enums = {}
    
    validate_type([
        {"name": "John"},
        {"name": "Jane"}
    ], {"array": {"userDefined": "Person"}}, all_structs, all_enums, False)
    
    try:
        validate_type([
            {"name": "John"},
            {"name": 123}
        ], {"array": {"userDefined": "Person"}}, all_structs, all_enums, False)
        assert False, "Should have raised ValueError for invalid array element"
    except ValueError:
        pass
    
    print("  PASS: Array of user-defined types")


def test_types_helper_functions():
    print("Testing types helper functions...")
    
    all_structs = {
        "Person": {
            "name": "Person",
            "extends": "Entity",
            "fields": [
                {"name": "name", "type": {"builtIn": "string"}},
            ]
        },
        "Entity": {
            "name": "Entity",
            "fields": [
                {"name": "id", "type": {"builtIn": "string"}},
            ]
        }
    }
    all_enums = {
        "Status": {
            "name": "Status",
            "values": [
                {"name": "ACTIVE"},
                {"name": "INACTIVE"},
            ]
        }
    }
    
    assert find_struct("Person", all_structs) is not None
    assert find_struct("NonExistent", all_structs) is None
    
    assert find_enum("Status", all_enums) is not None
    assert find_enum("NonExistent", all_enums) is None
    
    fields = get_struct_fields("Person", all_structs)
    field_names = [f["name"] for f in fields]
    assert "id" in field_names
    assert "name" in field_names
    
    print("  PASS: Types helper functions")


def test_validate_type_optional():
    print("Testing validate_type with optional types...")
    
    all_structs = {}
    all_enums = {}
    
    validate_type(None, {"builtIn": "string"}, all_structs, all_enums, True)
    validate_type("hello", {"builtIn": "string"}, all_structs, all_enums, True)
    
    try:
        validate_type(None, {"builtIn": "string"}, all_structs, all_enums, False)
        assert False, "Should have raised ValueError for None on non-optional"
    except ValueError:
        pass
    
    print("  PASS: validate_type with optional types")


def main():
    print("=" * 60)
    print("PulseRPC Python 2.7 Validation Tests")
    print("=" * 60)
    
    test_string_validation()
    test_int_validation()
    test_float_validation()
    test_bool_validation()
    test_array_validation()
    test_map_validation()
    test_enum_validation()
    test_struct_validation()
    test_struct_validation_with_inheritance()
    test_nested_struct_validation()
    test_array_of_user_types()
    test_types_helper_functions()
    test_validate_type_optional()
    
    print("=" * 60)
    print("ALL TESTS PASSED")
    print("=" * 60)


if __name__ == "__main__":
    main()
