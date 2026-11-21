# Python2 — code review findings (branch audit-fixes-python2, `5022168`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness unavailable, security (no findings), quality unavailable,
performance (no findings), regression.

## Surviving changes (all fixed)

1. [Medium · regression] Quickstart test switches to `python:2.7-slim` (no `curl`) but its readiness probe uses `curl` — `tests/integration/test_quickstart_python2.sh:363,333`
   - Evidence: diff changed `moxel/python2` → `python:2.7-slim`; the embedded DOCKERSCRIPT still calls `curl -s "$SERVER_URL"` in a 30-iteration loop. The slim image lacks `curl`, so the probe fails every time and the test exits 1 with "Server did not start".
   - Change: replace the `curl` probe with a stdlib Python 2 socket connection check (no extra packages), so the test works in `python:2.7-slim`.

## Deferred

None.
