## Why

When generating code from .pulse IDL files, clients need a way to detect whether their local IDL cache matches the server's current API definition. A checksum computed from the IDL's structural elements provides a stable, content-independent identifier that both parties can compute to verify contract alignment.

## What Changes

- Add a `checksum` field to the top-level `idl.json` output
- The checksum is a SHA-256 hash encoded as base64url (44 characters)
- The checksum is computed from structural elements only (whitespace and comments excluded)
- Element ordering is normalized so file reorganization doesn't affect the checksum
- Namespace-qualified names are used throughout for disambiguation

## Capabilities

### New Capabilities

- `idl-checksum`: Computes a stable SHA-256 checksum of the structural content of a .pulse IDL file, excluding whitespace, comments, and error messages. The checksum is stored in the generated `idl.json` as a top-level `checksum` field.

## Impact

- Parser package: new checksum computation logic
- All code generators: idl.json output will now include `checksum` field
- No breaking changes to existing fields
