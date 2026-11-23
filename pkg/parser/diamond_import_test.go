package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiamondImportDeclarationsMergedOnce reproduces the combined-import bug:
// when a root file imports two domain files and both domains import the same
// common file, the common declarations must be merged into the final IDL only
// once. Deduplication is by source file, not by declaration name, so genuine
// duplicates originating from different files must still be reported by
// strict validation.
func TestDiamondImportDeclarationsMergedOnce(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"common.pulse": `namespace common

struct Shared {
  value string
}

enum SharedEnum {
  FIRST
  SECOND
}
`,
		"domainA.pulse": `namespace domainA

import "common.pulse"

struct OnlyA {
  a string
}
`,
		"domainB.pulse": `namespace domainB

import "common.pulse"

struct OnlyB {
  b string
}
`,
		"admin.pulse": `namespace admin

import "domainA.pulse"
import "domainB.pulse"

struct Root {
  id string
}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	idl, err := ParseIDL(filepath.Join(dir, "admin.pulse"), mustRead(t, filepath.Join(dir, "admin.pulse")))
	if err != nil {
		t.Fatalf("ParseIDL failed on diamond imports: %v", err)
	}

	countName := func(name string) int {
		n := 0
		for _, s := range idl.Structs {
			if s.Name == name {
				n++
			}
		}
		for _, e := range idl.Enums {
			if e.Name == name {
				n++
			}
		}
		for _, i := range idl.Interfaces {
			if i.Name == name {
				n++
			}
		}
		for _, e := range idl.Errors {
			if e.Name == name {
				n++
			}
		}
		return n
	}

	if got := countName("common.Shared"); got != 1 {
		t.Errorf("common.Shared merged %d times, want exactly 1 (structs: %s)", got, joinedNames(idl.Structs, idl.Enums))
	}
	if got := countName("common.SharedEnum"); got != 1 {
		t.Errorf("common.SharedEnum merged %d times, want exactly 1 (enums: %s)", got, joinedNames(idl.Structs, idl.Enums))
	}

	if err := ValidateIDL(idl); err != nil {
		t.Fatalf("ValidateIDL failed on diamond imports: %v", err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func joinedNames(structs []*Struct, enums []*Enum) string {
	var parts []string
	for _, s := range structs {
		parts = append(parts, s.Name)
	}
	for _, e := range enums {
		parts = append(parts, e.Name)
	}
	return strings.Join(parts, ",")
}
