package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseValidate is a local helper so these focused invariant tests do not
// depend on the large parser_test.go helpers.
func parseValidate(t *testing.T, filename, input string) (*IDL, error) {
	t.Helper()
	idl, err := ParseIDL(filename, input)
	if err != nil {
		return nil, err
	}
	if err := ValidateIDL(idl); err != nil {
		return idl, err
	}
	return idl, nil
}

// G26/D11: string literals have no escape processing. Backslashes are literal
// characters and an escaped quote is a lexer error.
func TestInvariantG26StringLiteralsHaveNoEscapes(t *testing.T) {
	input := `namespace test

errors {
    1001 Plain "C:\temp\file"
}`
	idl, err := parseValidate(t, "test.pulse", input)
	if err != nil {
		t.Fatalf("literal backslashes should parse: %v", err)
	}
	if got, want := idl.Errors[0].Message, `C:\temp\file`; got != want {
		t.Errorf("message = %q, want literal %q (no escape decoding)", got, want)
	}

	escapedQuote := `namespace test

errors {
    1001 Bad "say \"hi\""
}`
	if _, err := ParseIDL("test.pulse", escapedQuote); err == nil {
		t.Error("an escaped quote must be a lexer error")
	}
}

// G27: a method return type is mandatory and `void` is not a keyword.
func TestInvariantG27ReturnTypeMandatory(t *testing.T) {
	if _, err := ParseIDL("test.pulse", `namespace test

interface I {
    missing()
}`); err == nil {
		t.Error("a method without a return type must fail to parse")
	}

	// `void` is not in the lexer keyword set, so it parses as a user-defined
	// type name and fails resolution rather than acting as a keyword.
	if _, err := parseValidate(t, "test.pulse", `namespace test

interface I {
    m() void
}`); err == nil {
		t.Error("void is not a built-in type and must not validate")
	}
}

// G28: parameters keep source order.
func TestInvariantG28ParametersArePositional(t *testing.T) {
	idl, err := parseValidate(t, "test.pulse", `namespace test

interface I {
    m(x int, y string, z bool) int
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	params := idl.Interfaces[0].Methods[0].Parameters
	got := []string{params[0].Name, params[1].Name, params[2].Name}
	want := []string{"x", "y", "z"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parameter order = %v, want %v", got, want)
		}
	}
}

// G31: import paths resolve relative to the importing file, and absolute paths
// are accepted.
func TestInvariantG31AbsoluteImportPath(t *testing.T) {
	tmpDir := t.TempDir()
	imported := filepath.Join(tmpDir, "inc.pulse")
	if err := os.WriteFile(imported, []byte(`namespace inc

struct Response {
    value string
}`), 0644); err != nil {
		t.Fatalf("write import: %v", err)
	}

	mainPath := filepath.Join(tmpDir, "main.pulse")
	main := fmt.Sprintf(`namespace main

import %q

struct Main {
    response inc.Response
}`, filepath.ToSlash(imported))
	if err := os.WriteFile(mainPath, []byte(main), 0644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	idl, err := parseValidate(t, mainPath, main)
	if err != nil {
		t.Fatalf("absolute import should parse: %v", err)
	}
	if !hasStruct(idl.Structs, "inc.Response") {
		t.Error("absolute import did not contribute inc.Response")
	}
}

// G33/G38: only imported declarations are namespace-prefixed; their field and
// parameter references are left as written, and root declarations are not
// prefixed.
func TestInvariantG33ImportPrefixingAsymmetry(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "inc.pulse"), []byte(`namespace inc

enum Status {
    ok
    err
}

struct Response {
    status Status
}`), 0644); err != nil {
		t.Fatalf("write import: %v", err)
	}

	main := `namespace main

import "inc.pulse"

struct Root {
    response inc.Response
}`
	mainPath := filepath.Join(tmpDir, "main.pulse")
	if err := os.WriteFile(mainPath, []byte(main), 0644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	idl, err := parseValidate(t, mainPath, main)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !hasStruct(idl.Structs, "Root") {
		t.Error("root declaration should keep its unprefixed name")
	}
	if !hasStruct(idl.Structs, "inc.Response") {
		t.Error("imported declaration should be prefixed")
	}
	if !hasEnum(idl.Enums, "inc.Status") {
		t.Error("imported enum should be prefixed")
	}

	for _, s := range idl.Structs {
		if s.Name != "inc.Response" {
			continue
		}
		if len(s.Fields) != 1 {
			t.Fatalf("inc.Response fields = %d, want 1", len(s.Fields))
		}
		if s.Fields[0].Type == nil || s.Fields[0].Type.UserDefined != "Status" {
			t.Errorf("imported field reference = %+v, want unqualified Status", s.Fields[0].Type)
		}
	}
}

func hasStruct(items []*Struct, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func hasEnum(items []*Enum, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

// G36: error codes are decimal and may be negative; no hex, underscores, or
// leading plus.
func TestInvariantG36ErrorCodeSyntax(t *testing.T) {
	idl, err := parseValidate(t, "test.pulse", `namespace test

errors {
    -1000 Negative "negative"
    0 Zero "zero"
    42 Positive "positive"
}`)
	if err != nil {
		t.Fatalf("decimal/negative codes should parse: %v", err)
	}
	wantCodes := []int{-1000, 0, 42}
	for i, want := range wantCodes {
		if idl.Errors[i].Code != want {
			t.Errorf("error %d code = %d, want %d", i, idl.Errors[i].Code, want)
		}
	}

	for _, bad := range []string{`+5`, `0x10`, `1_000`} {
		input := fmt.Sprintf(`namespace test

errors {
    %s Bad "bad"
}`, bad)
		if _, err := ParseIDL("test.pulse", input); err == nil {
			t.Errorf("error code %q must not parse", bad)
		}
	}
}

// G37: the single namespace declaration may appear anywhere among top-level
// elements.
func TestInvariantG37NamespaceMayAppearAnywhere(t *testing.T) {
	idl, err := parseValidate(t, "test.pulse", `struct Before {
    value string
}

namespace late

struct After {
    value int
}`)
	if err != nil {
		t.Fatalf("namespace after declarations should validate: %v", err)
	}
	if idl.RootNamespace != "late" {
		t.Errorf("RootNamespace = %q, want late", idl.RootNamespace)
	}
	for _, s := range idl.Structs {
		if s.Namespace != "late" {
			t.Errorf("struct %s namespace = %q, want late", s.Name, s.Namespace)
		}
	}
}

// G39: empty (or comments-only) files may omit a namespace; non-empty files
// may not.
func TestInvariantG39EmptyFilesMayOmitNamespace(t *testing.T) {
	for _, input := range []string{"", "   \n\n", "// only a comment\n"} {
		if _, err := parseValidate(t, "empty.pulse", input); err != nil {
			t.Errorf("empty file %q should validate without a namespace: %v", input, err)
		}
	}

	if _, err := parseValidate(t, "nonempty.pulse", `struct S {
    value string
}`); err == nil {
		t.Error("a non-empty file without a namespace must fail validation")
	} else if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("expected a namespace error, got: %v", err)
	}
}
