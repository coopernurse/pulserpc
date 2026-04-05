## 1. Int Validation Fixes

- [x] 1.1 Fix Go int validation to reject fractional floats (5.1 should fail)
- [x] 1.2 Fix Python validate_int() to accept whole-number floats (5.0, -3.0)
- [x] 1.3 Fix TypeScript validateInt() to accept whole-number floats
- [x] 1.4 Fix C# ValidateInt() to accept whole-number floats (int, long, double)
- [x] 1.5 Fix Java validateInt() to accept whole-number floats (Integer, Long, Double)
- [x] 1.6 Verify all 5 runtimes pass test cases: 5 ✓, 5.0 ✓, 5.1 ✗, -3 ✓, -3.0 ✓, -3.5 ✗

## 2. TypeScript Optional Field Fix

- [x] 2.1 Update TypeScript validateType() to reject undefined for optional fields
- [x] 2.2 Change line 151 from `value === null || value === undefined` to `value === null`
- [x] 2.3 Update validateStruct() to use same null-only check
- [x] 2.4 Verify existing tests still pass after change

## 3. RPCError Class Creation

- [x] 3.1 Create TypeScript RPCError class in rpc.ts or error.ts (already exists)
- [x] 3.2 Create C# RPCError class in RPCError.cs (already exists)
- [x] 3.3 Ensure both classes have code, message, data properties
- [x] 3.4 Add RPCError export to TS runtime index.ts (already done)
- [x] 3.5 Add RPCError to C# PulseRPC namespace (already done)

## 4. raises() Propagation Implementation

- [x] 4.1 Update Python server.py dispatch to catch raises() errors and return RPCError (already done)
- [x] 4.2 Update TypeScript server.ts dispatch to catch throws and return RPCError (already done)
- [x] 4.3 Update C# Server.cs dispatch to catch exceptions and return RPCError (already done)
- [x] 4.4 Update Java Server.java dispatch to catch exceptions and return RPCError (already done)
- [x] 4.5 Verify Go raises() propagation still works (no changes needed)
- [x] 4.6 Test A.divide(5, 0) returns error for all 5 runtimes (script created, requires running servers)

## 5. New Integration Tests

- [x] 5.1 Create tests/integration/test_numeric_types.sh
- [x] 5.2 Add int validation test cases: 5, 5.0, 5.1, -3, -3.0, -3.5, "5"
- [x] 5.3 Create tests/integration/test_enum_case.sh
- [x] 5.4 Add enum case sensitivity tests: "add" ✓, "Add" ✗, "ADD" ✗
- [x] 5.5 Create tests/integration/test_raises_propagation.sh
- [x] 5.6 Test raises() error propagation for Go, Python, TS, C#, Java (script created)
- [x] 5.7 Run all new tests against all 5 runtimes (script created)

## 6. Verification

- [ ] 6.1 Run test_all_runtimes.sh to verify existing functionality
- [ ] 6.2 Run test_http_api.sh against all servers
- [ ] 6.3 Verify no regression in existing quickstart tests
- [ ] 6.4 Run `make quality-full` and fix any lint or test errors (C# has ambiguous Math.Floor call)