"""Unit tests for the cross-runtime golden response comparator (plain unittest)."""

import json
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)

from compare_golden import run  # noqa: E402


class CompareGoldenTest(unittest.TestCase):
    def _compare(self, first_name, first_data, second_name, second_data):
        with tempfile.TemporaryDirectory() as directory:
            paths = []
            for name, data in ((first_name, first_data), (second_name, second_data)):
                path = os.path.join(directory, name + ".json")
                with open(path, "w", encoding="utf-8") as handle:
                    json.dump(data, handle)
                paths.append(path)
            return run(paths)

    def test_identical_files_exit_zero(self):
        response = {"validate": {"data": {"error": "E_OK", "invalid_fields": []}}}
        self.assertEqual(self._compare("python", response, "ts", response), 0)

    def test_changed_error_code_exits_one(self):
        first = {"validate": {"error": {"code": -32602, "message": "bad params"}}}
        second = {"validate": {"error": {"code": -32601, "message": "bad params"}}}
        self.assertEqual(self._compare("python", first, "ts", second), 1)

    def test_changed_result_exits_one(self):
        first = {"int_call": {"result": 5, "jsonrpc": "2.0", "id": 1}}
        second = {"int_call": {"result": 6, "jsonrpc": "2.0", "id": 1}}
        self.assertEqual(self._compare("python", first, "ts", second), 1)

    def test_prose_difference_exits_zero(self):
        first = {
            "invalid_params": {
                "error": {
                    "code": -32602,
                    "message": "expects 2 param(s). 3 given.",
                    "data": {"error": "detail A", "invalid_fields": []},
                }
            }
        }
        second = {
            "invalid_params": {
                "error": {
                    "code": -32602,
                    "message": "Invalid params: Function A.add expects 2-2 param(s), 3 given",
                    "data": "Expected 2 parameters, got 3",
                }
            }
        }
        self.assertEqual(self._compare("python", first, "java", second), 0)

    def test_missing_prose_exits_one(self):
        first = {"e": {"error": {"code": -32602, "message": "detail"}}}
        second = {"e": {"error": {"code": -32602}}}
        self.assertEqual(self._compare("python", first, "ts", second), 1)

    def test_ci_message_and_data_shape_divergences_exit_zero(self):
        """The exact prose/shape differences first seen in CI are not failures."""
        def response(code, message, data=None, with_data=True):
            error = {"code": code, "message": message}
            if with_data:
                error["data"] = data if data is not None else {}
            return json.dumps({"error": error, "id": 6, "jsonrpc": "2.0"})

        python = {
            "invalid_jsonrpc": response(-32600, "Invalid Request", "jsonrpc version must be '2.0'"),
            "invalid_params": response(-32602, "Function 'A.add' expects 2 param(s). 3 given.", with_data=False),
        }
        java = {
            "invalid_jsonrpc": response(-32600, "Invalid Request - jsonrpc must be '2.0'", with_data=False),
            "invalid_params": response(-32602, "Invalid params: Function 'A.add' expects 2 param(s), 3 given", with_data=False),
        }
        csharp = {
            "invalid_jsonrpc": response(-32600, "Invalid Request", "jsonrpc field is required"),
            "invalid_params": response(-32602, "Invalid params", "Expected 2 parameters, got 3"),
        }
        python2 = {
            "invalid_jsonrpc": response(-32600, "Invalid Request", "jsonrpc version must be '2.0'"),
            "invalid_params": response(
                -32602,
                "Function 'A.add' expects 2 param(s). 3 given.",
                {"error": "Function 'A.add' expects 2 param(s). 3 given.", "invalid_fields": []},
            ),
        }
        self.assertEqual(self._compare("python", python, "java", java), 0)
        self.assertEqual(self._compare("python", python, "csharp", csharp), 0)
        self.assertEqual(self._compare("python", python, "python2", python2), 0)

    def test_allowed_python2_reduced_data_exits_zero(self):
        full = {
            "validate": {
                "data": {
                    "error": "E_INVALID",
                    "invalid_fields": ["name"],
                    "valid": False,
                }
            }
        }
        reduced = {"validate": {"data": {"error": "E_INVALID", "invalid_fields": ["name"]}}}
        self.assertEqual(self._compare("python", full, "python2", reduced), 0)


if __name__ == "__main__":
    unittest.main()
