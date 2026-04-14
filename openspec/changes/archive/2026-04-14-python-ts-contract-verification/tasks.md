## 1. Python Runtime - Core Types

- [x] 1.1 Add enums to `types.py`: `EntityType`, `ChangeType`, `Direction`, `Severity`
- [x] 1.2 Add `ContractDelta` dataclass to `types.py`
- [x] 1.3 Add `VerificationResult` dataclass to `types.py`
- [x] 1.4 Add `extract_checksum()` helper function to `types.py` (if not already present)

## 2. Python Runtime - Diff Engine

- [x] 2.1 Create `diff.py` with `DiffIDL` function ported from Go implementation
- [x] 2.2 Implement `diff_interfaces`, `diff_interface_methods`, `diff_structs`, `diff_struct_fields`, `diff_enums`, `diff_errors` helper functions
- [x] 2.3 Implement `classify_severity` function matching Go severity matrix

## 3. Python Runtime - Auditors

- [x] 3.1 Add `ContractAuditor` abstract base class to `contract.py`
- [x] 3.2 Add `NoOpAuditor` implementation
- [x] 3.3 Add `LoggingAuditor` implementation  
- [x] 3.4 Add `FailFastAuditor` implementation

## 4. Python Runtime - Client Integration

- [x] 4.1 Add `options` parameter to `Client.__init__` with `auditor` and `verify_on_bootstrap` keys
- [x] 4.2 Add `_find_idl_json()` helper function to locate `idl.json` by walking up directory tree
- [x] 4.3 Add `_local_idl` attribute and auto-load from `idl.json` during init
- [x] 4.4 Add `set_local_idl(idl_json: str)` method
- [x] 4.5 Add `verify_compatibility()` method
- [x] 4.6 Implement `verify_on_bootstrap` logic (call verify_compatibility after bootstrap if enabled)
- [x] 4.7 Pass auditor to `verify_compatibility()` and invoke after computation

## 5. Python Tests - Diff Engine

- [x] 5.1 Create `tests/test_diff.py` with test cases matching Go/Java/C# behavior
- [x] 5.2 Test identical IDLs produces no deltas
- [x] 5.3 Test added optional field returns Info severity
- [x] 5.4 Test added required field returns Error severity
- [x] 5.5 Test removed field returns Info severity
- [x] 5.6 Test field made optional returns Info severity
- [x] 5.7 Test field made required returns Warning severity
- [x] 5.8 Test struct removed from server returns Error severity
- [x] 5.9 Test interface added to server returns Info severity

## 6. TypeScript Runtime - Core Types

- [x] 6.1 Add enums to `types.ts`: `EntityType`, `ChangeType`, `Direction`, `Severity`
- [x] 6.2 Add `ContractDelta` interface to `types.ts`
- [x] 6.3 Add `VerificationResult` interface to `types.ts`

## 7. TypeScript Runtime - Diff Engine

- [x] 7.1 Create `diff.ts` with `diffIDL` function ported from Go implementation
- [x] 7.2 Implement helper functions matching Go logic
- [x] 7.3 Implement `classifySeverity` function matching Go severity matrix

## 8. TypeScript Runtime - Auditors

- [x] 8.1 Add `IContractAuditor` interface to `contract.ts`
- [x] 8.2 Add `NoOpAuditor` class implementation
- [x] 8.3 Add `LoggingAuditor` class implementation
- [x] 8.4 Add `FailFastAuditor` class implementation

## 9. TypeScript Runtime - Client Integration

- [x] 9.1 Add `options` parameter to `Client` constructor with typed `ClientOptions` interface
- [x] 9.2 Add `_findIDLJson()` helper function to locate `idl.json` using `import.meta.url`
- [x] 9.3 Add `_localIDL` attribute and auto-load from `idl.json` during bootstrap
- [x] 9.4 Add `setLocalIDL(idlJson: string)` method
- [x] 9.5 Add `verifyCompatibility(): Promise<VerificationResult>` async method
- [x] 9.6 Implement `verifyOnBootstrap` logic
- [x] 9.7 Pass auditor to `verifyCompatibility()` and invoke after computation

## 10. TypeScript Tests - Diff Engine

- [x] 10.1 Create `tests/test_diff.ts` with test cases matching Go/Java/C# behavior
- [x] 10.2 Test cases for identical IDLs, added optional/required fields, removed fields, type changes

## 11. Quickstart Test Updates

- [x] 11.1 Verify Python quickstart test handles new client options (if auditor/logging added)
- [x] 11.2 Verify TypeScript quickstart test handles new client options (if auditor/logging added)
- [x] 11.3 Ensure existing quickstart test assertions still pass (no breaking changes to existing API)

## 12. Generator Test Updates

- [x] 12.1 Verify Python generator tests still pass with new runtime files
- [x] 12.2 Verify TypeScript generator tests still pass with new runtime files
- [x] 12.3 Verify quickstart tests still pass for Python and TypeScript
