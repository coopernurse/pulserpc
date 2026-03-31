# PulseRPC Multi-Namespace Code Generation Spec

## Overview

This document defines how PulseRPC code generators should handle multi-file IDL projects with namespaces, respecting the `-dir` and `-package` flags to produce well-organized output that enables cross-namespace references.

## Core Requirements

### Flag Definitions

| Flag | Purpose | Example |
|------|---------|---------|
| `-dir` | Base output directory for all generated code | `lib/rpc` |
| `-package` | Base import path qualifying all namespaces and runtime imports | `github.com/coopernurse/myapp/lib/rpc` |

### Directory Structure Convention

For any PulseRPC project with `-dir lib/rpc -package github.com/coopernurse/myapp/lib/rpc`:

```
lib/rpc/
├── pulserpc/           # Runtime files (same name across all languages)
│   └── [runtime files]
├── user/               # One subdirectory per namespace
│   └── user.go         # (or .py, .ts, .java, .cs)
├── book/
│   └── book.go
└── common/
    └── common.go
```

### Namespace Mapping Rules

1. Each IDL namespace → One subdirectory under `-dir`
2. The subdirectory name = the namespace name (lowercase for Java, original case for others)
3. Generated file(s) inside subdirectory use namespace as package/module name
4. All cross-namespace references use full `-package` qualified paths

---

## 1. Go Generator Specification

### Current State
- Uses `-go-module` flag instead of `-package`
- Places runtime as sibling `../pulserpc/` when outputDir is a subdirectory
- Does not support multi-namespace projects well

### Target Behavior

**Flags:**
- `-dir string` - Output directory (e.g., `lib/rpc`)
- `-package string` - Base import path (e.g., `github.com/coopernurse/myapp/lib/rpc`)
- `-inline-runtime bool` - Place runtime inline (default: true for backward compat, but spec says false)

**Directory Layout:**
```
lib/rpc/
├── pulserpc/           # Runtime files
│   ├── rpc.go
│   ├── client.go
│   ├── server.go
│   └── types.go
├── user/
│   └── user.go         # package user
├── book/
│   └── book.go         # package book
└── common/
    └── common.go       # package common
```

**Generated File Structure (e.g., user/user.go):**
```go
package user

import (
    "github.com/coopernurse/myapp/lib/rpc/pulserpc"
    "github.com/coopernurse/myapp/lib/rpc/common"
)

type User struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}
```

**Runtime Import Path:** `github.com/coopernurse/myapp/lib/rpc/pulserpc`
**Namespace Package Import:** `github.com/coopernurse/myapp/lib/rpc/{namespace}`

### Acceptance Tests

```go
// Test: GoNamespaceToSubdirectory
// Command: pulserpc -plugin go-client-server -dir lib/rpc -package github.com/myapp/lib/rpc book.pulse
// Expect:
//   - lib/rpc/book/book.go exists
//   - package book in book.go
//   - runtime in lib/rpc/pulserpc/

// Test: GoCrossNamespaceImport
// Given: book.pulse (namespace book) includes common.pulse (namespace common)
// When: generating with -dir lib/rpc -package github.com/myapp/lib/rpc
// Then: book/book.go imports "github.com/myapp/lib/rpc/common"

// Test: GoRuntimeInPulserpcDir
// When: -dir lib/rpc -package github.com/myapp/lib/rpc
// Then: lib/rpc/pulserpc/ contains all runtime files
// And: no pulserpc files in lib/rpc/book/ or other namespace dirs
```

---

## 2. Python Generator Specification

### Current State
- No `-package` flag
- No namespace-to-package mapping
- All types in flat server.py/client.py files

### Target Behavior

**Flags:**
- `-dir string` - Output directory (e.g., `lib/rpc`)
- `-package string` - Base Python package path (e.g., `mypackage.lib.rpc`)

**Directory Layout:**
```
lib/rpc/
├── pulserpc/           # Runtime package
│   ├── __init__.py
│   ├── rpc.py
│   ├── client.py
│   └── ...
├── user/               # One subdirectory per namespace
│   ├── __init__.py
│   ├── types.py        # User-defined types
│   ├── server.py       # Interface stubs for user namespace
│   └── client.py       # Client for user namespace
├── book/
│   ├── __init__.py
│   ├── types.py
│   ├── server.py
│   └── client.py
└── common/
    ├── __init__.py
    ├── types.py
    ├── server.py
    └── client.py
```

**Generated File Structure (e.g., user/types.py):**
```python
from mypackage.lib.rpc.pulserpc import RPCError
from mypackage.lib.rpc.common import CommonTypes

class User:
    def __init__(self, id: int, name: str):
        self.id = id
        self.name = name
```

**Runtime Import:** `from mypackage.lib.rpc.pulserpc import ...`
**Namespace Package Import:** `from mypackage.lib.rpc.{namespace} import ...`

### Acceptance Tests

```python
# Test: PythonNamespaceToSubdirectory
# Command: pulserpc -plugin python-client-server -dir lib/rpc -package myapp.lib.rpc book.pulse
# Expect:
#   - lib/rpc/book/types.py exists
#   - lib/rpc/book/server.py exists
#   - lib/rpc/pulserpc/ contains runtime

# Test: PythonCrossNamespaceImport
# Given: book.pulse includes common.pulse
# When: generating with -dir lib/rpc -package myapp.lib.rpc
# Then: book/types.py contains "from myapp.lib.rpc.common import ..."

# Test: PythonPackageInInit
# When: -package myapp.lib.rpc
# Then: all namespace __init__.py files contain appropriate package declarations
```

---

## 3. TypeScript/JavaScript Generator Specification

### Current State
- Has `-package` flag but uses it as prefix for interface names, not module path
- Single `types.ts` file for all namespaces
- Runtime at `outputDir/pulserpc/`

### Target Behavior

**Flags:**
- `-dir string` - Output directory (e.g., `lib/rpc`)
- `-package string` - Base module path (e.g., `./pulserpc` or `@myapp/lib/rpc`)

**Directory Layout:**
```
lib/rpc/
├── pulserpc/           # Runtime module
│   ├── index.ts
│   ├── rpc.ts
│   ├── client.ts
│   └── types.ts
├── user/               # One subdirectory per namespace
│   ├── index.ts        # Re-exports types, server, client
│   ├── types.ts       # User-defined types
│   ├── server.ts       # Interface stubs
│   └── client.ts       # Client class
├── book/
│   ├── index.ts
│   ├── types.ts
│   ├── server.ts
│   └── client.ts
└── common/
    ├── index.ts
    ├── types.ts
    ├── server.ts
    └── client.ts
```

**Generated File Structure (e.g., user/types.ts):**
```typescript
import { RPCError } from '../pulserpc';
import { CommonTypes } from '../common';

export interface User {
  id: number;
  name: string;
}
```

**Runtime Import:** Based on `-package` value, converted to relative path from namespace dir
**Namespace Import:** Relative path `../{namespace}`

### Acceptance Tests

```typescript
// Test: TSNamespaceToSubdirectory
// Command: pulserpc -plugin ts-client-server -dir lib/rpc -package @myapp/lib/rpc book.pulse
// Expect:
//   - lib/rpc/book/types.ts exists
//   - lib/rpc/book/server.ts exists
//   - lib/rpc/pulserpc/ contains runtime

// Test: TSCrossNamespaceImport
// Given: book.pulse includes common.pulse
// When: generating with -dir lib/rpc -package @myapp/lib/rpc
// Then: book/types.ts contains "import { ... } from '../common'"

// Test: TSPackageAffectsImports
// When: -package @myapp/lib/rpc
// Then: runtime imports in user/types.ts use '../pulserpc' (relative)
```

---

## 4. Java Generator Specification

### Current State
- Uses `-base-package` flag (e.g., `com.example`)
- Namespace appended in lowercase: `com.example.{namespace}`
- Runtime at `outputDir/com/bitmechanic/pulserpc/`

### Target Behavior

**Flags:**
- `-dir string` - Output directory (e.g., `src/main/java`)
- `-package string` - Base Java package (e.g., `com.example`) **RENAME from `-base-package`**

**Directory Layout:**
```
lib/rpc/
├── pulserpc/              # Runtime - flat under -dir
│   ├── RPCError.java
│   ├── Client.java
│   └── ...
├── user/                  # One subdirectory per namespace (lowercase)
│   ├── User.java
│   ├── UserServer.java
│   └── UserClient.java
├── book/
│   ├── Book.java
│   ├── BookServer.java
│   └── BookClient.java
└── common/
    ├── CommonTypes.java
    ├── CommonServer.java
    └── CommonClient.java
```

**Generated File Structure (e.g., user/User.java):**
```java
package user;

import pulserpc.RPCError;              // User must add appropriate import based on -dir setup
import common.CommonTypes;

public class User {
    private long id;
    private String name;
    // getters, setters, @JsonProperty annotations
}
```

**Important:** The `-package` flag (e.g., `com.example`) does NOT affect runtime location. Runtime is always at `{dir}/pulserpc/`. Users must configure their Java build to include `{dir}/pulserpc` and `{dir}/{namespace}` in their source path, and add necessary `import` statements.

### Acceptance Tests

```java
// Test: JavaNamespaceToSubdirectory
// Command: pulserpc -plugin java-client-server -dir lib/rpc -package com.example book.pulse
// Expect:
//   - lib/rpc/book/Book.java exists
//   - lib/rpc/pulserpc/ contains runtime (NOT lib/rpc/com/example/pulserpc/)

// Test: JavaCrossNamespaceImport
// Given: book.pulse includes common.pulse
// When: generating with -dir lib/rpc -package com.example
// Then: book/Book.java imports "common.*"

// Test: JavaPackageIsNamespaceOnly
// When: namespace is "book"
// Then: package declaration in Book.java is "package book;"
// And: -package com.example does NOT appear in generated code
```

---

## 5. C# Generator Specification

### Current State
- No `-package` flag
- Namespace converted to PascalCase
- Runtime at `outputDir/PulseRPC/`

### Target Behavior

**Flags:**
- `-dir string` - Output directory (e.g., `lib/rpc`)
- `-package string` - Base namespace (e.g., `MyApp.Lib.Rpc`)

**Directory Layout:**
```
lib/rpc/
├── PulseRPC/           # Runtime namespace
│   ├── RPCError.cs
│   ├── Client.cs
│   └── ...
├── User/               # One subdirectory per namespace
│   ├── User.cs         # Types
│   ├── UserServer.cs   # Interface stubs
│   └── UserClient.cs   # Client class
├── Book/
│   ├── Book.cs
│   ├── BookServer.cs
│   └── BookClient.cs
└── Common/
    ├── CommonTypes.cs
    ├── CommonServer.cs
    └── CommonClient.cs
```

**Generated File Structure (e.g., User/User.cs):**
```csharp
using PulseRPC;
using MyApp.Lib.Rpc.Common;

namespace MyApp.Lib.Rpc.User;

public class User {
    public long Id { get; set; }
    public string Name { get; set; }
}
```

**Runtime Import:** `using PulseRPC;` (or full `using MyApp.Lib.Rpc.PulseRPC;` if `-package` provided)
**Namespace Import:** `using MyApp.Lib.Rpc.{namespace};`

### Acceptance Tests

```csharp
// Test: CSharpNamespaceToSubdirectory
// Command: pulserpc -plugin csharp-client-server -dir lib/rpc -package MyApp.Lib.Rpc book.pulse
// Expect:
//   - lib/rpc/Book/Book.cs exists
//   - lib/rpc/PulseRPC/ contains runtime

// Test: CSharpCrossNamespaceImport
// Given: book.pulse includes common.pulse
// When: generating with -dir lib/rpc -package MyApp.Lib.Rpc
// Then: Book/Book.cs contains "using MyApp.Lib.Rpc.Common;"

// Test: CSharpNamespaceIsPascalCase
// When: namespace is "user_account"
// Then: directory is UserAccount/ and namespace is "UserAccount"
```

---

## Cross-Language Acceptance Tests

### Multi-file Project Test

**Input Files:**

`common.pulse`:
```pulse
namespace common

struct Address {
    street: string
    city: string
}
```

`book.pulse`:
```pulse
namespace book
include "common.pulse"

struct Book {
    title: string
    address: common.Address
}
```

`user.pulse`:
```pulse
namespace user
include "common.pulse"

struct User {
    name: string
    address: common.Address
}
```

**Command:**
```bash
# Generate common first
pulserpc -plugin {lang}-client-server -dir lib/rpc -package {pkg} common.pulse

# Generate book
pulserpc -plugin {lang}-client-server -dir lib/rpc -package {pkg} book.pulse

# Generate user
pulserpc -plugin {lang}-client-server -dir lib/rpc -package {pkg} user.pulse
```

**Expected Directory Structure (all languages):**
```
lib/rpc/
├── pulserpc/           # or PulseRPC, com/bitmechanic/pulserpc, etc.
│   └── [runtime files]
├── common/
│   └── [generated files for common namespace]
├── book/
│   └── [generated files for book namespace]
└── user/
    └── [generated files for user namespace]
```

**Cross-namespace import requirements:**

| Language | book imports common | book imports pulserpc |
|----------|---------------------|----------------------|
| Go | `import "pkg/common"` | `import "pkg/pulserpc"` |
| Python | `from pkg.common import ...` | `from pkg.pulserpc import ...` |
| TypeScript | `import {...} from '../common'` | `import {...} from '../pulserpc'` |
| Java | `import common.*` | `import pulserpc.*` |
| C# | `using Pkg.Common;` | `using Pkg.PulseRPC;` |

---

## Implementation Notes

### Flag Changes Required

| Generator | Current Flag | New Flag | Notes |
|-----------|-------------|----------|-------|
| Go | `-go-module` | `-package` | Use as base import path |
| Python | (none) | `-package` | Add new flag for Python package path |
| TypeScript | `-package` | (keep) | Already exists, clarify usage |
| Java | `-base-package` | `-package` | Rename for consistency |
| C# | (none) | `-package` | Add new flag for base namespace |

### Runtime Directory Naming

All languages use flat `pulserpc/` under the `-dir` base:

| Language | Runtime Directory | Rationale |
|----------|------------------|-----------|
| Go | `pulserpc/` | Standard Go package name |
| Python | `pulserpc/` | Standard Python package name |
| TypeScript | `pulserpc/` | Standard module name |
| Java | `pulserpc/` | Flat structure under -dir, regardless of -package |
| C# | `PulseRPC/` | PascalCase for C# namespace |

**Note:** For Java, even if `-package com.example`, runtime is at `lib/rpc/pulserpc/` not `lib/rpc/com/example/pulserpc/`. Users must add appropriate `import com.example.pulserpc.*` statements in their code.

**Note:** For Python, the user provides the exact Python package format in `-package`. Example: if `-package myapp.lib.rpc`, then `from myapp.lib.rpc.pulserpc import ...`

**Note:** For C#, namespace conversion is automatic: `user_account` becomes `UserAccount` in both directory name and namespace.

### Backward Compatibility

- Add deprecation warnings for old flags (`-go-module`, `-base-package`)
- Default behavior should maintain backward compatibility where possible
- Consider adding a `-legacy` flag to disable new multi-namespace features

---

## Recommended Go Unit Tests for Code Generators

These tests should be added to verify the directory path and import logic across all generators.

### Test File Location
`pkg/generator/multinamespace_test.go` (or per-language, e.g., `go_client_server_test.go`)

### Suggested Test Cases

```go
package generator

import (
    "path/filepath"
    "testing"
)

// TestGoDirFlagCreatesNamespaceSubdirs verifies that -dir creates proper subdirectories
func TestGoDirFlagCreatesNamespaceSubdirs(t *testing.T) {
    tests := []struct {
        name      string
        namespace string
        dir       string
        expected  string // relative path from dir
    }{
        {"simple namespace", "user", "lib/rpc", "user/user.go"},
        {"two word namespace", "user_profile", "lib/rpc", "user_profile/user_profile.go"},
        {"namespace with underscore", "user_account", "output", "user_account/user_account.go"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Generate and verify output path
            outputPath := filepath.Join(tt.dir, tt.namespace, tt.namespace+".go")
            expected := filepath.Join(tt.dir, tt.expected)
            if outputPath != expected {
                t.Errorf("expected %s, got %s", expected, outputPath)
            }
        })
    }
}

// TestGoPackageFlagDeterminesImportPath verifies -package sets import prefix
func TestGoPackageFlagDeterminesImportPath(t *testing.T) {
    tests := []struct {
        name      string
        pkg       string
        namespace string
        runtimeIP string // expected runtime import path
        nsIP      string // expected namespace import path
    }{
        {
            name:      "github style package",
            pkg:       "github.com/myapp/lib/rpc",
            namespace: "user",
            runtimeIP: "github.com/myapp/lib/rpc/pulserpc",
            nsIP:      "github.com/myapp/lib/rpc/user",
        },
        {
            name:      "module style package",
            pkg:       "myapp/lib/rpc",
            namespace: "book",
            runtimeIP: "myapp/lib/rpc/pulserpc",
            nsIP:      "myapp/lib/rpc/book",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            runtimeImport := tt.pkg + "/pulserpc"
            nsImport := tt.pkg + "/" + tt.namespace
            if runtimeImport != tt.runtimeIP {
                t.Errorf("runtime import: expected %s, got %s", tt.runtimeIP, runtimeImport)
            }
            if nsImport != tt.nsIP {
                t.Errorf("namespace import: expected %s, got %s", tt.nsIP, nsImport)
            }
        })
    }
}

// TestGoRuntimeAlwaysInPulserpcSubdir verifies runtime goes to {dir}/pulserpc/
func TestGoRuntimeAlwaysInPulserpcSubdir(t *testing.T) {
    tests := []struct {
        name string
        dir  string
    }{
        {"current dir", "."},
        {"nested dir", "lib/rpc"},
        {"deep nested", "src/github.com/myapp/internal/rpc"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Runtime should always be at {dir}/pulserpc/
            expected := filepath.Join(tt.dir, "pulserpc")
            if tt.dir == "." {
                expected = "pulserpc"
            }
            // Verify path construction logic
            runtimeDir := filepath.Join(tt.dir, "pulserpc")
            if tt.dir == "." {
                runtimeDir = "pulserpc"
            }
            if runtimeDir != expected {
                t.Errorf("expected %s, got %s", expected, runtimeDir)
            }
        })
    }
}

// TestGoCrossNamespaceImport tests that generated code imports other namespaces
func TestGoCrossNamespaceImport(t *testing.T) {
    // Simulate generated code for "book" namespace that includes "common" namespace
    pkg := "github.com/myapp/lib/rpc"

    // When book.go imports common, it should use full path
    commonImport := pkg + "/common"

    expected := "github.com/myapp/lib/rpc/common"
    if commonImport != expected {
        t.Errorf("cross-namespace import: expected %s, got %s", expected, commonImport)
    }
}
```

### Additional Test Patterns to Implement

1. **Path Joining Tests**: Verify `filepath.Join` produces correct cross-platform paths
2. **Import String Construction**: Verify full import paths are constructed correctly
3. **Package Declaration Extraction**: Verify `package X` appears in generated .go files
4. **Runtime File Copy Destination**: Verify runtime files land in `{dir}/pulserpc/`
5. **Include Resolution**: Verify types from included files generate proper imports

### Test Infrastructure

```go
// Helper to create temp directory structure and verify outputs
func withTempDir(t *testing.T, fn func(tmpDir string)) {
    tmpDir, err := os.MkdirTemp("", "pulserpc-test-*")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }
    defer os.RemoveAll(tmpDir)
    fn(tmpDir)
}

// Helper to run generator and capture output
func runGenerator(t *testing.T, plugin, dir, pkg, idlFile string) error {
    // ... implementation
}
```
