## 1. Modify PythonNamespacePaths to split PackageBase by dots

- [x] 1.1 Update `ResolveNamespaceDir()` to split `PackageBase` by dots when constructing path
- [x] 1.2 Update `ResolveRuntimeDir()` to split `PackageBase` by dots when constructing path
- [x] 1.3 Update `GenerateInitPy()` to use split package path for imports

## 2. Update Python unit tests

- [x] 2.1 Update `TestPythonNamespacePathsResolveNamespaceDir` test cases for nested package paths
- [x] 2.2 Update `TestPythonNamespacePathsResolveRuntimeDir` test cases for nested package paths
- [x] 2.3 Update `TestPythonNamespacePathsResolveOutputPath` test cases for nested package paths
- [x] 2.4 Update `TestPythonNamespacePathsEnsureNamespaceDir` test cases for nested package paths
- [x] 2.5 Update `TestPythonNamespacePathsEnsureRuntimeDir` test cases for nested package paths
- [x] 2.6 Update `TestPythonNamespacePathsWithNestedOutputDirs` test case
- [x] 2.7 Update `TestPythonNamespacePathsMultipleNamespaces` test case
- [x] 2.8 Run unit tests to verify changes

## 3. Update Python integration tests

- [x] 3.1 Update `tests/integration/test_quickstart_python.sh` to expect nested directory structure
- [x] 3.2 Run "make quality-full" to verify

## 4. Update documentation

- [x] 4.1 Update `docs/languages/python/quickstart.md` to show correct package structure
- [x] 4.2 Update `openspec/specs/package-output-layout/spec.md` with corrected Python behavior
