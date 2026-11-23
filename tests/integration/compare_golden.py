#!/usr/bin/env python3
"""Compare per-runtime golden response files and fail on unexplained differences.

Collector/comparator contract
-----------------------------
* The collector writes one JSON file per runtime into ${GOLDEN_DIR}
  (default: /tmp/pulserpc-golden).
* File naming is strictly ``<runtime>.json`` for each of the seven runtimes:
  python, ts, csharp, java, go, python2, ts-cjs.
* Each file holds the same set of named responses (a JSON object keyed by case
  name, or a JSON list compared positionally). Responses are compared
  case-by-case after normalization.
* Normalization performed before/during comparison:
    - D7 optional fields: a key with value ``null`` is treated as omitted.
    - Numeric ids larger than 2^53 are normalized (representation may differ
      between runtimes, e.g. JSON number vs string). Ids <= 2^53 are NOT
      normalized, so a real type/value change still fails.

Allowed divergences (everything else is a hard failure)
-------------------------------------------------------
1. R3: fail-fast vs accumulating validation -> differing ``invalid_fields``
   counts, allowed only when one list is a subset of the other.
2. D7: optional field present as ``null`` in one runtime and omitted in the
   other.
3. python2: reduced validation ``data`` shape containing only
   ``{"error", "invalid_fields"}``.
4. Numeric id representation differences, but only for ids > 2^53.

Usage:
    python tests/integration/compare_golden.py                # uses GOLDEN_DIR
    python tests/integration/compare_golden.py A.json B.json  # explicit pair
"""

import json
import os
import sys

GOLDEN_DIR_ENV = "GOLDEN_DIR"
DEFAULT_GOLDEN_DIR = "/tmp/pulserpc-golden"
RUNTIMES = ("python", "ts", "csharp", "java", "go", "python2", "ts-cjs")
BIG_INT = 2 ** 53
PYTHON2 = "python2"
INVALID_FIELDS_KEY = "invalid_fields"
DATA_KEY = "data"


def _is_id_key(key):
    if key is None:
        return False
    lowered = str(key).lower()
    return lowered == "id" or lowered.endswith("_id") or lowered.endswith("id")


def _canonical_id(value):
    """Return a canonical token for big numeric ids, else a (value, type) pair."""
    number = None
    if isinstance(value, bool):
        return ("bool", value)
    if isinstance(value, (int, float)):
        number = value
    elif isinstance(value, str):
        try:
            number = int(value)
        except (TypeError, ValueError):
            try:
                number = float(value)
            except (TypeError, ValueError):
                number = None
    if isinstance(number, (int, float)) and abs(number) > BIG_INT:
        return ("BIG_ID",)
    return (type(value).__name__, value)


def _drop_none(mapping):
    return {k: v for k, v in mapping.items() if v is not None}


def _canonical(element):
    try:
        return json.dumps(element, sort_keys=True, default=str)
    except (TypeError, ValueError):
        return repr(element)


def _list_subset(a, b):
    """True when one list's elements are a subset of the other's."""
    sa = set(_canonical(e) for e in a)
    sb = set(_canonical(e) for e in b)
    return sa <= sb or sb <= sa


def _dict_subset_equal(a, b, ctx, path):
    """Python2 reduced-data allowance: shared keys equal, smaller keys a subset.

    Only applied to the ``data`` object when python2 is one of the two sides.
    """
    if not (isinstance(a, dict) and isinstance(b, dict)):
        return False
    small, large = (a, b) if set(a) <= set(b) else ((b, a) if set(b) <= set(a) else (None, None))
    if small is None:
        return False
    for key in small:
        if compare(large[key], small[key], path + "." + str(key), ctx, key=key):
            return False
    return True


def compare(a, b, path, ctx, key=None):
    """Recursively compare two values, returning a list of diff messages."""
    out = []

    if _is_id_key(key):
        if _canonical_id(a) != _canonical_id(b):
            out.append("%s: id representation/value differs: %r vs %r" % (path, a, b))
        return out

    if key == DATA_KEY and ctx["python2_involved"]:
        if _dict_subset_equal(a, b, ctx, path):
            return out

    if isinstance(a, dict) and isinstance(b, dict):
        da, db = _drop_none(a), _drop_none(b)  # D7
        for k in sorted(set(da) | set(db), key=str):
            if k not in da:
                out.append("%s.%s: missing in %s" % (path, k, ctx["name_a"]))
                continue
            if k not in db:
                out.append("%s.%s: missing in %s" % (path, k, ctx["name_b"]))
                continue
            if k == INVALID_FIELDS_KEY and isinstance(da[k], list) and isinstance(db[k], list):
                if _list_subset(da[k], db[k]):  # R3
                    continue
            out.extend(compare(da[k], db[k], path + "." + str(k), ctx, key=k))
        return out

    if isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            out.append("%s: list length differs: %d vs %d" % (path, len(a), len(b)))
        for i in range(min(len(a), len(b))):
            out.extend(compare(a[i], b[i], "%s[%d]" % (path, i), ctx))
        return out

    if a != b or type(a) is not type(b):
        out.append("%s: %r (%s) vs %r (%s)" % (path, a, type(a).__name__, b, type(b).__name__))
    return out


def _load(path):
    with open(path, "r", encoding="utf-8") as handle:
        return json.load(handle)


def resolve_files(argv):
    """Return an ordered dict of {runtime_name: path}."""
    if argv:
        return {os.path.splitext(os.path.basename(p))[0]: p for p in argv}
    golden_dir = os.environ.get(GOLDEN_DIR_ENV, DEFAULT_GOLDEN_DIR)
    return {r: os.path.join(golden_dir, r + ".json") for r in RUNTIMES}


def run(argv=None):
    argv = list(sys.argv[1:] if argv is None else argv)
    files = resolve_files(argv)
    diffs = []

    reference_name = None
    reference = None
    loaded = {}
    for name, path in files.items():
        try:
            loaded[name] = _load(path)
        except Exception as exc:  # noqa: BLE001 - report and fail loudly
            diffs.append("[%s] cannot load %s: %s" % (name, path, exc))
            continue
        if reference_name is None:
            reference_name, reference = name, loaded[name]

    if reference is not None:
        for name, data in loaded.items():
            if name == reference_name:
                continue
            ctx = {
                "name_a": reference_name,
                "name_b": name,
                "python2_involved": PYTHON2 in (reference_name, name),
            }
            for message in compare(reference, data, "%s -> %s" % (reference_name, name), ctx):
                diffs.append(message)

    if diffs:
        print("Golden response mismatch(es) found:")
        for message in diffs:
            print("  - " + message)
        return 1
    print("All %d golden response file(s) match (allowed divergences ignored)." % len(loaded))
    return 0


def main():
    return run()


if __name__ == "__main__":
    sys.exit(main())
