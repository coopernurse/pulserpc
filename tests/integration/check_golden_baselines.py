#!/usr/bin/env python3
"""Per-runtime golden baseline checker (regression guard).

Contract
--------
``tests/integration/test_golden_responses.sh`` writes one JSON file per
runtime to ``${GOLDEN_DIR:-/tmp/pulserpc-golden}/<runtime>.json``.  Each file
is a JSON object mapping a case name to a *JSON-encoded string* holding the
normalized response for that case.

This checker guards against *within-runtime* regressions.  The cross-runtime
comparator (``compare_golden.py``) intentionally ignores certain divergences
between runtimes (notably human-readable ``message`` text).  That means a
message that changes for a single runtime will sail through the cross-runtime
comparator.  This tool closes that gap by comparing every runtime against a
committed baseline snapshot.

For each of the seven runtimes -- python, ts, csharp, java, go, python2,
ts-cjs -- it reads the current file from ``GOLDEN_DIR`` (default
``/tmp/pulserpc-golden``) and the committed baseline from
``GOLDEN_BASELINE_DIR`` (default ``tests/integration/golden``, resolved
relative to this script's directory).  Both the outer case map and each
stored response string are parsed as JSON and re-serialized canonically with
``sort_keys=True``; the canonical forms are then compared *exactly*.

There is deliberately no normalization of prose, no tolerated divergence,
and no fuzzy matching: a changed ``message``, ``result``, ``code``, or any
added/removed case is a hard failure.  Per-case differences (missing case,
added case, changed value -- showing baseline vs current) are printed to
stdout/stderr.  The process exits 0 only when all seven runtimes match;
it exits 1 on any mismatch, any missing current runtime file, or any missing
baseline file.
"""

import json
import os
import sys

RUNTIMES = ("python", "ts", "csharp", "java", "go", "python2", "ts-cjs")

DEFAULT_GOLDEN_DIR = "/tmp/pulserpc-golden"
DEFAULT_BASELINE_SUBDIR = "golden"


def canonicalize_response(raw):
    """Return a canonical JSON string for a stored response value.

    ``raw`` is the JSON-encoded string value from a golden file.  It is
    parsed as JSON and re-serialized with sorted keys so that key ordering /
    whitespace in the stored form cannot cause a spurious difference -- but
    every actual value (including ``message``) is compared exactly.
    """
    return json.dumps(json.loads(raw), sort_keys=True, separators=(",", ":"))


def load_case_map(path):
    """Load a golden file and return ``{case_name: canonical_response}``.

    Raises ``ValueError`` when the file is missing or its contents are not a
    JSON object mapping string values.
    """
    with open(path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        raise ValueError("expected a JSON object at top level")
    cases = {}
    for name, raw in data.items():
        if not isinstance(raw, str):
            raise ValueError("case %r is not a JSON string" % name)
        cases[name] = canonicalize_response(raw)
    return cases


def check(golden_dir, baseline_dir, runtimes=RUNTIMES):
    """Compare current golden files against baselines.

    Returns 0 when every runtime matches, otherwise 1.  All problems are
    printed (never short-circuited) so a single run reports every regression.
    """
    failures = []

    for runtime in runtimes:
        current_path = os.path.join(golden_dir, runtime + ".json")
        baseline_path = os.path.join(baseline_dir, runtime + ".json")

        try:
            current = load_case_map(current_path)
        except (OSError, ValueError, TypeError) as exc:
            failures.append("%s: cannot read current file %s: %s"
                            % (runtime, current_path, exc))
            continue

        try:
            baseline = load_case_map(baseline_path)
        except (OSError, ValueError, TypeError) as exc:
            failures.append("%s: cannot read baseline file %s: %s"
                            % (runtime, baseline_path, exc))
            continue

        current_names = set(current)
        baseline_names = set(baseline)

        for name in sorted(baseline_names - current_names):
            failures.append("%s: case %r REMOVED (baseline: %s)"
                            % (runtime, name, baseline[name]))
        for name in sorted(current_names - baseline_names):
            failures.append("%s: case %r ADDED (current: %s)"
                            % (runtime, name, current[name]))
        for name in sorted(current_names & baseline_names):
            if current[name] != baseline[name]:
                failures.append(
                    "%s: case %r CHANGED\n"
                    "    baseline: %s\n"
                    "    current:  %s"
                    % (runtime, name, baseline[name], current[name]))

    if failures:
        print("Golden baseline check FAILED (%d problem(s)):" % len(failures),
              file=sys.stderr)
        for problem in failures:
            print("  - " + problem, file=sys.stderr)
        return 1

    print("Golden baseline check PASSED: %d runtime(s) match exactly."
          % len(runtimes))
    return 0


def main(argv=None):
    golden_dir = os.environ.get("GOLDEN_DIR", DEFAULT_GOLDEN_DIR)
    baseline_dir = os.environ.get("GOLDEN_BASELINE_DIR")
    if baseline_dir is None:
        baseline_dir = os.path.join(
            os.path.dirname(os.path.abspath(__file__)),
            DEFAULT_BASELINE_SUBDIR,
        )
    return check(golden_dir, baseline_dir)


if __name__ == "__main__":
    sys.exit(main())
