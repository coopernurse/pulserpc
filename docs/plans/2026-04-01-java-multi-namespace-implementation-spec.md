# Java Multi-Namespace Generator Implementation Spec

**Date:** 2026-04-01
**Source:** `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` (Section 4: Java Generator Specification - **REVISED for idiomatic Java packages**)
**Scope:** Java generator behavior for fully-qualified package-per-namespace output
**Goal:** Implement Section 4 in small, agent-sized steps with recurring verification gates, following Java's reverse-domain package naming conventions.

## 1. Rename Java Flag from `-base-package` to `-package`

**Background:** Java currently uses `-base-package` flag. The spec mandates renaming to `-package` for consistency across all language generators.

**Tasks:**
1. Locate the Java generator flag definition in `pkg/generator/java_client_server.go` and rename it from `-base-package` to `-package`.
2. Update all flag parsing, configuration plumbing, and internal references to use the new flag name.
3. Ensure other language plugins are unaffected by this change.
4. Add/update unit tests to verify both flags work: new `-package` should work, and optionally add deprecation warning for old `-base-package` if still referenced anywhere.

**Deliverable:**
- Java generator accepts `-package` flag (e.g., `com.myapp.rpc`) as the base Java package prefix.
- Old `-base-package` flag either removed or deprecated with clear warning message.

**Acceptance Tests:**
```go
// Test: JavaPackageFlagParsing
// Command: pulserpc -plugin java-client-server -dir src/main/java -package com.myapp.rpc book.pulse
// Then: generator config contains Package="com.myapp.rpc"
```

**Make Target:**
- `make test-generator-java` (to verify flag parsing tests pass)
- `make quality` (ensure no lint issues)

---

## 2. Implement Fully-Qualified Package Declarations

**Background:** Unlike Python/Go which use simple package names, Java follows idiomatic reverse-domain package naming. The `-package` flag serves as a prefix for all namespace package declarations.

**Example:**
With `-package com.myapp.rpc -dir src/main/java` and namespace `user`:
- Directory: `src/main/java/com/myapp/rpc/user/`
- Package declaration: `package com.myapp.rpc.user;`
- Files: `src/main/java/com/myapp/rpc/user/User.java`, `UserServer.java`, `UserClient.java`

**Tasks:**
1. Modify Java generator to construct fully-qualified package names: `{package}.{namespace}`
2. Generate package declarations as `package {package}.{namespace};` in all Java files.
3. Construct output paths that mirror the package structure: `{dir}/{package-dirs}/{namespace}/`
4. Convert package dots to directory separators (e.g., `com.example` → `com/example/`).
5. Add unit tests to verify package declarations include the `-package` prefix correctly.

**Deliverable:**
- Generated Java files contain fully-qualified package declarations like `package com.myapp.rpc.user;`
- Directory structure mirrors the package structure (e.g., `com/myapp/rpc/user/`)

**Acceptance Tests:**
```go
// Test: JavaQualifiedPackageDeclaration
// Given: book.pulse with namespace "book"
// When: generating with -dir src/main/java -package com.myapp.rpc
// Then: src/main/java/com/myapp/rpc/book/Book.java contains "package com.myapp.rpc.book;"
// And: directory structure is src/main/java/com/myapp/rpc/book/

// Test: JavaNestedPackageStructure
// Given: user.pulse with namespace "user"
// When: generating with -dir lib -package org.company.internal.services
// Then: lib/org/company/internal/services/user/User.java contains "package org.company.internal.services.user;"
// And: runtime is at lib/pulserpc/ (not under package structure)
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 3. Implement Namespace-to-Directory Output Layout with Package Mirroring

**Background:** Java must generate namespace-specific files in directory structures that mirror their fully-qualified package names. This follows Maven/Gradle conventions.

**Tasks:**
1. Modify Java generator path resolution to create `{dir}/{package-dirs}/{namespace}/` structure.
2. Convert package dots to directory separators when constructing paths.
3. Generate Java files: `{dir}/{package-dirs}/{namespace}/{Type}.java` and related files.
4. Ensure namespace directory names respect case from the namespace (unlike Python/Go which lowercase them).
5. For multi-namespace projects (book.pulse includes common.pulse), ensure output includes both namespace directories at `{dir}/{package-dirs}/{namespace}/`.
6. Add path construction tests covering nested output dirs, multiple namespaces, and deep package hierarchies.

**Deliverable:**
- Deterministic path logic that mirrors Java package conventions.
- Directory structure: `{dir}/{package-dirs}/{namespace}/{files}.java`

**Acceptance Tests:**
```go
// Test: JavaNamespacedDirectoryStructure
// Command: pulserpc -plugin java-client-server -dir src/main/java -package com.myapp.rpc book.pulse
// Expect:
//   - src/main/java/com/myapp/rpc/book/Book.java exists
//   - src/main/java/com/myapp/rpc/book/BookServer.java exists  
//   - src/main/java/com/myapp/rpc/book/BookClient.java exists
//   - Package declaration is "package com.myapp.rpc.book;"

// Test: JavaMultipleNamespacesSamePackageRoot
// Given: user.pulse and project.pulse with different namespaces
// When: generating with -dir src/main/java -package com.myapp.rpc
// Then: both namespaces appear under same package root:
//   - src/main/java/com/myapp/rpc/user/
//   - src/main/java/com/myapp/rpc/project/

// Test: JavaNamespaceCasePreserved
// Given: namespace "UserProfile" with uppercase
// Then: directory is src/main/java/com/myapp/rpc/UserProfile/
// And: package declaration is "package com.myapp.rpc.UserProfile;"
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 4. Verification Gate A (Verify Steps 1-3)

**Tasks:**
1. Ask an agent to verify implementation quality and spec alignment for Steps 1-3 (flag rename, qualified packages, namespace directory layout).
2. Run quality and Java integration gates:
   - `make quality`
   - `make test-quickstart-java`
   - `make test-generator-java`
3. Fix regressions from these runs, then rerun failed targets until green.
4. Manually verify generated files have proper package declarations and directory structure.

**Exit Criteria:**
- All three targets pass and no open regressions remain from Steps 1-3.
- Generated Java files show `package {package}.{namespace};` declarations.
- Directory structure correctly mirrors package structure ({package-dirs}/{namespace}).

---

## 5. Implement Runtime Output to `{dir}/pulserpc/`

**Background:** Java runtime must always be generated to `{dir}/pulserpc/` (flat under `-dir`), regardless of `-package` flag value. Runtime is NOT placed in the package hierarchy.

**Tasks:**
1. Update Java runtime generation logic to output to `{dir}/pulserpc/`.
2. Ensure runtime files (RPCError.java, Client.java, Server.java, etc.) are written directly to the `pulserpc/` directory at the root of `-dir`.
3. Verify that `-package com.myapp.rpc` does NOT cause runtime to be placed at `{dir}/com/myapp/rpc/pulserpc/`.
4. Add tests that assert runtime location is always `{dir}/pulserpc/` for various `-dir` and `-package` values.
5. Runtime package declaration should be simply `package pulserpc;` (not qualified by `-package`).
6. Ensure backwards compatibility: when `-package` is omitted or empty, behavior should remain consistent.

**Deliverable:**
- Runtime files consistently generated to `{dir}/pulserpc/` directory.
- Runtime location is independent of `-package` flag value.
- Runtime package stays as simple `package pulserpc;`.

**Acceptance Tests:**
```go
// Test: JavaRuntimeInPulserpcDir
// When: -dir src/main/java -package com.myapp.rpc
// Then: src/main/java/pulserpc/ contains all runtime files
// And: NO runtime files in src/main/java/com/myapp/rpc/book/ or namespace dirs
// And: runtime package declarations are "package pulserpc;"

// Test: JavaRuntimeLocationWithNestedDirAndPackage
// When: -dir lib -package org.company.services
// Then: runtime is at lib/pulserpc/ (not lib/org/company/services/pulserpc/)

// Test: JavaRuntimeWithoutPackageFlag
// When: generating without -package flag
// Then: runtime still at {dir}/pulserpc/
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 6. Implement Fully-Qualified Cross-Namespace Imports

**Background:** When book.pulse includes common.pulse, Java must generate fully-qualified imports using the `-package` prefix: `import com.myapp.rpc.common.MyType;`

**Tasks:**
1. Modify Java import generation logic to construct fully-qualified imports for cross-namespace references: `import {package}.{namespace}.{Type};`
2. Ensure include-based type references resolve to the correct fully-qualified namespace package.
3. Construct import paths using dots (not slashes): `com.myapp.rpc.common.CommonTypes` not `com/myapp/rpc/common`.
4. Add tests for `book` -> `common` and `user` -> `common` import strings with various `-package` values.
5. Test that types from included namespaces are properly imported and usable.

**Deliverable:**
- Cross-namespace imports are fully-qualified package names.
- Imports match the directory structure and package declarations.

**Acceptance Tests:**
```go
// Test: JavaCrossNamespaceImport
// Given: book.pulse (namespace book) includes common.pulse (namespace common)
// When: generating with -dir src/main/java -package com.myapp.rpc
// Then: book/Book.java contains "import com.myapp.rpc.common.CommonTypes;"
// And: book/Book.java can reference common types directly

// Test: JavaMultipleNamespaceImportsWithSameRoot
// Given: user.pulse includes both common.pulse and book.pulse
// When: generating with -package org.company.services
// Then: user/User.java contains imports for both "org.company.services.common.*" and "org.company.services.book.*"

// Test: JavaQualifiedImportWithoutAsterisk
// Given: book.pulse uses specific types from common.Address and common.User
// Then: imports should be specific: "import com.myapp.rpc.common.Address;" and "import com.myapp.rpc.common.User;"
// Or: use qualified wildcards: "import com.myapp.rpc.common.*;"
```

**Make Target:**
- `make test-generator-java`
- `make quality`

---

## 7. Implement Runtime Imports Through pulserpc Package

**Background:** Java must generate runtime imports as `import pulserpc.RPCError;` or `import pulserpc.*;` using the simple `pulserpc` package name (not qualified by `-package`).

**Tasks:**
1. Modify Java import generation to use simple `pulserpc` package for runtime imports (e.g., `import pulserpc.RPCError;`).
2. Verify that runtime package stays separate from the qualified namespace packages.
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
// When: generating with -dir src/main/java -package com.myapp.rpc
// Then: generated files contain "import pulserpc.RPCError;" or "import pulserpc.*;"
// And: does NOT contain "com.myapp.rpc.pulserpc"

// Test: JavaRuntimeImportWithoutPackageFlag
// When: generating without -package flag (backwards compat)
// Then: runtime imports still use "import pulserpc.*;" and namespace packages are simple names
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 8. Verification Gate B (Verify Steps 5-7)

**Tasks:**
1. Ask an agent to verify implementation quality and spec alignment for Steps 5-7 (runtime output, qualified cross-namespace imports, runtime imports).
2. Run quality and Java integration gates:
   - `make quality`
   - `make test-quickstart-java`
   - `make test-generator-java`
3. Fix regressions from these runs, then rerun failed targets until green.
4. Verify generated import strings and package declarations are correct by inspecting actual generated files.

**Exit Criteria:**
- All three targets pass and generated imports/paths are validated by tests.
- Runtime is confirmed to be at `{dir}/pulserpc/` with package `pulserpc`.
- Cross-namespace imports are fully-qualified with `-package` prefix.
- Package declarations match directory structure.

---

## 9. Add Multi-File End-to-End Java Coverage

**Background:** The spec requires testing the complete flow: common.pulse, book.pulse, user.pulse with proper cross-namespace imports and directory structure.

**Complete Example Structure:**
With `-dir src/main/java -package com.myapp.rpc`:
```
src/main/java/
├── pulserpc/                          # Runtime
│   ├── RPCError.java                  # package pulserpc;
│   ├── Contract.java
│   └── ...
├── com/myapp/rpc/common/             # common.pulse types
│   ├── CommonTypes.java               # package com.myapp.rpc.common;
│   ├── CommonServer.java
│   └── CommonClient.java
├── com/myapp/rpc/book/               # book.pulse types
│   ├── Book.java                      # package com.myapp.rpc.book;
│   ├── BookServer.java
│   └── BookClient.java
└── com/myapp/rpc/user/               # user.pulse types
    ├── User.java                      # package com.myapp.rpc.user;
    ├── UserServer.java
    └── UserClient.java
```

**Tasks:**
1. Add integration tests for the `common.pulse`, `book.pulse`, `user.pulse` multi-file flow.
2. Assert output tree includes `pulserpc/` and all namespace directories under `{dir}/{package-dirs}/`.
3. Assert that `book` and `user` modules import from `common` via fully-qualified imports.
4. Verify runtime imports in all generated files use `pulserpc` package.
5. Test with various `-dir` values (current dir, nested dirs, absolute paths).

**Deliverable:**
- Multi-file namespace generation is enforced in tests.
- Directory structure matches Maven/Gradle conventions.

**Acceptance Tests:**
```go
// Test: JavaMultiFileProjectStructure
// Command sequence:
//   pulserpc -plugin java-client-server -dir src/main/java -package com.myapp.rpc common.pulse
//   pulserpc -plugin java-client-server -dir src/main/java -package com.myapp.rpc book.pulse
//   pulserpc -plugin java-client-server -dir src/main/java -package com.myapp.rpc user.pulse
// Expect:
//   - src/main/java/pulserpc/ exists with runtime files at package pulserpc
//   - src/main/java/com/myapp/rpc/common/ exists with Common*.java at package com.myapp.rpc.common
//   - src/main/java/com/myapp/rpc/book/ exists with Book*.java at package com.myapp.rpc.book
//   - src/main/java/com/myapp/rpc/user/ exists with User*.java at package com.myapp.rpc.user
//   - book/Book.java contains "import com.myapp.rpc.common.CommonTypes;"
//   - user/User.java contains "import com.myapp.rpc.common.CommonTypes;"
//   - All files import runtime via "import pulserpc.*;"

// Test: JavaMultiFileWithGradleLayout
// When: using -dir lib/java/generated -package org.mycompany.services
// Then: same structure: lib/java/generated/org/mycompany/services/{namespace}/
```

**Make Target:**
- `make test-generator-java`
- `make quality`
- `make test-quickstart-java`

---

## 10. Update Java Build Configuration and Documentation

**Background:** With fully-qualified packages, Java build configuration becomes straightforward. Users simply include the `-dir` as a source directory, and all packages are automatically resolved.

**Tasks:**
1. Update or create user-facing documentation explaining fully-qualified package behavior:
    - Explain that `-package` becomes part of the generated package declarations.
    - Document that namespace packages are `{package}.{namespace}`.
    - Show how runtime stays at simple `pulserpc` package separate from user packages.
    - Provide example Maven/Gradle configuration showing generated source directories.
    - Include IDE setup notes (IntelliJ, Eclipse) for generated source directories.
2. Add build configuration examples for Maven and Gradle that work with the qualified package structure.
3. Create troubleshooting section for common import issues in IDEs.
4. Document migration from old `-base-package` to new `-package` with proper semantics.

**Deliverable:**
- Java-specific documentation in `docs/java.md` or similar, explaining qualified package behavior.
- Build configuration templates that match the directory structure to package names.

**Acceptance Tests:**
```go
// Test: DocumentationExists
// Check: docs/java.md exists and contains:
//   - Explanation of fully-qualified package declarations
//   - Relationship between -package, namespace, and generated directories
//   - Example: -package com.myapp.rpc → files at src/main/java/com/myapp/rpc/{namespace}/
//   - Maven/Gradle source directory configuration examples
//   - IDE configuration notes
```

**Make Target:**
- `make quality` (ensure docs pass linting if applicable)

---

## 11. Stabilize Java Test Reliability and Fixtures

**Background:** With qualified packages, test assertions must handle package path construction correctly. Need reliable tests for package-to-path conversion.

**Tasks:**
1. Review and update existing Java generator tests to support fully-qualified packages:
    - Extract package-to-path conversion logic into reusable helper
    - Use filepath.Join for cross-platform path assertions
    - Don't assume specific temp directory naming
    - Handle file ordering issues deterministically
2. Consolidate reusable test fixtures/helpers for:
    - Fully-qualified package layout verification
    - Import statement assertions (checking for `{package}.{namespace}` pattern)
    - Multi-file project setup with qualified packages
    - Runtime location validation (separate from namespace packages)
3. Ensure all Java tests are deterministic when run locally and in CI.
4. Add test helpers that can verify package declarations, import statements, and directory structure alignment.

**Deliverable:**
- Reliable Java generator test suite for multi-namespace qualified package behavior.
- Reusable test fixtures that can be shared across test cases.

**Acceptance Tests:**
```go
// Test: JavaTestsAreDeterministic
// Run: make test-generator-java multiple times
// Expect: All runs pass with identical results

// Test: JavaPackageToPathHelper
// Verify: helper function correctly converts "com.myapp.rpc" to "com/myapp/rpc" on Unix and "com\\myapp\\rpc" on Windows

// Test: JavaQualifiedPackageAssertions
// Given: -package com.myapp.rpc -namespace user
// Verify: test helpers correctly assert both package declaration and directory path
```

**Make Target:**
- `make test-generator-java` (run multiple times to verify stability)
- `make quality`

---

## 12. Final Verification Gate (Verify Steps 9-11 and Full Java Scope)

**Tasks:**
1. Ask an agent to verify all Step 9-11 deliverables and overall Section 4 compliance.
2. Review generated code for complete multi-file project to ensure:
    - Directory structure: `{dir}/{package-dirs}/{namespace}/` matches Maven/Gradle conventions
    - Package declarations: `package {package}.{namespace};` are correct
    - Runtime imports: use simple `import pulserpc.*;`
    - Cross-namespace imports: fully-qualified `import {package}.{namespace}.{Type};`
    - All imports resolve correctly and code compiles
3. Compile generated code with Maven and Gradle to verify build integration.
4. Run full relevant targets for Java:
    - `make quality`
    - `make test-runtime-java` (if exists)
    - `make test-quickstart-java`
    - `make test-generator-java`
5. Fix any regressions, rerun impacted targets, and do not close until all are green.
6. Perform IDE import test: import generated project into IntelliJ/Eclipse and verify no import errors.

**Exit Criteria:**
- All relevant quality/runtime/quickstart/generator targets pass.
- Generated Java code compiles successfully in Maven, Gradle, and IDEs.
- No known regressions remain in Java code generation behavior.
- Documentation is complete and accurate with Maven/Gradle examples.
- Tests are reliable and deterministic.

---

**Summary of Make Targets by Step:**
- All steps: `make quality` + `make test-generator-java`
- Integration-heavy steps: Add `make test-quickstart-java`
- Final verification: Add `make test-runtime-java` if available

**Idiomatic Java-Specific Notes:**
- **Runtime is ALWAYS at** `{dir}/pulserpc/` with package `pulserpc` (not qualified)
- **Package declarations are fully-qualified:** `package {package}.{namespace};` (e.g., `com.myapp.rpc.user`)
- **Directory structure mirrors packages:** `{dir}/{package-dirs}/{namespace}/` (e.g., `src/main/java/com/myapp/rpc/user/`)
- **Cross-namespace imports are fully-qualified:** `import com.myapp.rpc.common.CommonTypes;`
- **Runtime imports stay simple:** `import pulserpc.RPCError;` (no package prefix needed)
- **Build tools work seamlessly:** Maven/Gradle can use `-dir` as source directory, packages auto-resolve
- **Follows Java naming conventions:** Reverse domain package names, hierarchical structure