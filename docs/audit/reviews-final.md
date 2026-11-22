# Final integration review — consolidated main (`e07f0ca`)

Final cross-branch review of the 36-commit / 153-file merge. Each of the 8
branches was individually 5-axis reviewed and its findings fixed beforehand.
This review looked for issues that only appear after merging.

## Verified clean

- All runtime test suites pass on merged main (Go, Python 136, Java, C# 100,
  ts-node 9/9, ts-cjs 13/13, Python2 in Docker).
- `test_all_runtimes.sh` matrix + `TestAllRuntimesMatrixIsComplete` are coherent.
- The ts-node/ts-cjs generator conflict was resolved to a single unified
  `Promise.all` batch implementation; both `ts_batch_test.go` tests pass.
- Windows path fixes build cleanly (verified with stub webui dist).

## Fixed during final review

- `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`: checklist said "only Go and C#
  generated servers" support batches, but Java's generated server gained batch
  support (java-15). Updated to "Go, C# and Java". Matching comment fix in
  `pkg/generator/jsonrpc_compliance_test.go`.

## Documented, non-blocking

- **TS runtime envelope-error divergence**: `ts-node` client throws `RPCError`
  (-32600) for malformed response envelopes while `ts-cjs` throws plain `Error`.
  Both satisfy their acceptance criteria ("makes `client.call` throw"), and all
  tests pass. This is a code-style inconsistency between two independently
  fixed runtimes, not a correctness or acceptance-criteria violation. Left as-is
  to avoid late churn; candidates for a future convergence pass:
  - ts-node defines `RPCErrorCode` constants; ts-cjs uses raw numeric codes.
  - ts-node has a single-owner `checkParamCount`; ts-cjs inlines the range check.
- **Batch doc nuance**: generated *test* servers (TS) support batches while
  production generated TS/Python servers do not; the guide correctly scopes the
  claim to "generated servers" and marks batch support optional.

## Reviewer notes

The correctness, regression, and quality integration reviewers hit their step
limits before emitting formal findings; the items above are the concrete
observations they surfaced, each independently verified or recorded here. No
cross-branch correctness or regression defects were confirmed.
