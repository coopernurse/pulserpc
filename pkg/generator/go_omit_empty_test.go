package generator

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const omitEmptyTestIDL = `namespace opt

struct Item {
  required_name string
  optional_note string [optional]
}

interface Svc {
  get() Item
}
`

// generateOmitEmptyModule runs the Go generator with the optional -omit-empty
// flag and returns the generated namespace file.
func generateOmitEmptyModule(t *testing.T, enabled bool) string {
	t.Helper()
	idl := parseTestIDL(t, omitEmptyTestIDL)
	dir := t.TempDir()
	fs := flag.NewFlagSet("omit-empty", flag.ContinueOnError)
	fs.String("dir", "", "")
	p := NewGoClientServer()
	p.RegisterFlags(fs)
	if err := fs.Set("dir", dir); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("package", "example.com/omitempty"); err != nil {
		t.Fatal(err)
	}
	if enabled {
		if err := fs.Set("omit-empty", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Generate(idl, fs); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "opt", "opt.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The default keeps optional-only omitempty tags.
func TestGoOmitEmptyDefaultKeepsOptionalOnlyTags(t *testing.T) {
	code := generateOmitEmptyModule(t, false)
	if !strings.Contains(code, "`json:\"optional_note,omitempty\"`") {
		t.Errorf("optional field lost omitempty:\n%s", code)
	}
	if strings.Contains(code, "`json:\"required_name,omitempty\"`") {
		t.Errorf("default mode added omitempty to a required field:\n%s", code)
	}
	if !strings.Contains(code, "`json:\"required_name\"`") {
		t.Errorf("required field has an unexpected JSON tag:\n%s", code)
	}
}

// -omit-empty=true adds omitempty to every generated struct field.
func TestGoOmitEmptyAddsTagToRequiredFields(t *testing.T) {
	code := generateOmitEmptyModule(t, true)
	for _, want := range []string{
		"`json:\"required_name,omitempty\"`",
		"`json:\"optional_note,omitempty\"`",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("omit-empty mode missing %q:\n%s", want, code)
		}
	}
}

// The flag is registered with a false default so existing generation is
// unchanged unless callers opt in.
func TestGoOmitEmptyFlagDefault(t *testing.T) {
	fs := flag.NewFlagSet("omit-empty", flag.ContinueOnError)
	p := NewGoClientServer()
	p.RegisterFlags(fs)
	f := fs.Lookup("omit-empty")
	if f == nil {
		t.Fatal("omit-empty flag is not registered")
	}
	if f.DefValue != "false" {
		t.Errorf("omit-empty default = %q, want false", f.DefValue)
	}
}
