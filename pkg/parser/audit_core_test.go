package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// core-02: error codes are unique across the merged IDL, including imports.
func TestErrorCodesAreUniqueAcrossImportedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.pulse"), []byte(`namespace a
errors {
    1001 ErrA "a"
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.pulse"), []byte(`namespace b
errors {
    1001 ErrB "b"
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	root := createTestFile(t, tmpDir, "root.pulse", `namespace root
import "a.pulse"
import "b.pulse"
struct Root {
    value string
}
`)

	idl, err := parseIDLFromFile(t, root)
	if err != nil {
		t.Fatalf("parseIDLFromFile: %v", err)
	}
	if err := ValidateIDL(idl); err == nil || !strings.Contains(err.Error(), "duplicate error code: 1001") {
		t.Fatalf("expected cross-file duplicate error code rejection, got: %v", err)
	}
}

// core-03: extends must name a struct, not a built-in or an enum.
func TestExtendsRequiresStructParent(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"enum", "namespace test\nenum Color { red green }\nstruct X extends Color {}\n"},
		{"interface", "namespace test\ninterface I { Ping() string }\nstruct X extends I {}\n"},
		{"error", "namespace test\nerrors { 1001 E \"e\" }\nstruct X extends E {}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idl, err := ParseIDL("test.pulse", tc.input)
			if err != nil {
				t.Fatalf("ParseIDL: %v", err)
			}
			if err := ValidateIDL(idl); err == nil || !strings.Contains(err.Error(), "parent must be a struct") {
				t.Errorf("expected parent-must-be-struct error, got: %v", err)
			}
		})
	}

	// The grammar rejects `extends string` syntactically; exercise the AST
	// directly to prove the built-in escape hatch was removed from validation.
	t.Run("builtin", func(t *testing.T) {
		idl := &IDL{RootNamespace: "test", Structs: []*Struct{{Name: "X", Namespace: "test", Extends: "string"}}}
		if err := ValidateIDL(idl); err == nil || !strings.Contains(err.Error(), "parent must be a struct") {
			t.Errorf("expected parent-must-be-struct error, got: %v", err)
		}
	})

	t.Run("valid struct parent", func(t *testing.T) {
		assertValid(t, "namespace test\nstruct Base { value string }\nstruct X extends Base {}\n")
	})
}

// core-07: duplicate member names must be rejected.
func TestDuplicateMemberNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"enum value", "enum E { a a }", "duplicate enum value name: a"},
		{"field", "struct S { a string\na int }", "duplicate field name: a"},
		{"parameter", "interface I { M(a string, a int) string }", "duplicate parameter name: a"},
		{"method", "interface I { M() string\nM() int }", "duplicate method name: M"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidationError(t, tc.input, tc.contains)
		})
	}
}
