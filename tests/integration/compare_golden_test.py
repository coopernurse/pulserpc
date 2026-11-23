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
        first = {"validate": {"data": {"error": "E_OK", "invalid_fields": []}}}
        second = {"validate": {"data": {"error": "E_BAD_CODE", "invalid_fields": []}}}
        self.assertEqual(self._compare("python", first, "ts", second), 1)

    def test_allowed_python2_reduced_data_exits_zero(self):
        full = {
            "validate": {
                "data": {
                    "error": "E_INVALID",
                    "invalid_fields": ["name"],
                    "extra_detail": "present in other runtimes",
                }
            }
        }
        reduced = {"validate": {"data": {"error": "E_INVALID", "invalid_fields": ["name"]}}}
        self.assertEqual(self._compare("python", full, "python2", reduced), 0)


if __name__ == "__main__":
    unittest.main()
