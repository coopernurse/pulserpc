"""Tests for diff.py contract verification functionality"""

import unittest
from datetime import datetime
from pulserpc.diff import diff_idl
from pulserpc.rpctypes import (
    EntityType, ChangeType, Direction, Severity, ContractDelta, VerificationResult
)


class TestDiffIDL(unittest.TestCase):
    """Test cases for diff_idl function"""

    def test_identical_idls_produces_no_deltas(self):
        """Test that identical IDLs produces no deltas"""
        idl = {
            "interfaces": [
                {"name": "TestService", "methods": [
                    {"name": "getData", "parameters": [], "returnType": {"builtIn": "string"}}
                ]}
            ],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [
                {"name": "TestEnum", "values": [{"name": "Value1"}, {"name": "Value2"}]}
            ],
            "errors": [
                {"name": "TestError", "code": 100}
            ]
        }
        deltas = diff_idl(idl, idl)
        self.assertEqual(len(deltas), 0)

    def test_added_optional_field_returns_info(self):
        """Test that server has optional field client doesn't have returns Info severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False},
                    {"name": "field2", "type": {"builtIn": "string"}, "optional": True}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.FIELD)
        self.assertEqual(delta.change_type, ChangeType.ADDED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_LESS)
        self.assertEqual(delta.severity, Severity.INFO)

    def test_added_required_field_returns_error(self):
        """Test that server has required field client doesn't have returns Error severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False},
                    {"name": "field2", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.FIELD)
        self.assertEqual(delta.change_type, ChangeType.ADDED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_LESS)
        self.assertEqual(delta.severity, Severity.ERROR)

    def test_removed_field_returns_info(self):
        """Test that client has field server doesn't have returns Info severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False},
                    {"name": "field2", "type": {"builtIn": "string"}, "optional": True}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.FIELD)
        self.assertEqual(delta.change_type, ChangeType.REMOVED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_MORE)
        self.assertEqual(delta.severity, Severity.INFO)

    def test_field_made_optional_returns_info(self):
        """Test that field changed from required to optional returns Info severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": True}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.FIELD)
        self.assertEqual(delta.change_type, ChangeType.MODIFIED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_LESS)
        self.assertEqual(delta.severity, Severity.INFO)
        self.assertIn("required to optional", delta.description)

    def test_field_made_required_returns_warning(self):
        """Test that field changed from optional to required returns Warning severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": True}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": [
                    {"name": "field1", "type": {"builtIn": "string"}, "optional": False}
                ]}
            ],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.FIELD)
        self.assertEqual(delta.change_type, ChangeType.MODIFIED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_LESS)
        self.assertEqual(delta.severity, Severity.WARNING)
        self.assertIn("optional to required", delta.description)

    def test_struct_removed_returns_error(self):
        """Test that struct removed from server returns Error severity"""
        client_idl = {
            "interfaces": [],
            "structs": [
                {"name": "TestStruct", "fields": []}
            ],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [],
            "structs": [],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.STRUCT)
        self.assertEqual(delta.change_type, ChangeType.REMOVED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_MORE)
        self.assertEqual(delta.severity, Severity.ERROR)

    def test_interface_added_returns_info(self):
        """Test that interface added to server returns Info severity"""
        client_idl = {
            "interfaces": [],
            "structs": [],
            "enums": [],
            "errors": []
        }
        server_idl = {
            "interfaces": [
                {"name": "TestService", "methods": []}
            ],
            "structs": [],
            "enums": [],
            "errors": []
        }
        deltas = diff_idl(client_idl, server_idl)
        self.assertEqual(len(deltas), 1)
        delta = deltas[0]
        self.assertEqual(delta.entity_type, EntityType.INTERFACE)
        self.assertEqual(delta.change_type, ChangeType.ADDED)
        self.assertEqual(delta.direction, Direction.CLIENT_HAS_LESS)
        self.assertEqual(delta.severity, Severity.INFO)


if __name__ == '__main__':
    unittest.main()