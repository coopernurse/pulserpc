#!/usr/bin/env python
"""Run every Python 2 runtime test module (no pytest required)."""

import os
import sys
import traceback

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
for path in (ROOT, HERE):
    if path not in sys.path:
        sys.path.insert(0, path)

import test_api_surface
import test_client
import test_contract
import test_rpc
import test_server
import test_transport
import pulserpc.test_validation as test_validation

MODULES = [
    ("validation", test_validation),
    ("contract", test_contract),
    ("server", test_server),
    ("client", test_client),
    ("transport", test_transport),
    ("rpc", test_rpc),
    ("api_surface", test_api_surface),
]


def main():
    print("=" * 60)
    print("PulseRPC Python 2.7 Runtime Tests")
    print("=" * 60)

    failed = []
    for name, module in MODULES:
        print("\n--- %s ---" % name)
        try:
            module.main()
        except Exception:
            failed.append(name)
            traceback.print_exc()

    print("\n" + "=" * 60)
    if failed:
        print("FAILED modules: %s" % ", ".join(failed))
        sys.exit(1)
    print("ALL TESTS PASSED")
    print("=" * 60)


if __name__ == "__main__":
    main()
