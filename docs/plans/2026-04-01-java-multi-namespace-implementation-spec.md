# Java Multi-Namespace Generator Implementation Spec

**Date:** 2026-04-01
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 4: Java Generator Specification)
**Scope:** Java generator behavior for namespace-per-directory output
**Goal:** Implement Section 4 in small, agent-sized steps with recurring verification gates.

## 1. Rename Java Flag from `-base-package` to `-package`

**Background:** Java currently uses `-base-package` flag. The spec mandates renaming to `-package` for consistency across all language generators.

**Tasks:**
1. Locate the Java generator flag definition in `pkg/generator/java_client_server.go` and rename it from `-base-package` to `-package`.
2. Update all flag parsing, configuration plumbing, and internal references to use the new flag name.
3. Ensure other language plugins are unaffected by this change.
4. Add/update unit tests to verify both flags work: new `-package` should work, and optionally add deprecation warning for old `-base-package` if still referenced anywhere.

**Deliverable:**
- Java generator accepts `-package` flag (e.g., `com.example`) as the base package identifier.
- Old `-base-package` flag either removed or deprecated with clear warning message.

**Acceptance Tests:**
```go
// Test: JavaPackageFlagParsing
// Command: pulserpc -plugin java-client-server -dir lib/rpc -package com.example book.pulse
// Then: generator config contains Package="com.example"
```

**Make Target:**
- `make test-generator-java` (to verify flag parsing tests pass)
- `make quality` (ensure no lint issues)

---

## 2. Clarify Java Package Flag Semantics

**Background:** Unlike other languages, Java's `-package` flag does NOT appear in generated code or affect runtime location. It exists only as configuration metadata. The generated code uses simple package names based solely on the namespace (e.g., `package user;`).

**Tasks:**
1. Document within the Java generator that `-package` is metadata-only and does not prefix generated package declarations.
2. Add inline code comments explaining why package declarations are simple namespace names (not qualified by `-package`).
3. Ensure the package declaration logic generates `package {namespace};` without any `-package` prefix.
4. Add unit tests to verify package declarations use only namespace name regardless of `-package` flag value.

**Deliverable:**
- Generated Java files contain simple package declarations like `package user;`, not `package com.example.user;`.
- Code comments clearly explain the `-package` metadata-only semantics.

**Acceptance Tests:**
```go
// Test: JavaPackageIsNamespaceOnly
// Given: book.pulse with namespace "book"
// When: generating with -dir lib/rpc -package com.example
// Then: book/Book.java contains "package book;"
// And: book/Book.java does NOT contain "com.example" anywhere

// Test: JavaPackageFlagDoesNotAffectOutputPath
// Given: any pulse file
// When: generating with -package com.example
// Then: runtime is at lib/rpc/pulserpc/ (NOT lib/rpc/com/example/pulserpc/)
// And: namespace files are at lib/rpc/{namespace}/ (NOT lib/rpc/com/example/{namespace}/)
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 3. Implement Namespace-to-Directory Output Layout

**Background:** Java must generate one subdirectory per namespace (lowercase) under `-dir`, with generated files placed directly in those subdirectories.

**Tasks:**
1. Modify Java generator path resolution to create `{dir}/{namespace}/` structure for each namespace.
2. Generate Java files directly in namespace directories: `{dir}/{namespace}/{Type}.java`, `{namespace}Server.java`, `{namespace}Client.java`.
3. Ensure namespace directory names are lowercase (e.g., `book/`, `user/`, `common/`).
4. For multi-namespace projects (book.pulse includes common.pulse), ensure output includes both `book/` and `common/` directories at the same level under `-dir`.
5. Add path construction tests covering nested output dirs and multiple namespaces.

**Deliverable:**
- Deterministic path logic for generating files into `{dir}/{namespace}/` directories.
- Namespace directories are lowercase regardless of namespace case in IDL.

**Acceptance Tests:**
```go
// Test: JavaNamespaceToSubdirectory
// Command: pulserpc -plugin java-client-server -dir lib/rpc -package com.example book.pulse
// Expect:
//   - lib/rpc/book/Book.java exists
//   - lib/rpc/book/BookServer.java exists  
//   - lib/rpc/book/BookClient.java exists
//   - Package declaration is "package book;"

// Test: JavaLowercaseNamespaceDirectory
// Given: namespace "UserProfile" with uppercase
// Then: directory is lib/rpc/userprofile/ (lowercase)
// And: package declaration is "package userprofile;"
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 4. Verification Gate A (Verify Steps 1-3)

**Tasks:**
1. Ask an agent to verify implementation quality and spec alignment for Steps 1-3 (flag rename, package semantics, namespace directory layout).
2. Run quality and Java integration gates:
   - `make quality`
   - `make test-quickstart-java`
   - `make test-generator-java`
3. Fix regressions from these runs, then rerun failed targets until green.

**Exit Criteria:**
- All three targets pass and no open regressions remain from Steps 1-3.
- Generated Java files show `package {namespace};` declarations.
`-dir` creates proper subdirectory structure under the output directory.

---

## 5. Implement Runtime Output to `{dir}/pulserpc/`

**Background:** Java runtime must always be generated to `{dir}/pulserpc/` (flat under `-dir`), regardless of `-package` flag value.

**Tasks:**
1. Update Java runtime generation logic to output to `{dir}/pulserpc/` instead of any package-prefixed path.
2. Ensure runtime files (RPCError.java, Client.java, Server.java, etc.) are written directly to the `pulserpc/` directory.
3. Verify that `-package com.example` does NOT cause runtime to be placed at `{dir}/com/example/pulserpc/`.
4. Add tests that assert runtime location is always `{dir}/pulserpc/` for various `-dir` values (nested, current dir, etc.).
5. Ensure backwards compatibility: when `-package` is omitted or empty, behavior should remain consistent.

**Deliverable:**
- Runtime files consistently generated to `{dir}/pulserpc/` directory.
- Runtime location is independent of `-package` flag value.

**Acceptance Tests:**
```go
// Test: JavaRuntimeInPulserpcDir
// When: -dir lib/rpc -package com.example
// Then: lib/rpc/pulserpc/ contains all runtime files
// And: NO runtime files in lib/rpc/book/ or other namespace dirs
// And: runtime is NOT at lib/rpc/com/example/pulserpc/

// Test: JavaRuntimeLocationWithNestedDir
// When: -dir src/main/java/lib/rpc -package com.example
// Then: runtime is at src/main/java/lib/rpc/pulserpc/
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 6. Implement Cross-Namespace Imports

**Background:** When book.pulse includes common.pulse, Java must generate `import common.CommonTypes;` (simple package import, not qualified by `-package`).

**Tasks:**
1. Modify Java import generation logic to use simple namespace package names for cross-namespace references (e.g., `import common.CommonTypes;`).
2. Ensure include-based type references resolve to the correct namespace package import.
3. Do NOT prefix imports with `-package` value (e.g., should NOT generate `import com.example.common.CommonTypes;`).
4. Add tests for `book` -> `common` and `user` -> `common` import strings.
5. Test that types from included namespaces are properly imported and usable.

**Deliverable:**
- Cross-namespace imports use simple namespace package names.
- Imports are stable and spec-compliant.

**Acceptance Tests:**
```go
// Test: JavaCrossNamespaceImport
// Given: book.pulse (namespace book) includes common.pulse (namespace common)
// When: generating with -dir lib/rpc -package com.example
// Then: book/Book.java imports "common.CommonTypes" or "common.*"
// And: book/Book.java does NOT contain "com.example.common"

// Test: JavaMultipleNamespaceImports
// Given: user.pulse includes both common.pulse and book.pulse
// Then: user/User.java contains imports for both "common.*" and "book.*"
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 7. Implement Runtime Imports Through pulserpc Package

**Background:** Java must generate runtime imports as `import pulserpc.RPCError;` or `import pulserpc.*;` (simple package name, not qualified by `-package`).

**Tasks:**
1. Modify Java import generation to use simple `pulserpc` package for runtime imports (e.g., `import pulserpc.RPCError;`).
2. Do NOT prefix runtime imports with `-package` value (e.g., should NOT generate `import com.example.pulserpc.RPCError;`).
3. Ensure backwards-compatible behavior when `-package` is omitted or empty.
4. Add tests for both configured and empty-package modes to verify runtime import stability.
5. Test that runtime classes (RPCError, Client, Server, etc.) are properly imported in generated code.

**Deliverable:**
- Runtime imports use simple `pulserpc` package name.
- All generated files can resolve runtime symbols correctly.

**Acceptance Tests:**
```go
// Test: JavaRuntimeImportViaPulserpcPackage
// Given: any pulse file with namespace
// When: generating with -dir lib/rpc -package com.example
// Then: generated files contain "import pulserpc.RPCError;" or "import pulserpc.*;"
// And: does NOT contain "com.example.pulserpc"

// Test: JavaRuntimeImportWithoutPackageFlag
// When: generating without -package flag (backwards compat)
// Then: runtime imports still use "import pulserpc.*;" (unchanged behavior)
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 8. Verification Gate B (Verify Steps 5-7)

**Tasks:**
1. Ask an agent to verify implementation quality and spec alignment for Steps 5-7 (runtime output, cross-namespace imports, runtime imports).
2. Run quality and Java integration gates:
   - `make quality`
   - `make test-quickstart-java`
   - `make test-generator-java`
3. Fix regressions from these runs, then rerun failed targets until green.
4. Verify generated import strings are correct by inspecting actual generated files in test runs.

**Exit Criteria:**
- All three targets pass and generated imports/paths are validated by tests.
- Runtime is confirmed to be at `{dir}/pulserpc/`.
- Cross-namespace imports use simple namespace package names only.

---

## 9. Add Multi-File End-to-End Java Coverage

**Background:** The spec requires testing the complete flow: common.pulse, book.pulse, user.pulse with proper cross-namespace imports and directory structure.

**Tasks:**
1. Add integration tests for the `common.pulse`, `book.pulse`, `user.pulse` multi-file flow.
2. Assert output tree includes `pulserpc/`, `common/`, `book/`, and `user/` under the selected `-dir`.
3. Assert that `book` and `user` modules import from `common` via simple package imports.
4. Verify runtime imports in all generated files point to `pulserpc` package.
5. Test with various `-dir` values (current dir, nested dirs, absolute paths).

**Deliverable:**
- Multi-file namespace generation is enforced in tests.
- Directory structure matches spec exactly for complex scenarios.

**Acceptance Tests:**
```go
// Test: JavaMultiFileProjectStructure
// Command sequence:
//   pulserpc -plugin java-client-server -dir lib/rpc -package com.example common.pulse
//   pulserpc -plugin java-client-server -dir lib/rpc -package com.example book.pulse
//   pulserpc -plugin java-client-server -dir lib/rpc -package com.example user.pulse
// Expect:
//   - lib/rpc/pulserpc/ exists with runtime files
//   - lib/rpc/common/ exists with Common*.java files
//   - lib/rpc/book/ exists with Book*.java files
//   - lib/rpc/user/ exists with User*.java files
//   - book/Book.java imports "common.CommonTypes" or "common.*"
//   - user/User.java imports "common.CommonTypes" or "common.*"
//   - All files import runtime via "pulserpc.*"

// Test: JavaMultiFileWithNestedDir
// When: using -dir src/main/java/lib/rpc
// Then: same structure under nested directory
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 10. Update Java Build Configuration and Documentation

**Background:** Users must configure their Java build to include `{dir}/pulserpc` and `{dir}/{namespace}` in source path, and add necessary `import` statements in their application code.

**Tasks:**
1. Update or create user-facing documentation explaining Java-specific behavior:
   - Explain that `-package` is metadata-only and does not appear in generated code.
   - Document that users must manually add `import pulserpc.*;` in their application code.
   - Document that users must manually add `import {namespace}.*;` for cross-namespace usage.
   - Provide example Maven/Gradle configuration showing how to add generated directories to source path.
2. Add build configuration examples for common Java build tools (Maven, Gradle).
3. Create troubleshooting section for common import path mistakes specific to Java.
4. Document backward compatibility: behavior is compatible when upgrading from old `-base-package` to new `-package`.

**Deliverable:**
- Java-specific documentation in `docs/java.md` or similar, explaining unique semantics.
- Build configuration templates for Maven and Gradle.

**Acceptance Tests:**
```go
// Test: DocumentationExists
// Check: docs/java.md exists and contains:
//   - Explanation of -package metadata-only semantics
//   - Example build configurations
//   - Manual import requirements
//   - Migration guide from -base-package to -package
```

**Make Target:**
- `make quality` (ensure docs pass linting if applicable)

---

## 11. Stabilize Java Test Reliability and Fixtures

**Background:** Java generator tests may have brittle assertions (path separators, temp-dir assumptions, file ordering). Need to make them reliable across environments.

**Tasks:**
1. Review and update existing Java generator tests to remove brittle assertions:
   - Use filepath.Join for cross-platform path assertions
   - Avoid hardcoded path separators
   - Don't assume specific temp directory naming
   - Handle file ordering issues deterministically
2. Consolidate reusable test fixtures/helpers for:
   - Namespace layout verification
   - Import statement assertions
   - Multi-file project setup
   - Runtime location validation
3. Ensure all Java tests are deterministic when run locally and in CI.
4. Add test helpers that can verify generated file structure, imports, and package declarations.

**Deliverable:**
- Reliable Java generator test suite for multi-namespace behavior.
- Reusable test fixtures that can be shared across test cases.

**Acceptance Tests:**
```go
// Test: JavaTestsAreDeterministic
// Run: make test-generator-java multiple times
// Expect: All runs pass with identical results

// Test: JavaCrossPlatformPaths
// Run: tests on both Unix and Windows (or simulate)
// Expect: No path-related test failures
```

**Make Target:**
- `make test-generator-java` (run multiple times to verify stability)
- `make quality`

---

## 12. Final Verification Gate (Verify Steps 9-11 and Full Java Scope)

**Tasks:**
1. Ask an agent to verify all Step 9-11 deliverables and overall Section 4 compliance.
2. Review generated code for complete multi-file project to ensure:
   - Directory structure matches spec exactly
   - Package declarations are simple namespace names
   - Runtime imports use `pulserpc.*`
   - Cross-namespace imports use `{namespace}.*`
   - `-package` flag does not appear anywhere in generated code
3. Run full relevant targets for Java:
   - `make quality`
   - `make test-runtime-java` (if exists)
   - `make test-quickstart-java`
   - `make test-generator-java`
4. Fix any regressions, rerun impacted targets, and do not close until all are green.
5. Perform manual verification: generate complex project and verify it compiles with proper build configuration.

**Exit Criteria:**
- All relevant quality/quickstart/generator targets pass.
- Generated Java code compiles successfully with proper import statements.
- No known regressions remain in Java code generation behavior.
- Documentation is complete and accurate.
- Tests are reliable and deterministic.

---

**Summary of Make Targets by Step:**
- All steps: `make quality` + `make test-generator-java`
- Integration-heavy steps: Add `make test-quickstart-java`
- Final verification: Add `make test-runtime-java` if available

**Java-Specific Notes:**
- Runtime is ALWAYS at `{dir}/pulserpc/` (not under `-package`)
- Package declarations are ONLY the namespace name: `package user;`
- Cross-namespace imports use simple package names: `import common.CommonTypes;`
- Users must manually add imports in their application code (this cannot be generated)
- Namespace directories are lowercase even if namespace has uppercase letters