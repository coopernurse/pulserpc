## 1. TypeScript Generator - Package Directory Structure

- [x] 1.1 Update `TSNamespacePaths.ResolveRuntimeDir()` to create `{dir}/{package}/pulserpc/` instead of splitting package by dots
- [x] 1.2 Update `TSNamespacePaths.ResolveNamespaceDir()` to create `{dir}/{package}/{namespace}/` instead of splitting package by dots
- [x] 1.3 Add unit tests for new directory resolution behavior (updated existing tests)
- [x] 1.4 Update `ts_client_server.go` to use new path resolution

## 2. TypeScript Generator - IDL Placement

- [x] 2.1 Move `writeIDLJSONTs()` to write `idl.json` inside namespace directory
- [x] 2.2 Update multi-namespace case to place `idl.json` in primary namespace directory
- [x] 2.3 Update single-namespace flat mode to place `idl.json` at output root
- [x] 2.4 Add unit tests for IDL placement scenarios (updated existing tests)

## 3. TypeScript Generator - Import Paths

- [x] 3.1 Verify `tsRuntimeImportPath()` returns correct relative path from namespace directory
- [x] 3.2 Verify `tsCrossNamespaceImportPath()` returns correct relative path
- [x] 3.3 Add unit tests for import path generation with package flag (updated existing tests)

## 4. Python Generator - Package Directory Structure

- [x] 4.1 Update `PythonNamespacePaths.ResolveRuntimeDir()` to create `{dir}/{package}/pulserpc/` instead of splitting package by dots
- [x] 4.2 Update `PythonNamespacePaths.ResolveNamespaceDir()` to create `{dir}/{package}/{namespace}/` instead of splitting package by dots
- [x] 4.3 Add unit tests for new directory resolution behavior (updated existing tests)
- [x] 4.4 Update `python_client_server.go` to use new path resolution

## 5. Python Generator - IDL Placement

- [x] 5.1 Move `writeIDLJSON()` to write `idl.json` inside namespace directory
- [x] 5.2 Update multi-namespace case to place `idl.json` in primary namespace directory
- [x] 5.3 Update single-namespace flat mode to place `idl.json` at output root
- [x] 5.4 Add unit tests for IDL placement scenarios (updated existing tests)

## 6. Playground Manager - Default Package Handling

- [x] 6.1 Remove default `-package com.example.generated` for non-Java generators in `manager.go`
- [x] 6.2 Keep default package only for Java generator
- [x] 6.3 Add integration test verifying playground mode works without default package

## 7. Test Updates

- [x] 7.1 Update TypeScript integration tests to expect new directory structure
- [x] 7.2 Update Python integration tests to expect new directory structure
- [x] 7.3 Update quickstart examples to work with new structure
- [x] 7.4 Run full test suite and fix any failures

## 8. Documentation

- [x] 8.1 Update `docs/MULTI_NAMESPACE_CODE_GEN_SPEC.md` with clarified package flag semantics
- [x] 8.2 Update CLI help text for `-package` flag in TypeScript and Python generators
- [x] 8.3 Add migration notes for users with existing generated code