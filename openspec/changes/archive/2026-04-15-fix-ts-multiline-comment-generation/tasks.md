## 1. Fix TypeScript Comment Generation

- [x] 1.1 Fix line 701: Split enum value comments in `generateTypesTs` (non-namespace)
- [x] 1.2 Fix line 742: Split field comments in `generateTypesTs` (non-namespace)
- [x] 1.3 Fix line 803: Split enum value comments in `generateTypesTsForNamespace` (namespace)
- [x] 1.4 Fix line 840: Split field comments in `generateTypesTsForNamespace` (namespace)

## 2. Add Tests

- [x] 2.1 Add test for multi-line field comment in `ts_client_server_test.go`
- [x] 2.2 Add test for multi-line enum value comment in `ts_client_server_test.go`

## 3. Verify

- [x] 3.1 Run existing tests to ensure no regressions
- [x] 3.2 Manually verify generated output for multi-line comments
