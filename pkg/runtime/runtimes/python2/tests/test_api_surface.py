"""API-surface guard for the deliberately feature-reduced Python 2 runtime.

Python 2 intentionally exports 19 symbols; the Python 3 runtime exports the
full surface. The 14 omitted symbols are documented in
``docs/languages/python/python2.md`` (see the "Python 2 support policy"
section). This test fails on either kind of drift: accidentally adding a
Python-3-only symbol, or removing a promised one.
"""

import io
import os

import pulserpc

EXPECTED_ALL = [
    "RPCError",
    "validate_type",
    "validate_string",
    "validate_int",
    "validate_float",
    "validate_bool",
    "validate_array",
    "validate_map",
    "validate_enum",
    "validate_struct",
    "find_struct",
    "find_enum",
    "get_struct_fields",
    "Server",
    "Client",
    "Contract",
    "Transport",
    "HttpTransport",
    "InProcTransport",
]

# Every Python 3 symbol intentionally absent from Python 2 (python2-05).
OMITTED_FROM_PYTHON3 = [
    "extract_checksum",
    "EntityType",
    "ChangeType",
    "Direction",
    "Severity",
    "ContractDelta",
    "VerificationResult",
    "ValidationError",
    "ValidationResult",
    "diff_idl",
    "ContractAuditor",
    "NoOpAuditor",
    "LoggingAuditor",
    "FailFastAuditor",
]


def _support_doc_path():
    here = os.path.dirname(os.path.abspath(__file__))
    # tests -> python2 -> runtimes -> runtime -> pkg -> repo root
    parts = [os.pardir] * 5 + ["docs", "languages", "python", "python2.md"]
    return os.path.normpath(os.path.join(here, *parts))


def test_exported_api_matches_documented_set():
    print("Testing exported Python 2 API surface...")
    actual = list(pulserpc.__all__)
    assert len(actual) == 19, "expected 19 exports, got %d: %r" % (len(actual), actual)
    assert set(actual) == set(EXPECTED_ALL), (
        "__all__ drifted: missing %r, unexpected %r"
        % (sorted(set(EXPECTED_ALL) - set(actual)), sorted(set(actual) - set(EXPECTED_ALL)))
    )
    for name in OMITTED_FROM_PYTHON3:
        assert name not in actual, "%s must stay out of the Python 2 API" % name
    print("  PASS: exported Python 2 API surface")


def test_omitted_symbols_are_documented():
    print("Testing documentation of omitted Python 3 symbols...")
    path = _support_doc_path()
    assert os.path.isfile(path), "support policy doc not found at %s" % path
    with io.open(path, "r", encoding="utf-8") as f:
        text = f.read()
    for name in OMITTED_FROM_PYTHON3:
        assert name in text, "%s is not listed in the Python 2 support policy" % name
    print("  PASS: documentation of omitted Python 3 symbols")


def main():
    test_exported_api_matches_documented_set()
    test_omitted_symbols_are_documented()
    print("  test_api_surface: ALL PASSED")
