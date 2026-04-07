## Context

The Python generator currently treats `-package` values with dots as a single directory name. For `-package myapp.rpc`, it creates `myapp.rpc/` as one directory. This is not idiomatic Python, which uses dot-separated package names that translate to nested directories at the filesystem level.

Current behavior:
```
/tmp/python
└── myapp.rpc          ← single directory named "myapp.rpc"
    ├── book/
    └── pulserpc/
```

Desired behavior:
```
/tmp/python
└── myapp/             ← nested directories split by dots
    └── rpc/
        ├── book/
        └── pulserpc/
```

## Goals / Non-Goals

**Goals:**
- Split `-package` values by dots to create nested directory structure for Python
- Update all affected unit tests and integration tests
- Update documentation to reflect correct usage

**Non-Goals:**
- Changing behavior of other generators (TypeScript, Java, Go, C#)
- Changing the underlying package flag semantics

## Decisions

1. **Split `PackageBase` by dots when constructing paths**
   - The `PythonNamespacePaths.ResolveNamespaceDir()` and `ResolveRuntimeDir()` methods currently use `filepath.Join(p.BaseDir, p.PackageBase)` directly
   - Change to: `filepath.Join(p.BaseDir, splitByDot(p.PackageBase)...)` to split `myapp.lib.rpc` into `["myapp", "lib", "rpc"]`
   - The `splitByDot()` helper already exists in `python_namespace_paths.go:168`

2. **Update import statements in generated `__init__.py` files**
   - The `GenerateInitPy()` method uses `runtimePkg := p.PackageBase + ".pulserpc"` which becomes incorrect
   - Change to use the split package path for proper imports

## Risks / Trade-offs

[Risk] Existing code relying on flat directory structure → [Mitigation] This is a deliberate behavior change; tests and docs updated to reflect new structure
