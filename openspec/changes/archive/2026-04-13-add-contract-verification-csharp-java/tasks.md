## 1. C# Runtime - Core Types

- [x] 1.1 Create `ContractDelta.cs` with EntityType, ChangeType, Direction, Severity enums and ContractDelta record
- [x] 1.2 Create `VerificationResult.cs` class with Compatible, ServerChecksum, ClientChecksum, Deltas, Timestamp properties
- [x] 1.3 Create `IContractAuditor.cs` interface with Audit(VerificationResult) and Name properties
- [x] 1.4 Create `NoOpAuditor.cs` implementation of IContractAuditor
- [x] 1.5 Create `LoggingAuditor.cs` implementation of IContractAuditor
- [x] 1.6 Create `FailFastAuditor.cs` implementation of IContractAuditor

## 2. C# Runtime - DiffEngine

- [x] 2.1 Create `DiffEngine.cs` with DiffIDL() method
- [x] 2.2 Implement extractInterfaces, extractStructs, extractEnums, extractErrors helper methods
- [x] 2.3 Implement diffInterfaces, diffStructs, diffEnums, diffErrors methods
- [x] 2.4 Implement diffInterfaceMethods with methodsEqual comparison
- [x] 2.5 Implement diffStructFields with fieldsEqualDetailed comparison
- [x] 2.6 Implement diffEnumValues helper method
- [x] 2.7 Implement ClassifySeverity() method per severity matrix
- [x] 2.8 Implement ComputeChecksum() using SHA-256

## 3. C# Runtime - Client Integration

- [x] 3.1 Modify `Client.cs` to add IContractAuditor field
- [x] 3.2 Add ClientOptions inner class with WithAuditor() and VerifyOnBootstrap() methods
- [x] 3.3 Add VerifyCompatibility(CancellationToken) async method
- [x] 3.4 Add SetLocalIDL() method for testing with custom IDL
- [x] 3.5 Call VerifyCompatibility() after bootstrap if VerifyOnBootstrap is set

## 4. C# Code Generator

- [x] 4.1 Embed IDL_JSON constant in generated client code
- [x] 4.2 Parse IDL_JSON in client constructor to enable verification
- [x] 4.3 Generate WithAuditor() option in generated client classes
- [x] 4.4 Generate VerifyOnBootstrap() option in generated client classes

## 5. Java Runtime - Core Types

- [x] 5.1 Create `ContractDelta.java` with EntityType, ChangeType, Direction, Severity enums and ContractDelta class
- [x] 5.2 Create `VerificationResult.java` class with compatible, serverChecksum, clientChecksum, deltas, timestamp fields
- [x] 5.3 Create `ContractAuditor.java` interface with audit() and name() methods and static factory methods (noOp, logging, failFast)

## 6. Java Runtime - DiffEngine

- [x] 6.1 Create `DiffEngine.java` with diffIDL() method
- [x] 6.2 Implement extractInterfaces, extractStructs, extractEnums, extractErrors helper methods
- [x] 6.3 Implement diffInterfaces, diffStructs, diffEnums, diffErrors methods
- [x] 6.4 Implement diffInterfaceMethods with methodsEqual comparison
- [x] 6.5 Implement diffStructFields with field comparison logic
- [x] 6.6 Implement diffEnumValues helper method
- [x] 6.7 Implement classifySeverity() method per severity matrix
- [x] 6.8 Implement computeChecksum() using SHA-256

## 7. Java Runtime - Client Integration

- [x] 7.1 Modify `Client.java` to add ContractAuditor field
- [x] 7.2 Add withAuditor() builder method
- [x] 7.3 Add verifyOnBootstrap() builder method
- [x] 7.4 Add verifyCompatibility() method
- [x] 7.5 Add setLocalIDL() method for testing with custom IDL
- [x] 7.6 Call verifyCompatibility() after bootstrap if verifyOnBootstrap is set

## 8. Java Code Generator

- [x] 8.1 Embed IDL_JSON constant in generated client code
- [x] 8.2 Parse IDL_JSON in client constructor to enable verification
- [x] 8.3 Generate withAuditor() option in generated client classes
- [x] 8.4 Generate verifyOnBootstrap() option in generated client classes

## 9. Documentation

- [x] 9.1 Add "Contract Compatibility Verification" section to `docs/languages/csharp/reference.md`
- [x] 9.2 Add "Contract Compatibility Verification" section to `docs/languages/java/reference.md`
- [x] 9.3 Document built-in auditors (NoOp, Logging, FailFast) in both docs
- [x] 9.4 Document severity levels table in both docs
- [x] 9.5 Document ClientOptions usage in both docs

## 10. Testing

- [x] 10.1 Create C# unit tests for DiffEngine
- [x] 10.2 Create C# unit tests for Severity classification
- [x] 10.3 Create C# unit tests for all three auditor implementations
- [x] 10.4 Create Java unit tests for DiffEngine
- [x] 10.5 Create Java unit tests for Severity classification
- [x] 10.6 Create Java unit tests for all three auditor implementations

## 11. Quality Assurance

- [x] 11.1 Run `make quality` and fix any lint errors
- [x] 11.2 Run `make test-runtimes` and fix any test failures