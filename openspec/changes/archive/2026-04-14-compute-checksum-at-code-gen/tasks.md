## 1. Go Generator - Add Checksum to idl.json

- [x] 1.1 Modify `writeIDLJSONGo()` to compute SHA-256 checksum of IDL
- [x] 1.2 Add `checksum` field to output JSON structure
- [x] 1.3 Test generating new idl.json with checksum field
- [x] 1.4 Verify backward compat: existing idl.json parsers still work

## 2. Go Runtime - Read Checksum at Runtime

- [x] 2.1 Modify `VerifyCompatibility()` to extract checksums from IDL data
- [x] 2.2 Read `ClientChecksum` from `localIDL` or `contract.idlParsed`
- [x] 2.3 Read `ServerChecksum` from server's `idlParsed`
- [x] 2.4 Update Go integration tests to use stored checksums
- [x] 2.5 Remove or deprecate `ComputeChecksum()` function (keep for test compat)

## 3. Go Runtime - Verify Server Response

- [x] 3.1 Verify server's `pulserpc-idl` response includes checksum
- [x] 3.2 Confirm server changes not needed (idlParsed already contains checksum)

## 4. C# Runtime - Remove Checksum Computation

- [x] 4.1 Remove `DiffEngine.ComputeChecksum()` method
- [x] 4.2 Modify generated client constructor to extract checksum from embedded `IDL_JSON`
- [x] 4.3 Update C# `VerifyCompatibility()` to read checksums instead of computing
- [x] 4.4 Update C# tests to use stored checksums

## 5. Java Runtime - Remove Checksum Computation

- [x] 5.1 Remove `DiffEngine.computeChecksum()` method
- [x] 5.2 Modify generated client to extract checksum from embedded `IDL_JSON`
- [x] 5.3 Update Java `verifyCompatibility()` to read checksums instead of computing
- [x] 5.4 Update Java tests to use stored checksums

## 6. Python Runtime - Read Checksum from idl.json

- [x] 6.1 Implement contract compatibility verification (if not yet implemented)
- [x] 6.2 Read `checksum` field from local `idl.json`
- [x] 6.3 Read `checksum` from server's `pulserpc-idl` response
- [x] 6.4 Create Python unit tests for checksum extraction
- [x] 6.5 Verify no SHA-256 implementation needed

## 7. TypeScript Runtime - Read Checksum from Embedded IDL

- [x] 7.1 Implement contract compatibility verification (if not yet implemented)
- [x] 7.2 Read `checksum` field from embedded IDL
- [x] 7.3 Read `checksum` from server's `pulserpc-idl` response
- [x] 7.4 Create TypeScript unit tests for checksum extraction
- [x] 7.5 Verify no SHA-256 implementation needed

## 8. Quality Assurance

- [x] 8.1 Run `make quality` and fix any lint errors
- [x] 8.2 Run `make test-runtimes` and fix any test failures
- [x] 8.3 Regenerate example code to verify idl.json format
