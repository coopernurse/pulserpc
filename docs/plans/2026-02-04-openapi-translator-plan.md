# OpenAPI ↔ Pulse IDL Translator

**Date:** February 4, 2026  
**Status:** Draft  
**Author:** AI Planning Assistant

## Overview

Add a bidirectional translation mode to PulseRPC that converts OpenAPI/Swagger specifications to `.pulse` IDL files and vice versa. This enables users to:
- Import existing REST API definitions and generate JSON-RPC clients/servers
- Export Pulse-defined services as OpenAPI specs for REST tooling compatibility

## Design Decisions

### HTTP Method Mapping (OpenAPI → Pulse)
**Decision:** Prefix method names with HTTP verb (e.g., `getUsers`, `postUser`, `deleteUser`)

This preserves HTTP semantic hints while remaining idiomatic for RPC method naming.

### Pulse → OpenAPI Output Style
**Decision:** Generate standard REST-style OpenAPI (one endpoint per method using POST)

Each Pulse interface method becomes a dedicated POST endpoint (e.g., `POST /users/create`), making the output consumable by standard REST tooling without requiring JSON-RPC knowledge.

### Handling Unsupported OpenAPI Features
- **Multiple response codes:** Collapse to single success type; document error mappings in comments
- **Path/query/header params:** Flatten all into method parameters with naming convention
- **Security schemes:** Drop with warning; include as IDL comments for reference
- **oneOf/anyOf:** Not supported; emit warning
- **allOf:** Map to Pulse `extends` keyword where possible
- **File uploads/binary:** Not supported; emit error

---

## Phase 1: Foundation & CLI Infrastructure

### Objective
Set up the CLI interface, project structure, and dependency management for the OpenAPI translator feature.

### Tasks
1. Add CLI flags to `cmd/pulse/pulse.go`:
   - `-openapi-to-pulse <input.yaml|json>` - Convert OpenAPI spec to Pulse IDL
   - `-pulse-to-openapi <input.pulse>` - Convert Pulse IDL to OpenAPI spec
   - `-output-dir <dir>` - Output directory (default: `./generated`)
   - `-openapi-version <3.0|3.1>` - Target OpenAPI version for output (default: 3.1)
   
2. Create package structure:
   ```
   pkg/openapi/
   ├── parser.go       # OpenAPI spec parsing
   ├── to_pulse.go     # OpenAPI → Pulse translator
   ├── from_pulse.go   # Pulse → OpenAPI translator
   ├── types.go        # Shared types/interfaces
   └── testdata/       # Test fixtures
   ```

3. Add `kin-openapi/openapi3` dependency to `go.mod` for OpenAPI parsing

4. Create stub functions that return "not implemented" errors

### Acceptance Criteria
- [ ] `pulserpc -openapi-to-pulse example.yaml` parses flags without error (can return "not implemented")
- [ ] `pulserpc -pulse-to-openapi example.pulse` parses flags without error (can return "not implemented")
- [ ] `-h` output documents new flags with usage examples
- [ ] `go build` succeeds with new package structure
- [ ] `go mod tidy` completes without errors
- [ ] Unit test file exists for each new `.go` file (even if tests are minimal)

---

## Phase 2: OpenAPI Parser & Type Mapping

### Objective
Parse OpenAPI 3.x specifications and map OpenAPI Schema Objects to internal representation suitable for Pulse IDL generation.

### Tasks
1. Implement `pkg/openapi/parser.go`:
   - Load and validate OpenAPI 3.0/3.1 YAML or JSON files
   - Resolve `$ref` references (local and external)
   - Extract `components/schemas` into normalized type map
   - Handle circular references gracefully

2. Implement type mapping in `pkg/openapi/types.go`:
   - OpenAPI `string` → Pulse `string`
   - OpenAPI `integer` (int32/int64) → Pulse `int`
   - OpenAPI `number` (float/double) → Pulse `float`
   - OpenAPI `boolean` → Pulse `bool`
   - OpenAPI `array` → Pulse `[]Type`
   - OpenAPI `object` with `additionalProperties` → Pulse `map[string]Type`
   - OpenAPI `object` with `properties` → Pulse struct candidate
   - OpenAPI `enum` → Pulse enum candidate
   - OpenAPI `allOf` → Pulse struct with `extends` (if single $ref + object)

3. Implement unsupported type handling:
   - `oneOf`/`anyOf` → emit warning, use first option or `string` fallback
   - `format: binary`/`format: byte` → emit error
   - Recursive types → detect and emit warning with `[optional]` break

### Acceptance Criteria
- [ ] Successfully parses Petstore OpenAPI 3.0 example spec
- [ ] Successfully parses Petstore OpenAPI 3.1 example spec
- [ ] Correctly maps all primitive types (string, integer, number, boolean)
- [ ] Correctly identifies array types with proper element type
- [ ] Correctly identifies object schemas as struct candidates
- [ ] Correctly identifies string enums as enum candidates
- [ ] Handles `$ref` to local `#/components/schemas/X` references
- [ ] Emits warning (not crash) for `oneOf`/`anyOf` schemas
- [ ] Emits error for binary/byte format fields
- [ ] Unit tests cover all type mapping scenarios
- [ ] Unit tests verify circular reference detection

---

## Phase 3: OpenAPI → Pulse IDL Generator

### Objective
Generate valid `.pulse` IDL files from parsed OpenAPI specifications.

### Tasks
1. Implement namespace derivation in `pkg/openapi/to_pulse.go`:
   - Use `info.title` converted to valid identifier (lowercase, underscores)
   - Fallback to filename if title invalid

2. Implement struct generation:
   - Convert each `components/schemas` object to Pulse struct
   - Map `required` array to determine `[optional]` annotations
   - Handle `allOf` by generating `extends` clause when pattern matches
   - Generate doc comments from `description` fields

3. Implement enum generation:
   - Convert string schemas with `enum` to Pulse enum
   - Generate doc comments from `description`

4. Implement interface/method generation:
   - Group operations by `tags[0]` (or path prefix if no tags) → interfaces
   - Generate method names: `{httpMethod}{operationId}` or `{httpMethod}{pathToMethodName}`
   - Map path parameters to method parameters (e.g., `/users/{id}` → `id string`)
   - Map query parameters to method parameters
   - Map request body schema to method parameter (named `body` or schema name)
   - Map successful response (200/201/default) schema to return type
   - Mark return as `[optional]` if response can be empty (204)

5. Implement output formatting:
   - Generate well-formatted `.pulse` file with proper indentation
   - Include header comment with source OpenAPI file and generation timestamp
   - Preserve operation descriptions as method comments

### Acceptance Criteria
- [ ] Generated `.pulse` file passes `pulserpc` parser validation
- [ ] Namespace is derived from OpenAPI `info.title`
- [ ] All `components/schemas` objects become Pulse structs
- [ ] All string enums become Pulse enums  
- [ ] Required vs optional fields are correctly annotated
- [ ] `allOf` with single ref + object generates `extends` clause
- [ ] Operations are grouped into interfaces by tag
- [ ] Method names follow `{verb}{OperationId}` pattern (e.g., `getListPets`)
- [ ] Path parameters become method parameters
- [ ] Query parameters become method parameters
- [ ] Request body becomes method parameter
- [ ] 200/201 response schema becomes return type
- [ ] 204 No Content results in `[optional]` return
- [ ] Doc comments are generated from descriptions
- [ ] Integration test: Petstore spec → valid Pulse IDL → generates working code

---

## Phase 4: Pulse → OpenAPI Generator

### Objective
Generate valid OpenAPI 3.1 specifications from Pulse IDL files.

### Tasks
1. Implement `pkg/openapi/from_pulse.go`:
   - Use existing Pulse parser (`pkg/parser`) to load IDL
   - Generate `openapi: "3.1.0"` header
   - Derive `info.title` from namespace

2. Implement schema generation:
   - Pulse struct → OpenAPI `components/schemas` object
   - Pulse struct with `extends` → OpenAPI `allOf` composition
   - Pulse enum → OpenAPI string schema with `enum` array
   - Pulse `[optional]` fields → excluded from `required` array
   - Pulse `[]Type` → OpenAPI array with `items`
   - Pulse `map[string]Type` → OpenAPI object with `additionalProperties`

3. Implement path generation:
   - Each interface becomes a tag
   - Each method becomes a path: `POST /{interface}/{method}`
   - Method parameters become request body schema (wrapped in object)
   - Return type becomes 200 response schema
   - `[optional]` return adds 204 No Content response
   - Generate `operationId` from `{interface}_{method}`

4. Implement output formatting:
   - Generate valid YAML (preferred) or JSON based on output filename
   - Include `servers` array with placeholder `http://localhost:8080`
   - Generate `description` from IDL comments

### Acceptance Criteria
- [ ] Generated OpenAPI spec passes validation (e.g., Swagger Editor, Spectral)
- [ ] `openapi` field is set to `"3.1.0"`
- [ ] `info.title` matches Pulse namespace
- [ ] All Pulse structs appear in `components/schemas`
- [ ] All Pulse enums appear in `components/schemas` with `enum` array
- [ ] Struct inheritance generates `allOf` correctly
- [ ] Optional fields are not in `required` array
- [ ] Each interface method generates a POST endpoint
- [ ] Path follows `/{interface}/{method}` pattern
- [ ] Request body contains method parameters
- [ ] Response 200 contains return type schema
- [ ] `operationId` follows `{interface}_{method}` pattern
- [ ] Output is valid YAML or JSON based on extension
- [ ] Integration test: Pulse IDL → OpenAPI → reimport matches original types

---

## Phase 5: Warning/Error Reporting & Edge Cases

### Objective
Implement comprehensive diagnostics and handle edge cases gracefully.

### Tasks
1. Implement warning system:
   - Collect warnings during translation (don't abort)
   - Print warnings to stderr at end of translation
   - Include source location (line/path) where possible
   - Add `-strict` flag to treat warnings as errors

2. Implement OpenAPI → Pulse warnings:
   - `oneOf`/`anyOf` encountered (with fallback type used)
   - Security scheme dropped
   - Response codes other than 200/201/204 dropped
   - Header/cookie parameters dropped
   - `format: binary` field skipped
   - Circular reference broken with `[optional]`

3. Implement Pulse → OpenAPI warnings:
   - Method parameter names that conflict with OpenAPI reserved words
   - Very long paths that may cause issues

4. Handle edge cases:
   - Empty OpenAPI `paths` object → generate IDL with only types
   - Empty Pulse interfaces → generate OpenAPI with only schemas
   - Anonymous/inline schemas → generate synthetic names
   - Duplicate type names across namespaces → prefix with namespace
   - OpenAPI `nullable: true` → map to `[optional]`

### Acceptance Criteria
- [ ] Warnings are printed to stderr, not stdout
- [ ] Each warning includes source location when available
- [ ] `-strict` flag causes non-zero exit on warnings
- [ ] `oneOf`/`anyOf` generates warning with fallback type noted
- [ ] Security schemes generate warning listing dropped schemes
- [ ] Non-2xx response codes generate warning
- [ ] Header/cookie params generate warning with names
- [ ] Binary fields cause error (not just warning)
- [ ] Empty paths → valid IDL with structs/enums only
- [ ] Inline schemas get synthetic names (`AnonymousType1`, etc.)
- [ ] Duplicate names are disambiguated
- [ ] `nullable: true` becomes `[optional]`
- [ ] All warnings have actionable message text

---

## Phase 6: Documentation & Examples

### Objective
Document the feature comprehensively with examples and known limitations.

### Tasks
1. Create `docs/get-started/openapi-translation.md`:
   - Feature overview and use cases
   - Installation/requirements
   - Quick start examples
   - Full CLI reference

2. Create `docs/advanced/openapi-mapping-reference.md`:
   - Complete type mapping table (OpenAPI ↔ Pulse)
   - Method naming conventions
   - Interface grouping logic
   - Path generation rules

3. Create example files in `examples/openapi/`:
   - `petstore.yaml` - Standard Petstore OpenAPI spec
   - `petstore.pulse` - Expected generated Pulse IDL
   - `simple-api.pulse` - Simple Pulse IDL for round-trip testing
   - `simple-api.openapi.yaml` - Expected generated OpenAPI

4. Add to existing docs:
   - Update CLI reference page with new flags
   - Add "OpenAPI Integration" section to main docs navigation
   - Update README.md with brief mention of feature

### Acceptance Criteria
- [ ] `docs/get-started/openapi-translation.md` exists with all sections
- [ ] Quick start can be followed by new user successfully
- [ ] CLI reference documents all flags with examples
- [ ] Type mapping table is complete and accurate
- [ ] `examples/openapi/` contains all example files
- [ ] Generated Pulse from `petstore.yaml` matches `petstore.pulse`
- [ ] Generated OpenAPI from `simple-api.pulse` matches `simple-api.openapi.yaml`
- [ ] Navigation includes new documentation pages
- [ ] README mentions OpenAPI translation feature
- [ ] All documentation links are valid (no 404s)

---

## Phase 7: Testing & Integration

### Objective
Comprehensive test coverage including unit tests, integration tests, and round-trip validation.

### Tasks
1. Unit tests in `pkg/openapi/`:
   - `parser_test.go` - OpenAPI parsing edge cases
   - `to_pulse_test.go` - Type mapping and generation
   - `from_pulse_test.go` - Reverse mapping and generation
   - `types_test.go` - Shared type utilities

2. Integration tests in `tests/integration/`:
   - `test_openapi_to_pulse.sh` - CLI integration for import
   - `test_pulse_to_openapi.sh` - CLI integration for export
   - `test_openapi_roundtrip.sh` - Import → Export → validate equivalence

3. Add to CI pipeline:
   - Run OpenAPI tests as part of `make test`
   - Add OpenAPI-specific test target `make test-openapi`

4. Create conformance test:
   - Generate Pulse from complex OpenAPI spec
   - Generate code from Pulse (all languages)
   - Verify generated code compiles/type-checks

### Acceptance Criteria
- [ ] Unit test coverage ≥80% for new `pkg/openapi/` code
- [ ] `make test` includes and passes OpenAPI tests
- [ ] `make test-openapi` target exists and passes
- [ ] Integration test validates CLI exit codes
- [ ] Integration test validates output file creation
- [ ] Round-trip test confirms type preservation
- [ ] Generated Pulse from Petstore compiles to working code (Python)
- [ ] Generated Pulse from Petstore compiles to working code (TypeScript)
- [ ] CI pipeline runs all new tests
- [ ] No regressions in existing test suite

---

## Known Limitations

Users should be aware of these fundamental limitations:

| OpenAPI Feature | Support | Behavior |
|-----------------|---------|----------|
| Multiple HTTP methods per path | ❌ | Each becomes separate method with verb prefix |
| Path parameters | ✅ | Become method parameters |
| Query parameters | ✅ | Become method parameters |
| Header parameters | ⚠️ | Dropped with warning |
| Cookie parameters | ⚠️ | Dropped with warning |
| Multiple response codes | ⚠️ | Only 200/201 preserved; others dropped with warning |
| `oneOf`/`anyOf` | ⚠️ | Warning; uses first type or `string` fallback |
| `allOf` composition | ✅ | Maps to `extends` keyword |
| `discriminator` | ❌ | Dropped (no Pulse equivalent) |
| File uploads/binary | ❌ | Error; not supported |
| Security schemes | ⚠️ | Dropped with warning |
| Callbacks/webhooks | ❌ | Dropped with warning |
| Server variables | ❌ | Dropped |
| `nullable: true` | ✅ | Maps to `[optional]` |
| Default values | ❌ | Dropped (no Pulse equivalent) |
| Examples | ❌ | Dropped |
| External `$ref` | ⚠️ | Attempted; may fail |

---

## Timeline Estimate

| Phase | Estimated Effort | Dependencies |
|-------|------------------|--------------|
| Phase 1: Foundation | 1-2 days | None |
| Phase 2: Parser | 2-3 days | Phase 1 |
| Phase 3: OpenAPI → Pulse | 3-4 days | Phase 2 |
| Phase 4: Pulse → OpenAPI | 2-3 days | Phase 1 |
| Phase 5: Warnings/Edge Cases | 1-2 days | Phases 3, 4 |
| Phase 6: Documentation | 1-2 days | Phases 3, 4 |
| Phase 7: Testing | 2-3 days | Phases 3, 4, 5 |

**Total:** 12-19 days

Phases 3 and 4 can be developed in parallel after Phase 2 completes.

---

## Future Enhancements (Out of Scope)

- Support for OpenAPI 2.0 (Swagger) input
- Swagger UI integration for generated OpenAPI
- Custom mapping rules via configuration file
- Streaming/SSE endpoint mapping
- gRPC/Protobuf as additional target format
