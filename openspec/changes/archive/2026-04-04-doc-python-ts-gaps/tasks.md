## 1. CLI Reference Updates

- [x] 1.1 Add `--use-pydantic` flag to Python plugin table in `docs/get-started/cli-reference.md`
- [x] 1.2 Clarify `-package` flag behavior for Python (import paths vs package structure)
- [x] 1.3 Clarify `-package` flag behavior for TypeScript (module path prefix)
- [x] 1.4 Expand `-generate-test-files` example with details on output files

## 2. Python Reference Updates

- [x] 2.1 Add "Pydantic Models" section documenting `--use-pydantic` flag and `models.py` usage
- [x] 2.2 Add "InProcTransport" section documenting in-process RPC transport
- [x] 2.3 Add `Client.validate_request` and `Client.validate_response` flags to Client Usage section
- [x] 2.4 Add `Client.notify()` method documentation for fire-and-forget RPC
- [x] 2.5 Add "Multi-Namespace Projects" section documenting directory structure and imports
- [x] 2.6 Update "Migration Notes" if needed for multi-namespace behavior

## 3. Python Quickstart Updates

- [x] 3.1 Add multi-namespace code generation example using `-dir` and `-package` flags
- [x] 3.2 Show output directory structure for multi-namespace example

## 4. TypeScript Reference Updates

- [x] 4.1 Add "Multi-Namespace Projects" section documenting directory structure
- [x] 4.2 Document `index.ts` re-export files and their purpose
- [x] 4.3 Document cross-namespace import patterns using `../{namespace}/types`
- [x] 4.4 Add `Client.ready()` method documentation for async initialization
- [x] 4.5 Document `validateRequests` and `validateResponses` Server options

## 5. TypeScript Quickstart Updates

- [x] 5.1 Add note about multi-namespace output structure when multiple namespaces exist
- [x] 5.2 Document `idl.json` placement requirement (must be at root)

## 6. Review and Verify

- [x] 6.1 Verify all code examples compile/run correctly
- [x] 6.2 Check for consistency between Python and TypeScript documentation structure
- [x] 6.3 Ensure cross-references between documents are accurate
