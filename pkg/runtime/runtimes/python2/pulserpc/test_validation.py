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
    
    # bool is rejected explicitly even though it subclasses int in Python 2.
    try:
        validate_int(True)
        assert False, "Should have raised TypeError for bool True"
    except TypeError:
        pass

    try:
        validate_int(False)
        assert False, "Should have raised TypeError for bool False"
    except TypeError:
        pass

    # NaN/infinity must raise TypeError, not an unwrapped ValueError/OverflowError
    # from int(value).
    try:
        validate_int(float("nan"))
        assert False, "Should have raised TypeError for NaN"
    except TypeError:
        pass

    try:
        validate_int(float("inf"))
        assert False, "Should have raised TypeError for infinity"
    except TypeError:
        pass

    try:
        validate_int(float("-inf"))
        assert False, "Should have raised TypeError for negative infinity"
    except TypeError:
        pass

    # D1/R25: IDL int is signed 32-bit, inclusive at both boundaries.
    validate_int(-2147483648)
    validate_int(2147483647)
    validate_int(-2147483648.0)
    validate_int(2147483647.0)
    for out_of_range in (2147483648, -2147483649, 2 ** 63, -(2 ** 63),
                         2147483648.0, -2147483649.0):
        try:
            validate_int(out_of_range)
            assert False, "Should have raised TypeError for %r" % (out_of_range,)
        except TypeError as exc:
            assert "signed 32-bit" in str(exc), str(exc)

    print("  PASS: Int validation")


def test_float_validation():
    print("Testing float validation...")
    validate_float(1.5)
    validate_float(-3.14)
    validate_float(0.0)
    validate_float(42)
    validate_float(100)
    # Non-finite floats remain valid for the float type, matching Python 3's
    # acceptance set (only validate_int rejects NaN/infinity).
    validate_float(float("nan"))
    validate_float(float("inf"))

    # bool subclasses int in Python 2, but JSON true/false are not floats.
    for bool_value in (True, False):
        try:
            validate_float(bool_value)
            assert False, "Should have raised TypeError for bool %r" % bool_value
        except TypeError as exc:
            assert "Expected float, got bool" in str(exc), str(exc)

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


def test_simple_name_lookup_with_namespaces():
    """Unqualified user-defined names resolve to namespaced definitions."""
    from contract import Contract
    from validation import ValidationContext

    structs = {
        "a.b.Address": {
            "name": "a.b.Address",
            "fields": [{"name": "zip", "type": {"builtIn": "string"}}],
        }
    }
    enums = {
        "x.y.Color": {
            "name": "x.y.Color",
            "values": [{"name": "RED"}, {"name": "GREEN"}],
        }
    }
    idl = {
        "interfaces": [{
            "name": "Svc",
            "methods": [{
                "name": "f",
                "parameters": [
                    {"name": "addr", "type": {"userDefined": "Address"}},
                    {"name": "color", "type": {"userDefined": "Color"}},
                ],
            }],
        }],
        "structs": list(structs.values()),
        "enums": list(enums.values()),
    }
    contract = Contract(idl)
    ctx = ValidationContext(structs, enums)
    assert ctx.struct_simple_names["Address"] == "a.b.Address"
    assert ctx.enum_simple_names["Color"] == "x.y.Color"

    # Cached path through Contract.
    contract.validate_request("Svc", "f", [{"zip": "1"}, "RED"])
    # Un-cached direct helper path.
    validate_type({"zip": "1"}, {"userDefined": "Address"}, structs, enums)
    validate_type("GREEN", {"userDefined": "Color"}, structs, enums)
    # Fully qualified names still resolve via the exact-key lookup.
    validate_type({"zip": "2"}, {"userDefined": "a.b.Address"}, structs, enums)
    validate_type("RED", {"userDefined": "x.y.Color"}, structs, enums)


def test_simple_name_duplicate_first_match_is_deterministic():
    """Ambiguous simple names resolve to the first key, as the old scan did."""
    from validation import ValidationContext

    structs = {
        "first.pkg.Shared": {
            "name": "first.pkg.Shared",
            "fields": [{"name": "a", "type": {"builtIn": "string"}}],
        },
        "second.pkg.Shared": {
            "name": "second.pkg.Shared",
            "fields": [{"name": "b", "type": {"builtIn": "int"}}],
        },
    }
    ctx = ValidationContext(structs, {})
    assert ctx.struct_simple_names["Shared"] == "first.pkg.Shared"

    validate_type({"a": "ok"}, {"userDefined": "Shared"}, structs, {}, False, ctx)
    try:
        validate_type({"b": 1}, {"userDefined": "Shared"}, structs, {}, False, ctx)
        assert False, "expected ValueError: 'a' is required by first.pkg.Shared"
    except ValueError:
        pass


def test_enum_membership_error_text_with_cached_set():
    """Frozenset membership must not change the ordered error text."""
    from validation import ValidationContext

    enums = {
        "ns.Status": {
            "name": "ns.Status",
            "values": [{"name": "ON"}, {"name": "OFF"}, {"name": "UNKNOWN"}],
        }
    }
    ctx = ValidationContext({}, enums)
    ordered = ["ON", "OFF", "UNKNOWN"]
    assert ctx.enum_values["ns.Status"] == ordered
    assert ctx.enum_value_sets["ns.Status"] == frozenset(ordered)

    validate_type("ON", {"userDefined": "Status"}, {}, enums, False, ctx)
    try:
        validate_type("BAD", {"userDefined": "Status"}, {}, enums, False, ctx)
        assert False, "expected ValueError for invalid enum value"
    except ValueError as exc:
        text = str(exc)
        assert "Invalid value for enum Status: 'BAD'" in text, text
        assert "Allowed values: %s" % ordered in text, text

    # Direct helper keeps the same text when given a precomputed set.
    try:
        validate_enum("BAD", "Status", ["ON", "OFF"], frozenset(["ON", "OFF"]))
        assert False, "expected ValueError for invalid enum value"
    except ValueError as exc:
        assert "Allowed values: ['ON', 'OFF']" in str(exc), str(exc)


def test_inherited_field_overrides():
    """Child fields override inherited ones in place, including via cache."""
    from contract import Contract
    from validation import ValidationContext

    structs = {
        "Base": {
            "name": "Base",
            "fields": [
                {"name": "id", "type": {"builtIn": "string"}},
                {"name": "value", "type": {"builtIn": "string"}},
                {"name": "extra", "type": {"builtIn": "string"}},
            ],
        },
        "Child": {
            "name": "Child",
            "extends": "Base",
            "fields": [
                {"name": "value", "type": {"builtIn": "int"}},
                {"name": "own", "type": {"builtIn": "bool"}},
            ],
        },
    }
    fields = get_struct_fields("Child", structs)
    assert [f["name"] for f in fields] == ["id", "value", "extra", "own"]
    assert fields[1]["type"] == {"builtIn": "int"}
    assert ValidationContext(structs, {}).struct_fields["Child"] == fields

    idl = {
        "interfaces": [{
            "name": "Svc",
            "methods": [{
                "name": "f",
                "parameters": [{"name": "c", "type": {"userDefined": "Child"}}],
            }],
        }],
        "structs": list(structs.values()),
        "enums": [],
    }
    contract = Contract(idl)
    contract.validate_request(
        "Svc", "f", [{"id": "x", "value": 7, "extra": "e", "own": True}]
    )
    for bad in (
        {"id": "x", "value": "not-int", "extra": "e", "own": True},
        {"value": 7, "extra": "e", "own": True},
    ):
        try:
            contract.validate_request("Svc", "f", [bad])
            assert False, "expected RPCError for %r" % bad
        except Exception:
            pass


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
