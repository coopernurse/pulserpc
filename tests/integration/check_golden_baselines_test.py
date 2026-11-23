"""Unit tests for check_golden_baselines.py (plain unittest, no pytest)."""

import json
import os
import shutil
import tempfile
import unittest
from unittest import mock

try:
    from tests.integration import check_golden_baselines as checker
except ImportError:  # pragma: no cover - supports running the file directly
    import check_golden_baselines as checker


def make_golden_file(directory, runtime, cases):
    """Write ``cases`` ({name: response dict}) as a golden JSON file."""
    encoded = {name: json.dumps(value, sort_keys=True)
               for name, value in cases.items()}
    with open(os.path.join(directory, runtime + ".json"), "w",
              encoding="utf-8") as handle:
        json.dump(encoded, handle, sort_keys=True)


def sample_cases(message="requested id 99 is invalid"):
    """A small set of cases including an error carrying a message."""
    return {
        "int_call": {"jsonrpc": "2.0", "result": 5, "id": 1},
        "validation_invalid_fields": {
            "jsonrpc": "2.0",
            "error": {
                "code": -32602,
                "message": message,
                "data": {"invalid_fields": ["id"]},
            },
            "id": 99,
        },
    }


class GoldenBaselineCheckerTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="golden-baseline-test-")
        self.current_dir = os.path.join(self.tmp, "current")
        self.baseline_dir = os.path.join(self.tmp, "baseline")
        os.makedirs(self.current_dir)
        os.makedirs(self.baseline_dir)
        for runtime in checker.RUNTIMES:
            make_golden_file(self.current_dir, runtime, sample_cases())
            make_golden_file(self.baseline_dir, runtime, sample_cases())

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def with_env(self):
        return mock.patch.dict(os.environ, {
            "GOLDEN_DIR": self.current_dir,
            "GOLDEN_BASELINE_DIR": self.baseline_dir,
        })

    def test_identical_returns_zero(self):
        with self.with_env():
            self.assertEqual(checker.main(), 0)

    def test_changed_message_returns_one(self):
        # A message-only change is exactly the divergence the cross-runtime
        # comparator ignores; it must be a hard failure here.
        make_golden_file(
            self.current_dir, "python",
            sample_cases(message="requested id 99 was not valid"),
        )
        with self.with_env():
            self.assertEqual(checker.main(), 1)

    def test_missing_baseline_file_returns_one(self):
        os.remove(os.path.join(self.baseline_dir, "go.json"))
        with self.with_env():
            self.assertEqual(checker.main(), 1)


if __name__ == "__main__":
    unittest.main()
