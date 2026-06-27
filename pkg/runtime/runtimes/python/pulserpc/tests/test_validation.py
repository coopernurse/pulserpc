"""Tests for validation functions"""

import unittest
from pulserpc.validation import (
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
from pulserpc.contract import Contract, build_validation_result
from pulserpc.rpctypes import ValidationError, ValidationResult


class TestValidateString(unittest.TestCase):
    def test_success(self):
        self.assertEqual(validate_string("hello"), [])
        self.assertEqual(validate_string(""), [])

    def test_failure(self):
        errs = validate_string(123)
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected string", errs[0].message)
        self.assertEqual(errs[0].path, "")


class TestValidateInt(unittest.TestCase):
    def test_success(self):
        self.assertEqual(validate_int(0), [])
        self.assertEqual(validate_int(42), [])
        self.assertEqual(validate_int(-100), [])
        self.assertEqual(validate_int(5.0), [])

    def test_failure(self):
        errs = validate_int("123")
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected int", errs[0].message)

        errs2 = validate_int(3.14)
        self.assertEqual(len(errs2), 1)
        self.assertIn("Expected int", errs2[0].message)


class TestValidateFloat(unittest.TestCase):
    def test_success(self):
        self.assertEqual(validate_float(3.14), [])
        self.assertEqual(validate_float(42), [])

    def test_failure(self):
        errs = validate_float("3.14")
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected float", errs[0].message)


class TestValidateBool(unittest.TestCase):
    def test_success(self):
        self.assertEqual(validate_bool(True), [])
        self.assertEqual(validate_bool(False), [])

    def test_failure(self):
        self.assertEqual(len(validate_bool(1)), 1)
        self.assertEqual(len(validate_bool("true")), 1)


class TestValidateArray(unittest.TestCase):
    def test_success(self):
        element_validator = lambda v, p: validate_string(v, p)
        self.assertEqual(validate_array(["a", "b", "c"], element_validator), [])
        self.assertEqual(validate_array([], element_validator), [])

    def test_wrong_type(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array("not a list", element_validator)
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected list", errs[0].message)

    def test_element_failure(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array(["a", 123, "c"], element_validator)
        self.assertEqual(len(errs), 1)
        self.assertIn("[1]", errs[0].path)

    def test_path_tracking(self):
        element_validator = lambda v, p: validate_string(v, p)
        errs = validate_array(["a", "b", 42], element_validator, ".items")
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].path, ".items[2]")


class TestValidateMap(unittest.TestCase):
    def test_success(self):
        value_validator = lambda v, p: validate_int(v, p)
        self.assertEqual(validate_map({"a": 1, "b": 2}, value_validator), [])
        self.assertEqual(validate_map({}, value_validator), [])

    def test_wrong_type(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map("not a dict", value_validator)
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected dict", errs[0].message)

    def test_value_failure(self):
        value_validator = lambda v, p: validate_int(v, p)
        errs = validate_map({"a": "not an int"}, value_validator)
        self.assertEqual(len(errs), 1)
        self.assertIn("[a]", errs[0].path)


class TestValidateEnum(unittest.TestCase):
    def test_success(self):
        self.assertEqual(validate_enum("kindle", "Platform", ["kindle", "nook"]), [])
        self.assertEqual(validate_enum("nook", "Platform", ["kindle", "nook"]), [])

    def test_wrong_type(self):
        errs = validate_enum(123, "Platform", ["kindle", "nook"])
        self.assertEqual(len(errs), 1)
        self.assertIn("Expected string for enum", errs[0].message)

    def test_invalid_value(self):
        errs = validate_enum("invalid", "Platform", ["kindle", "nook"])
        self.assertEqual(len(errs), 1)
        self.assertIn("Invalid value for enum", errs[0].message)


class TestValidateStruct(unittest.TestCase):
    def setUp(self):
        self.structs = {
            "User": {
                "fields": [
                    {"name": "id", "type": {"builtIn": "string"}, "optional": False},
                    {"name": "name", "type": {"builtIn": "string"}, "optional": False},
                ]
            }
        }
        self.enums = {}

    def test_success(self):
        errs = validate_struct(
            {"id": "123", "name": "Alice"},
            "User", self.structs["User"], self.structs, self.enums
        )
        self.assertEqual(errs, [])

    def test_missing_required_fields(self):
        errs = validate_struct(
            {}, "User", self.structs["User"], self.structs, self.enums
        )
        self.assertEqual(len(errs), 2)
        self.assertEqual(errs[0].path, ".id")
        self.assertEqual(errs[1].path, ".name")

    def test_optional_field(self):
        structs = {
            "User": {
                "fields": [
                    {"name": "id", "type": {"builtIn": "string"}, "optional": False},
                    {"name": "email", "type": {"builtIn": "string"}, "optional": True},
                ]
            }
        }
        self.assertEqual(
            validate_struct({"id": "123"}, "User", structs["User"], structs, self.enums),
            []
        )
        self.assertEqual(
            validate_struct({"id": "123", "email": "a@b.com"}, "User", structs["User"], structs, self.enums),
            []
        )

    def test_with_extends(self):
        structs = {
            "Base": {
                "fields": [{"name": "id", "type": {"builtIn": "string"}, "optional": False}],
            },
            "User": {
                "extends": "Base",
                "fields": [{"name": "name", "type": {"builtIn": "string"}, "optional": False}],
            },
        }
        self.assertEqual(
            validate_struct({"id": "123", "name": "Alice"}, "User", structs["User"], structs, self.enums),
            []
        )
        errs = validate_struct({"name": "Alice"}, "User", structs["User"], structs, self.enums)
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].path, ".id")

    def test_collects_multiple_errors(self):
        structs = {
            "Person": {
                "fields": [
                    {"name": "username", "type": {"builtIn": "string"}},
                    {"name": "age", "type": {"builtIn": "int"}},
                    {"name": "email", "type": {"builtIn": "string"}},
                ]
            }
        }
        errs = validate_struct(
            {"username": "alice", "age": "not-a-number", "email": 42},
            "Person", structs["Person"], structs, self.enums
        )
        self.assertEqual(len(errs), 2)
        self.assertEqual(errs[0].path, ".age")
        self.assertEqual(errs[1].path, ".email")


class TestValidateType(unittest.TestCase):
    def test_string(self):
        self.assertEqual(validate_type("hello", {"builtIn": "string"}, {}, {}), [])

    def test_optional_none(self):
        self.assertEqual(validate_type(None, {"builtIn": "string"}, {}, {}, True), [])
        errs = validate_type(None, {"builtIn": "string"}, {}, {}, False)
        self.assertEqual(len(errs), 1)
        self.assertIn("cannot be None", errs[0].message)

    def test_array(self):
        type_def = {"array": {"builtIn": "string"}}
        self.assertEqual(validate_type(["a", "b"], type_def, {}, {}), [])
        errs = validate_type(["a", 123], type_def, {}, {})
        self.assertEqual(len(errs), 1)
        self.assertIn("[1]", errs[0].path)

    def test_map(self):
        type_def = {"mapValue": {"builtIn": "int"}}
        self.assertEqual(validate_type({"a": 1, "b": 2}, type_def, {}, {}), [])
        errs = validate_type({"a": "not int"}, type_def, {}, {})
        self.assertEqual(len(errs), 1)
        self.assertIn("[a]", errs[0].path)

    def test_nested_struct_in_array(self):
        structs = {
            "Child": {
                "fields": [
                    {"name": "name", "type": {"builtIn": "string"}},
                    {"name": "age", "type": {"builtIn": "int"}},
                ]
            },
            "Person": {
                "fields": [
                    {"name": "name", "type": {"builtIn": "string"}},
                    {"name": "children", "type": {"array": {"userDefined": "Child"}}},
                ]
            },
        }
        self.assertEqual(
            validate_type(
                {"name": "Alice", "children": [{"name": "Bob", "age": 10}, {"name": "Charlie", "age": 12}]},
                {"userDefined": "Person"}, structs, {}
            ),
            []
        )
        errs = validate_type(
            {"name": "Alice", "children": [{"name": "Bob", "age": 10}, {"name": "Charlie", "age": "twelve"}]},
            {"userDefined": "Person"}, structs, {}
        )
        self.assertEqual(len(errs), 1)
        self.assertEqual(errs[0].path, ".children[1].age")


class TestBuildValidationResult(unittest.TestCase):
    def test_no_errors(self):
        result = build_validation_result([])
        self.assertTrue(result.valid)
        self.assertIsNone(result.error)
        self.assertIsNone(result.invalid_fields)

    def test_errors_with_paths(self):
        errors = [
            ValidationError(path=".username", message="Missing required field"),
            ValidationError(path=".email", message="Expected string, got int"),
        ]
        result = build_validation_result(errors)
        self.assertFalse(result.valid)
        self.assertIsNotNone(result.error)
        self.assertEqual(result.invalid_fields, [".username", ".email"])

    def test_errors_without_paths(self):
        errors = [
            ValidationError(path="", message="Top-level error"),
        ]
        result = build_validation_result(errors)
        self.assertFalse(result.valid)
        self.assertIsNotNone(result.error)
        self.assertIsNone(result.invalid_fields)


class TestContractValidate(unittest.TestCase):
    def setUp(self):
        idl = {
            "structs": [
                {
                    "name": "Person",
                    "fields": [
                        {"name": "username", "type": {"builtIn": "string"}},
                        {"name": "age", "type": {"builtIn": "int"}},
                        {"name": "email", "type": {"builtIn": "string"}},
                    ],
                },
                {
                    "name": "Status",
                    "fields": [
                        {"name": "code", "type": {"builtIn": "int"}},
                    ],
                },
            ],
            "enums": [
                {
                    "name": "Color",
                    "values": [{"name": "red"}, {"name": "green"}, {"name": "blue"}],
                }
            ],
        }
        self.contract = Contract(idl)

    def test_valid_struct(self):
        result = self.contract.validate("Person", {"username": "alice", "age": 30, "email": "alice@example.com"})
        self.assertTrue(result.valid)

    def test_missing_fields(self):
        result = self.contract.validate("Person", {"username": "alice"})
        self.assertFalse(result.valid)
        self.assertIsNotNone(result.error)
        self.assertIsNotNone(result.invalid_fields)

    def test_invalid_field_type(self):
        result = self.contract.validate("Person", {"username": "alice", "age": "thirty", "email": 42})
        self.assertFalse(result.valid)
        self.assertEqual(len(result.invalid_fields), 2)

    def test_valid_enum(self):
        result = self.contract.validate("Color", "red")
        self.assertTrue(result.valid)

    def test_invalid_enum(self):
        result = self.contract.validate("Color", "yellow")
        self.assertFalse(result.valid)
        self.assertIsNotNone(result.error)

    def test_unknown_type(self):
        result = self.contract.validate("NonExistent", {})
        self.assertFalse(result.valid)


if __name__ == "__main__":
    unittest.main()
