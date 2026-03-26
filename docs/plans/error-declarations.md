# Plan: Add Error Declaration Support to PulseRPC Parser

## Context

PulseRPC currently supports JSON-RPC error responses in its runtime libraries, but the IDL has no way to document or declare the errors that interface methods might raise. This makes it difficult for developers to:
- Know what errors a method can return
- Generate type-safe error handling in client code
- Document error contracts in the service definition

This plan adds an `errors` block to the IDL grammar and a `raises(...)` clause to interface methods, enabling declarative error definitions that can be used for code generation and documentation.

## Syntax Overview

```idl
namespace example

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
    1004 AlreadyExists "Already Exists"
}

interface pets {
    getListPets(limit int) []Pet

    postCreatePets(createPetsBody NewPet) createPetsResponse raises(InvalidInput, PermissionDenied)

    getShowPetById(petId string) Pet raises(NotFound)
}
```

Error names can be namespaced:
```idl
// errors.pulse
namespace errors

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
}

// service.pulse
import "errors.pulse"

namespace service

interface pets {
    getShowPetById(petId string) Pet raises(errors.NotFound, errors.InvalidInput)
}
```

---

## Part 1: Parser Changes

### File: `pkg/parser/parser.go`

#### 1.1 Add Lexer Tokens

Around line 15-35, add new tokens to `idlLexer`:

```go
{Name: "Errors", Pattern: `\berrors\b`},
{Name: "Raises", Pattern: `\braises\b`},
{Name: "IntLiteral", Pattern: `[0-9]+`},
```

**Rationale**: The `Errors` keyword declares the error block, `Raises` is used in methods, and `IntLiteral` parses error codes.

#### 1.2 Add Grammar Types

After line 57 (after `EnumDef`), add new grammar structs:

```go
// ErrorsDef represents an errors block
type ErrorsDef struct {
    Pos    lexer.Position
    Errors []*ErrorDef `parser:"'{' @@* '}'"`
}

// ErrorDef represents a single error declaration: <code> <name> <message>
type ErrorDef struct {
    Pos     lexer.Position
    Code    int    `parser:"@IntLiteral"`
    Name    string `parser:"@Ident"`
    Message string `parser:"@StringLiteral"`
}
```

**Rationale**: Follows the existing pattern for struct/array/enum definitions. Uses `IntLiteral` for error codes and `StringLiteral` for messages.

#### 1.3 Update IDLElement Union

Around line 51, add to `IDLElement`:

```go
type IDLElement struct {
    Pos       lexer.Position
    Namespace *NamespaceDef `parser:"  'namespace' @@"`
    Interface *InterfaceDef `parser:"| 'interface' @@"`
    Struct    *StructDef    `parser:"| 'struct' @@"`
    Enum      *EnumDef      `parser:"| 'enum' @@"`
    Errors    *ErrorsDef    `parser:"| 'errors' @@"`  // NEW
}
```

**Rationale**: Makes errors a first-class top-level element alongside interfaces, structs, and enums.

#### 1.4 Update MethodDef Grammar

Around line 86-93, add raises clause to `MethodDef`:

```go
type MethodDef struct {
    Pos            lexer.Position
    Name           string          `parser:"@Ident '('"`
    Parameters     []*ParameterDef `parser:"( @@ (',' @@)* )? ')'"`
    ReturnType     *TypeExpr       `parser:"@@"`
    ReturnOptional bool            `parser:"( @Optional )?"`
    Raises         []*QualifiedName `parser:"( 'raises' '(' @@ ( ',' @@ )* ')' )?"`  // NEW
}
```

**Rationale**: Makes raises optional and accepts a comma-separated list of qualified error names (e.g., `errors.NotFound`).

### File: `pkg/parser/idl.go`

#### 2.1 Add Error Definition to IDL

Around line 12, add to `IDL` struct:

```go
type IDL struct {
    RootNamespace string       `json:"rootNamespace,omitempty"`
    Interfaces    []*Interface `json:"interfaces,omitempty"`
    Structs       []*Struct    `json:"structs,omitempty"`
    Enums         []*Enum      `json:"enums,omitempty"`
    Errors        []*ErrorDef  `json:"errors,omitempty"`  // NEW
}
```

#### 2.2 Add ErrorDef Struct

After line 72 (after `Enum`), add:

```go
// ErrorDef represents a declared error with code, name, and message
type ErrorDef struct {
    Pos       lexer.Position `json:"-"`
    Name      string         `json:"name"`      // e.g., "NotFound" or "errors.NotFound" if imported
    Namespace string         `json:"namespace,omitempty"` // Namespace where declared
    Code      int            `json:"code"`      // JSON-RPC error code
    Message   string         `json:"message"`   // Error message template
    Comment   string         `json:"comment,omitempty"` // Documentation comment
}
```

#### 2.3 Add Raises to Method

Around line 25-31, add to `Method` struct:

```go
type Method struct {
    Pos            lexer.Position `json:"-"`
    Name           string         `json:"name"`
    Parameters     []*Parameter   `json:"parameters,omitempty"`
    ReturnType     *Type          `json:"returnType"`
    ReturnOptional bool           `json:"returnOptional,omitempty"`
    Raises         []string       `json:"raises,omitempty"`  // NEW - list of error names
}
```

**Rationale**: Stores the list of error names as strings (e.g., `["NotFound", "errors.InvalidInput"]`). Validation will verify these exist.

### File: `pkg/parser/parser.go` (Processing Changes)

#### 3.1 Process Errors in ParseIDL

Around line 475-480 (after initializing IDL), initialize Errors slice:

```go
idl := &IDL{
    RootNamespace: namespace,
    Interfaces:    make([]*Interface, 0),
    Structs:       make([]*Struct, 0),
    Enums:         make([]*Enum, 0),
    Errors:        make([]*ErrorDef, 0),  // NEW
}
```

#### 3.2 Process Errors Elements

Around line 483-566 (in element processing loop), add error processing:

```go
for _, elem := range file.Elements {
    if elem.Interface != nil {
        // ... existing interface processing ...
    } else if elem.Struct != nil {
        // ... existing struct processing ...
    } else if elem.Enum != nil {
        // ... existing enum processing ...
    } else if elem.Errors != nil {  // NEW
        // Extract errors block comment
        errorsComment := extractPrecedingComments(filteredInput, elem.Errors.Pos)

        // Process each error declaration
        for _, e := range elem.Errors.Errors {
            // Extract individual error comment
            errorComment := extractPrecedingComments(filteredInput, e.Pos)

            idl.Errors = append(idl.Errors, &ErrorDef{
                Pos:       e.Pos,
                Name:      e.Name,
                Namespace: namespace,
                Code:      e.Code,
                Message:   e.Message,
                Comment:   errorComment,
            })
        }
    }
}
```

**Rationale**: Follows the same pattern as enum processing - extracts comments for each error and adds to IDL.

#### 3.3 Process Raises in Methods

Around line 502-509 (in method processing), convert raises names:

```go
for _, m := range elem.Interface.Methods {
    method := &Method{
        Pos:            m.Pos,
        Name:           m.Name,
        Parameters:     make([]*Parameter, 0),
        ReturnType:     convertTypeExpr(m.ReturnType),
        ReturnOptional: m.ReturnOptional,
    }

    // NEW: Convert raises qualified names to strings
    if len(m Raises) > 0 {
        methodRaises := make([]string, 0, len(m Raises))
        for _, raisesName := range m.Raise {
            methodRaises = append(methodRaises, raisesName.String())
        }
        method.Raise = methodRaises
    }

    // ... existing parameter processing ...
}
```

#### 3.4 Import Resolution for Errors

Around line 569-648 (in import merging section), add error type handling:

```go
for _, imported := range importedIDLs {
    importedNamespace := imported.namespace
    importedIDL := imported.idl
    if importedNamespace != "" {
        // Build a map of unqualified to qualified names
        typeMap := make(map[string]string)
        for _, s := range importedIDL.Structs { /* ... existing ... */ }
        for _, e := range importedIDL.Enums { /* ... existing ... */ }
        for _, i := range importedIDL.Interfaces { /* ... existing ... */ }

        // NEW: Add error definitions to type map
        for _, err := range importedIDL.Errors {
            if err.Namespace == importedNamespace {
                typeMap[err.Name] = importedNamespace + "." + err.Name
            }
        }

        // ... existing type processing ...

        // NEW: Prefix error names from imported file
        for _, err := range importedIDL.Errors {
            if err.Namespace == importedNamespace {
                err.Name = importedNamespace + "." + err.Name
            }
            idl.Errors = append(idl.Errors, err)
        }
    } else {
        // No namespace - add errors as-is
        idl.Errors = append(idl.Errors, importedIDL.Errors...)
    }
}
```

### File: `pkg/parser/validator.go`

#### 4.1 Add Error Validation

Add validation rules for error declarations:

1. **Duplicate Error Code Detection** (around line 40-100, in type registry section):
```go
// Register all errors and check for duplicate codes
errorCodes := make(map[int]lexer.Position)  // code -> position
for _, err := range idl.Errors {
    baseName := getBaseName(err.Name)
    if !validateIdentifierName(baseName, errors, err.Pos.Line, err.Pos.Column) {
        continue
    }
    if existingPos, exists := errorCodes[err.Code]; exists {
        errors.Add(&ValidationError{
            Line:   err.Pos.Line,
            Column: err.Pos.Column,
            Msg:    fmt.Sprintf("duplicate error code: %d (previously defined at %d:%d)", err.Code, existingPos.Line, existingPos.Column),
        })
    } else {
        errorCodes[err.Code] = err.Pos
    }
}

// Also check for duplicate error names in type registry
if existingPos, exists := typeRegistry[err.Name]; exists {
    errors.Add(&ValidationError{
        Line:   err.Pos.Line,
        Column: err.Pos.Column,
        Msg:    fmt.Sprintf("duplicate type name: %s (previously defined as %s at %d:%d)", err.Name, typeNames[err.Name], existingPos.Line, existingPos.Column),
    })
} else {
    typeRegistry[err.Name] = err.Pos
    typeNames[err.Name] = "error"
}
```

2. **Validate Raises References** (around line 100-140, in second pass):
```go
// NEW: Validate method raises clauses reference existing errors
for _, iface := range idl.Interfaces {
    for _, method := range iface.Methods {
        for _, raisesName := range method.Raise {
            // Check if the error exists
            _, exists := typeRegistry[raisesName]
            if !exists {
                errors.Add(&ValidationError{
                    Line:   method.Pos.Line,
                    Column: method.Pos.Column,
                    Msg:    fmt.Sprintf("method %s raises unknown error: %s", method.Name, raisesName),
                })
            }
        }
    }
}
```

---

## Part 2: Parser Tests

### File: `pkg/parser/parser_test.go`

#### 5.1 Add Test Sections

Add new test sections following the existing pattern:

```go
// ============================================================================
// Error Declaration Tests
// ============================================================================

func TestValidErrorsBlock(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
        1002 InvalidInput "Invalid Input"
    }

    struct Dummy {}
    `
    assertValid(t, input)
}

func TestErrorsWithRaises(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
        1002 InvalidInput "Invalid Input"
    }

    interface Service {
        getValue(id string) string raises(NotFound)
        setValue(id string, value int) string raises(InvalidInput)
    }
    `
    assertValid(t, input)
}

func TestErrorsWithComments(t *testing.T) {
    input := `
    namespace test

    // Common error codes for this service
    errors {
        // Resource not found
        1001 NotFound "Not Found"
        // Input validation failed
        1002 InvalidInput "Invalid Input"
    }

    struct Dummy {}
    `
    idl := parseAndValidate(t, input)

    // Verify comments are captured
    require.Len(t, idl.Errors, 2)
    assert.Equal(t, "Resource not found", idl.Errors[0].Comment)
    assert.Equal(t, "Input validation failed", idl.Errors[1].Comment)
}

func TestNamespacedErrorsInRaises(t *testing.T) {
    input := `
    namespace test

    import "errors.pulse"

    interface Service {
        getValue(id string) string raises(errors.NotFound)
    }
    `
    // This would need errors.pulse to exist
    // For unit testing, we can test the qualified name parsing
    assertValid(t, input)
}

// ============================================================================
// Error Validation Tests
// ============================================================================

func TestDuplicateErrorCodes(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
        1001 DuplicateError "Duplicate"
    }

    struct Dummy {}
    `
    assertValidationError(t, input, "duplicate error code")
}

func TestInvalidErrorCodeNotInteger(t *testing.T) {
    input := `
    namespace test

    errors {
        "string" NotFound "Not Found"
    }

    struct Dummy {}
    `
    assertParseError(t, input)
}

func TestRaisesUnknownError(t *testing.T) {
    input := `
    namespace test

    interface Service {
        getValue(id string) string raises(UnknownError)
    }
    `
    assertValidationError(t, input, "unknown error")
}

func TestErrorsBlockWithoutNamespace(t *testing.T) {
    input := `
    errors {
        1001 NotFound "Not Found"
    }
    `
    assertValidationError(t, input, "must declare a namespace")
}

func TestErrorMessageMissing(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound
    }

    struct Dummy {}
    `
    assertParseError(t, input)
}

func TestErrorNameMissing(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 "Not Found"
    }

    struct Dummy {}
    `
    assertParseError(t, input)
}

func TestJSONRPCReservedErrorCodes(t *testing.T) {
    input := `
    namespace test

    errors {
        -32700 ParseError "Parse error"
        -32600 InvalidRequest "Invalid Request"
    }

    struct Dummy {}
    `
    // Should validate - users can override standard codes
    // But we may want to warn about this
    assertValid(t, input)
}

func TestMultipleErrorsInRaises(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
        1002 InvalidInput "Invalid Input"
        1003 PermissionDenied "Permission Denied"
    }

    interface Service {
        process(id string, data string) string raises(NotFound, InvalidInput, PermissionDenied)
    }
    `
    assertValid(t, input)
}

func TestRaisesClauseOptional(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
    }

    interface Service {
        getValue(id string) string
        getValueWithError(id string) string raises(NotFound)
    }
    `
    assertValid(t, input)
}

func TestErrorNameCollisionWithType(t *testing.T) {
    input := `
    namespace test

    errors {
        1001 NotFound "Not Found"
    }

    struct NotFound {}
    `
    assertValidationError(t, input, "duplicate type name")
}
```

---

## Part 3: Generator Integration Tests

### File: `examples/conform.pulse`

#### 6.1 Add Error Declarations to Conform Test

Update `/workspace/examples/conform.pulse` to include error declarations:

```idl
// Add after the enum section, before structs:

// Standard error codes for testing
errors {
    1001 ValidationError "Validation failed"
    1002 NotFoundError "Resource not found"
}
```

Then add methods with raises clauses:

```idl
interface A {
    // ... existing methods ...

    // NEW: Method that can raise errors
    divide(a int, b int) float raises(ValidationError)

    // NEW: Method with optional return that can error
    findItem(id string) RepeatResponse [optional] raises(NotFoundError)
}
```

### File: `tests/integration/test_generator.sh`

#### 6.2 Update Test Expectations

The integration test harness will need to be updated to:
1. Verify that error constants are generated in the output
2. Verify that raises information is preserved in generated code
3. Test that error handling works correctly (this will be added in the runtime implementation phase)

For now, the tests should:
1. Parse `conform.pulse` successfully
2. Verify the IDL contains the error definitions
3. Verify the methods have the raises clauses

### Create Test IDL File

Create `/workspace/examples/errors.pulse`:

```idl
namespace errors

// Standard error codes
errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
    1004 AlreadyExists "Already Exists"
}
```

Create `/workspace/examples/errors-service.pulse`:

```idl
namespace example

import "errors.pulse"

// Pet management types
struct Pet {
    petId   string
    name    string
    status  string
}

struct NewPet {
    name    string
    status  string
}

struct createPetsResponse {
    petId   string
    name    string
    status  string
}

// Pet service interface
interface pets {
    // getListPets returns a list of pets
    getListPets(limit int) []Pet

    // postCreatePets creates a new pet
    postCreatePets(createPetsBody NewPet) createPetsResponse raises(errors.InvalidInput, errors.PermissionDenied)

    // getShowPetById returns a specific pet by ID
    getShowPetById(petId string) Pet raises(errors.NotFound)
}
```

### Update Integration Test

Add a new test file `/workspace/tests/integration/test_errors.sh`:

```bash
#!/bin/bash
# Test error declarations in IDL

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../common.sh"

echo "Testing error declarations..."

# Test 1: Parse errors.pulse
echo "  Test 1: Parse errors.pulse"
"${PULSERPC}" examples/errors.pulse > /dev/null

# Test 2: Parse errors-service.pulse
echo "  Test 2: Parse errors-service.pulse with imports"
"${PULSERPC}" examples/errors-service.pulse > /dev/null

# Test 3: Verify error definitions are in AST
echo "  Test 3: Verify error definitions in AST"
OUTPUT="$(${PULSERPC} examples/errors.pulse -ast)"
if ! echo "$OUTPUT" | grep -q "NotFound"; then
    echo "ERROR: Error 'NotFound' not found in AST"
    exit 1
fi

# Test 4: Verify raises clauses are preserved
echo "  Test 4: Verify raises clauses in methods"
OUTPUT="$(${PULSERPC}" examples/errors-service.pulse -ast)"
if ! echo "$OUTPUT" | grep -q "raises"; then
    echo "ERROR: Raises clause not found in AST"
    exit 1
fi

echo "All error declaration tests passed!"
```

---

## Part 4: IDL Reference Documentation

### File: `docs/idl-guide/syntax.md`

#### 7.1 Add Error Declarations Section

Add after the Enums section or before Interfaces:

```markdown
## Error Declarations

Define error codes that methods can raise:

\`\`\`idl
errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
}
\`\`\`

Each error declaration has three parts:
- **Error code**: Integer value for the JSON-RPC `code` field
- **Name**: Identifier used in `raises()` clauses
- **Message**: String literal for the JSON-RPC `message` field

### Error Codes

Error codes can be any integer. JSON-RPC reserves certain ranges:
- `-32700` to `-32000`: Standard JSON-RPC errors
- `-32099` to `-32000`: Server errors
- `0` and positive integers: Application-defined errors

### Using Errors in Methods

Methods can declare which errors they raise using the `raises()` clause:

\`\`\`idl
interface UserService {
    getUser(userId string) User raises(NotFound)
    createUser(user User) UserResponse raises(InvalidInput, PermissionDenied)
}
\`\`\`

### Namespaced Errors

Errors can be imported from other files:

\`\`\`idl
// common/errors.pulse
namespace common

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
}

// service.pulse
import "common/errors.pulse"

namespace myservice

interface Service {
    getValue(id string) string raises(common.NotFound)
}
\`\`\`

### Error Documentation

Errors can be documented with comments:

\`\`\`idl
// Standard application errors
errors {
    // Returned when a requested resource is not found
    1001 NotFound "Not Found"

    // Returned when input validation fails
    1002 InvalidInput "Invalid Input"
}
\`\`\`
```

### Create New Documentation File: `docs/idl-guide/errors.md`

```markdown
---
title: Error Handling
parent: IDL Guide
nav_order: 4
layout: default
---

# Error Handling

PulseRPC provides declarative error definitions for documenting and handling errors in your services.

## Declaring Errors

Use the `errors` block to define error codes:

\`\`\`idl
namespace myservice

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
    1004 AlreadyExists "Already Exists"
}
\`\`\`

## Error Declaration Syntax

Each error has three components:

1. **Code**: Integer - sent as the JSON-RPC `code` field
2. **Name**: Identifier - referenced in `raises()` clauses
3. **Message**: String literal - sent as the JSON-RPC `message` field

\`\`\`
<code> <name> <message>
\`\`\`

Example:
\`\`\`idl
1001 NotFound "Not Found"
//     ^         ^
//     |         +-- Message (string literal)
//     +------------ Name (identifier)
// Code (integer)
\`\`\`

## Using Raises Clauses

Methods declare which errors they can raise:

\`\`\`idl
interface UserService {
    getUser(userId string) User raises(NotFound)
    createUser(user User) UserResponse raises(InvalidInput, PermissionDenied)
    deleteUser(userId string) string raises(NotFound, PermissionDenied)
}
\`\`\`

### Rules

- `raises()` is optional - methods without it can only succeed
- Multiple errors are comma-separated
- Error names can be qualified (namespaced)

## Importing Errors

Errors can be defined in a separate file and imported:

**common/errors.pulse:**
\`\`\`idl
namespace common

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
}
\`\`\`

**myservice.pulse:**
\`\`\`idl
import "common/errors.pulse"

namespace myservice

interface Service {
    getValue(id string) string raises(common.NotFound)
}
\`\`\`

## JSON-RPC Error Response

When an error occurs, the JSON-RPC response includes:

\`\`\`json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1001,
    "message": "Not Found",
    "data": null
  }
}
\`\`\`

The `code` and `message` values come from the error declaration.

## Reserved Error Codes

JSON-RPC 2.0 defines these standard error codes:

| Code | Message | Meaning |
|------|---------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | JSON-RPC request is invalid |
| -32601 | Method not found | Method does not exist |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Internal server error |

You can use codes outside these ranges for application-specific errors.

## Best Practices

### Use Semantic Error Codes

\`\`\`idl
// Good: Semantic, domain-specific
errors {
    1001 UserNotFound "User not found"
    1002 InvalidEmail "Invalid email address"
    1003 DuplicateEmail "Email already registered"
}

// Avoid: Generic codes
errors {
    1 Error "Error"
    2 Error2 "Another error"
}
\`\`\`

### Group Related Operations

\`\`\`idl
interface UserService {
    // User lookups
    getUser(id string) User raises(UserNotFound)

    // User creation
    createUser(user User) UserResponse raises(InvalidEmail, DuplicateEmail)

    // User modification
    updateUser(id string, updates User) User raises(UserNotFound, InvalidEmail)
}
\`\`\`

### Document Error Conditions

\`\`\`idl
// Standard error codes for user operations
errors {
    // User not found in database
    1001 UserNotFound "User not found"

    // Email format validation failed
    1002 InvalidEmail "Invalid email format"

    // Email already registered to another account
    1003 DuplicateEmail "Email already registered"
}
\`\`\`

## Validation Rules

The parser enforces these rules:

1. **Namespace required**: Errors must be in a namespace
2. **Unique codes**: Each error code must be unique within a file
3. **Unique names**: Error names cannot conflict with types (structs, enums, interfaces)
4. **Valid references**: `raises()` can only reference declared errors
5. **Integer codes**: Error codes must be integer literals
6. **String messages**: Error messages must be string literals

## Examples

### Simple Example

\`\`\`idl
namespace checkout

errors {
    1001 OutOfStock "Out of Stock"
    1002 InvalidQuantity "Invalid Quantity"
}

interface OrderService {
    createOrder(items []OrderItem) Order raises(OutOfStock, InvalidQuantity)
}
\`\`\`

### Complex Example with Imports

\`\`\`idl
// errors.pulse
namespace api

errors {
    1001 NotFound "Resource not found"
    1002 InvalidInput "Invalid input"
    1003 Unauthorized "Unauthorized access"
}
\`\`\`

\`\`\`idl
// service.pulse
import "errors.pulse"

namespace userservice

struct User {
    userId string
    email  string
}

interface UserService {
    getUser(userId string) User raises(api.NotFound)
    createUser(user User) User raises(api.InvalidInput, api.Unauthorized)
}
\`\`\`

## Language Mapping

Error declarations generate language-specific constructs:

| Language | Generated Code |
|----------|----------------|
| Go | Constants with error codes and helpers |
| Python | Exception classes with code/message |
| TypeScript | Error classes and type guards |
| Java | Exception classes with error codes |
| C# | Exception classes with error codes |

See the [Language Reference](../languages/) for details on error handling in each language.
```

---

## Verification

### Testing Checklist

1. **Parser Tests**
   - [ ] Basic errors block parses correctly
   - [ ] Raises clauses parse correctly
   - [ ] Namespaced error references parse
   - [ ] Comments are extracted for errors
   - [ ] Multiple errors in raises work
   - [ ] Empty errors block handled

2. **Validation Tests**
   - [ ] Duplicate error codes detected
   - [ ] Unknown error names in raises detected
   - [ ] Namespace required for errors
   - [ ] Error name collisions with types detected
   - [ ] Invalid error code format rejected

3. **Integration Tests**
   - [ ] conform.pulse includes errors
   - [ ] Generated AST includes error definitions
   - [ ] Raises clauses preserved in methods

4. **Documentation**
   - [ ] syntax.md updated with error syntax
   - [ ] errors.md created with comprehensive guide
   - [ ] Examples provided for common patterns

### Manual Testing

```bash
# Test basic parsing
./target/pulserpc examples/errors.pulse

# Test with imports
./target/pulserpc examples/errors-service.pulse

# Run parser tests
go test ./pkg/parser/... -v -run "Error"

# Run integration tests
make test-generator
```

---

## Summary of Changes

| File | Changes |
|------|---------|
| `pkg/parser/parser.go` | Add lexer tokens, grammar types, processing logic |
| `pkg/parser/idl.go` | Add ErrorDef struct, update Method and IDL |
| `pkg/parser/validator.go` | Add error code and raises validation |
| `pkg/parser/parser_test.go` | Add comprehensive error declaration tests |
| `examples/conform.pulse` | Add error declarations and raises clauses |
| `examples/errors.pulse` | New file with standard errors |
| `examples/errors-service.pulse` | New file demonstrating errors in service |
| `tests/integration/test_errors.sh` | New integration test for errors |
| `docs/idl-guide/syntax.md` | Add error declarations section |
| `docs/idl-guide/errors.md` | New comprehensive error documentation |

---

## Implementation Notes

1. **StringLiteral parsing**: The parser's `StringLiteral` pattern captures the quotes. We'll need to strip them or update the pattern to exclude quotes. Check how other string literals are handled in the codebase.

2. **Raises as QualifiedName**: Using `QualifiedName` for raises allows namespaced references. We need to ensure the `.String()` method is called during conversion.

3. **Error code validation**: The lexer uses `\b` word boundaries for keywords. For `IntLiteral`, we don't need word boundaries since we're matching digits.

4. **Comment extraction**: The existing `extractPrecedingComments` function works for individual errors. The errors block itself can also have a comment.

5. **Future work**: Runtime implementation (generating error classes/constants in each language) is a separate task not covered in this plan.
